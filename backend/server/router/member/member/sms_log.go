package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type SmsLogRouter struct{}

// InitSmsLogRouter 初始化短信日志管理路由信息
func (s *SmsLogRouter) InitSmsLogRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	smsLogGroup := Router.Group("member").Group("smsLog")
	smsLogRouter := smsLogGroup.Use(middleware.OperationRecord())
	smsLogRouterWithoutRecord := smsLogGroup
	{
		smsLogRouter.DELETE("deleteSmsLog", smsLogApi.DeleteSmsLog)             // 删除短信日志
		smsLogRouter.DELETE("deleteSmsLogByIds", smsLogApi.DeleteSmsLogByIds)   // 批量删除短信日志
	}
	{
		smsLogRouterWithoutRecord.GET("getSmsLogList", smsLogApi.GetSmsLogList) // 获取短信日志列表
	}
}
