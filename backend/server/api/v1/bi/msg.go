package bi

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MsgApi struct{}

// CreateMsg 创建消息
// @Tags Msg
// @Summary 创建消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Msg true "创建消息"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /msg/createMsg [post]
func (msgApi *MsgApi) CreateMsg(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var msg bi.Msg
	err := c.ShouldBindJSON(&msg)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	msg.CreatedBy = utils.GetUserID(c)
	err = msgService.CreateMsg(ctx, &msg)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteMsg 删除消息
// @Tags Msg
// @Summary 删除消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Msg true "删除消息"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /msg/deleteMsg [delete]
func (msgApi *MsgApi) DeleteMsg(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := msgService.DeleteMsg(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteMsgByIds 批量删除消息
// @Tags Msg
// @Summary 批量删除消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /msg/deleteMsgByIds [delete]
func (msgApi *MsgApi) DeleteMsgByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := msgService.DeleteMsgByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateMsg 更新消息
// @Tags Msg
// @Summary 更新消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Msg true "更新消息"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /msg/updateMsg [put]
func (msgApi *MsgApi) UpdateMsg(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var msg bi.Msg
	err := c.ShouldBindJSON(&msg)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	msg.UpdatedBy = utils.GetUserID(c)
	err = msgService.UpdateMsg(ctx, msg)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindMsg 用id查询消息
// @Tags Msg
// @Summary 用id查询消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询消息"
// @Success 200 {object} response.Response{data=bi.Msg,msg=string} "查询成功"
// @Router /msg/findMsg [get]
func (msgApi *MsgApi) FindMsg(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	remsg, err := msgService.GetMsg(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(remsg, c)
}

// GetMsgList 分页获取消息列表
// @Tags Msg
// @Summary 分页获取消息列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.MsgSearch true "分页获取消息列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /msg/getMsgList [get]
func (msgApi *MsgApi) GetMsgList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()
	var pageInfo biReq.MsgSearch

	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	list, total, err := msgService.GetMsgInfoList(ctx, pageInfo)
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

// GetMsgPublic 不需要鉴权的消息接口
// @Tags Msg
// @Summary 不需要鉴权的消息接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /msg/getListPublic [get]
func (msgApi *MsgApi) GetListPublic(c *gin.Context) {
	ctx := c.Request.Context()

	var pageInfo biReq.MsgSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := msgService.GetListPublic(ctx, pageInfo)
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

// CreatePublic 创建新记录（不鉴权）
// @Tags Msg
// @Summary 创建交易记录
// @Accept application/json
// @Produce application/json
// @Param data query biReq.MsgSearch true "成功"
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /msg/createPublic [POST]
func (msgApi *MsgApi) CreatePublic(c *gin.Context) {
	ctx := c.Request.Context()

	var msg bi.Msg
	err := c.ShouldBindJSON(&msg)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err = msgService.CreatePublic(ctx, &msg)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败："+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// UpdateMsg 更新消息（不鉴权）
// @Tags Msg
// @Summary 更新消息
// @Accept application/json
// @Produce application/json
// @Param data body bi.Msg true "更新消息"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /msg/update [put]
func (msgApi *MsgApi) UpdatePublic(c *gin.Context) {
	ctx := c.Request.Context()

	var msg bi.Msg
	err := c.ShouldBindJSON(&msg)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	msg.UpdatedBy = utils.GetUserID(c)
	err = msgService.UpdatePublic(ctx, msg)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// GetPageInfoPublic 获取消息分页信息（总数量、分页大小、总页数）（不鉴权）
func (msgApi *MsgApi) GetPageInfoPublic(c *gin.Context) {
	ctx := c.Request.Context()

	var pageInfo biReq.MsgSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	// 调用修改后的服务层方法，获取总数量、分页大小和总页数
	total, pageSize, pageCount, err := msgService.GetPageInfoPublic(ctx, pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	// 构建分页信息结构体返回
	pageInfoResp := struct {
		Total     int64 `json:"total"`      // 消息总数量
		PageSize  int   `json:"page_size"`  // 分页大小
		PageCount int64 `json:"page_count"` // 总页数
	}{
		Total:     total,
		PageSize:  pageSize,
		PageCount: pageCount,
	}

	response.OkWithDetailed(pageInfoResp, "获取分页信息成功", c)
}

// DeleteDuplicatePublic 删除重复消息数据（不鉴权）
// @Router /msg/deleteDuplicatePublic [delete]
func (msgApi *MsgApi) DeleteDuplicatePublic(c *gin.Context) {
	ctx := c.Request.Context()
	forceStr := c.DefaultQuery("force", "false")
	force, err := strconv.ParseBool(forceStr)

	if err != nil {
		global.GVA_LOG.Error("参数解析失败", zap.Error(err))
		response.FailWithMessage("参数force必须为true或false", c)
		return
	}

	// 调用服务层清理方法
	deletedCount, err := msgService.DeleteDuplicate(ctx, force)
	if err != nil {
		global.GVA_LOG.Error("清理重复数据失败!", zap.Error(err))
		response.FailWithMessage("清理失败:"+err.Error(), c)
		return
	}

	response.OkWithDetailed(deletedCount, "重复数据清理成功，共清理 "+strconv.FormatInt(deletedCount, 10)+" 条记录", c)
}

// matchPublic 搜索消息数据的IC型号（不鉴权）
// @Router /msg/matchPublic [get]
func (msgApi *MsgApi) MatchPublic(c *gin.Context) {
	ctx := c.Request.Context()
	var pageInfo biReq.MsgSearch
	err := c.ShouldBindQuery(&pageInfo)

	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	total, err := msgService.Match(ctx, pageInfo)

	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	response.OkWithDetailed(total, "获取成功", c)
}
