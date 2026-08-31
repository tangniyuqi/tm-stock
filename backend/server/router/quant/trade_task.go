package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type TradeTaskRouter struct{}

// InitTradeTaskRouter 初始化 交易任务 路由信息
func (s *TradeTaskRouter) InitTradeTaskRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	tradeTaskRouter := quantRouter.Group("tradeTask").Use(middleware.OperationRecord())
	tradeTaskRouterWithoutRecord := quantRouter.Group("tradeTask")
	tradeTaskRouterWithoutAuth := quantRouterWithoutAuth.Group("tradeTask")
	{
		tradeTaskRouter.POST("createTradeTask", tradeTaskApi.CreateTradeTask)             // 新建交易任务
		tradeTaskRouter.DELETE("deleteTradeTask", tradeTaskApi.DeleteTradeTask)           // 删除交易任务
		tradeTaskRouter.DELETE("deleteTradeTaskByIds", tradeTaskApi.DeleteTradeTaskByIds) // 批量删除交易任务
		tradeTaskRouter.PUT("updateTradeTask", tradeTaskApi.UpdateTradeTask)              // 更新交易任务
	}
	{
		tradeTaskRouterWithoutRecord.GET("findTradeTask", tradeTaskApi.FindTradeTask)       // 根据ID获取交易任务
		tradeTaskRouterWithoutRecord.GET("getTradeTaskList", tradeTaskApi.GetTradeTaskList) // 获取交易任务列表
	}
	{
		tradeTaskRouterWithoutAuth.GET("getTradeTaskPublic", tradeTaskApi.GetTradeTaskPublic) // 交易任务开放接口
	}
}
