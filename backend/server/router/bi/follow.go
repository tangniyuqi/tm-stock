package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type FollowRouter struct{}

// InitFollowRouter 初始化 跟进 路由信息
func (s *FollowRouter) InitFollowRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	followRouter := biRouter.Group("follow").Use(middleware.OperationRecord())
	followRouterWithoutRecord := biRouter.Group("follow")
	followRouterWithoutAuth := biRouterWithoutAuth.Group("follow")
	{
		followRouter.POST("createFollow", followApi.CreateFollow)             // 新建跟进
		followRouter.DELETE("deleteFollow", followApi.DeleteFollow)           // 删除跟进
		followRouter.DELETE("deleteFollowByIds", followApi.DeleteFollowByIds) // 批量删除跟进
		followRouter.PUT("updateFollow", followApi.UpdateFollow)              // 更新跟进
	}
	{
		followRouterWithoutRecord.GET("findFollow", followApi.FindFollow)       // 根据ID获取跟进
		followRouterWithoutRecord.GET("getFollowList", followApi.GetFollowList) // 获取跟进列表
	}
	{
		followRouterWithoutAuth.GET("getFollowPublic", followApi.GetFollowPublic) // 跟进开放接口
	}
}
