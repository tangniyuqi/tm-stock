package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type MsgRouter struct{}

// InitMsgRouter 初始化 消息 路由信息
func (s *MsgRouter) InitMsgRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	msgRouter := biRouter.Group("msg").Use(middleware.OperationRecord())
	msgRouterWithoutRecord := biRouter.Group("msg")
	msgRouterWithoutAuth := biRouterWithoutAuth.Group("msg")
	{
		msgRouter.POST("createMsg", msgApi.CreateMsg)             // 新建消息
		msgRouter.DELETE("deleteMsg", msgApi.DeleteMsg)           // 删除消息
		msgRouter.DELETE("deleteMsgByIds", msgApi.DeleteMsgByIds) // 批量删除消息
		msgRouter.PUT("updateMsg", msgApi.UpdateMsg)              // 更新消息
	}
	{
		msgRouterWithoutRecord.GET("findMsg", msgApi.FindMsg)       // 根据ID获取消息
		msgRouterWithoutRecord.GET("getMsgList", msgApi.GetMsgList) // 获取消息列表
	}
	{
		msgRouterWithoutAuth.GET("getListPublic", msgApi.GetListPublic)
		msgRouterWithoutAuth.GET("getPageInfoPublic", msgApi.GetPageInfoPublic)
		msgRouterWithoutAuth.POST("createPublic", msgApi.CreatePublic)
		msgRouterWithoutAuth.PUT("updatePublic", msgApi.UpdatePublic)
		msgRouterWithoutAuth.DELETE("deleteDuplicatePublic", msgApi.DeleteDuplicatePublic) // 删除重复消息
		msgRouterWithoutAuth.GET("matchPublic", msgApi.MatchPublic)                        // 匹配IC型号
	}
}
