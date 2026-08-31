package quant

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"go.uber.org/zap"
)

// aiAnalysisResult 大模型返回的单只股票分析结果
type aiAnalysisResult struct {
	Fundamentals string `json:"fundamentals"` // 基本面深度分析
	Financial    string `json:"financial"`    // 财务分析
	Realization  string `json:"realization"`  // 项目落地、业绩兑现分析
	Momentum     string `json:"momentum"`     // 题材热度、资金与股价动量
	Risk         string `json:"risk"`         // 个股与题材风险提示
}

// maxAiAnalyzePerBatch 单次批量分析的最大股票数，避免整体耗时过长
const maxAiAnalyzePerBatch = 50

// AiAnalyzeStocks AI 自动分析股票（异步任务化）
// 提交后立即创建执行任务并返回 task_id，逐只分析在后台 goroutine 中执行，
// 进度与日志实时写入任务表，供前端执行进度页面轮询展示
func (baseStockService *BaseStockService) AiAnalyzeStocks(ctx context.Context, req quantReq.AiAnalyzeStockReq, userID uint) (taskID uint, err error) {
	// 1. 校验厂商、解析模型与密钥
	provider, err := resolveAIProvider(req.Provider)
	if err != nil {
		return 0, err
	}
	modelName, baseURL, cfgKey, err := resolveAIModel(provider, req.Model)
	if err != nil {
		return 0, err
	}
	apiKey, err := resolveAPIKey(req.ApiKey, cfgKey, provider.Name)
	if err != nil {
		return 0, err
	}
	if len(req.StockIds) == 0 {
		return 0, fmt.Errorf("请至少选择一只待分析的股票")
	}

	// 2. 查询待分析股票
	var stocks []quant.BaseStock
	if err = global.GVA_DB.Where("id IN ?", req.StockIds).Find(&stocks).Error; err != nil {
		global.GVA_LOG.Error("查询待分析股票失败!", zap.Error(err))
		return 0, err
	}
	if len(stocks) == 0 {
		return 0, fmt.Errorf("未找到待分析的股票")
	}
	if len(stocks) > maxAiAnalyzePerBatch {
		stocks = stocks[:maxAiAnalyzePerBatch]
	}

	// 3. 创建任务：总进度 = 待分析股票数；任务名称单只显示股票名，多只显示数量
	taskName := fmt.Sprintf("分析基础股票：%d支", len(stocks))
	if len(stocks) == 1 {
		taskName = fmt.Sprintf("分析基础股票：%s", safeString(stocks[0].Name))
	}
	params := map[string]any{
		"count":         len(stocks),
		"stock_ids":     req.StockIds, // 供任务重启时重建执行请求
		"provider":      req.Provider, // 存配置 key（如 qwen），重启时可直接命中 ai.providers
		"model":         modelName,
		"web_search":    req.WebSearch,
		"api_format":    req.ApiFormat,
		"search_engine": req.SearchEngine,
	}
	var scheduledAtArgs []time.Time
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() {
		scheduledAtArgs = append(scheduledAtArgs, *req.ScheduledAt)
	}
	taskID, err = aiTaskService.CreateAiTask(ctx, "ai_analyze", taskName, params, len(stocks), len(stocks), userID, scheduledAtArgs...)
	if err != nil {
		return 0, err
	}

	// 如果是定时任务，不立即执行（等待调度器扫描到期望执行时间后发起）
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() && req.ScheduledAt.After(time.Now()) {
		return taskID, nil
	}
	// 后台异步执行
	go baseStockService.runAiAnalyzeTask(taskID, req, userID, provider, modelName, baseURL, apiKey, stocks, resolveWebSearch(provider, modelName, req.WebSearch))
	return taskID, nil
}

