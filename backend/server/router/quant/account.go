package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type AccountRouter struct{}

// InitAccountRouter 初始化 账户 路由信息
func (s *AccountRouter) InitAccountRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	accountRouter := quantRouter.Group("account").Use(middleware.OperationRecord())
	accountRouterWithoutRecord := quantRouter.Group("account")
	accountRouterWithoutAuth := quantRouterWithoutAuth.Group("account")
	{
		accountRouter.POST("createAccount", accountApi.CreateAccount)             // 新建账户
		accountRouter.DELETE("deleteAccount", accountApi.DeleteAccount)           // 删除账户
		accountRouter.DELETE("deleteAccountByIds", accountApi.DeleteAccountByIds) // 批量删除账户
		accountRouter.PUT("updateAccount", accountApi.UpdateAccount)              // 更新账户
	}
	{
		accountRouterWithoutRecord.GET("findAccount", accountApi.FindAccount)       // 根据ID获取账户
		accountRouterWithoutRecord.GET("getAccountList", accountApi.GetAccountList) // 获取账户列表
		accountRouterWithoutRecord.GET("getMyAccount", accountApi.GetMyAccount)     // 获取我的账户
	}
	{
		accountRouterWithoutAuth.GET("getAccountPublic", accountApi.GetAccountPublic) // 账户开放接口
	}
}
