package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type FeedbackApi struct{}

// CreateFeedback 创建反馈
// @Tags Feedback
// @Summary 创建反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Feedback true "创建反馈"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /feedback/createFeedback [post]
func (feedbackApi *FeedbackApi) CreateFeedback(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var feedback cms.Feedback
	err := c.ShouldBindJSON(&feedback)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	feedback.CreatedBy = utils.GetUserID(c)
	err = feedbackService.CreateFeedback(ctx, &feedback)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("创建失败!")
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteFeedback 删除反馈
// @Tags Feedback
// @Summary 删除反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Feedback true "删除反馈"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /feedback/deleteFeedback [delete]
func (feedbackApi *FeedbackApi) DeleteFeedback(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := feedbackService.DeleteFeedback(ctx, id, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("删除失败!")
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteFeedbackByIds 批量删除反馈
// @Tags Feedback
// @Summary 批量删除反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /feedback/deleteFeedbackByIds [delete]
func (feedbackApi *FeedbackApi) DeleteFeedbackByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := feedbackService.DeleteFeedbackByIds(ctx, ids, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("批量删除失败!")
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateFeedback 更新反馈
// @Tags Feedback
// @Summary 更新反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Feedback true "更新反馈"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /feedback/updateFeedback [put]
func (feedbackApi *FeedbackApi) UpdateFeedback(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var feedback cms.Feedback
	err := c.ShouldBindJSON(&feedback)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	feedback.UpdatedBy = utils.GetUserID(c)
	err = feedbackService.UpdateFeedback(ctx, feedback)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("更新失败!")
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindFeedback 用id查询反馈
// @Tags Feedback
// @Summary 用id查询反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询反馈"
// @Success 200 {object} response.Response{data=cms.Feedback,msg=string} "查询成功"
// @Router /feedback/findFeedback [get]
func (feedbackApi *FeedbackApi) FindFeedback(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	refeedback, err := feedbackService.GetFeedback(ctx, id)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("查询失败!")
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(refeedback, c)
}

// GetFeedbackList 分页获取反馈列表
// @Tags Feedback
// @Summary 分页获取反馈列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cmsReq.FeedbackSearch true "分页获取反馈列表"
// @Success 200 {object} response.Response{data=response.PageResult{list=[]cms.Feedback},msg=string} "获取成功"
// @Router /feedback/getFeedbackList [get]
func (feedbackApi *FeedbackApi) GetFeedbackList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo cmsReq.FeedbackSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := feedbackService.GetFeedbackInfoList(ctx, pageInfo)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("获取失败!")
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

// CreateFeedbackPublic C 端公开提交反馈（无需登录）
// @Tags Feedback
// @Summary C 端公开提交反馈
// @Accept application/json
// @Produce application/json
// @Param data body cms.Feedback true "反馈内容"
// @Success 200 {object} response.Response{msg=string} "提交成功"
// @Router /feedback/createFeedbackPublic [post]
func (feedbackApi *FeedbackApi) CreateFeedbackPublic(c *gin.Context) {
	ctx := c.Request.Context()
	var feedback cms.Feedback
	err := c.ShouldBindJSON(&feedback)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = feedbackService.CreateFeedbackPublic(ctx, &feedback)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("提交反馈失败!")
		response.FailWithMessage("提交失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("提交成功，感谢您的反馈", c)
}

// GetFeedbackPublic 不需要鉴权的反馈接口
// @Tags Feedback
// @Summary 不需要鉴权的反馈接口（保留兼容）
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /feedback/getFeedbackPublic [get]
func (feedbackApi *FeedbackApi) GetFeedbackPublic(c *gin.Context) {
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的反馈接口信息",
	}, "获取成功", c)
}