// runAiAnalyzeTask 后台执行批量 AI 分析任务：逐只串行分析并上报进度，完成写入汇总结果
func (baseStockService *BaseStockService) runAiAnalyzeTask(taskID uint, req quantReq.AiAnalyzeStockReq, userID uint, provider AIProvider, modelName, baseURL, apiKey string, stocks []quant.BaseStock, enableWebSearch bool) {
	// 可取消的后台 context：用户停止任务时中断正在飞行的大模型请求
	bgCtx, cancel := context.WithCancel(context.Background())
	aiTaskService.RegisterTaskCancel(taskID, cancel)
	defer aiTaskService.UnregisterTaskCancel(taskID)
	total := len(stocks)
	_ = aiTaskService.UpdateAiTaskProgress(taskID, 0, "", fmt.Sprintf("开始批量AI分析，共 %d 只股票", total))

	analyzed := 0
	var failed []string
	for i, stock := range stocks {
		// 任务被用户停止：提前退出，不覆盖取消状态
		if aiTaskService.IsAiTaskCanceled(taskID) {
			return
		}
		done := i + 1
		label := fmt.Sprintf("%s(%s)", safeString(stock.Name), safeString(stock.Symbol))
		_ = aiTaskService.UpdateAiTaskProgress(taskID, done-1, label, fmt.Sprintf("正在分析 %s ...", label))

		result, aErr := analyzeSingleStock(bgCtx, provider, modelName, baseURL, apiKey, stock, enableWebSearch, req.ApiFormat, resolveSearchEngine(req.SearchEngine))
		if aErr != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", label, aErr))
			_ = aiTaskService.UpdateAiTaskProgress(taskID, done, label, fmt.Sprintf("%s 分析失败：%v", label, aErr))
			continue
		}
		updates := buildAnalysisUpdates(result)
		if len(updates) == 0 {
			failed = append(failed, fmt.Sprintf("%s: 大模型未返回有效分析内容", label))
			_ = aiTaskService.UpdateAiTaskProgress(taskID, done, label, fmt.Sprintf("%s：大模型未返回有效分析内容", label))
			continue
		}
		updates["updated_by"] = userID
		if uErr := global.GVA_DB.Model(&quant.BaseStock{}).Where("id = ?", stock.ID).Updates(updates).Error; uErr != nil {
			global.GVA_LOG.Error("更新股票分析结果失败!", zap.Error(uErr))
			failed = append(failed, fmt.Sprintf("%s: %v", label, uErr))
			_ = aiTaskService.UpdateAiTaskProgress(taskID, done, label, fmt.Sprintf("%s 更新失败：%v", label, uErr))
			continue
		}
		analyzed++
		_ = aiTaskService.UpdateAiTaskProgress(taskID, done, label, fmt.Sprintf("%s 分析完成", label))
	}
	_ = aiTaskService.UpdateAiTaskProgress(taskID, total, "", fmt.Sprintf("AI分析完成：成功 %d/%d 只，失败 %d 只", analyzed, total, len(failed)))
	if len(failed) > 0 {
		aiTaskService.FinishAiTask(taskID, AiTaskStatusSuccess, map[string]any{
			"analyzed": analyzed,
			"total":    total,
			"failed":   failed,
		}, "")
		return
	}
	aiTaskService.FinishAiTask(taskID, AiTaskStatusSuccess, map[string]any{
		"analyzed": analyzed,
		"total":    total,
	}, "")
}

