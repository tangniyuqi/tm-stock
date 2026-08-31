package quant

type ServiceGroup struct {
	AccountService
	ConfigService
	NewsService
	StrategyService
	TradeRecordService
	NewsKeywordService
	EventService
	ReportService
	BaseStockService
	TradeTaskService
	UpdateService
	WencaiService
	ScreenerRecordService
	ThemeService
	ThemeTopicService
	ThemeStockService
	AiTaskService
}

// aiTaskService 包内直接使用的任务服务实例
// （service/quant 无法引用外层 service 包以避免循环依赖，各服务均为无状态结构体，可直接实例化）
var aiTaskService = new(AiTaskService)

// themeStockService / baseStockService 包内直接使用的业务服务实例
// （供 AiTaskService 重启任务等跨服务调用使用，模式与 aiTaskService 一致）
var themeStockService = new(ThemeStockService)
var baseStockService = new(BaseStockService)
