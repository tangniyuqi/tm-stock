package client

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type MemberRouter struct{}

// InitMemberRouter 注册 C 端会员路由（需客户登录态，走独立中间件 ClientJWTAuth）
func (s *MemberRouter) InitMemberRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	memberRouter := clientRouter.Group("member")
	memberRouter.Use(middleware.ClientJWTAuth())
	{
		memberRouter.GET("profile", memberApi.Profile)
		memberRouter.PUT("profile", memberApi.UpdateProfile)
		memberRouter.PUT("mobile", memberApi.BindMobile)
		memberRouter.PUT("password", memberApi.ChangePwd)
		memberRouter.POST("realname", memberApi.RealName)
		memberRouter.GET("settings", memberApi.AccountSetting)
		memberRouter.PUT("name", memberApi.UpdateAccount)
		memberRouter.GET("asset/detail", memberApi.GetAssetOverview)
		memberRouter.GET("asset_log/list", memberApi.List)
	}
}
