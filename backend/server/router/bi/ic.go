package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type IcRouter struct{}

// InitIcRouter 初始化 IC 路由信息
func (s *IcRouter) InitIcRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	icRouter := biRouter.Group("ic").Use(middleware.OperationRecord())
	icRouterWithoutRecord := biRouter.Group("ic")
	icRouterWithoutAuth := biRouterWithoutAuth.Group("ic")
	{
		icRouter.POST("createIc", icApi.CreateIc)             // 新建IC
		icRouter.DELETE("deleteIc", icApi.DeleteIc)           // 删除IC
		icRouter.DELETE("deleteIcByIds", icApi.DeleteIcByIds) // 批量删除IC
		icRouter.PUT("updateIc", icApi.UpdateIc)              // 更新IC
	}
	{
		icRouterWithoutRecord.GET("findIc", icApi.FindIc)       // 根据ID获取IC
		icRouterWithoutRecord.GET("getIcList", icApi.GetIcList) // 获取IC列表
	}
	{
		icRouterWithoutAuth.GET("getIcPublic", icApi.GetIcPublic) // IC开放接口
	}
}
