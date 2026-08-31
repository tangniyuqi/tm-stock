package quant

import (
	"github.com/gin-gonic/gin"
)

type UpdateRouter struct{}

// InitUpdateRouter 初始化 Update 路由信息
func (s *UpdateRouter) InitUpdateRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	updateRouter := quantRouter.Group("update")
	{
		updateRouter.GET("client", updateApi.ClientUpdate)
	}
}
