package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type FeedbackRouter struct{}

// InitFeedbackRouter 初始化 反馈 路由信息
func (s *FeedbackRouter) InitFeedbackRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cmsGroup := Router.Group("cms")
	feedbackRouter := cmsGroup.Group("feedback").Use(middleware.OperationRecord())
	feedbackRouterWithoutRecord := cmsGroup.Group("feedback")
	feedbackRouterWithoutAuth := PublicRouter.Group("client").Group("cms").Group("feedback")
	{
		feedbackRouter.POST("createFeedback", feedbackApi.CreateFeedback)             // 新建反馈
		feedbackRouter.DELETE("deleteFeedback", feedbackApi.DeleteFeedback)           // 删除反馈
		feedbackRouter.DELETE("deleteFeedbackByIds", feedbackApi.DeleteFeedbackByIds) // 批量删除反馈
		feedbackRouter.PUT("updateFeedback", feedbackApi.UpdateFeedback)              // 更新反馈
	}
	{
		feedbackRouterWithoutRecord.GET("findFeedback", feedbackApi.FindFeedback)       // 根据ID获取反馈
		feedbackRouterWithoutRecord.GET("getFeedbackList", feedbackApi.GetFeedbackList) // 获取反馈列表
	}
	{
		// C 端公开接口：/client/cms/feedback/create（意见反馈，无需登录）
		feedbackRouterWithoutAuth.POST("create", feedbackApi.CreateFeedbackPublic) // 提交反馈
	}
}
