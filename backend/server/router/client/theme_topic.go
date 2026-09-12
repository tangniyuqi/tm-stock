package client

import (
	"github.com/gin-gonic/gin"
)

// ThemeTopicRouter 注册 C 端题材动态公开路由（无需登录）
type ThemeTopicRouter struct{}

// InitThemeTopicRouter 注册题材动态公开路由
func (s *ThemeTopicRouter) InitThemeTopicRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	themeTopicRouter := clientRouter.Group("quant").Group("themeTopic")
	{
		themeTopicRouter.GET("list", themeTopicApi.List)     // 题材动态列表
		themeTopicRouter.GET("detail", themeTopicApi.Detail) // 题材动态详情
	}
}
