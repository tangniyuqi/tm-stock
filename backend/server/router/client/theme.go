package client

import (
	"github.com/gin-gonic/gin"
)

// ThemeRouter 注册 C 端题材公开路由（无需登录）
type ThemeRouter struct{}

// InitThemeRouter 注册题材公开路由
func (s *ThemeRouter) InitThemeRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	themeRouter := clientRouter.Group("theme")
	{
		themeRouter.GET("treasure", themeApi.Treasure) // 题材宝典概览
		themeRouter.GET("detail", themeApi.Detail)     // 题材详情
	}
}
