package quant

import "github.com/flipped-aurora/gin-vue-admin/server/service"

type ApiGroup struct {
	AccountApi
	ConfigApi
	NewsApi
	StrategyApi
	TradeRecordApi
	NewsKeywordApi
	EventApi
	ReportApi
	BaseStockApi
	TradeTaskApi
	UpdateApi
	WencaiApi
	ScreenerRecordApi
	ThemeApi
	ThemeTopicApi
	ThemeStockApi
	AiTaskApi
}

var (
	accountService        = service.ServiceGroupApp.QuantServiceGroup.AccountService
	configService         = service.ServiceGroupApp.QuantServiceGroup.ConfigService
	newsService           = service.ServiceGroupApp.QuantServiceGroup.NewsService
	strategyService       = service.ServiceGroupApp.QuantServiceGroup.StrategyService
	tradeRecordService    = service.ServiceGroupApp.QuantServiceGroup.TradeRecordService
	newsKeywordService    = service.ServiceGroupApp.QuantServiceGroup.NewsKeywordService
	eventService          = service.ServiceGroupApp.QuantServiceGroup.EventService
	reportService         = service.ServiceGroupApp.QuantServiceGroup.ReportService
	baseStockService      = service.ServiceGroupApp.QuantServiceGroup.BaseStockService
	tradeTaskService      = service.ServiceGroupApp.QuantServiceGroup.TradeTaskService
	updateService         = service.ServiceGroupApp.QuantServiceGroup.UpdateService
	wencaiService         = service.ServiceGroupApp.QuantServiceGroup.WencaiService
	screenerRecordService = service.ServiceGroupApp.QuantServiceGroup.ScreenerRecordService
	themeService          = service.ServiceGroupApp.QuantServiceGroup.ThemeService
	themeTopicService     = service.ServiceGroupApp.QuantServiceGroup.ThemeTopicService
	themeStockService     = service.ServiceGroupApp.QuantServiceGroup.ThemeStockService
	aiTaskService         = service.ServiceGroupApp.QuantServiceGroup.AiTaskService
)
