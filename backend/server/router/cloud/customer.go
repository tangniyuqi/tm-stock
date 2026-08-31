package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type CustomerRouter struct{}

// InitCustomerRouter 初始化 客户 路由信息
func (s *CustomerRouter) InitCustomerRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cloudRouter := Router.Group("cloud")
	cloudRouterWithoutAuth := PublicRouter.Group("cloud")

	customerRouter := cloudRouter.Group("customer").Use(middleware.OperationRecord())
	customerRouterWithoutRecord := cloudRouter.Group("customer")
	customerRouterWithoutAuth := cloudRouterWithoutAuth.Group("customer")
	{
		customerRouter.POST("createCustomer", customerApi.CreateCustomer)             // 新建客户
		customerRouter.DELETE("deleteCustomer", customerApi.DeleteCustomer)           // 删除客户
		customerRouter.DELETE("deleteCustomerByIds", customerApi.DeleteCustomerByIds) // 批量删除客户
		customerRouter.PUT("updateCustomer", customerApi.UpdateCustomer)              // 更新客户
	}
	{
		customerRouterWithoutRecord.GET("findCustomer", customerApi.FindCustomer)       // 根据ID获取客户
		customerRouterWithoutRecord.GET("getCustomerList", customerApi.GetCustomerList) // 获取客户列表
	}
	{
		customerRouterWithoutAuth.GET("getCustomerPublic", customerApi.GetCustomerPublic) // 客户开放接口
	}
}
