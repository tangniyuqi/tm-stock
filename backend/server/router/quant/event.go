package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type EventRouter struct{}

// InitEventRouter 初始化 事件 路由信息
func (s *EventRouter) InitEventRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	eventRouter := quantRouter.Group("event").Use(middleware.OperationRecord())
	eventRouterWithoutRecord := quantRouter.Group("event")
	eventRouterWithoutAuth := quantRouterWithoutAuth.Group("event")
	{
		eventRouter.POST("createEvent", eventApi.CreateEvent)             // 新建事件
		eventRouter.DELETE("deleteEvent", eventApi.DeleteEvent)           // 删除事件
		eventRouter.DELETE("deleteEventByIds", eventApi.DeleteEventByIds) // 批量删除事件
		eventRouter.PUT("updateEvent", eventApi.UpdateEvent)              // 更新事件
	}
	{
		eventRouterWithoutRecord.GET("findEvent", eventApi.FindEvent)       // 根据ID获取事件
		eventRouterWithoutRecord.GET("getEventList", eventApi.GetEventList) // 获取事件列表
	}
	{
		eventRouterWithoutAuth.GET("getEventPublic", eventApi.GetEventPublic) // 事件开放接口
	}
}
