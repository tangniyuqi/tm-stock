package quant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/request"
	"go.uber.org/zap"
)

// AIProvider 大模型厂商配置
type AIProvider struct {
	Name         string // 厂商名称
	BaseURL      string // 厂商级 OpenAI 兼容 chat/completions 接口地址
	ApiKey       string // 厂商级 API 密钥（从配置文件 ai.providers 读取）
	DefaultModel string // 默认模型（models 列表第一个，其次为旧配置 model 字段）
	WebSearch    *bool  // 厂商级联网搜索开关（模型未单独配置时生效；nil 继承全局 ai.web-search）
	ApiFormat    string // 厂商级接口格式（模型未单独配置时生效；默认 responses）
	// ReasoningEffort 思考强度（厂商级默认，模型未单独配置时生效；none=关闭思考，low/high/max=思考强度，留空不控制）
	ReasoningEffort string
	// MaxOutputTokens 单次回答最大输出 token 数（厂商级默认，模型未单独配置时生效；<=0 使用默认 8192）
	MaxOutputTokens int
	Models          []AIModel // 模型列表（一个厂商可配置多个大模型）
}

// AIModel 单个大模型配置
type AIModel struct {
	Name        string // 模型ID（接口请求中的 model 字段）
	DisplayName string // 显示名称（前端下拉展示）
	BaseURL     string // 模型级接口地址（留空继承厂商级）
	ApiKey      string // 模型级密钥（留空继承厂商级）
	WebSearch   *bool  // 联网搜索（模型级覆盖厂商级；nil 继承厂商级 WebSearch）
	ApiFormat   string // 接口格式（模型级覆盖厂商级，留空继承厂商级；responses=Responses API 原生 web_search 联网搜索，chat-completions=OpenAI 兼容接口）
	// ReasoningEffort 思考强度（模型级覆盖厂商级；none=关闭思考，low/high/max=思考强度，留空继承厂商级/不控制）
	// 深度思考模型（如 DeepSeek V4）思考内容计入 max-output-tokens 预算，思考过长会耗尽预算导致最终回答为空
	ReasoningEffort string
	// MaxOutputTokens 单次回答最大输出 token 数（模型级覆盖厂商级；<=0 继承厂商级/默认 8192）
	MaxOutputTokens int
}

// resolveAIProvider 从配置文件 ai.providers 读取厂商配置
// name/base-url/api-key/models 的唯一数据源为 config.yaml 的 ai.providers.<厂商>
func resolveAIProvider(name string) (AIProvider, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	cfg, ok := global.GVA_CONFIG.AI.Providers[key]
	// 兼容旧任务：早期任务 params 中 provider 存的是厂商显示名称（如"千问"），按 name 反查配置 key
	if !ok {
		for k, p := range global.GVA_CONFIG.AI.Providers {
			if strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(name)) {
				key, cfg, ok = k, p, true
				break
			}
		}
	}
	if !ok {
		return AIProvider{}, fmt.Errorf("不支持的大模型厂商: %s，请在服务端配置文件 config.yaml 的 ai.providers 中配置（如 deepseek/doubao/qwen/kimi/zhipu/minimax/claude/gpt/gemini）", name)
	}
	displayName := strings.TrimSpace(cfg.Name)
	if displayName == "" {
		displayName = key
	}
	provider := AIProvider{
		Name:         displayName,
		BaseURL:      strings.TrimSpace(cfg.BaseURL),
		ApiKey:       strings.TrimSpace(cfg.ApiKey),
		DefaultModel: strings.TrimSpace(cfg.Model),
		WebSearch:    cfg.WebSearch,
		ApiFormat:    strings.TrimSpace(cfg.ApiFormat),
	}
	provider.ReasoningEffort = strings.TrimSpace(cfg.ReasoningEffort)
	provider.MaxOutputTokens = cfg.MaxOutputTokens
	for _, m := range cfg.Models {
		modelName := strings.TrimSpace(m.Name)
		if modelName == "" {
			continue
		}
		display := strings.TrimSpace(m.DisplayName)
		if display == "" {
			display = modelName
		}
		provider.Models = append(provider.Models, AIModel{
			Name:            modelName,
			DisplayName:     display,
			BaseURL:         strings.TrimSpace(m.BaseURL),
			ApiKey:          strings.TrimSpace(m.ApiKey),
			WebSearch:       m.WebSearch,
			ApiFormat:       strings.TrimSpace(m.ApiFormat),
			ReasoningEffort: strings.TrimSpace(m.ReasoningEffort),
			MaxOutputTokens: m.MaxOutputTokens,
		})
	}
	// models 列表第一个作为默认模型，未配置 models 时兜底使用厂商级 model 字段
	if len(provider.Models) > 0 {
		provider.DefaultModel = provider.Models[0].Name
	}
	if provider.BaseURL == "" {
		return AIProvider{}, fmt.Errorf("厂商 %s 未配置 base-url，请在 config.yaml 的 ai.providers.%s.base-url 中填写", displayName, key)
	}
	if provider.DefaultModel == "" {
		return AIProvider{}, fmt.Errorf("厂商 %s 未配置任何模型，请在 config.yaml 的 ai.providers.%s.models 中配置", displayName, key)
	}
	return provider, nil
}

// resolveAIModel 解析最终调用的大模型配置：模型ID、接口地址、密钥
// 优先级：请求指定模型 > 厂商默认模型；模型级 base-url/api-key 覆盖厂商级
func resolveAIModel(provider AIProvider, reqModel string) (modelName, baseURL, apiKey string, err error) {
	modelName = strings.TrimSpace(reqModel)
	if modelName == "" {
		modelName = provider.DefaultModel
	}
	if modelName == "" {
		return "", "", "", fmt.Errorf("未指定模型，且厂商 %s 未配置默认模型", provider.Name)
	}
	baseURL, apiKey = provider.BaseURL, provider.ApiKey
	for _, m := range provider.Models {
		if m.Name == modelName {
			if b := m.BaseURL; b != "" {
				baseURL = b
			}
			if k := m.ApiKey; k != "" {
				apiKey = k
			}
			return modelName, baseURL, apiKey, nil
		}
	}
	// 配置了 models 列表但未命中，说明模型不存在，直接报错防止误用
	if len(provider.Models) > 0 {
		var names []string
		for _, m := range provider.Models {
			names = append(names, m.Name)
		}
		return "", "", "", fmt.Errorf("厂商 %s 未配置模型 %s，可用模型: %s", provider.Name, modelName, strings.Join(names, " / "))
	}
	return modelName, baseURL, apiKey, nil
}

// resolveAIModelExtras 解析最终调用大模型的输出上限与思考强度配置（模型级优先，其次厂商级，最后默认值）
// 返回 maxOutputTokens（>0 时有效）与 reasoningEffort（非空时有效）
func resolveAIModelExtras(provider AIProvider, modelName string) (maxOutputTokens int, reasoningEffort string) {
	for _, m := range provider.Models {
		if m.Name == modelName {
			if m.MaxOutputTokens > 0 {
				maxOutputTokens = m.MaxOutputTokens
			}
			if strings.TrimSpace(m.ReasoningEffort) != "" {
				reasoningEffort = strings.TrimSpace(m.ReasoningEffort)
			}
			break
		}
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = provider.MaxOutputTokens
	}
	if reasoningEffort == "" {
		reasoningEffort = strings.TrimSpace(provider.ReasoningEffort)
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = aiMaxOutputTokens
	}
	return maxOutputTokens, strings.ToLower(reasoningEffort)
}

// resolveAPIKey 解析调用大模型使用的 API 密钥：优先使用请求中携带的密钥（向后兼容），
// 否则使用配置解析出的密钥（模型级优先，其次厂商级）
func resolveAPIKey(reqKey, cfgKey, providerName string) (string, error) {
	if key := strings.TrimSpace(reqKey); key != "" {
		return key, nil
	}
	if key := strings.TrimSpace(cfgKey); key != "" {
		return key, nil
	}
	return "", fmt.Errorf("%s API密钥未配置", providerName)
}

// aiStockItem 大模型返回的单只股票数据
type aiStockItem struct {
	Symbol    string  `json:"symbol"`    // 6位股票代码
	Name      string  `json:"name"`      // 股票名称
	Tier      int32   `json:"tier"`      // 梯队: 1龙头 2跟风 3边缘
	Relevance float64 `json:"relevance"` // 相关度 0-100
	Reason    string  `json:"reason"`    // 入选逻辑
}

