package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type DealRouter struct{}

// InitDealRouter 初始化 商机 路由信息
func (s *DealRouter) InitDealRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cloudRouter := Router.Group("cloud")
	cloudRouterWithoutAuth := PublicRouter.Group("cloud")

	dealRouter := cloudRouter.Group("deal").Use(middleware.OperationRecord())
	dealRouterWithoutRecord := cloudRouter.Group("deal")
	dealRouterWithoutAuth := cloudRouterWithoutAuth.Group("deal")
	{
		dealRouter.POST("createDeal", dealApi.CreateDeal)             // 新建商机
		dealRouter.DELETE("deleteDeal", dealApi.DeleteDeal)           // 删除商机
		dealRouter.DELETE("deleteDealByIds", dealApi.DeleteDealByIds) // 批量删除商机
		dealRouter.PUT("updateDeal", dealApi.UpdateDeal)              // 更新商机
	}
	{
		dealRouterWithoutRecord.GET("findDeal", dealApi.FindDeal)       // 根据ID获取商机
		dealRouterWithoutRecord.GET("getDealList", dealApi.GetDealList) // 获取商机列表
	}
	{
		dealRouterWithoutAuth.GET("getDealPublic", dealApi.GetDealPublic) // 商机开放接口
	}
}
