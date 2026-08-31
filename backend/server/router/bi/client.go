package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ClientRouter struct{}

// InitClientRouter 初始化 客户端 路由信息
func (s *ClientRouter) InitClientRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	clientRouter := biRouter.Group("client").Use(middleware.OperationRecord())
	clientRouterWithoutRecord := biRouter.Group("client")
	clientRouterWithoutAuth := biRouterWithoutAuth.Group("client")
	{
		clientRouter.POST("createClient", clientApi.CreateClient)             // 新建客户端
		clientRouter.DELETE("deleteClient", clientApi.DeleteClient)           // 删除客户端
		clientRouter.DELETE("deleteClientByIds", clientApi.DeleteClientByIds) // 批量删除客户端
		clientRouter.PUT("updateClient", clientApi.UpdateClient)              // 更新客户端
	}
	{
		clientRouterWithoutRecord.GET("findClient", clientApi.FindClient)       // 根据ID获取客户端
		clientRouterWithoutRecord.GET("getClientList", clientApi.GetClientList) // 获取客户端列表
	}
	{
		clientRouterWithoutAuth.GET("getClientPublic", clientApi.GetClientPublic) // 客户端开放接口
	}
}