// aiChatResponse OpenAI 兼容 chat/completions 响应结构
type aiChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// resolveWebSearch 解析本次调用是否启用联网搜索，优先级：请求参数 web_search > 模型级 web-search > 厂商级 web-search > 全局 ai.web-search 开关（默认开启）
func resolveWebSearch(provider AIProvider, modelName string, reqWebSearch *bool) bool {
	if reqWebSearch != nil {
		return *reqWebSearch
	}
	for _, m := range provider.Models {
		if m.Name == modelName {
			if m.WebSearch != nil {
				return *m.WebSearch
			}
			break
		}
	}
	if provider.WebSearch != nil {
		return *provider.WebSearch
	}
	// 全局开关 ai.web-search：未显式配置（nil）视为开启
	if global.GVA_CONFIG.AI.WebSearch != nil && !*global.GVA_CONFIG.AI.WebSearch {
		return false
	}
	return true
}

// resolveAIModelFormat 解析模型接口格式，优先级：请求参数 api_format > 模型级 api-format > 厂商级 api-format > 全局 ai.responses-api 开关，默认 responses
// responses=Responses API（原生 web_search 联网搜索，由模型服务端执行）；chat-completions=OpenAI 兼容接口（联网搜索时服务端按所选搜索引擎预检索）
func resolveAIModelFormat(provider AIProvider, modelName, reqFormat string) string {
	if f := normalizeAIFormat(reqFormat); f != "" {
		return f
	}
	for _, m := range provider.Models {
		if m.Name == modelName {
			if f := strings.TrimSpace(m.ApiFormat); f != "" {
				return normalizeAIFormat(f)
			}
			break
		}
	}
	if f := strings.TrimSpace(provider.ApiFormat); f != "" {
		return normalizeAIFormat(f)
	}
	// 全局开关 ai.responses-api：未显式配置（nil）视为开启
	if global.GVA_CONFIG.AI.ResponsesAPI != nil && !*global.GVA_CONFIG.AI.ResponsesAPI {
		return "chat-completions"
	}
	return "responses"
}

// normalizeAIFormat 归一化接口格式：chat-completions/chat → chat-completions，其余非空值按 responses 处理，空串原样返回
func normalizeAIFormat(format string) string {
	f := strings.TrimSpace(format)
	if strings.EqualFold(f, "chat-completions") || strings.EqualFold(f, "chat") {
		return "chat-completions"
	}
	if f == "" {
		return ""
	}
	return "responses"
}

