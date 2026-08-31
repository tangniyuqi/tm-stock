package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type WencaiRouter struct{}

func (s *WencaiRouter) InitWencaiRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	wencaiRouter := Router.Group("wencai").Use(middleware.OperationRecord())
	//wencaiRouterWithoutRecord := Router.Group("wencai")
	wencaiRouterWithoutAuth := PublicRouter.Group("wencai")

	_ = wencaiRouter
	{
	}
	{
		wencaiRouterWithoutAuth.GET("query", wencaiApi.QueryWencai) // 问财选股查询
	}
}
