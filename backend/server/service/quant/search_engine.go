package quant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/request"
	"go.uber.org/zap"
)

// 支持的联网搜索引擎 key（chat/completions 接口模式下 AI 联网检索使用）
// 与 config.yaml 的 web-search 配置节、前端 web/src/data/webSearchOptions.js 的 value 保持一致；
// 默认引擎为 baidu（百度搜索）
const (
	SearchEngineBaidu  = "baidu"  // 百度搜索（默认引擎）
	SearchEngineBocha  = "bocha"  // 博查AI 搜索
	SearchEngineAzure  = "azure"  // Azure AI 搜索（暂未实现，选择后回退默认引擎）
	SearchEngineTavily = "tavily" // Tavily 搜索
	SearchEngineSerper = "serper" // Serper 谷歌搜索
)

// newsSearchItem 联网检索返回的单条资讯（各搜索引擎统一转换为该结构）
type newsSearchItem struct {
	Datetime string `json:"datetime"` // 资讯时间 YYYY-MM-DD HH:MM:SS
	Title    string `json:"title"`    // 资讯标题
	Content  string `json:"content"`  // 资讯内容/摘要
	Src      string `json:"src"`      // 资讯来源/链接
}

// resolveSearchEngine 解析本次调用使用的联网搜索引擎：请求参数优先，未指定时默认 baidu（百度搜索）
func resolveSearchEngine(reqEngine string) string {
	engine := strings.ToLower(strings.TrimSpace(reqEngine))
	if engine == "" {
		return SearchEngineBaidu
	}
	return engine
}

// aiSearchNewsContext 按指定搜索引擎联网检索与关键词相关的资讯，返回可拼入 prompt 的参考资料文本
// 检索失败或无结果时返回空字符串，不阻断 AI 分析流程
func aiSearchNewsContext(ctx context.Context, engine, keyword string, limit int) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || limit <= 0 {
		return ""
	}
	items := searchNewsWithEngine(ctx, engine, keyword, limit)
	var sb strings.Builder
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		content := strings.TrimSpace(item.Content)
		if title == "" && content == "" {
			continue
		}
		sb.WriteString("- ")
		if item.Datetime != "" {
			sb.WriteString("[")
			sb.WriteString(item.Datetime)
			sb.WriteString("] ")
		}
		if title != "" {
			sb.WriteString(title)
		}
		if content != "" {
			sb.WriteString("：")
			sb.WriteString(truncateRunes(content, aiNewsContentLimit))
		}
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		return ""
	}
	return sb.String()
}

// searchNewsWithEngine 按指定引擎联网检索资讯（统一转换为 newsSearchItem 结构）
// 引擎未配置/检索失败/暂未实现时记录日志并回退默认引擎 baidu
func searchNewsWithEngine(ctx context.Context, engine, keyword string, limit int) []newsSearchItem {
	switch resolveSearchEngine(engine) {
	case SearchEngineTavily:
		items, err := searchTavilyNews(ctx, keyword, limit)
		if err != nil {
			global.GVA_LOG.Warn("Tavily 联网检索失败，回退默认引擎", zap.String("keyword", keyword), zap.Error(err))
		} else if len(items) > 0 {
			return items
		}
	case SearchEngineSerper:
		items, err := searchSerperNews(ctx, keyword, limit)
		if err != nil {
			global.GVA_LOG.Warn("Serper 联网检索失败，回退默认引擎", zap.String("keyword", keyword), zap.Error(err))
		} else if len(items) > 0 {
			return items
		}
	case SearchEngineBocha:
		items, err := searchBochaNews(ctx, keyword, limit)
		if err != nil {
			global.GVA_LOG.Warn("博查AI 联网检索失败，回退默认引擎", zap.String("keyword", keyword), zap.Error(err))
		} else if len(items) > 0 {
			return items
		}
	case SearchEngineAzure:
		global.GVA_LOG.Warn("Azure 搜索暂未实现，回退默认引擎", zap.String("keyword", keyword))
	}
	// 默认引擎：百度搜索
	items, err := searchBaiduNews(ctx, keyword, limit)
	if err != nil {
		global.GVA_LOG.Warn("百度搜索联网检索失败", zap.String("keyword", keyword), zap.Error(err))
		return nil
	}
	return items
}