// aiResponsesResponse OpenAI Responses API 响应结构（聚合所有 output_text 内容块）
type aiResponsesResponse struct {
	Output []struct {
		Type    string `json:"type"` // message / web_search_call / reasoning 等
		Content []struct {
			Type string `json:"type"` // output_text
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

// responsesEndpoint 由 chat/completions 端点推导 Responses API 端点（如 https://api.deepseek.com/chat/completions → https://api.deepseek.com/responses）
func responsesEndpoint(baseURL string) string {
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return strings.TrimSuffix(baseURL, "/chat/completions") + "/responses"
	}
	return baseURL + "/responses"
}

// aiResponsesCompletion 调用 OpenAI Responses API，返回最终回答内容
// enableWebSearch 为 true 时携带 web_search 工具并强制联网搜索（由模型服务端执行，无需服务端预检索）
// maxOutputTokens 为单次回答最大输出 token 数（含思考内容，按配置解析）；reasoningEffort 为思考强度控制（none/low/high/max，空则不控制）
func aiResponsesCompletion(ctx context.Context, baseURL, apiKey, providerName, modelName, instructions, input string, enableWebSearch bool, maxOutputTokens int, reasoningEffort string) (string, error) {
	payload := map[string]any{
		"model":             modelName,
		"instructions":      instructions,
		"input":             input,
		"temperature":       0.3,
		"stream":            false,
		"max_output_tokens": maxOutputTokens, // 显式设置输出上限，避免长文本被截断导致 JSON 不完整
		// 强制模型仅输出合法 JSON 对象（Responses API 的结构化输出格式为 text.format.json_object）
		"text": map[string]any{"format": map[string]any{"type": "json_object"}},
	}
	// 思考强度控制：深度思考模型（如 DeepSeek V4）思考内容计入 max_output_tokens 预算，
	// 复杂长输出任务思考过长会耗尽预算导致最终回答为空（返回 incomplete），需按配置降低/关闭思考
	if reasoningEffort != "" {
		effort := reasoningEffort
		// Responses API 不支持直接关闭思考（thinking: disabled 会被静默忽略），用 low 等效降低思考保证最终输出
		if effort == "none" || effort == "off" || effort == "disabled" {
			effort = "low"
		}
		payload["reasoning"] = map[string]any{"effort": effort}
	}
	if enableWebSearch {
		payload["tools"] = []map[string]any{{"type": "web_search"}}
		// OpenAI gpt 系列原生 Responses API 支持对象形式 tool_choice（强制搜索）；
		// 千问等兼容端点默认开启 thinking 模式，tool_choice 不允许设为 required/对象，只能省略或用 "auto"
		if strings.HasPrefix(strings.ToLower(modelName), "gpt") {
			payload["tool_choice"] = map[string]any{"type": "web_search"}
		} else {
			payload["tool_choice"] = "auto"
		}
	}
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	resp, err := request.HttpRequestWithContextAndTimeout(ctx, responsesEndpoint(baseURL), http.MethodPost, headers, nil, payload, aiRequestTimeout())
	if err != nil {
		return "", fmt.Errorf("调用大模型(%s)失败: %w", providerName, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("读取大模型响应失败: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("大模型(%s)接口返回异常(%d): %s", providerName, resp.StatusCode, string(body))
	}

	var respBody aiResponsesResponse
	if err = json.Unmarshal(body, &respBody); err != nil {
		return "", fmt.Errorf("解析大模型响应失败: %w", err)
	}
	var sb strings.Builder
	for _, item := range respBody.Output {
		if item.Type != "message" {
			continue // 跳过 web_search_call / reasoning 等非最终回答项
		}
		for _, part := range item.Content {
			if part.Type != "output_text" {
				continue
			}
			if text := strings.TrimSpace(part.Text); text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		}
	}
	content := strings.TrimSpace(sb.String())
	if content == "" {
		return "", fmt.Errorf("大模型(%s)未返回有效内容%s", providerName, aiEmptyContentHint)
	}
	return content, nil
}

// aiCompletionContent 统一获取大模型回答内容：
// - Responses API 模型：走原生联网搜索（web_search 工具），不拼服务端检索资料
// - 其余模型：启用联网搜索时服务端按所选搜索引擎预检索最新资讯拼入提示词，再走 chat/completions
// apiFormat 为请求级接口模式（可选，覆盖配置）；searchEngine 为请求级搜索引擎（chat-completions 模式生效，默认百度搜索）
func aiCompletionContent(ctx context.Context, provider AIProvider, modelName, baseURL, apiKey, systemPrompt, prompt, searchKeyword string, enableWebSearch bool, apiFormat, searchEngine string) (string, error) {
	maxOutputTokens, reasoningEffort := resolveAIModelExtras(provider, modelName)
	if resolveAIModelFormat(provider, modelName, apiFormat) == "responses" {
		return aiResponsesCompletion(ctx, baseURL, apiKey, provider.Name, modelName, systemPrompt, prompt, enableWebSearch, maxOutputTokens, reasoningEffort)
	}
	if enableWebSearch {
		if searchCtx := aiSearchNewsContext(ctx, searchEngine, searchKeyword, aiMaxSearchNews); searchCtx != "" {
			prompt += "\n\n【联网检索到的最新公开资料（供参考，仅可引用其中明确的信息，不得编造）】\n" + searchCtx
		}
	}
	payload := buildAIPayload(modelName, []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": prompt},
	}, maxOutputTokens, reasoningEffort)
	return aiChatCompletion(ctx, baseURL, apiKey, provider.Name, payload)
}

// buildAIPayload 构造 OpenAI 兼容 chat/completions 请求体
// maxTokens 为单次回答最大输出 token 数（含思考内容，按配置解析）；reasoningEffort 为思考强度控制（none/low/high/max，空则不控制）
func buildAIPayload(modelName string, messages []map[string]any, maxTokens int, reasoningEffort string) map[string]any {
	payload := map[string]any{
		"model":       modelName,
		"messages":    messages,
		"temperature": 0.3,
		"stream":      false,
		// 显式设置较长的输出上限，避免 reason 等长文本内容被模型默认 max_tokens 截断导致 JSON 不完整
		"max_tokens": maxTokens,
		// 强制模型仅输出合法 JSON 对象，避免输出 markdown 代码块或多余文字导致解析失败
		// 注意：json_object 模式要求提示词中包含"json"字样（提示词中已含"只输出一个JSON对象"）
		"response_format": map[string]any{"type": "json_object"},
	}
	// 思考强度控制：深度思考模型（如 DeepSeek V4）思考内容计入 max_tokens 预算，
	// 复杂长输出任务思考过长会耗尽预算导致最终回答为空，需按配置关闭/降低思考
	if reasoningEffort != "" {
		switch reasoningEffort {
		case "none", "off", "disabled":
			// 关闭思考（DeepSeek 官方与硅基流动实测有效）
			payload["thinking"] = map[string]any{"type": "disabled"}
		default:
			payload["reasoning_effort"] = reasoningEffort
		}
	}
	return payload
}

// aiChatCompletion 调用 OpenAI 兼容 chat/completions 接口，返回最终回答内容
func aiChatCompletion(ctx context.Context, baseURL, apiKey, providerName string, payload map[string]any) (string, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	resp, err := request.HttpRequestWithContextAndTimeout(ctx, baseURL, http.MethodPost, headers, nil, payload, aiRequestTimeout())
	if err != nil {
		return "", fmt.Errorf("调用大模型(%s)失败: %w", providerName, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("读取大模型响应失败: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("大模型(%s)接口返回异常(%d): %s", providerName, resp.StatusCode, string(body))
	}

	var chatResp aiChatResponse
	if err = json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析大模型响应失败: %w", err)
	}
	if len(chatResp.Choices) == 0 || strings.TrimSpace(chatResp.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("大模型(%s)未返回有效内容%s", providerName, aiEmptyContentHint)
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// aiEmptyContentHint 大模型未返回有效内容时的排查提示：
// 深度思考模型（如 DeepSeek V4）思考内容计入输出 token 预算，思考过长会耗尽预算导致最终回答为空
const aiEmptyContentHint = "（若为深度思考模型，思考内容计入输出 token 预算，思考过长可能耗尽预算导致无输出；可在 config.ai.yaml 为该厂商/模型配置 reasoning-effort: none 或 low 关闭/降低思考，并适当调大 max-output-tokens）"

// aiMaxSearchNews AI 联网检索时最多引用的新闻条数
const aiMaxSearchNews = 5

// aiRequestTimeout 返回 AI 大模型请求超时时间
// 优先使用配置 ai.timeout（单位秒）；未配置或 <=0 时使用默认 10 分钟。
// 深度推理类模型（如硅基流动的 DeepSeek-V4 系列）生成长文本分析耗时较长，
// 3 分钟等过短的超时会在等待响应头阶段报 context deadline exceeded
func aiRequestTimeout() time.Duration {
	if t := global.GVA_CONFIG.AI.Timeout; t > 0 {
		return time.Duration(t) * time.Second
	}
	return 10 * time.Minute
}

// aiMaxOutputTokens AI 调用时显式设置的输出 token 上限（chat/completions 的 max_tokens / Responses API 的 max_output_tokens）
// 取 DeepSeek V4 官方最大输出 384K 的十分之一（38400），作为各厂商/模型未单独配置时的兜底默认值，
// 兼顾长文本分析与思考预算，避免思考过长耗尽预算导致"未返回有效内容"
// 若模型输出超过该上限仍可能被截断，reason 输出长度要求已与之匹配（见提示词"不超过2000字"）
const aiMaxOutputTokens = 38400

// aiNewsContentLimit AI 联网检索时单条新闻摘要最大字符数
const aiNewsContentLimit = 200

// aiDataFreshnessRules 数据时效要求（拼入 AI 提示词，防止模型用训练数据旧值冒充最新数据）
const aiDataFreshnessRules = `【数据时效要求（必须严格遵守）】
1. 数据来源优先级：【联网检索到的最新公开资料】> 公开披露可查证的数据（须注明报告期/日期）> 方向性描述。
2. 严禁使用训练数据中的过时数值冒充最新数据：凡资料中未提供的近期具体数值（如最近报告期营收/净利润/增速/最新股价等），一律如实标注"(数据待核实)"或仅做方向性描述，不得凭空补全。
3. 涉及财务/行情数值时必须写明对应时间（报告期或日期），例如"2025年三季报""截至2025-12-31"，禁止输出无时间锚点的具体数字。
4. 若检索资料为空或未提及某方面信息，必须在对应维度明确说明"未检索到该方面最新信息"。`

// AiAddThemeStocks AI智能选股（异步任务化）
// 提交后立即创建执行任务并返回 task_id，AI 选股在后台 goroutine 中执行，
// 处理进度与日志实时写入任务表，供前端执行进度页面轮询展示
func (themeStockService *ThemeStockService) AiAddThemeStocks(ctx context.Context, req quantReq.AiAddThemeStockReq, userID uint) (taskID uint, err error) {
	// 1. 校验题材存在
	var theme quant.Theme
	if err = global.GVA_DB.First(&theme, req.ThemeId).Error; err != nil {
		return 0, fmt.Errorf("题材不存在: %w", err)
	}
	themeName := ""
	if theme.Name != nil {
		themeName = *theme.Name
	}
	if themeName == "" {
		return 0, fmt.Errorf("题材名称为空")
	}

	// 2. 提交时完成厂商/模型/密钥校验与解析，避免创建无效任务
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
	maxStocks := req.MaxStocks
	if maxStocks <= 0 || maxStocks > 100 {
		maxStocks = 30
	}

	// 3. 创建任务：总进度/子任务数预估 = 最多 maxStocks 只候选股票（AI返回后按实际数量修正，见 executeAiAddTask）
	// 名称中"最多X只"表示候选上限，AI 返回后更新为实际支数，避免运行中误导
	inDate := req.InDate
	if inDate.IsZero() {
		inDate = time.Now()
	}
	params := map[string]any{
		"theme_id":       req.ThemeId,
		"theme_name":     themeName,
		"max_stocks":     maxStocks,
		"provider":       req.Provider, // 存配置 key（如 qwen），重启时可直接命中 ai.providers
		"model":          modelName,
		"web_search":     req.WebSearch,
		"api_format":     req.ApiFormat,
		"search_engine":  req.SearchEngine,
		"exist_handling": req.ExistHandling,
		"min_relevance":  req.MinRelevance,
		"in_date":        inDate.Format("2006-01-02"),
	}
	var scheduledAtArgs []time.Time
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() {
		scheduledAtArgs = append(scheduledAtArgs, *req.ScheduledAt)
	}
	taskID, err = aiTaskService.CreateAiTask(ctx, "ai_add", fmt.Sprintf("分析题材股票：%s（%d只）", themeName, maxStocks), params, maxStocks, maxStocks, userID, scheduledAtArgs...)
	if err != nil {
		return 0, err
	}

	// 4. 如果是定时任务，不立即执行（等待调度器扫描）
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() && req.ScheduledAt.After(time.Now()) {
		return taskID, nil
	}
	// 立即执行
	go themeStockService.runAiAddTask(taskID, req, userID, provider, modelName, baseURL, apiKey, themeName, maxStocks)
	return taskID, nil
}

// runAiAddTask 后台执行 AI 选股任务：执行完成后写入任务结果
func (themeStockService *ThemeStockService) runAiAddTask(taskID uint, req quantReq.AiAddThemeStockReq, userID uint, provider AIProvider, modelName, baseURL, apiKey, themeName string, maxStocks int) {
	// 可取消的后台 context：用户停止任务时中断正在飞行的大模型请求
	bgCtx, cancel := context.WithCancel(context.Background())
	aiTaskService.RegisterTaskCancel(taskID, cancel)
	defer aiTaskService.UnregisterTaskCancel(taskID)
	added, updated, flagged, skipped, runErr := themeStockService.executeAiAddTask(bgCtx, taskID, req, userID, provider, modelName, baseURL, apiKey, themeName, maxStocks)
	// 用户停止导致的 context 取消：状态已置为"已取消"，静默退出，不写失败日志
	if errors.Is(runErr, context.Canceled) {
		return
	}
	if runErr != nil {
		_ = aiTaskService.UpdateAiTaskProgress(taskID, 0, "", "任务失败："+runErr.Error())
		aiTaskService.FinishAiTask(taskID, AiTaskStatusFailed, nil, runErr.Error())
		return
	}
	aiTaskService.FinishAiTask(taskID, AiTaskStatusSuccess, map[string]any{
		"added":   added,
		"updated": updated,
		"flagged": flagged,
		"skipped": skipped,
	}, "")
}

// executeAiAddTask AI 选股任务核心执行逻辑（原 AiAddThemeStocks 主体）
// 每个处理阶段实时上报进度与日志到任务表
func (themeStockService *ThemeStockService) executeAiAddTask(ctx context.Context, taskID uint, req quantReq.AiAddThemeStockReq, userID uint, provider AIProvider, modelName, baseURL, apiKey, themeName string, maxStocks int) (added, updated, flagged int, skipped []string, err error) {
	progress := func(done int, current, logLine string) {
		_ = aiTaskService.UpdateAiTaskProgress(taskID, done, current, logLine)
	}
	// 进度只按候选股票数计算（不含AI调用步），保证"进度/子任务数/名称支数"三者一致：
	// 创建时 total=maxStocks 为预估，AI 返回后按实际候选数修正
	progress(0, "AI选股调用中", fmt.Sprintf("开始AI选股（题材：%s，最多%d支，模型：%s）", themeName, maxStocks, modelName))

	// 3. 调用大模型获取候选股票
	items, err := callAIStockPick(ctx, provider, modelName, baseURL, apiKey, themeName, maxStocks, resolveWebSearch(provider, modelName, req.WebSearch), req.ApiFormat, resolveSearchEngine(req.SearchEngine))
	if err != nil {
		return 0, 0, 0, nil, err
	}
	if len(items) == 0 {
		return 0, 0, 0, nil, fmt.Errorf("大模型未返回有效股票")
	}
	progress(0, "候选股票处理中", fmt.Sprintf("AI选股完成，返回 %d 只候选股票，开始匹配本地股票池", len(items)))

	// 修正任务元信息：子任务数=实际候选数、总进度=实际候选数
	// 任务名称保持创建时的预估支数不变，不随 AI 实际返回结果更新
	if metaErr := aiTaskService.UpdateAiTaskMeta(taskID, len(items), len(items), ""); metaErr != nil {
		global.GVA_LOG.Error("AI选股更新任务元信息失败!", zap.Uint("taskID", taskID), zap.Error(metaErr))
	}

	// 4. 相关度过滤阈值：AI返回的相关度低于该值不入库（<=0 时默认 60）
	minRelevance := req.MinRelevance
	if minRelevance <= 0 {
		minRelevance = 60
	}

	// 5. 查询该题材下已存在的股票：构建去重集合，同时扫描存量记录并自动标记已暴雷股票（ST/退市/财务造假等）
	var existStocks []quant.ThemeStock
	if findErr := global.GVA_DB.Preload("Stock").Where("theme_id = ?", req.ThemeId).Find(&existStocks).Error; findErr != nil {
		global.GVA_LOG.Error("AI查询题材已存在股票失败!", zap.Error(findErr))
		return 0, 0, 0, nil, fmt.Errorf("查询题材已存在股票失败: %w", findErr)
	}
	existSet := make(map[int64]struct{}, len(existStocks))
	for i := range existStocks {
		ts := &existStocks[i]
		if ts.StockId == nil {
			continue
		}
		existSet[*ts.StockId] = struct{}{}
		// 已下架记录不重复处理
		if ts.Status != nil && *ts.Status == -1 {
			continue
		}
		// 存量记录中关联股票已暴雷 → 标记下架并在 reason 前置风险提示，保留记录便于追溯
		if isBlacklistedStock(ts.Stock) {
			oldReason := ""
			if ts.Reason != nil {
				oldReason = *ts.Reason
			}
			riskNote := fmt.Sprintf("⚠️【风险提示】该股已暴雷（%s），AI选股自动标记下架。", blacklistReason(ts.Stock))
			newReason := riskNote
			if strings.TrimSpace(oldReason) != "" {
				newReason = riskNote + "\n\n" + oldReason
			}
			if updateErr := global.GVA_DB.Model(&quant.ThemeStock{}).Where("id = ?", ts.ID).Updates(map[string]any{
				"status":     int8(-1),
				"reason":     newReason,
				"updated_by": userID,
			}).Error; updateErr != nil {
				global.GVA_LOG.Error("AI标记已暴雷题材股票失败!", zap.Error(updateErr))
			} else {
				flagged++
			}
		}
	}
	// 已存在股票处理方式：除明确传 skip 外，默认覆盖
	overwriteExisting := strings.ToLower(strings.TrimSpace(req.ExistHandling)) != "skip"

	// 6. 匹配本地股票池并组装待插入数据
	inDate := req.InDate
	if inDate.IsZero() {
		inDate = time.Now()
	}
	var newStocks []quant.ThemeStock
	processed := 0 // 已完成进度，逐只候选股票递增（AI 调用不占进度）
	for _, item := range items {
		// 任务被用户停止：提前退出，不覆盖取消状态
		if aiTaskService.IsAiTaskCanceled(taskID) {
			return 0, updated, flagged, skipped, nil
		}
		processed++
		symbol := normalizeSymbol(item.Symbol)
		stock, findErr := findStockBySymbolOrName(symbol, strings.TrimSpace(item.Name))
		if findErr != nil {
			skipped = append(skipped, fmt.Sprintf("%s(%s): 本地股票池未匹配", item.Name, item.Symbol))
			progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 本地股票池未匹配，跳过", item.Name, item.Symbol))
			continue
		}
		// 黑名单硬性剔除兜底：ST/退市/财务造假等暴雷标的（大模型已剔除，此处双保险）
		if isBlacklistedStock(&stock) {
			skipped = append(skipped, fmt.Sprintf("%s(%s): 黑名单剔除(%s)", item.Name, item.Symbol, blacklistReason(&stock)))
			progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 黑名单剔除（%s）", item.Name, item.Symbol, blacklistReason(&stock)))
			continue
		}
		relevance := item.Relevance
		if relevance <= 0 || relevance > 100 {
			relevance = 50
		}
		// 相关度过滤：归一化后的相关度低于阈值不入库（新增与覆盖更新统一生效）
		if relevance < minRelevance {
			skipped = append(skipped, fmt.Sprintf("%s(%s): 相关度%.0f低于过滤阈值%.0f，不入库", item.Name, item.Symbol, relevance, minRelevance))
			progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 相关度%.0f低于阈值%.0f，不入库", item.Name, item.Symbol, relevance, minRelevance))
			continue
		}
		tier := item.Tier
		if tier < 1 || tier > 3 {
			tier = 3
		}
		reason := strings.TrimSpace(item.Reason)
		if _, exists := existSet[int64(stock.ID)]; exists {
			if overwriteExisting {
				// 存量记录已被标记下架（暴雷股），不允许覆盖复活
				if isBlacklistedStock(&stock) {
					skipped = append(skipped, fmt.Sprintf("%s(%s): 已暴雷自动下架，跳过覆盖", item.Name, item.Symbol))
					progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 已暴雷自动下架，跳过覆盖", item.Name, item.Symbol))
					continue
				}
				// 覆盖模式：更新该题材下已存在记录的AI分析入选逻辑、梯队、相关度、纳入日期、状态（保留人工编辑的reason与sort，不覆盖）
				updates := map[string]any{
					"ai_reason":  reason,
					"tier":       tier,
					"relevance":  relevance,
					"in_date":    inDate,
					"status":     int8(1),
					"updated_by": userID,
				}
				if updateErr := global.GVA_DB.Model(&quant.ThemeStock{}).
					Where("theme_id = ? AND stock_id = ?", req.ThemeId, stock.ID).
					Updates(updates).Error; updateErr != nil {
					global.GVA_LOG.Error("AI覆盖更新已存在题材股票失败!", zap.Error(updateErr))
					skipped = append(skipped, fmt.Sprintf("%s(%s): 该题材下已存在，覆盖更新失败", item.Name, item.Symbol))
					progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 该题材下已存在，覆盖更新失败", item.Name, item.Symbol))
				} else {
					updated++
					progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 该题材下已存在，覆盖更新成功", item.Name, item.Symbol))
				}
			} else {
				skipped = append(skipped, fmt.Sprintf("%s(%s): 该题材下已存在", item.Name, item.Symbol))
				progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 该题材下已存在，跳过", item.Name, item.Symbol))
			}
			continue
		}
		stockId := int64(stock.ID)
		themeId := req.ThemeId
		newStocks = append(newStocks, quant.ThemeStock{
			ThemeId:   &themeId,
			StockId:   &stockId,
			Relevance: &relevance,
			AiReason:  &reason,
			InDate:    &inDate,
			Status:    int8Ptr(1),
			Tier:      &tier,
			// 不设置 Sort，由人工排序控制（数据库默认 0）
			GVA_MODEL_ADDON: global.GVA_MODEL_ADDON{
				CreatedBy: userID,
				UpdatedBy: userID,
			},
		})
		existSet[int64(stock.ID)] = struct{}{} // 防止 AI 返回重复股票
		progress(processed, fmt.Sprintf("%s(%s)", item.Name, item.Symbol), fmt.Sprintf("%s(%s): 入库成功", item.Name, item.Symbol))
	}

	if len(newStocks) == 0 {
		progress(processed, "", fmt.Sprintf("AI选股处理完成：新增0只，覆盖更新%d只，下架暴雷股%d只，跳过%d只", updated, flagged, len(skipped)))
		return 0, updated, flagged, skipped, nil
	}

	// 7. 批量插入
	if err = global.GVA_DB.Create(&newStocks).Error; err != nil {
		global.GVA_LOG.Error("AI添加题材股票批量插入失败!", zap.Error(err))
		return 0, updated, flagged, skipped, err
	}
	// 8. 同步题材股票数量
	if err = syncThemeStockCount(global.GVA_DB, []int32{req.ThemeId}); err != nil {
		global.GVA_LOG.Error("AI添加题材股票后同步股票数量失败!", zap.Error(err))
	}
	progress(processed, "", fmt.Sprintf("AI选股完成：新增%d只，覆盖更新%d只，下架暴雷股%d只，跳过%d只", len(newStocks), updated, flagged, len(skipped)))
	return len(newStocks), updated, flagged, skipped, nil
}

// AiUpdateThemeStock AI 智能更新单只题材股票（异步任务化）
// 提交后立即创建执行任务并返回 task_id，分析在后台 goroutine 中执行，进度实时写入任务表
func (themeStockService *ThemeStockService) AiUpdateThemeStock(ctx context.Context, req quantReq.AiUpdateThemeStockReq, userID uint) (taskID uint, err error) {
	// 校验厂商、解析模型与密钥
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

	// 查询记录并加载题材与股票信息（提交时校验记录存在）
	var ts quant.ThemeStock
	if err = global.GVA_DB.Preload("Theme").Preload("Stock").First(&ts, req.ID).Error; err != nil {
		return 0, fmt.Errorf("题材股票记录不存在: %w", err)
	}
	themeName := ""
	if ts.Theme != nil && ts.Theme.Name != nil {
		themeName = *ts.Theme.Name
	}
	stockName := ""
	if ts.Stock != nil && ts.Stock.Name != nil {
		stockName = *ts.Stock.Name
	}
	params := map[string]any{
		"id":            req.ID,
		"stock_name":    stockName,
		"theme_name":    themeName,
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
	taskID, err = aiTaskService.CreateAiTask(ctx, "ai_update_one", fmt.Sprintf("分析题材股票：%s", stockName), params, 1, 1, userID, scheduledAtArgs...)
	if err != nil {
		return 0, err
	}

	// 如果是定时任务，不立即执行（等待调度器扫描到期望执行时间后发起）
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() && req.ScheduledAt.After(time.Now()) {
		return taskID, nil
	}
	// 立即执行
	go themeStockService.runAiUpdateOneTask(taskID, req, userID, provider, modelName, baseURL, apiKey, themeName)
	return taskID, nil
}

// runAiUpdateOneTask 后台执行单只 AI 更新任务
func (themeStockService *ThemeStockService) runAiUpdateOneTask(taskID uint, req quantReq.AiUpdateThemeStockReq, userID uint, provider AIProvider, modelName, baseURL, apiKey, themeName string) {
	// 可取消的后台 context：用户停止任务时中断正在飞行的大模型请求
	bgCtx, cancel := context.WithCancel(context.Background())
	aiTaskService.RegisterTaskCancel(taskID, cancel)
	defer aiTaskService.UnregisterTaskCancel(taskID)
	// 重新查询记录并加载题材与股票信息
	var ts quant.ThemeStock
	if err := global.GVA_DB.Preload("Theme").Preload("Stock").First(&ts, req.ID).Error; err != nil {
		_ = aiTaskService.UpdateAiTaskProgress(taskID, 1, "", "任务失败："+err.Error())
		aiTaskService.FinishAiTask(taskID, AiTaskStatusFailed, nil, err.Error())
		return
	}
	stockName := ""
	if ts.Stock != nil && ts.Stock.Name != nil {
		stockName = *ts.Stock.Name
	}
	_ = aiTaskService.UpdateAiTaskProgress(taskID, 0, stockName, fmt.Sprintf("开始分析个股：%s（题材：%s，模型：%s）", stockName, themeName, modelName))
	// 单只任务开始前检测一次：任务已被用户停止则不再执行
	if aiTaskService.IsAiTaskCanceled(taskID) {
		return
	}
	if runErr := themeStockService.aiUpdateOneThemeStock(bgCtx, &ts, provider, modelName, baseURL, apiKey, resolveWebSearch(provider, modelName, req.WebSearch), req.ApiFormat, resolveSearchEngine(req.SearchEngine), userID); runErr != nil {
		// 用户停止导致的 context 取消：状态已置为"已取消"，静默退出，不写失败日志
		if errors.Is(runErr, context.Canceled) {
			return
		}
		_ = aiTaskService.UpdateAiTaskProgress(taskID, 1, stockName, "分析失败："+runErr.Error())
		aiTaskService.FinishAiTask(taskID, AiTaskStatusFailed, nil, runErr.Error())
		return
	}
	_ = aiTaskService.UpdateAiTaskProgress(taskID, 1, stockName, "AI更新成功")
	aiTaskService.FinishAiTask(taskID, AiTaskStatusSuccess, map[string]any{"id": req.ID, "stock_name": stockName}, "")
}

// AiUpdateThemeStocks AI 智能批量更新题材股票（异步任务化）
// 一次完成厂商/模型/密钥校验与解析，按所选记录顺序串行分析并逐只上报进度与结果
func (themeStockService *ThemeStockService) AiUpdateThemeStocks(ctx context.Context, req quantReq.AiUpdateThemeStocksReq, userID uint) (taskID uint, err error) {
	ids := req.IDs
	if len(ids) == 0 {
		return 0, fmt.Errorf("请选择要更新的股票")
	}

	// 校验厂商、解析模型与密钥（仅一次）
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

	// 查询所选记录并加载题材与股票信息（提交时校验，同时取题材名称用于任务展示）
	var tsList []quant.ThemeStock
	if err = global.GVA_DB.Preload("Theme").Preload("Stock").Where("id IN ?", ids).Find(&tsList).Error; err != nil {
		global.GVA_LOG.Error("AI批量更新题材股票查询失败!", zap.Error(err))
		return 0, err
	}
	themeName := ""
	for i := range tsList {
		if tsList[i].Theme != nil && tsList[i].Theme.Name != nil && strings.TrimSpace(*tsList[i].Theme.Name) != "" {
			themeName = *tsList[i].Theme.Name
			break
		}
	}
	params := map[string]any{
		"ids":           ids,
		"count":         len(ids),
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
	taskID, err = aiTaskService.CreateAiTask(ctx, "ai_update_batch", fmt.Sprintf("分析题材股票：%s（%d支）", themeName, len(ids)), params, len(ids), len(ids), userID, scheduledAtArgs...)
	if err != nil {
		return 0, err
	}

	// 如果是定时任务，不立即执行（等待调度器扫描到期望执行时间后发起）
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() && req.ScheduledAt.After(time.Now()) {
		return taskID, nil
	}
	// 立即执行
	go themeStockService.runAiUpdateBatchTask(taskID, req, userID, provider, modelName, baseURL, apiKey, themeName)
	return taskID, nil
}

// runAiUpdateBatchTask 后台执行批量 AI 更新任务：逐只串行分析并上报进度，完成写入汇总结果
func (themeStockService *ThemeStockService) runAiUpdateBatchTask(taskID uint, req quantReq.AiUpdateThemeStocksReq, userID uint, provider AIProvider, modelName, baseURL, apiKey, themeName string) {
	// 可取消的后台 context：用户停止任务时中断正在飞行的大模型请求
	bgCtx, cancel := context.WithCancel(context.Background())
	aiTaskService.RegisterTaskCancel(taskID, cancel)
	defer aiTaskService.UnregisterTaskCancel(taskID)
	ids := req.IDs
	var tsList []quant.ThemeStock
	if err := global.GVA_DB.Preload("Theme").Preload("Stock").Where("id IN ?", ids).Find(&tsList).Error; err != nil {
		_ = aiTaskService.UpdateAiTaskProgress(taskID, 0, "", "任务失败："+err.Error())
		aiTaskService.FinishAiTask(taskID, AiTaskStatusFailed, nil, err.Error())
		return
	}
	byID := make(map[uint]*quant.ThemeStock, len(tsList))
	for i := range tsList {
		byID[tsList[i].ID] = &tsList[i]
	}
	_ = aiTaskService.UpdateAiTaskProgress(taskID, 0, "", fmt.Sprintf("开始批量AI更新，共 %d 只个股", len(ids)))

	// 按用户所选顺序串行处理，逐只上报进度并汇总结果
	results := make([]quantReq.AiUpdateThemeStockResult, 0, len(ids))
	enableWebSearch := resolveWebSearch(provider, modelName, req.WebSearch)
	for idx, id := range ids {
		// 任务被用户停止：提前退出，不覆盖取消状态
		if aiTaskService.IsAiTaskCanceled(taskID) {
			return
		}
		done := idx + 1
		ts, ok := byID[id]
		if !ok {
			results = append(results, quantReq.AiUpdateThemeStockResult{ID: id, Success: false, Message: "记录不存在"})
			_ = aiTaskService.UpdateAiTaskProgress(taskID, done, fmt.Sprintf("记录#%d", id), fmt.Sprintf("记录 #%d 不存在，跳过", id))
			continue
		}
		stockName := ""
		if ts.Stock != nil && ts.Stock.Name != nil {
			stockName = *ts.Stock.Name
		}
		result := quantReq.AiUpdateThemeStockResult{ID: ts.ID, StockName: stockName}
		if updateErr := themeStockService.aiUpdateOneThemeStock(bgCtx, ts, provider, modelName, baseURL, apiKey, enableWebSearch, req.ApiFormat, resolveSearchEngine(req.SearchEngine), userID); updateErr != nil {
			result.Success = false
			result.Message = updateErr.Error()
			results = append(results, result)
			global.GVA_LOG.Error("AI批量更新单只题材股票失败!", zap.Uint("id", ts.ID), zap.Error(updateErr))
			_ = aiTaskService.UpdateAiTaskProgress(taskID, done, stockName, fmt.Sprintf("更新 %s 失败：%s", stockName, updateErr.Error()))
		} else {
			result.Success = true
			result.Message = "更新成功"
			results = append(results, result)
			_ = aiTaskService.UpdateAiTaskProgress(taskID, done, stockName, fmt.Sprintf("更新 %s 成功", stockName))
		}
	}
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	_ = aiTaskService.UpdateAiTaskProgress(taskID, len(ids), "", fmt.Sprintf("批量更新完成：成功 %d/%d 只", successCount, len(ids)))
	aiTaskService.FinishAiTask(taskID, AiTaskStatusSuccess, results, "")
}

// RestartAiAddTask 重启已取消的AI选股任务：按任务参数重建请求并重新发起后台执行
func (themeStockService *ThemeStockService) RestartAiAddTask(task quant.QuantAiTask, params map[string]any, userID uint) error {
	req := quantReq.AiAddThemeStockReq{
		ThemeId:       int32(taskParamsInt(params, "theme_id")),
		MaxStocks:     taskParamsInt(params, "max_stocks"),
		Provider:      taskParamsString(params, "provider"),
		Model:         taskParamsString(params, "model"),
		WebSearch:     taskParamsBoolPtr(params, "web_search"),
		ApiFormat:     taskParamsString(params, "api_format"),
		SearchEngine:  taskParamsString(params, "search_engine"),
		ExistHandling: taskParamsString(params, "exist_handling"),
		MinRelevance:  taskParamsFloat(params, "min_relevance"),
	}
	if s := taskParamsString(params, "in_date"); s != "" {
		if d, err := time.Parse("2006-01-02", s); err == nil {
			req.InDate = d
		}
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
	themeName := taskParamsString(params, "theme_name")
	if themeName == "" {
		var theme quant.Theme
		if err := global.GVA_DB.First(&theme, req.ThemeId).Error; err != nil {
			return fmt.Errorf("题材不存在: %w", err)
		}
		if theme.Name != nil {
			themeName = *theme.Name
		}
	}
	if err := aiTaskService.ResetAiTask(task.ID, req.MaxStocks, req.MaxStocks, userID); err != nil {
		return err
	}
	go themeStockService.runAiAddTask(task.ID, req, userID, provider, modelName, baseURL, apiKey, themeName, req.MaxStocks)
	return nil
}

// RestartAiUpdateOneTask 重启已取消的单只AI更新任务
func (themeStockService *ThemeStockService) RestartAiUpdateOneTask(task quant.QuantAiTask, params map[string]any, userID uint) error {
	req := quantReq.AiUpdateThemeStockReq{
		ID:           uint(taskParamsInt(params, "id")),
		Provider:     taskParamsString(params, "provider"),
		Model:        taskParamsString(params, "model"),
		WebSearch:    taskParamsBoolPtr(params, "web_search"),
		ApiFormat:    taskParamsString(params, "api_format"),
		SearchEngine: taskParamsString(params, "search_engine"),
	}
	if req.ID == 0 {
		return fmt.Errorf("任务参数缺少记录ID，无法重启")
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
	if err := aiTaskService.ResetAiTask(task.ID, 1, 1, userID); err != nil {
		return err
	}
	go themeStockService.runAiUpdateOneTask(task.ID, req, userID, provider, modelName, baseURL, apiKey, taskParamsString(params, "theme_name"))
	return nil
}

// RestartAiUpdateBatchTask 重启已取消的批量AI更新任务
func (themeStockService *ThemeStockService) RestartAiUpdateBatchTask(task quant.QuantAiTask, params map[string]any, userID uint) error {
	req := quantReq.AiUpdateThemeStocksReq{
		IDs:          taskParamsUintSlice(params, "ids"),
		Provider:     taskParamsString(params, "provider"),
		Model:        taskParamsString(params, "model"),
		WebSearch:    taskParamsBoolPtr(params, "web_search"),
		ApiFormat:    taskParamsString(params, "api_format"),
		SearchEngine: taskParamsString(params, "search_engine"),
	}
	if len(req.IDs) == 0 {
		return fmt.Errorf("任务参数缺少股票ID列表，无法重启")
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
	if err := aiTaskService.ResetAiTask(task.ID, len(req.IDs), len(req.IDs), userID); err != nil {
		return err
	}
	go themeStockService.runAiUpdateBatchTask(task.ID, req, userID, provider, modelName, baseURL, apiKey, taskParamsString(params, "theme_name"))
	return nil
}

// aiUpdateOneThemeStock 更新单只题材股票（单只与批量 AI 更新共用）
// provider/modelName/baseURL/apiKey 已由调用方完成校验与解析
func (themeStockService *ThemeStockService) aiUpdateOneThemeStock(ctx context.Context, ts *quant.ThemeStock, provider AIProvider, modelName, baseURL, apiKey string, enableWebSearch bool, apiFormat, searchEngine string, userID uint) error {
	if ts.Stock == nil || ts.Stock.Name == nil || strings.TrimSpace(*ts.Stock.Name) == "" {
		return fmt.Errorf("股票信息不完整，无法分析")
	}
	themeName := ""
	if ts.Theme != nil && ts.Theme.Name != nil {
		themeName = *ts.Theme.Name
	}
	if strings.TrimSpace(themeName) == "" {
		return fmt.Errorf("题材名称为空")
	}

	// 调用大模型分析该个股
	symbol := ""
	if ts.Stock.Symbol != nil {
		symbol = *ts.Stock.Symbol
	}
	item, err := callAIStockAnalyze(ctx, provider, modelName, baseURL, apiKey, strings.TrimSpace(themeName), symbol, *ts.Stock.Name, enableWebSearch, apiFormat, searchEngine)
	if err != nil {
		return err
	}

	// 归一化并更新记录
	reason := strings.TrimSpace(item.Reason)
	if reason == "" {
		return fmt.Errorf("大模型未返回有效的入选逻辑")
	}
	relevance := item.Relevance
	if relevance <= 0 || relevance > 100 {
		relevance = 50
	}
	tier := item.Tier
	if tier < 1 || tier > 3 {
		tier = 3
	}
	updates := map[string]any{
		"ai_reason":  reason,
		"tier":       tier,
		"relevance":  relevance,
		"updated_by": userID,
	}
	if err = global.GVA_DB.Model(&quant.ThemeStock{}).Where("id = ?", ts.ID).Updates(updates).Error; err != nil {
		global.GVA_LOG.Error("AI更新题材股票失败!", zap.Error(err))
		return err
	}
	return nil
}

// callAIStockAnalyze 调用大模型分析单只个股在题材中的入选逻辑、梯队与相关度
// enableWebSearch 为 true 时启用联网搜索，让大模型检索最新公开信息
// apiFormat 为请求级接口模式（可选，覆盖配置）；searchEngine 为请求级搜索引擎（chat-completions 模式生效，默认百度搜索）
func callAIStockAnalyze(ctx context.Context, provider AIProvider, modelName, baseURL, apiKey, themeName, symbol, stockName string, enableWebSearch bool, apiFormat, searchEngine string) (aiStockItem, error) {
	var item aiStockItem
	prompt := fmt.Sprintf(`你是一位资深的A股题材分析专家，熟悉A股市场的热点题材、概念板块与产业链上下游。

请针对题材"%s"，重新分析个股"%s（%s）"在该题材中的入选资格，并输出更新后的分析结果。

【筛选标准与优先级规则（必须严格遵守）】

一、 黑名单硬性剔除与处置规则（风控前置）
若目标公司命中以下任意一条，必须将其列为【排除标的】，并在输出中将 tier 调整为 4、relevance 调整为最低值，且必须在 reason 的风险提示段落中明确说明剔除原因：
1. 风险警示：ST、*ST、已发布退市风险警示或处于退市流程中的公司；
2. 财务与合规风险：近期财务暴雷、业绩大幅预亏、或存在重大监管处罚/违规立案的公司；
3. 伪概念与炒作：历史上多次恶意蹭概念、仅签署无实质约束力的框架协议、或股票名称带关键词但主营业务几乎无关的公司；
4. 流动性枯竭：成交极度匮乏、流动性欠佳的微盘/边缘标的。

二、 相关性与选股优先级（从高到低排序依据）
第一优先级（Tier 1：核心主线 / 核心龙头）：
- 主营业务/核心产品直接对应本题材，相关业务在营收或利润中占比高（业务纯度高）；
- 拥有量产/落地项目、明确客户订单或官方公告披露的核心成果；
- 属于该题材产业链的核心节点企业、细分领域头部龙头，市值与流动性良好。

第二优先级（Tier 2：关键配套 / 高壁垒）：
- 掌握题材的核心关键技术、专利或独家工艺；
- 为本题材核心产业链提供关键零部件、核心原材料或高壁垒配套支撑（具备强议价能力）。

第三优先级（Tier 3：下游应用 / 设备服务）：
- 为题材提供下游场景落地应用、生产设备制造或基础运维服务；
- 虽属于泛领域龙头，但本题材相关业务属于其战略布局的核心新增长极。

三、 综合加权与同级排序规则
在同一 Tier 内，按以下权重依次进行细化排序：
1. 业务纯度与弹性（相关业务营收/利润占比越高越靠前）
2. 产业链话语权（细分领域市占率 Top 龙头优先）
3. 技术壁垒与订单落地确定性

【输出字段】
1. tier：梯队，1=题材龙头，2=跟风补涨，3=边缘沾边
2. relevance：相关度，0-100的整数
3. reason：入选逻辑（综合概述，总字数控制在 600~2000 字），内容必须详实可信。凡涉及营收/净利润/增速/毛利率/市占率/产能/订单金额/客户/机构调研等具体信息，必须给出具体数值或名称；严禁编造，仅可写入公开披露、可查证的数据，不得虚构。
必须严格按照以下3个维度依次输出，每个维度独立成一段，段与段之间用一个空行(\n\n)分隔。严禁输出段落标题（如“核心逻辑”、“基本面”等），直接输出具体分析内容：

   - 第1段（核心逻辑与身份）：阐述公司入选该概念的核心原因，明确公司在概念产业链中的位置（如龙头/核心供应商/技术攻关者），以及该概念对应产品/服务的具体业务形态与核心技术壁垒。
   - 第2段（基本面与落地验证）：阐述公司行业地位（量化市占率或排名），展示业务落地验证履历。写明核心合作客户全称（或官方代称）、在手订单/合同金额、现有产能储备及后续扩产规划（写明具体产能数字），以及产品在下游客户侧的验证与量产供货进度、项目落地周期。
   - 第3段（财务分析与业绩兑现）：分析今年最新财务表现（必须写明具体报告期与披露日期）。写明营业收入规模、同比/环比增速及收入结构（各业务板块占比）；写明归母净利润与扣非净利润（扣非须单独说明，体现剔除一次性收益后的真实盈利能力）及同比增速；写明毛利率、净利率等利润率指标；若已披露业绩预告/快报，须写明预告区间及同比变动。

%s

只输出一个JSON对象，不要输出任何其他文字或markdown标记，格式如下：
{"symbol":"688498","name":"源杰科技","tier":1,"relevance":95,"reason":"国产高速光芯片龙头，主营2.5G~100G激光器芯片（DFB/EML）等光通信核心器件，深度绑定CPO与算力两大主线，在AI算力光模块放量背景下是板块中稀缺的兼具技术壁垒和业绩弹性的标的。\n\n100G EML等高速光芯片细分领域市占率居行业前列（约XX%%），与XX光模块、XX光模块等头部客户保持深度合作，产品已通过XX、XX等机构验证并进入量产供货阶段，在手订单金额XX亿元，XX项目预计XX年完成落地，后续扩产/研发投入约XX亿元。\n\n2025年三季报（披露日期2025-10-XX）营收XX亿元、同比XX%%、环比XX%%，高速光芯片业务营收占比XX%%；归母净利润XX亿元、扣非净利润XX亿元（剔除一次性收益后同比XX%%），毛利率XX%%、净利率XX%%、营业利润率XX%%；2025年度业绩预告显示归母净利润XX至XX亿元、同比变动XX%%。"}`, themeName, stockName, symbol, aiDataFreshnessRules)

	if enableWebSearch {
		if searchCtx := aiSearchNewsContext(ctx, searchEngine, fmt.Sprintf("%s %s %s", themeName, stockName, symbol), aiMaxSearchNews); searchCtx != "" {
			prompt += "\n\n【联网检索到的最新公开资料（供参考，仅可引用其中明确的信息，不得编造）】\n" + searchCtx
		}
	}

	content, err := aiCompletionContent(ctx, provider, modelName, baseURL, apiKey, "你是一个A股题材分析助手，只输出JSON对象，不要输出任何多余内容。", prompt, fmt.Sprintf("%s %s %s", themeName, stockName, symbol), enableWebSearch, apiFormat, searchEngine)
	if err != nil {
		return item, err
	}
	content = stripCodeFence(strings.TrimSpace(content))

	if err = json.Unmarshal([]byte(content), &item); err != nil {
		// 容错：模型输出可能因输出 token 上限被截断，尝试补全截断的 JSON 后再解析
		if repaired, ok := repairTruncatedJSON(content); ok && json.Unmarshal([]byte(repaired), &item) == nil {
			return item, nil
		}
		return item, fmt.Errorf("解析大模型返回的个股分析失败: %w，原始内容: %.200s", err, content)
	}
	return item, nil
}

// callAIStockPick 调用大模型返回题材相关股票 JSON 数组
// enableWebSearch 为 true 时 AI 分析前自动检索相关最新新闻拼入提示词
// apiFormat 为请求级接口模式（可选，覆盖配置）；searchEngine 为请求级搜索引擎（chat-completions 模式生效，默认百度搜索）
func callAIStockPick(ctx context.Context, provider AIProvider, modelName, baseURL, apiKey, themeName string, maxStocks int, enableWebSearch bool, apiFormat, searchEngine string) ([]aiStockItem, error) {
	prompt := fmt.Sprintf(`你是一位资深的A股题材分析专家，熟悉A股市场的热点题材、概念板块与产业链上下游。

请针对题材"%s"，从A股市场中选择最相关的%d只股票，按相关度从高到低排列（严格按照题材业务真实相关度从高到低排序）。

【筛选标准与优先级规则（必须严格遵守）】

一、 黑名单硬性剔除与处置规则（风控前置）
若目标公司命中以下任意一条，必须将其列为【排除标的】，并在输出中将 tier 调整为 4、relevance 调整为最低值，且必须在 reason 的风险提示段落中明确说明剔除原因：
1. 风险警示：ST、*ST、已发布退市风险警示或处于退市流程中的公司；
2. 财务与合规风险：近期财务暴雷、业绩大幅预亏、或存在重大监管处罚/违规立案的公司；
3. 伪概念与炒作：历史上多次恶意蹭概念、仅签署无实质约束力的框架协议、或股票名称带关键词但主营业务几乎无关的公司；
4. 流动性枯竭：成交极度匮乏、流动性欠佳的微盘/边缘标的。

二、 相关性与选股优先级（从高到低排序依据）
第一优先级（Tier 1：核心主线 / 核心龙头）：
- 主营业务/核心产品直接对应本题材，相关业务在营收或利润中占比高（业务纯度高）；
- 拥有量产/落地项目、明确客户订单或官方公告披露的核心成果；
- 属于该题材产业链的核心节点企业、细分领域头部龙头，市值与流动性良好。

第二优先级（Tier 2：关键配套 / 高壁垒）：
- 掌握题材的核心关键技术、专利或独家工艺；
- 为本题材核心产业链提供关键零部件、核心原材料或高壁垒配套支撑（具备强议价能力）。

第三优先级（Tier 3：下游应用 / 设备服务）：
- 为题材提供下游场景落地应用、生产设备制造或基础运维服务；
- 虽属于泛领域龙头，但本题材相关业务属于其战略布局的核心新增长极。

三、 综合加权与同级排序规则
在同一 Tier 内，按以下权重依次进行细化排序：
1. 业务纯度与弹性（相关业务营收/利润占比越高越靠前）
2. 产业链话语权（细分领域市占率 Top 龙头优先）
3. 技术壁垒与订单落地确定性

【每只股票必须包含以下字段】
1. symbol：6位数字股票代码（如 000001）
2. name：股票名称
3. tier：梯队，1=题材龙头，2=跟风补涨，3=边缘沾边，4=排除标的
4. relevance：相关度，0-100的整数
5. reason：入选逻辑（综合概述，总字数控制在 600~2000 字），内容必须详实可信。凡涉及营收/净利润/增速/毛利率/市占率/产能/订单金额/客户/机构调研等具体信息，必须给出具体数值或名称；严禁编造，仅可写入公开披露、可查证的数据，不得虚构。
必须严格按照以下3个维度依次输出，每个维度独立成一段，段与段之间用一个空行(\n\n)分隔。严禁输出段落标题（如“核心逻辑”、“基本面”等），直接输出具体分析内容：

   - 第1段（核心逻辑与身份）：阐述公司入选该概念的核心原因，明确公司在概念产业链中的位置（如龙头/核心供应商/技术攻关者），以及该概念对应产品/服务的具体业务形态与核心技术壁垒。
   - 第2段（基本面与落地验证）：阐述公司行业地位（量化市占率或排名），展示业务落地验证履历。写明核心合作客户全称（或官方代称）、在手订单/合同金额、现有产能储备及后续扩产规划（写明具体产能数字），以及产品在下游客户侧的验证与量产供货进度、项目落地周期。
   - 第3段（财务分析与业绩兑现）：分析今年最新财务表现（必须写明具体报告期与披露日期）。写明营业收入规模、同比/环比增速及收入结构（各业务板块占比）；写明归母净利润与扣非净利润（扣非须单独说明，体现剔除一次性收益后的真实盈利能力）及同比增速；写明毛利率、净利率等利润率指标；若已披露业绩预告/快报，须写明预告区间及同比变动。
%s

只输出一个JSON数组，不要输出任何其他文字或markdown标记，格式如下：
[{"symbol":"688498","name":"源杰科技","tier":1,"relevance":95,"reason":"国产高速光芯片龙头，主营2.5G~100G激光器芯片（DFB/EML）等光通信核心器件，深度绑定CPO与算力两大主线，在AI算力光模块放量背景下是板块中稀缺的兼具技术壁垒和业绩弹性的标的。\n\n100G EML等高速光芯片细分领域市占率居行业前列（约XX%%），与XX光模块、XX光模块等头部客户保持深度合作，产品已通过XX、XX等机构验证并进入量产供货阶段，在手订单金额XX亿元，XX项目预计XX年完成落地，后续扩产/研发投入约XX亿元。\n\n2025年三季报（披露日期2025-10-XX）营收XX亿元、同比XX%%、环比XX%%，高速光芯片业务营收占比XX%%；归母净利润XX亿元、扣非净利润XX亿元（剔除一次性收益后同比XX%%），毛利率XX%%、净利率XX%%、营业利润率XX%%；2025年度业绩预告显示归母净利润XX至XX亿元、同比变动XX%%。"}]`, themeName, maxStocks, aiDataFreshnessRules)

	if enableWebSearch {
		if searchCtx := aiSearchNewsContext(ctx, searchEngine, themeName, aiMaxSearchNews); searchCtx != "" {
			prompt += "\n\n【联网检索到的最新公开资料（供参考，仅可引用其中明确的信息，不得编造）】\n" + searchCtx
		}
	}

	content, err := aiCompletionContent(ctx, provider, modelName, baseURL, apiKey, "你是一个A股题材分析助手，只输出JSON数组，不要输出任何多余内容。", prompt, themeName, enableWebSearch, apiFormat, searchEngine)
	if err != nil {
		return nil, err
	}
	content = stripCodeFence(strings.TrimSpace(content))

	var items []aiStockItem
	if err = json.Unmarshal([]byte(content), &items); err != nil {
		// 容错1：模型输出可能因输出 token 上限被截断，尝试补全截断的 JSON 后再解析
		if repaired, ok := repairTruncatedJSON(content); ok {
			if json.Unmarshal([]byte(repaired), &items) == nil {
				return items, nil
			}
			// 截断后修复出的可能是单个对象，按单只股票解析并包装为数组
			var single aiStockItem
			if json.Unmarshal([]byte(repaired), &single) == nil {
				return []aiStockItem{single}, nil
			}
		}
		// 容错2：模型可能未按要求返回数组而是直接返回单个对象，按单只股票解析并包装为数组
		var single aiStockItem
		if json.Unmarshal([]byte(content), &single) == nil {
			return []aiStockItem{single}, nil
		}
		return nil, fmt.Errorf("解析大模型返回的股票列表失败: %w，原始内容: %.200s", err, content)
	}
	return items, nil
}

// normalizeSymbol 规范化股票代码：去除前缀字母与点号，仅保留6位数字
func normalizeSymbol(symbol string) string {
	re := regexp.MustCompile(`\d{6}`)
	return re.FindString(symbol)
}

// findStockBySymbolOrName 优先按代码匹配，其次按名称匹配本地股票池
func findStockBySymbolOrName(symbol, name string) (stock quant.BaseStock, err error) {
	db := global.GVA_DB.Model(&quant.BaseStock{})
	if symbol != "" {
		err = db.Where("symbol = ?", symbol).First(&stock).Error
		if err == nil {
			return stock, nil
		}
	}
	if name != "" {
		err = global.GVA_DB.Model(&quant.BaseStock{}).Where("name = ?", name).First(&stock).Error
		if err == nil {
			return stock, nil
		}
	}
	return stock, fmt.Errorf("未匹配到股票")
}

// stripCodeFence 去除大模型返回内容中的 markdown 代码块包裹（```json ... ```）
func stripCodeFence(content string) string {
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*(.*?)\\s*```$")
	if matches := re.FindStringSubmatch(content); len(matches) == 2 {
		return matches[1]
	}
	return content
}

// repairTruncatedJSON 尝试修复被模型输出截断的 JSON
// 模型输出可能因输出 token 上限被强制截断（典型场景为最后一个字符串字段 reason 写到一半），
// 截断可能发生在：1.字符串值中间（补闭合引号）；2.对象闭合处（补右大括号）；3.数组闭合处（补右中括号）。
// 按"最可能"到"最不可能"的顺序依次尝试补全组合，修复成功返回 true，失败返回 false（由调用方走标准解析错误流程）
func repairTruncatedJSON(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasSuffix(s, "}") || strings.HasSuffix(s, "]") {
		return s, false
	}
	candidate := s
	// 截断处若恰好是转义符（如 \n 的 \），先去掉它避免破坏闭合引号
	if strings.HasSuffix(candidate, "\\") {
		candidate = strings.TrimRight(candidate, "\\")
	}
	var tmp any
	// 补全组合按优先级尝试：字符串中间截断 + 对象 + 数组
	for _, suffix := range []string{`"}]`, `"}`, `"]`, `}]`, `}`, `]`} {
		repaired := candidate + suffix
		if json.Unmarshal([]byte(repaired), &tmp) == nil {
			return repaired, true
		}
	}
	return s, false
}

// blacklistFraudKeywords 财务造假/重大违规关键词（用于匹配 risk 风险提示字段）
var blacklistFraudKeywords = []string{
	"造假", "舞弊", "虚增", "财务造假", "重大违法", "立案", "行政处罚", "证监会处罚", "退市风险",
}

// isBlacklistedStock 黑名单/暴雷判断：ST/*ST/S*ST 风险警示、退市/暂停上市、财务造假等暴雷标的
// 判断依据：1.股票名称 2.list_status 上市状态 3.delist_date 退市日期 4.risk 风险提示关键词
func isBlacklistedStock(stock *quant.BaseStock) bool {
	if stock == nil {
		return false
	}
	if stock.Name != nil && isBlacklistedStockName(*stock.Name) {
		return true
	}
	if stock.ListStatus != nil {
		switch strings.ToUpper(strings.TrimSpace(*stock.ListStatus)) {
		case "D", "P": // D-退市 P-暂停上市
			return true
		}
	}
	if stock.DelistDate != nil && !stock.DelistDate.IsZero() {
		return true
	}
	if stock.Risk != nil {
		risk := *stock.Risk
		for _, kw := range blacklistFraudKeywords {
			if strings.Contains(risk, kw) {
				return true
			}
		}
	}
	return false
}

// isBlacklistedStockName 名称判断：ST/*ST/S*ST 风险警示、退市整理期标的
func isBlacklistedStockName(name string) bool {
	n := strings.ToUpper(strings.TrimSpace(name))
	if strings.HasPrefix(n, "S*ST") || strings.HasPrefix(n, "*ST") || strings.HasPrefix(n, "ST") {
		return true
	}
	return strings.HasSuffix(n, "退")
}

// blacklistReason 返回暴雷原因描述（用于风险提示文案）
func blacklistReason(stock *quant.BaseStock) string {
	if stock == nil {
		return "暴雷风险"
	}
	var reasons []string
	if stock.Name != nil && isBlacklistedStockName(*stock.Name) {
		reasons = append(reasons, "ST/退市风险警示")
	}
	if stock.ListStatus != nil {
		switch strings.ToUpper(strings.TrimSpace(*stock.ListStatus)) {
		case "D":
			reasons = append(reasons, "已退市")
		case "P":
			reasons = append(reasons, "暂停上市")
		}
	}
	if stock.DelistDate != nil && !stock.DelistDate.IsZero() {
		reasons = append(reasons, fmt.Sprintf("已列入退市安排(退市日期 %s)", stock.DelistDate.Format("2006-01-02")))
	}
	if stock.Risk != nil {
		for _, kw := range blacklistFraudKeywords {
			if strings.Contains(*stock.Risk, kw) {
				reasons = append(reasons, "财务造假/重大违规")
				break
			}
		}
	}
	if len(reasons) == 0 {
		return "暴雷风险"
	}
	return strings.Join(reasons, "、")
}

func int8Ptr(v int8) *int8 { return &v }
