package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type TradeRecordRouter struct{}

func (s *TradeRecordRouter) InitTradeRecordRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	tradeRecordRouter := quantRouter.Group("tradeRecord").Use(middleware.OperationRecord())
	tradeRecordRouterWithoutRecord := quantRouter.Group("tradeRecord")
	tradeRecordRouterWithoutAuth := quantRouterWithoutAuth.Group("tradeRecord")
	{
		tradeRecordRouter.POST("createTradeRecord", tradeRecordApi.CreateTradeRecord)
		tradeRecordRouter.DELETE("deleteTradeRecord", tradeRecordApi.DeleteTradeRecord)
		tradeRecordRouter.DELETE("deleteTradeRecordByIds", tradeRecordApi.DeleteTradeRecordByIds)
		tradeRecordRouter.PUT("updateTradeRecord", tradeRecordApi.UpdateTradeRecord)
	}
	{
		tradeRecordRouterWithoutRecord.GET("findTradeRecord", tradeRecordApi.FindTradeRecord)
		tradeRecordRouterWithoutRecord.GET("getTradeRecordList", tradeRecordApi.GetTradeRecordList)
	}
	{
		tradeRecordRouterWithoutAuth.GET("getTradeRecordPublic", tradeRecordApi.GetTradeRecordPublic)
		tradeRecordRouterWithoutAuth.POST("createPublic", tradeRecordApi.CreatePublic)
	}
}
