package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type MemberRouter struct{}

// InitMemberRouter 初始化 用户 路由信息
func (s *MemberRouter) InitMemberRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	memberRouter := biRouter.Group("member").Use(middleware.OperationRecord())
	memberRouterWithoutRecord := biRouter.Group("member")
	memberRouterWithoutAuth := biRouterWithoutAuth.Group("member")
	{
		memberRouter.POST("createMember", memberApi.CreateMember)             // 新建用户
		memberRouter.DELETE("deleteMember", memberApi.DeleteMember)           // 删除用户
		memberRouter.DELETE("deleteMemberByIds", memberApi.DeleteMemberByIds) // 批量删除用户
		memberRouter.PUT("updateMember", memberApi.UpdateMember)              // 更新用户
	}
	{
		memberRouterWithoutRecord.GET("findMember", memberApi.FindMember)       // 根据ID获取用户
		memberRouterWithoutRecord.GET("getMemberList", memberApi.GetMemberList) // 获取用户列表
	}
	{
		memberRouterWithoutAuth.GET("getMemberPublic", memberApi.GetMemberPublic) // 用户开放接口
	}
}
