package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ScreenerRecordApi struct{}

// CreateScreenerRecord 创建AI选股器查询记录
// @Tags ScreenerRecord
// @Summary 创建AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ScreenerRecord true "创建AI选股器查询记录"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /screenerRecord/createScreenerRecord [post]
func (screenerRecordApi *ScreenerRecordApi) CreateScreenerRecord(c *gin.Context) {
	ctx := c.Request.Context()

	var screenerRecord quant.ScreenerRecord
	err := c.ShouldBindJSON(&screenerRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if screenerRecord.MemberId == nil || *screenerRecord.MemberId == 0 {
		userID := uint32(utils.GetUserID(c))
		screenerRecord.MemberId = &userID
	}
	
	// 如果提供了prompt，使用RecordQuery逻辑（去重+计数）
	if screenerRecord.Prompt != nil && *screenerRecord.Prompt != "" {
		global.GVA_LOG.Info("使用RecordQuery逻辑", 
			zap.Uint32("member_id", *screenerRecord.MemberId), 
			zap.String("prompt", *screenerRecord.Prompt))
		err = screenerRecordService.RecordQuery(ctx, screenerRecord.MemberId, *screenerRecord.Prompt)
		if err != nil {
			global.GVA_LOG.Error("记录失败!", zap.Error(err))
			response.FailWithMessage("记录失败:"+err.Error(), c)
			return
		}
		response.OkWithMessage("记录成功", c)
		return
	}
	
	// 否则使用原有的创建逻辑
	screenerRecord.CreatedBy = utils.GetUserID(c)
	err = screenerRecordService.CreateScreenerRecord(ctx, &screenerRecord)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteScreenerRecord 删除AI选股器查询记录
// @Tags ScreenerRecord
// @Summary 删除AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ScreenerRecord true "删除AI选股器查询记录"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /screenerRecord/deleteScreenerRecord [delete]
func (screenerRecordApi *ScreenerRecordApi) DeleteScreenerRecord(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := screenerRecordService.DeleteScreenerRecord(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteScreenerRecordByIds 批量删除AI选股器查询记录
// @Tags ScreenerRecord
// @Summary 批量删除AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /screenerRecord/deleteScreenerRecordByIds [delete]
func (screenerRecordApi *ScreenerRecordApi) DeleteScreenerRecordByIds(c *gin.Context) {
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := screenerRecordService.DeleteScreenerRecordByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateScreenerRecord 更新AI选股器查询记录
// @Tags ScreenerRecord
// @Summary 更新AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ScreenerRecord true "更新AI选股器查询记录"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /screenerRecord/updateScreenerRecord [put]
func (screenerRecordApi *ScreenerRecordApi) UpdateScreenerRecord(c *gin.Context) {
	ctx := c.Request.Context()

	var screenerRecord quant.ScreenerRecord
	err := c.ShouldBindJSON(&screenerRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	screenerRecord.UpdatedBy = utils.GetUserID(c)
	err = screenerRecordService.UpdateScreenerRecord(ctx, screenerRecord)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindScreenerRecord 用ID查询AI选股器查询记录
// @Tags ScreenerRecord
// @Summary 用ID查询AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询AI选股器查询记录"
// @Success 200 {object} response.Response{data=quant.ScreenerRecord,msg=string} "查询成功"
// @Router /screenerRecord/findScreenerRecord [get]
func (screenerRecordApi *ScreenerRecordApi) FindScreenerRecord(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Query("id")
	rescreenerRecord, err := screenerRecordService.GetScreenerRecord(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(rescreenerRecord, c)
}

// GetScreenerRecordList 分页获取AI选股器查询记录列表
// @Tags ScreenerRecord
// @Summary 分页获取AI选股器查询记录列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ScreenerRecordSearch true "分页获取AI选股器查询记录列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /screenerRecord/getScreenerRecordList [get]
func (screenerRecordApi *ScreenerRecordApi) GetScreenerRecordList(c *gin.Context) {
	ctx := c.Request.Context()

	var pageInfo quantReq.ScreenerRecordSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if authorityId := utils.GetUserAuthorityId(c); authorityId != 1 {	
		userID := utils.GetUserID(c)
		pageInfo.MemberId = &userID
	}

	list, total, err := screenerRecordService.GetScreenerRecordInfoList(ctx, pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
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
