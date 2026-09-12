package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type AdRouter struct{}

// InitAdRouter 初始化 广告表 路由信息
func (s *AdRouter) InitAdRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cmsGroup := Router.Group("cms")
	adRouter := cmsGroup.Group("ad").Use(middleware.OperationRecord())
	adRouterWithoutRecord := cmsGroup.Group("ad")
	adRouterWithoutAuth := PublicRouter.Group("cms").Group("ad")
	{
		adRouter.POST("createAd", adApi.CreateAd)             // 新建广告表
		adRouter.DELETE("deleteAd", adApi.DeleteAd)           // 删除广告表
		adRouter.DELETE("deleteAdByIds", adApi.DeleteAdByIds) // 批量删除广告表
		adRouter.PUT("updateAd", adApi.UpdateAd)              // 更新广告表
	}
	{
		adRouterWithoutRecord.GET("findAd", adApi.FindAd)       // 根据ID获取广告表
		adRouterWithoutRecord.GET("getAdList", adApi.GetAdList) // 获取广告表列表
	}
	{
		adRouterWithoutAuth.GET("getAdPublic", adApi.GetAdPublic) // 广告表开放接口
	}
}
