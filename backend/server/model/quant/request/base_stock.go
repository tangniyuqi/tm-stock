package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type BaseStockSearch struct {
	TsCode     *string    `json:"ts_code" form:"ts_code"`
	Symbol     *string    `json:"symbol" form:"symbol"`
	Name       *string    `json:"name" form:"name"`
	Area       *string    `json:"area" form:"area"`
	Industry   *string    `json:"industry" form:"industry"`
	Fullname   *string    `json:"fullname" form:"fullname"`
	Enname     *string    `json:"enname" form:"enname"`
	Cnspell    *string    `json:"cnspell" form:"cnspell"`
	Market     *string    `json:"market" form:"market"`
	Exchange   *string    `json:"exchange" form:"exchange"`
	CurrType   *string    `json:"curr_type" form:"curr_type"`
	ListStatus *string    `json:"list_status" form:"list_status"`
	ListDate   *time.Time `json:"list_date" form:"list_date"`
	DelistDate *time.Time `json:"delist_date" form:"delist_date"`
	IsHs       *string    `json:"is_hs" form:"is_hs"`
	ActName    *string    `json:"act_name" form:"act_name"`
	ActEntType *string    `json:"act_ent_type" form:"act_ent_type"`

	Q *string `json:"q" form:"q"`
	request.PageInfo
}

// AiAnalyzeStockReq AI 自动分析股票请求
type AiAnalyzeStockReq struct {
	StockIds     []int64    `json:"stock_ids" form:"stock_ids"`         // 待分析股票ID列表
	Provider     string     `json:"provider" form:"provider"`           // 大模型厂商: deepseek/doubao/qwen/kimi/zhipu/minimax/claude/gpt/gemini
	Model        string     `json:"model" form:"model"`                 // 模型ID（如 deepseek-v4-flash），留空使用厂商默认模型
	ApiKey       string     `json:"api_key" form:"api_key"`             // API密钥
	WebSearch    *bool      `json:"web_search" form:"web_search"`       // 是否启用联网搜索（可选；不传则跟随模型配置 web-search，默认关闭；开启后自动检索最新新闻供大模型参考）
	ApiFormat    string     `json:"api_format" form:"api_format"`       // 接口模式（可选；responses=Responses API，chat-completions=OpenAI 兼容接口；留空跟随模型/厂商/全局 ai.responses-api 配置）
	SearchEngine string     `json:"search_engine" form:"search_engine"` // 联网搜索引擎（chat-completions 模式且开启联网搜索时生效；baidu 默认，可选 tavily/serper/bocha 等，与 config.yaml 的 web-search 配置节保持一致）
	ScheduledAt  *time.Time `json:"scheduled_at" form:"scheduled_at"`   // 计划执行时间（可选；不传或传空则立即执行）
}
