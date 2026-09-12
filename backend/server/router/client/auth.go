package client

import (
	"github.com/gin-gonic/gin"
)

type AuthRouter struct{}

// InitAuthRouter 注册 C 端鉴权路由（公开，无需登录态）
func (s *AuthRouter) InitAuthRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	authRouter := clientRouter.Group("auth")
	{
		authRouter.POST("send-code", authApi.SendCode)   // 发送短信验证码
		authRouter.POST("login", authApi.Login)          // 登录
		authRouter.POST("register", authApi.Register)    // 注册
		authRouter.POST("forgot-pwd", authApi.ForgotPwd) // 找回密码
		authRouter.POST("reset-pwd", authApi.ResetPwd)   // 重置密码
		authRouter.POST("logout", authApi.Logout)        // 登出
	}
}
