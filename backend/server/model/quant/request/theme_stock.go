package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

// ThemeStockSearch 用于分页和查询题材-股票关联
type ThemeStockSearch struct {
	ID             *int64      `json:"id" form:"id"`
	ThemeId        *int32      `json:"theme_id" form:"theme_id"`
	StockId        *int64      `json:"stock_id" form:"stock_id"`
	Reason         *string     `json:"reason" form:"reason"`
	Status         *int        `json:"status" form:"status"`
	Sort           *int32      `json:"sort" form:"sort"`
	Tier           *int32      `json:"tier" form:"tier"`
	CreatedAtRange []time.Time `json:"createdAtRange" form:"createdAtRange[]"`
	OrderKey       string      `json:"orderKey" form:"orderKey"` // 排序字段
	Desc           bool        `json:"desc" form:"desc"`         // 是否倒序
	request.PageInfo
}

// AiAddThemeStockReq AI智能添加题材股票请求
type AiAddThemeStockReq struct {
	ThemeId       int32      `json:"theme_id" form:"theme_id"`             // 概念/题材ID
	MaxStocks     int        `json:"max_stocks" form:"max_stocks"`         // 最大股票数
	InDate        time.Time  `json:"in_date" form:"in_date"`               // 纳入日期
	Provider      string     `json:"provider" form:"provider"`             // 大模型厂商: deepseek/doubao/qwen/kimi/zhipu/minimax
	Model         string     `json:"model" form:"model"`                   // 模型ID（如 deepseek-v4-flash），留空使用厂商默认模型
	ApiKey        string     `json:"api_key" form:"api_key"`               // API密钥
	WebSearch     *bool      `json:"web_search" form:"web_search"`         // 是否启用联网搜索（可选；不传则跟随模型配置 web-search，默认关闭）
	ApiFormat     string     `json:"api_format" form:"api_format"`         // 接口模式（可选；responses=Responses API，chat-completions=OpenAI 兼容接口；留空跟随模型/厂商/全局 ai.responses-api 配置）
	SearchEngine  string     `json:"search_engine" form:"search_engine"`   // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效；baidu 默认，可选 tavily/serper/bocha 等，与 config.yaml 的 web-search 配置节保持一致）
	ExistHandling string     `json:"exist_handling" form:"exist_handling"` // 已存在股票处理方式: overwrite=覆盖(默认) skip=跳过
	MinRelevance  float64    `json:"min_relevance" form:"min_relevance"`   // 相关度过滤阈值：AI返回的相关度低于该值不入库，<=0 时默认 60
	ScheduledAt   *time.Time `json:"scheduled_at" form:"scheduled_at"`     // 计划执行时间（可选；不传或传空则立即执行）
}

// AiUpdateThemeStockReq AI智能更新题材股票请求
type AiUpdateThemeStockReq struct {
	ID           uint       `json:"id" form:"id"`                       // 题材股票记录ID
	Provider     string     `json:"provider" form:"provider"`           // 大模型厂商: deepseek/doubao/qwen/kimi/zhipu/minimax/claude/gpt/gemini
	Model        string     `json:"model" form:"model"`                 // 模型ID（如 deepseek-v4-flash），留空使用厂商默认模型
	ApiKey       string     `json:"api_key" form:"api_key"`             // API密钥
	WebSearch    *bool      `json:"web_search" form:"web_search"`       // 是否启用联网搜索（可选；不传则跟随模型配置 web-search，默认关闭；开启后自动检索最新新闻供大模型参考）
	ApiFormat    string     `json:"api_format" form:"api_format"`       // 接口模式（可选；responses=Responses API，chat-completions=OpenAI 兼容接口；留空跟随模型/厂商/全局 ai.responses-api 配置）
	SearchEngine string     `json:"search_engine" form:"search_engine"` // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效；baidu 默认，可选 tavily/serper/bocha 等，与 config.yaml 的 web-search 配置节保持一致）
	ScheduledAt  *time.Time `json:"scheduled_at" form:"scheduled_at"`   // 计划执行时间（可选；不传或传空则立即执行）
}

// AiUpdateThemeStocksReq AI智能批量更新题材股票请求
type AiUpdateThemeStocksReq struct {
	IDs          []uint     `json:"ids" form:"ids"`                     // 题材股票记录ID列表
	Provider     string     `json:"provider" form:"provider"`           // 大模型厂商: deepseek/doubao/qwen/kimi/zhipu/minimax/claude/gpt/gemini
	Model        string     `json:"model" form:"model"`                 // 模型ID（如 deepseek-v4-flash），留空使用厂商默认模型
	ApiKey       string     `json:"api_key" form:"api_key"`             // API密钥
	WebSearch    *bool      `json:"web_search" form:"web_search"`       // 是否启用联网搜索（可选；不传则跟随模型配置 web-search，默认关闭；开启后自动检索最新新闻供大模型参考）
	ApiFormat    string     `json:"api_format" form:"api_format"`       // 接口模式（可选；responses=Responses API，chat-completions=OpenAI 兼容接口；留空跟随模型/厂商/全局 ai.responses-api 配置）
	SearchEngine string     `json:"search_engine" form:"search_engine"` // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效；baidu 默认，可选 tavily/serper/bocha 等，与 config.yaml 的 web-search 配置节保持一致）
	ScheduledAt  *time.Time `json:"scheduled_at" form:"scheduled_at"`   // 计划执行时间（可选；不传或传空则立即执行）
}

// AiUpdateThemeStockResult AI批量更新单只题材股票结果
type AiUpdateThemeStockResult struct {
	ID        uint   `json:"id"`         // 题材股票记录ID
	StockName string `json:"stock_name"` // 股票名称
	Success   bool   `json:"success"`    // 是否更新成功
	Message   string `json:"message"`    // 结果描述（成功/失败原因）
}
