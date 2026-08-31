package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type GroupRouter struct{}

// InitGroupRouter 初始化 群组 路由信息
func (s *GroupRouter) InitGroupRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	groupRouter := biRouter.Group("group").Use(middleware.OperationRecord())
	groupRouterWithoutRecord := biRouter.Group("group")
	groupRouterWithoutAuth := biRouterWithoutAuth.Group("group")
	{
		groupRouter.POST("createGroup", groupApi.CreateGroup)             // 新建群组
		groupRouter.DELETE("deleteGroup", groupApi.DeleteGroup)           // 删除群组
		groupRouter.DELETE("deleteGroupByIds", groupApi.DeleteGroupByIds) // 批量删除群组
		groupRouter.PUT("updateGroup", groupApi.UpdateGroup)              // 更新群组
	}
	{
		groupRouterWithoutRecord.GET("findGroup", groupApi.FindGroup)       // 根据ID获取群组
		groupRouterWithoutRecord.GET("getGroupList", groupApi.GetGroupList) // 获取群组列表
	}
	{
		groupRouterWithoutAuth.GET("getGroupPublic", groupApi.GetGroupPublic) // 群组开放接口
	}
}
