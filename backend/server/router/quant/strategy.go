package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type StrategyRouter struct{}

// InitStrategyRouter 初始化 策略 路由信息
func (s *StrategyRouter) InitStrategyRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	strategyRouter := quantRouter.Group("strategy").Use(middleware.OperationRecord())
	strategyRouterWithoutRecord := quantRouter.Group("strategy")
	strategyRouterWithoutAuth := quantRouterWithoutAuth.Group("strategy")
	{
		strategyRouter.POST("createStrategy", strategyApi.CreateStrategy)             // 新建策略
		strategyRouter.DELETE("deleteStrategy", strategyApi.DeleteStrategy)           // 删除策略
		strategyRouter.DELETE("deleteStrategyByIds", strategyApi.DeleteStrategyByIds) // 批量删除策略
		strategyRouter.PUT("updateStrategy", strategyApi.UpdateStrategy)              // 更新策略
	}
	{
		strategyRouterWithoutRecord.GET("findStrategy", strategyApi.FindStrategy)       // 根据ID获取策略
		strategyRouterWithoutRecord.GET("getStrategyList", strategyApi.GetStrategyList) // 获取策略列表
	}
	{
		strategyRouterWithoutAuth.GET("getStrategyPublic", strategyApi.GetStrategyPublic) // 策略开放接口
	}
}
