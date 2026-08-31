package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ConfigRouter struct{}

// InitConfigRouter 初始化 配置 路由信息
func (s *ConfigRouter) InitConfigRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	configRouter := quantRouter.Group("config").Use(middleware.OperationRecord())
	configRouterWithoutRecord := quantRouter.Group("config")
	configRouterWithoutAuth := quantRouterWithoutAuth.Group("config")
	{
		configRouter.POST("createConfig", configApi.CreateConfig)             // 新建配置
		configRouter.DELETE("deleteConfig", configApi.DeleteConfig)           // 删除配置
		configRouter.DELETE("deleteConfigByIds", configApi.DeleteConfigByIds) // 批量删除配置
		configRouter.PUT("updateConfig", configApi.UpdateConfig)              // 更新配置
	}
	{
		configRouterWithoutRecord.GET("findConfig", configApi.FindConfig)       // 根据ID获取配置
		configRouterWithoutRecord.GET("getConfigList", configApi.GetConfigList) // 获取配置列表
	}
	{
		configRouterWithoutAuth.GET("getConfigPublic", configApi.GetConfigPublic) // 配置开放接口
	}
}
