package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ReportRouter struct{}

// InitReportRouter 初始化 研报 路由信息
func (s *ReportRouter) InitReportRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	reportRouter := quantRouter.Group("report").Use(middleware.OperationRecord())
	reportRouterWithoutRecord := quantRouter.Group("report")
	reportRouterWithoutAuth := quantRouterWithoutAuth.Group("report")
	{
		reportRouter.POST("createReport", reportApi.CreateReport)             // 新建研报
		reportRouter.DELETE("deleteReport", reportApi.DeleteReport)           // 删除研报
		reportRouter.DELETE("deleteReportByIds", reportApi.DeleteReportByIds) // 批量删除研报
		reportRouter.PUT("updateReport", reportApi.UpdateReport)              // 更新研报
	}
	{
		reportRouterWithoutRecord.GET("findReport", reportApi.FindReport)       // 根据ID获取研报
		reportRouterWithoutRecord.GET("getReportList", reportApi.GetReportList) // 获取研报列表
	}
	{
		reportRouterWithoutAuth.GET("getReportPublic", reportApi.GetReportPublic) // 研报开放接口
	}
}
