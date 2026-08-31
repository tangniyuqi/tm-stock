package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ScreenerRecordRouter struct{}

// InitScreenerRecordRouter 初始化 AI选股器查询记录 路由信息
func (s *ScreenerRecordRouter) InitScreenerRecordRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")

	screenerRecordRouter := quantRouter.Group("screenerRecord").Use(middleware.OperationRecord())
	screenerRecordRouterWithoutRecord := quantRouter.Group("screenerRecord")
	{
		screenerRecordRouter.POST("createScreenerRecord", screenerRecordApi.CreateScreenerRecord)             // 新建AI选股器查询记录
		screenerRecordRouter.DELETE("deleteScreenerRecord", screenerRecordApi.DeleteScreenerRecord)           // 删除AI选股器查询记录
		screenerRecordRouter.DELETE("deleteScreenerRecordByIds", screenerRecordApi.DeleteScreenerRecordByIds) // 批量删除AI选股器查询记录
		screenerRecordRouter.PUT("updateScreenerRecord", screenerRecordApi.UpdateScreenerRecord)              // 更新AI选股器查询记录
	}
	{
		screenerRecordRouterWithoutRecord.GET("findScreenerRecord", screenerRecordApi.FindScreenerRecord)       // 根据ID获取AI选股器查询记录
		screenerRecordRouterWithoutRecord.GET("getScreenerRecordList", screenerRecordApi.GetScreenerRecordList) // 获取AI选股器查询记录列表
	}
}
