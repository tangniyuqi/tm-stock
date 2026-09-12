package client

import (
	"github.com/gin-gonic/gin"
)

// NewsRouter 注册 C 端 7x24 快讯公开路由（无需登录）
type NewsRouter struct{}

// InitNewsRouter 注册 7x24 快讯公开路由
func (s *NewsRouter) InitNewsRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	newsRouter := clientRouter.Group("quant").Group("news")
	{
		newsRouter.GET("list", newsApi.List)     // 7x24 快讯列表
		newsRouter.GET("detail", newsApi.Detail) // 7x24 快讯详情
	}
}
