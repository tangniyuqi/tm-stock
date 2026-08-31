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

type TradeRecordApi struct{}

// CreateTradeRecord 创建交易记录
// @Tags TradeRecord
// @Summary 创建交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.TradeRecord true "创建交易记录"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /tradeRecord/createTradeRecord [post]
func (tradeRecordApi *TradeRecordApi) CreateTradeRecord(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var tradeRecord quant.TradeRecord
	err := c.ShouldBindJSON(&tradeRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	tradeRecord.CreatedBy = utils.GetUserID(c)
	err = tradeRecordService.CreateTradeRecord(ctx, &tradeRecord)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteTradeRecord 删除交易记录
// @Tags TradeRecord
// @Summary 删除交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.TradeRecord true "删除交易记录"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /tradeRecord/deleteTradeRecord [delete]
func (tradeRecordApi *TradeRecordApi) DeleteTradeRecord(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := tradeRecordService.DeleteTradeRecord(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteTradeRecordByIds 批量删除交易记录
// @Tags TradeRecord
// @Summary 批量删除交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /tradeRecord/deleteTradeRecordByIds [delete]
func (tradeRecordApi *TradeRecordApi) DeleteTradeRecordByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := tradeRecordService.DeleteTradeRecordByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateTradeRecord 更新交易记录
// @Tags TradeRecord
// @Summary 更新交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.TradeRecord true "更新交易记录"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /tradeRecord/updateTradeRecord [put]
func (tradeRecordApi *TradeRecordApi) UpdateTradeRecord(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var tradeRecord quant.TradeRecord
	err := c.ShouldBindJSON(&tradeRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	tradeRecord.UpdatedBy = utils.GetUserID(c)
	err = tradeRecordService.UpdateTradeRecord(ctx, tradeRecord)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindTradeRecord 用ID查询交易记录
// @Tags TradeRecord
// @Summary 用ID查询交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询交易记录"
// @Success 200 {object} response.Response{data=quant.TradeRecord,msg=string} "查询成功"
// @Router /tradeRecord/findTradeRecord [get]
func (tradeRecordApi *TradeRecordApi) FindTradeRecord(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	retradeRecord, err := tradeRecordService.GetTradeRecord(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(retradeRecord, c)
}

// GetTradeRecordList 分页获取交易记录列表
// @Tags TradeRecord
// @Summary 分页获取交易记录列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.TradeRecordSearch true "分页获取交易记录列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /tradeRecord/getTradeRecordList [get]
func (tradeRecordApi *TradeRecordApi) GetTradeRecordList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.TradeRecordSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if authorityId := utils.GetUserAuthorityId(c); authorityId != 1 {
		// global.GVA_LOG.Info("当前用户的 authorityId", zap.Uint("authorityId", authorityId))
		userID := utils.GetUserID(c)
		pageInfo.MemberId = &userID
	}

	list, total, err := tradeRecordService.GetTradeRecordInfoList(ctx, pageInfo)
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

// GetTradeRecordPublic 不需要鉴权的交易记录接口
// @Tags TradeRecord
// @Summary 不需要鉴权的交易记录接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /tradeRecord/getTradeRecordPublic [get]
func (tradeRecordApi *TradeRecordApi) GetTradeRecordPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	tradeRecordService.GetTradeRecordPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的交易记录接口信息",
	}, "获取成功", c)
}

// CreatePublic 创建交易记录（不鉴权）
// @Tags News
// @Summary 创建交易记录（不鉴权）
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.NewsSearch true "成功"
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /tradeRecord/createPublic [POST]
func (tradeRecordApi *TradeRecordApi) CreatePublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var tradeRecord quant.TradeRecord
	err := c.ShouldBindJSON(&tradeRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err = tradeRecordService.CreatePublic(ctx, &tradeRecord)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}
