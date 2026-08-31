package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ThemeTopicRouter struct{}

// InitThemeTopicRouter 初始化 题材话题 路由信息
func (s *ThemeTopicRouter) InitThemeTopicRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	themeTopicRouter := quantRouter.Group("themeTopic").Use(middleware.OperationRecord())
	themeTopicRouterWithoutRecord := quantRouter.Group("themeTopic")
	themeTopicRouterWithoutAuth := quantRouterWithoutAuth.Group("themeTopic")
	{
		themeTopicRouter.POST("createThemeTopic", themeTopicApi.CreateThemeTopic)             // 新建题材话题
		themeTopicRouter.DELETE("deleteThemeTopic", themeTopicApi.DeleteThemeTopic)           // 删除题材话题
		themeTopicRouter.DELETE("deleteThemeTopicByIds", themeTopicApi.DeleteThemeTopicByIds) // 批量删除题材话题
		themeTopicRouter.PUT("updateThemeTopic", themeTopicApi.UpdateThemeTopic)              // 更新题材话题
	}
	{
		themeTopicRouterWithoutRecord.GET("findThemeTopic", themeTopicApi.FindThemeTopic)       // 根据ID获取题材话题
		themeTopicRouterWithoutRecord.GET("getThemeTopicList", themeTopicApi.GetThemeTopicList) // 获取题材话题列表
	}
	{
		themeTopicRouterWithoutAuth.GET("find", themeTopicApi.FindThemeTopic)                     // 根据ID获取题材话题(无需鉴权)
		themeTopicRouterWithoutAuth.GET("list", themeTopicApi.GetThemeTopicList)                  // 获取题材话题列表(无需鉴权)
		themeTopicRouterWithoutAuth.GET("getThemeTopicPublic", themeTopicApi.GetThemeTopicPublic) // 题材话题开放接口
	}
}
