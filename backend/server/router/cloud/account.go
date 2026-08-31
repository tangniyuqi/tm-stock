package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type AccountRouter struct{}

// InitAccountRouter 初始化 财务 路由信息
func (s *AccountRouter) InitAccountRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cloudRouter := Router.Group("cloud")
	cloudRouterWithoutAuth := PublicRouter.Group("cloud")

	accountRouter := cloudRouter.Group("account").Use(middleware.OperationRecord())
	accountRouterWithoutRecord := cloudRouter.Group("account")
	accountRouterWithoutAuth := cloudRouterWithoutAuth.Group("account")
	{
		accountRouter.POST("createAccount", accountApi.CreateAccount)             // 新建财务
		accountRouter.DELETE("deleteAccount", accountApi.DeleteAccount)           // 删除财务
		accountRouter.DELETE("deleteAccountByIds", accountApi.DeleteAccountByIds) // 批量删除财务
		accountRouter.PUT("updateAccount", accountApi.UpdateAccount)              // 更新财务
	}
	{
		accountRouterWithoutRecord.GET("findAccount", accountApi.FindAccount)       // 根据ID获取财务
		accountRouterWithoutRecord.GET("getAccountList", accountApi.GetAccountList) // 获取财务列表
	}
	{
		accountRouterWithoutAuth.GET("getAccountPublic", accountApi.GetAccountPublic) // 财务开放接口
	}
}
