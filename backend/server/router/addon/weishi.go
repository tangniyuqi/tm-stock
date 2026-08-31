package addon

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type WeishiRouter struct{}

func (s *WeishiRouter) InitWeishiRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	weishiRouter := Router.Group("weishi").Use(middleware.OperationRecord())
	weishiRouterWithoutRecord := Router.Group("weishi")
	weishiRouterWithoutAuth := PublicRouter.Group("weishi")
	{
		weishiRouter.POST("createWeishi", weishiApi.CreateWeishi)
		weishiRouter.DELETE("deleteWeishi", weishiApi.DeleteWeishi)
		weishiRouter.DELETE("deleteWeishiByIds", weishiApi.DeleteWeishiByIds)
		weishiRouter.PUT("updateWeishi", weishiApi.UpdateWeishi)
		weishiRouter.POST("batchCreateWeishi", weishiApi.BatchCreateWeishi)
		weishiRouter.POST("cancelWeishi", weishiApi.CancelWeishi)
	}
	{
		weishiRouterWithoutRecord.GET("findWeishi", weishiApi.FindWeishi)
		weishiRouterWithoutRecord.GET("getWeishiList", weishiApi.GetWeishiList)
	}
	{
		weishiRouterWithoutAuth.GET("getWeishiPublic", weishiApi.GetWeishiPublic)
	}
}