// analyzeSingleStock 调用大模型分析单只股票
// enableWebSearch 为 true 时 AI 分析前自动检索相关最新新闻拼入提示词
// apiFormat 为请求级接口模式（可选，覆盖配置）；searchEngine 为请求级搜索引擎（chat-completions 模式生效，默认百度搜索）
func analyzeSingleStock(ctx context.Context, provider AIProvider, modelName, baseURL, apiKey string, stock quant.BaseStock, enableWebSearch bool, apiFormat, searchEngine string) (aiAnalysisResult, error) {
	var result aiAnalysisResult
	name := safeString(stock.Name)
	symbol := safeString(stock.Symbol)
	industry := safeString(stock.Industry)
	fullname := safeString(stock.Fullname)

	prompt := fmt.Sprintf(`你是一位资深的A股基本面与题材分析师。

请对以下指定股票进行深度证券分析：
- 股票名称：%s
- 股票代码：%s
- 所属行业：%s
- 公司全称：%s

整体分为以下 5 个维度，每个维度独立成段，篇幅控制在 800-1000 字，要求逻辑严密、信息密度极高。

【输出格式要求】
1. JSON 中各字段的值是普通文本，禁止使用 markdown 标题/加粗符号（如 ##、**、>）。
2. 段落之间必须使用空行分隔，即两个换行符 \n\n（JSON 字符串中的转义形式）；严禁输出成不分段的连续长文本。
3. 每个维度内部：先写 1 句结论总结，开头不要加任何标题，再按该维度下【每一个小项】逐项独立成段：一个小项对应一个独立段落，段与段之间用 \n\n 空行分隔；每段以"小项名称："开头，内容完整包含具体数值与时间锚点，段内可再以"1." "2." "3."分行展开要点。

【硬性数据与真实性约束】
1. 凡涉及营收、净利润、增速、毛利率、市占率、产能、订单金额、客户名称、机构调研、资金持仓等具体信息，必须给出具体数值、单位、日期或具体名称。
2. 关键数据必须带上【报告期/截止时间】（例如：2025年三季报、2026年Q1、截至2026年6月30日）。
3. 严禁编造数据。仅可使用公开披露、可查证的权威数据（如定期报告、官方公告、券商研报、监管回复函）。
4. 若遇到上市公司未公开披露的敏感数据（如因保密协议未披露客户全称），必须明确标注"（官方未公开披露/保密）"或"(数据待核实)"，不得捏造虚假全称。

---

### 分析维度要求：

1. 【基本面深度分析 (Fundamentals)】
- 行业地位与市占率：给出具体的量化市占率数据、行业排名及竞争格局。
- 技术壁垒与落地进度：核心技术优势（专利/指标对比），当前产品量产进度（研制/小批量/大批量送样/大规模量产）。
- 核心客户资质：列出明确合作的具体客户名称（非统称），以及合作深度或供货份额。
- 产能与扩产：现有产能储备数值、产能利用率及后续扩产规划与投产时间（软件/服务类企业替换为付费用户数/服务器/算力规模）。

2. 【财务深度分析 (Financial)】
- 最新业绩与预告：写明最新一期的业绩（注明报告期与披露日期）。若有最新业绩预告/快报，写明区间及同比变动。
- 营收规模与结构：营业收入总额、同比/环比增速，拆解各主营业务板块的营收金额及占比。
- 盈利能力（重点）：归母净利润及同比增速；扣非净利润（单独列出并分析剔除一次性收益后的真实盈利能力）及同比增速。
- 利润率指标：毛利率、净利率、营业利润率的具体数值及同比/环比变动趋向。

3. 【项目落地与业绩兑现度 (Realization)】
- 在手订单：目前已披露的在手订单/重大合同的具体金额及交付节点。
- 业绩兑现预期：对比最近两个报告期的营收/净利润兑现情况，评估业绩放量速度。
- 验证与交付履历：产品/项目的认证周期（如车规认证、验厂等）、交付周期及过往标杆项目兑现履历。

4. 【题材热度与资金动量 (Momentum)】
- 概念标签：官方及市场认可的核心概念标签（概念炒作与主营相关度分析）。
- 资金与持仓异动：最新披露的北向资金持仓比例及变动、公募基金持仓占比及重仓变化（给出具体持股比例%%或变动幅度）。
- 机构关注度：最近 3-6 个月内的机构调研频次、参会机构数量及核心关注问题概要。

5. 【风险筛查与提示 (Risk)】
- 负面公告筛查：检索并列出近期的重大负面公告（如减持、质押风险、诉讼、监管函/立案调查、业绩不及预期等），写明具体内容与日期。若无则标注"暂无重大负面公告"。
- 潜在风险评估：行业周期风险、客户集中度过高风险、技术迭代风险、解禁压力或现金流风险等。

%s

只输出一个JSON对象，不要输出任何其他文字或markdown标记。各字段值中的每个小项段落之间必须用空行 \n\n 分隔（JSON 转义形式），确保分段清晰，格式如下：
{"fundamentals":"...","financial":"...","realization":"...","momentum":"...","risk":"..."}`, name, symbol, industry, fullname, aiDataFreshnessRules)

	content, err := aiCompletionContent(ctx, provider, modelName, baseURL, apiKey, "你是一个A股股票分析助手，只输出JSON对象，不要输出任何多余内容。", prompt, fmt.Sprintf("%s %s", name, symbol), enableWebSearch, apiFormat, searchEngine)
	if err != nil {
		return result, err
	}
	content = stripCodeFence(strings.TrimSpace(content))
	if err = json.Unmarshal([]byte(content), &result); err != nil {
		// 容错：模型输出可能因输出 token 上限被截断，尝试补全截断的 JSON 后再解析
		if repaired, ok := repairTruncatedJSON(content); ok && json.Unmarshal([]byte(repaired), &result) == nil {
			return result, nil
		}
		// 容错：标准解析失败（如字符串值中夹杂未转义引号/非法字符导致 JSON 结构破坏），按字段宽松提取
		if loose, ok := extractAnalysisFieldsLoose(content); ok {
			return loose, nil
		}
		return result, fmt.Errorf("解析大模型分析结果失败: %w，原始内容: %.200s", err, content)
	}
	return result, nil
}

// aiFieldPattern 匹配 aiAnalysisResult 各字段的 "key":"value"（value 支持转义引号，且允许被截断导致闭合引号缺失）
var aiFieldPattern = regexp.MustCompile(`"(fundamentals|financial|realization|momentum|risk)"\s*:\s*"((?:[^"\\]|\\.)*)"?`)

