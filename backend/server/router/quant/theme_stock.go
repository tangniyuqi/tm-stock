package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ThemeStockRouter struct{}

// InitThemeStockRouter 初始化 题材股票 路由信息
func (s *ThemeStockRouter) InitThemeStockRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	themeStockRouter := quantRouter.Group("themeStock").Use(middleware.OperationRecord())
	themeStockRouterWithoutRecord := quantRouter.Group("themeStock")
	themeStockRouterWithoutAuth := quantRouterWithoutAuth.Group("themeStock")
	{
		themeStockRouter.POST("createThemeStock", themeStockApi.CreateThemeStock)             // 新建题材股票
		themeStockRouter.POST("aiAddThemeStocks", themeStockApi.AiAddThemeStocks)             // AI智能添加题材股票
		themeStockRouter.POST("aiUpdateThemeStock", themeStockApi.AiUpdateThemeStock)         // AI智能更新题材股票
		themeStockRouter.POST("aiUpdateThemeStocks", themeStockApi.AiUpdateThemeStocks)       // AI智能批量更新题材股票
		themeStockRouter.DELETE("deleteThemeStock", themeStockApi.DeleteThemeStock)           // 删除题材股票
		themeStockRouter.DELETE("deleteThemeStockByIds", themeStockApi.DeleteThemeStockByIds) // 批量删除题材股票
		themeStockRouter.PUT("updateThemeStock", themeStockApi.UpdateThemeStock)              // 更新题材股票
	}
	{
		themeStockRouterWithoutRecord.GET("findThemeStock", themeStockApi.FindThemeStock)       // 根据ID获取题材股票
		themeStockRouterWithoutRecord.GET("getThemeStockList", themeStockApi.GetThemeStockList) // 获取题材股票列表
	}
	{
		themeStockRouterWithoutAuth.GET("getThemeStockPublic", themeStockApi.GetThemeStockPublic) // 题材股票开放接口
		themeStockRouterWithoutAuth.GET("find", themeStockApi.FindThemeStock)                     // 根据ID获取题材股票（开放接口）
		themeStockRouterWithoutAuth.GET("list", themeStockApi.GetThemeStockList)                  // 获取题材股票列表（开放接口）
	}
}