// searchBaiduNews 通过百度千帆 AI 搜索 API 检索资讯（默认引擎）
// 接口文档: https://cloud.baidu.com/doc/qianfan-api/s/Wmbq4z7e5
// 请求: POST {base-url}  header: {"Authorization: Bearer <API Key>"}
// body: {"messages":[{"content","role"}],"search_source":"baidu_search_v2","resource_type_filter":[{"type":"web","top_k"}]}
// 响应: references 数组（title/url/snippet/content/date），需在 config.yaml 的 web-search.baidu 配置 api-key
func searchBaiduNews(ctx context.Context, keyword string, limit int) ([]newsSearchItem, error) {
	cfg, ok := global.GVA_CONFIG.WebSearch[SearchEngineBaidu]
	if !ok {
		return nil, fmt.Errorf("百度搜索未配置（config.yaml 的 web-search.baidu）")
	}
	apiKey, baseURL := strings.TrimSpace(cfg.ApiKey), strings.TrimSpace(cfg.BaseURL)
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("百度搜索未配置 api-key / base-url")
	}
	body, err := httpSearchJSON(ctx, baseURL, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, map[string]any{
		"messages": []map[string]string{
			{"content": keyword, "role": "user"},
		},
		"search_source": "baidu_search_v2",
		"resource_type_filter": []map[string]any{
			{"type": "web", "top_k": limit},
		},
	})
	if err != nil {
		return nil, err
	}
	var respBody struct {
		RequestID  string `json:"request_id"`
		Code       string `json:"code"`
		Message    string `json:"message"`
		References []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
			Content string `json:"content"`
			Date    string `json:"date"`
		} `json:"references"`
	}
	if err = json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("解析百度搜索响应失败: %w", err)
	}
	// 接口异常时 code 非空（成功时无 code 字段）；个别场景 code 为 "0" 视为成功
	if respBody.Code != "" && respBody.Code != "0" {
		return nil, fmt.Errorf("百度搜索接口错误: %s", respBody.Message)
	}
	items := make([]newsSearchItem, 0, len(respBody.References))
	for _, r := range respBody.References {
		content := r.Content
		if strings.TrimSpace(content) == "" {
			content = r.Snippet
		}
		items = append(items, newsSearchItem{
			Datetime: r.Date,
			Title:    r.Title,
			Content:  content,
			Src:      r.URL,
		})
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

// searchTavilyNews 通过 Tavily 搜索 API 检索资讯
// 请求: POST {base-url}  body: {"api_key","query","max_results","search_depth"}
func searchTavilyNews(ctx context.Context, keyword string, limit int) ([]newsSearchItem, error) {
	cfg, ok := global.GVA_CONFIG.WebSearch[SearchEngineTavily]
	if !ok {
		return nil, fmt.Errorf("tavily 未配置（config.yaml 的 web-search.tavily）")
	}
	apiKey, baseURL := strings.TrimSpace(cfg.ApiKey), strings.TrimSpace(cfg.BaseURL)
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("tavily 未配置 api-key / base-url")
	}
	body, err := httpSearchJSON(ctx, baseURL, map[string]string{}, map[string]any{
		"api_key":      apiKey,
		"query":        keyword,
		"max_results":  limit,
		"search_depth": "basic",
	})
	if err != nil {
		return nil, err
	}
	var respBody struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err = json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("解析 tavily 响应失败: %w", err)
	}
	items := make([]newsSearchItem, 0, len(respBody.Results))
	for _, r := range respBody.Results {
		items = append(items, newsSearchItem{
			Title:   r.Title,
			Content: r.Content,
			Src:     r.URL,
		})
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

// searchSerperNews 通过 Serper（Google 搜索）API 检索资讯
// 请求: POST {base-url}  header: {"X-API-KEY"}  body: {"q","gl","hl","num"}
func searchSerperNews(ctx context.Context, keyword string, limit int) ([]newsSearchItem, error) {
	cfg, ok := global.GVA_CONFIG.WebSearch[SearchEngineSerper]
	if !ok {
		return nil, fmt.Errorf("serper 未配置（config.yaml 的 web-search.serper）")
	}
	apiKey, baseURL := strings.TrimSpace(cfg.ApiKey), strings.TrimSpace(cfg.BaseURL)
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("serper 未配置 api-key / base-url")
	}
	body, err := httpSearchJSON(ctx, baseURL, map[string]string{
		"X-API-KEY": apiKey,
	}, map[string]any{
		"q":   keyword,
		"gl":  "cn",
		"hl":  "zh-cn",
		"num": limit,
	})
	if err != nil {
		return nil, err
	}
	var respBody struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	if err = json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("解析 serper 响应失败: %w", err)
	}
	items := make([]newsSearchItem, 0, len(respBody.Organic))
	for _, r := range respBody.Organic {
		items = append(items, newsSearchItem{
			Title:   r.Title,
			Content: r.Snippet,
			Src:     r.Link,
		})
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

// searchBochaNews 通过博查AI 搜索 API 检索资讯
// 请求: POST {base-url}（完整端点，如 https://api.bochaai.com/v1/web-search）  header: {"Authorization: Bearer"}  body: {"query","summary","count","freshness"}
func searchBochaNews(ctx context.Context, keyword string, limit int) ([]newsSearchItem, error) {
	cfg, ok := global.GVA_CONFIG.WebSearch[SearchEngineBocha]
	if !ok {
		return nil, fmt.Errorf("博查AI 未配置（config.yaml 的 web-search.bocha）")
	}
	apiKey, baseURL := strings.TrimSpace(cfg.ApiKey), strings.TrimSpace(cfg.BaseURL)
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("博查AI 未配置 api-key / base-url")
	}
	body, err := httpSearchJSON(ctx, baseURL, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, map[string]any{
		"query":     keyword,
		"summary":   true, // 返回长文本摘要，供大模型参考
		"count":     limit,
		"freshness": "oneMonth",
	})
	if err != nil {
		return nil, err
	}
	var respBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			WebPages struct {
				Value []struct {
					Name    string `json:"name"`
					URL     string `json:"url"`
					Snippet string `json:"snippet"`
					Summary string `json:"summary"`
				} `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("解析博查AI 响应失败: %w", err)
	}
	if respBody.Code != "" && respBody.Code != "0" {
		return nil, fmt.Errorf("博查AI 接口错误: %s", respBody.Message)
	}
	items := make([]newsSearchItem, 0, len(respBody.Data.WebPages.Value))
	for _, r := range respBody.Data.WebPages.Value {
		content := r.Summary
		if strings.TrimSpace(content) == "" {
			content = r.Snippet
		}
		items = append(items, newsSearchItem{
			Title:   r.Name,
			Content: content,
			Src:     r.URL,
		})
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

// httpSearchJSON 发起一次 JSON POST 联网检索请求并读取响应体
func httpSearchJSON(ctx context.Context, url string, headers map[string]string, payload map[string]any) ([]byte, error) {
	resp, err := request.HttpRequestWithContextAndTimeout(ctx, url, http.MethodPost, headers, nil, payload, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("发起联网检索请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取联网检索响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("联网检索接口返回异常(%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// truncateRunes 按字符数截断字符串，避免破坏 UTF-8 编码
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
