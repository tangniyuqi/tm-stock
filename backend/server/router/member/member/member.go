package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type MemberRouter struct{}

// InitMemberRouter 初始化 C端会员管理 路由信息
func (s *MemberRouter) InitMemberRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	memberGroup := Router.Group("member")
	memberRouter := memberGroup.Group("member").Use(middleware.OperationRecord())
	memberRouterWithoutRecord := memberGroup.Group("member")
	{
		memberRouter.POST("createMember", memberApi.CreateMember)               // 新建会员
		memberRouter.POST("resetMemberPassword", memberApi.ResetMemberPassword) // 重置会员密码
		memberRouter.DELETE("deleteMember", memberApi.DeleteMember)             // 删除会员
		memberRouter.DELETE("deleteMemberByIds", memberApi.DeleteMemberByIds)   // 批量删除会员
		memberRouter.PUT("updateMember", memberApi.UpdateMember)                // 更新会员
	}
	{
		memberRouterWithoutRecord.GET("findMember", memberApi.FindMember)       // 根据ID获取会员
		memberRouterWithoutRecord.GET("getMemberList", memberApi.GetMemberList) // 获取会员列表
	}
}
