package quant

import "github.com/gin-gonic/gin"

type AiTaskRouter struct{}

// InitAiTaskRouter 初始化 AI 执行任务 路由信息
func (s *AiTaskRouter) InitAiTaskRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")

	aiTaskRouter := quantRouter.Group("aiTask")
	{
		aiTaskRouter.GET("getAiTask", aiTaskApi.GetAiTask)             // 根据ID获取AI执行任务（进度/日志/结果）
		aiTaskRouter.GET("getAiTaskList", aiTaskApi.GetAiTaskList)     // 分页获取AI执行任务列表
		aiTaskRouter.POST("stopAiTask", aiTaskApi.StopAiTask)          // 停止运行中的AI执行任务
		aiTaskRouter.POST("restartAiTask", aiTaskApi.RestartAiTask)    // 重启已取消的AI执行任务
	}
}
