package quant

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

type RouterGroup struct {
	AccountRouter
	ConfigRouter
	NewsRouter
	StrategyRouter
	TradeRecordRouter
	NewsKeywordRouter
	EventRouter
	ReportRouter
	BaseStockRouter
	TradeTaskRouter
	UpdateRouter
	WencaiRouter
	ScreenerRecordRouter
	ThemeRouter
	ThemeTopicRouter
	ThemeStockRouter
	AiTaskRouter
}

var (
	accountApi        = api.ApiGroupApp.QuantApiGroup.AccountApi
	configApi         = api.ApiGroupApp.QuantApiGroup.ConfigApi
	newsApi           = api.ApiGroupApp.QuantApiGroup.NewsApi
	strategyApi       = api.ApiGroupApp.QuantApiGroup.StrategyApi
	tradeRecordApi    = api.ApiGroupApp.QuantApiGroup.TradeRecordApi
	newsKeywordApi    = api.ApiGroupApp.QuantApiGroup.NewsKeywordApi
	eventApi          = api.ApiGroupApp.QuantApiGroup.EventApi
	reportApi         = api.ApiGroupApp.QuantApiGroup.ReportApi
	baseStockApi      = api.ApiGroupApp.QuantApiGroup.BaseStockApi
	tradeTaskApi      = api.ApiGroupApp.QuantApiGroup.TradeTaskApi
	updateApi         = api.ApiGroupApp.QuantApiGroup.UpdateApi
	wencaiApi         = api.ApiGroupApp.QuantApiGroup.WencaiApi
	screenerRecordApi = api.ApiGroupApp.QuantApiGroup.ScreenerRecordApi
	themeApi          = api.ApiGroupApp.QuantApiGroup.ThemeApi
	themeTopicApi     = api.ApiGroupApp.QuantApiGroup.ThemeTopicApi
	themeStockApi     = api.ApiGroupApp.QuantApiGroup.ThemeStockApi
	aiTaskApi         = api.ApiGroupApp.QuantApiGroup.AiTaskApi
)