// extractAnalysisFieldsLoose 宽松解析大模型返回的分析结果：
// 大模型输出可能因字符串值中夹杂未转义的引号、非法控制字符等导致标准 JSON 解析失败，
// 此时按字段 key 逐一提取字符串值（遇到未转义引号处截断，保证至少保留该字段有效内容）。
// 提取到至少一个非空字段视为成功。
func extractAnalysisFieldsLoose(content string) (aiAnalysisResult, bool) {
	var result aiAnalysisResult
	matches := aiFieldPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return result, false
	}
	found := 0
	for _, m := range matches {
		key := m[1]
		val := unescapeJSONValue(m[2])
		if strings.TrimSpace(val) == "" {
			continue
		}
		switch key {
		case "fundamentals":
			result.Fundamentals = val
		case "financial":
			result.Financial = val
		case "realization":
			result.Realization = val
		case "momentum":
			result.Momentum = val
		case "risk":
			result.Risk = val
		}
		found++
	}
	if found == 0 {
		return result, false
	}
	return result, true
}

// unescapeJSONValue 反转义 JSON 字符串值中的转义序列（如 \n、\"、\\、\uXXXX）；
// 内容包含非法转义或未转义换行等无法反转义时，原样返回，保证内容不丢失
func unescapeJSONValue(s string) string {
	unquoted, err := strconv.Unquote(`"` + s + `"`)
	if err != nil {
		return s
	}
	return unquoted
}

// buildAnalysisUpdates 将分析结果组装为待更新字段（仅保留非空内容）
// 存在有效分析内容时，一并记录本次大模型分析完成时间
func buildAnalysisUpdates(result aiAnalysisResult) map[string]any {
	updates := make(map[string]any)
	if v := strings.TrimSpace(result.Fundamentals); v != "" {
		updates["fundamentals"] = v
	}
	if v := strings.TrimSpace(result.Financial); v != "" {
		updates["financial"] = v
	}
	if v := strings.TrimSpace(result.Realization); v != "" {
		updates["realization"] = v
	}
	if v := strings.TrimSpace(result.Momentum); v != "" {
		updates["momentum"] = v
	}
	if v := strings.TrimSpace(result.Risk); v != "" {
		updates["risk"] = v
	}
	if len(updates) > 0 {
		updates["ai_analyzed_at"] = time.Now()
	}
	return updates
}

// safeString 指针字符串安全取值
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// RestartAiAnalyzeTask 重启已取消的AI股票分析任务：按任务参数重建请求并重新发起后台执行
func (baseStockService *BaseStockService) RestartAiAnalyzeTask(task quant.QuantAiTask, params map[string]any, userID uint) error {
	req := quantReq.AiAnalyzeStockReq{
		StockIds:     taskParamsInt64Slice(params, "stock_ids"),
		Provider:     taskParamsString(params, "provider"),
		Model:        taskParamsString(params, "model"),
		WebSearch:    taskParamsBoolPtr(params, "web_search"),
		ApiFormat:    taskParamsString(params, "api_format"),
		SearchEngine: taskParamsString(params, "search_engine"),
	}
	if len(req.StockIds) == 0 {
		return fmt.Errorf("任务参数缺少股票ID列表，无法重启（早期创建的任务未记录股票ID）")
	}
	provider, err := resolveAIProvider(req.Provider)
	if err != nil {
		return err
	}
	modelName, baseURL, cfgKey, err := resolveAIModel(provider, req.Model)
	if err != nil {
		return err
	}
	apiKey, err := resolveAPIKey(req.ApiKey, cfgKey, provider.Name)
	if err != nil {
		return err
	}
	var stocks []quant.BaseStock
	if err := global.GVA_DB.Where("id IN ?", req.StockIds).Find(&stocks).Error; err != nil {
		return err
	}
	if len(stocks) == 0 {
		return fmt.Errorf("未找到待分析的股票")
	}
	if len(stocks) > maxAiAnalyzePerBatch {
		stocks = stocks[:maxAiAnalyzePerBatch]
	}
	enableWebSearch := resolveWebSearch(provider, modelName, req.WebSearch)
	if err := aiTaskService.ResetAiTask(task.ID, len(stocks), len(stocks), userID); err != nil {
		return err
	}
	go baseStockService.runAiAnalyzeTask(task.ID, req, userID, provider, modelName, baseURL, apiKey, stocks, enableWebSearch)
	return nil
}
