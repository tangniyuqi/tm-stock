package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ThemeRouter struct{}

// InitThemeRouter 初始化 题材 路由信息
func (s *ThemeRouter) InitThemeRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	themeRouter := quantRouter.Group("theme").Use(middleware.OperationRecord())
	themeRouterWithoutRecord := quantRouter.Group("theme")
	themeRouterWithoutAuth := quantRouterWithoutAuth.Group("theme")
	{
		themeRouter.POST("createTheme", themeApi.CreateTheme)             // 新建题材
		themeRouter.DELETE("deleteTheme", themeApi.DeleteTheme)           // 删除题材
		themeRouter.DELETE("deleteThemeByIds", themeApi.DeleteThemeByIds) // 批量删除题材
		themeRouter.PUT("updateTheme", themeApi.UpdateTheme)              // 更新题材
	}
	{
		themeRouterWithoutRecord.GET("findTheme", themeApi.FindTheme)       // 根据ID获取题材
		themeRouterWithoutRecord.GET("getThemeList", themeApi.GetThemeList) // 获取题材列表
	}
	{
		themeRouterWithoutAuth.GET("getThemeListWithoutChildren", themeApi.GetThemeListWithoutChildren) // 根据层级获取题材列表
		themeRouterWithoutAuth.GET("getThemeWithChildren", themeApi.GetThemeWithChildren)               // 根据ID获取题材及其子节点
		themeRouterWithoutAuth.GET("getThemePublic", themeApi.GetThemePublic)                           // 题材开放接口
	}
}
