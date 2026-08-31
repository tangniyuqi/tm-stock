package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AiTaskApi struct{}

// GetAiTask 用id查询AI执行任务
// @Tags AiTask
// @Summary 用id查询AI执行任务（含进度、日志与结果）
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询AI执行任务"
// @Success 200 {object} response.Response{data=quant.QuantAiTask,msg=string} "查询成功"
// @Router /quant/aiTask/getAiTask [get]
func (aiTaskApi *AiTaskApi) GetAiTask(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	isAdmin := utils.GetUserAuthorityId(c) == 1
	task, err := aiTaskService.GetAiTask(ctx, id, userID, isAdmin)
	if err != nil {
		global.GVA_LOG.Error("查询AI执行任务失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(task, c)
}

// GetAiTaskList 分页获取AI执行任务列表
// @Tags AiTask
// @Summary 分页获取AI执行任务列表（运行中的任务优先展示）
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.AiTaskSearch true "分页获取AI执行任务列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /quant/aiTask/getAiTaskList [get]
func (aiTaskApi *AiTaskApi) GetAiTaskList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.AiTaskSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	isAdmin := utils.GetUserAuthorityId(c) == 1
	list, total, err := aiTaskService.GetAiTaskInfoList(ctx, pageInfo, userID, isAdmin)
	if err != nil {
		global.GVA_LOG.Error("获取AI执行任务列表失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// StopAiTask 停止运行中的AI执行任务
// @Tags AiTask
// @Summary 停止运行中的AI执行任务（标记为已取消，后台执行将尽快退出）
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body quantReq.AiTaskActionReq true "任务ID"
// @Success 200 {object} response.Response{msg=string} "停止成功"
// @Router /quant/aiTask/stopAiTask [post]
func (aiTaskApi *AiTaskApi) StopAiTask(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var req quantReq.AiTaskActionReq
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	isAdmin := utils.GetUserAuthorityId(c) == 1
	if err := aiTaskService.StopAiTask(ctx, req.ID, userID, isAdmin); err != nil {
		global.GVA_LOG.Error("停止AI执行任务失败!", zap.Error(err))
		response.FailWithMessage("停止失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("任务已停止", c)
}

// RestartAiTask 重启已取消的AI执行任务
// @Tags AiTask
// @Summary 重启已取消的AI执行任务（复用原任务记录，按原参数重新执行）
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body quantReq.AiTaskActionReq true "任务ID"
// @Success 200 {object} response.Response{data=uint,msg=string} "重启成功"
// @Router /quant/aiTask/restartAiTask [post]
func (aiTaskApi *AiTaskApi) RestartAiTask(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var req quantReq.AiTaskActionReq
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	isAdmin := utils.GetUserAuthorityId(c) == 1
	if err := aiTaskService.RestartAiTask(ctx, req.ID, userID, isAdmin); err != nil {
		global.GVA_LOG.Error("重启AI执行任务失败!", zap.Error(err))
		response.FailWithMessage("重启失败:"+err.Error(), c)
		return
	}
	response.OkWithData(req.ID, c)
}
