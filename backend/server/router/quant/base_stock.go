package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type BaseStockRouter struct{}

// InitBaseStockRouter 初始化 基础股票 路由信息
func (s *BaseStockRouter) InitBaseStockRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	baseStockRouter := quantRouter.Group("baseStock").Use(middleware.OperationRecord())
	baseStockRouterWithoutRecord := quantRouter.Group("baseStock")
	baseStockRouterWithoutAuth := quantRouterWithoutAuth.Group("baseStock")
	{
		baseStockRouter.POST("createBaseStock", baseStockApi.CreateBaseStock)             // 新建基础股票
		baseStockRouter.DELETE("deleteBaseStock", baseStockApi.DeleteBaseStock)           // 删除基础股票
		baseStockRouter.DELETE("deleteBaseStockByIds", baseStockApi.DeleteBaseStockByIds) // 批量删除基础股票
		baseStockRouter.PUT("updateBaseStock", baseStockApi.UpdateBaseStock)              // 更新基础股票
		baseStockRouter.GET("sync", baseStockApi.Sync)                                    // 同步基础股票数据
		baseStockRouter.POST("updateAllChangePct", baseStockApi.UpdateAllChangePct)       // 一键更新全部股票涨跌幅
		baseStockRouter.POST("aiAnalyzeStocks", baseStockApi.AiAnalyzeStocks)             // AI自动分析股票
		baseStockRouter.DELETE("clear", baseStockApi.Clear)                               // 清除全部基础股票数据
	}
	{
		baseStockRouterWithoutRecord.GET("findBaseStock", baseStockApi.FindBaseStock)       // 根据ID获取基础股票
		baseStockRouterWithoutRecord.GET("getBaseStockList", baseStockApi.GetBaseStockList) // 获取基础股票列表
	}
	{
		baseStockRouterWithoutAuth.GET("getBaseStockPublic", baseStockApi.GetBaseStockPublic) // 基础股票开放接口
	}
}
