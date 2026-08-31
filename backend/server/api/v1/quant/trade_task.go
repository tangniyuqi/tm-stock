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

type TradeTaskApi struct{}

// CreateTradeTask 创建交易任务
// @Tags TradeTask
// @Summary 创建交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.TradeTask true "创建交易任务"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /tradeTask/createTradeTask [post]
func (tradeTaskApi *TradeTaskApi) CreateTradeTask(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var tradeTask quant.TradeTask
	err := c.ShouldBindJSON(&tradeTask)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if tradeTask.MemberId == nil || *tradeTask.MemberId == 0 {
		userID := uint32(utils.GetUserID(c))
		tradeTask.MemberId = &userID
	}

	tradeTask.CreatedBy = utils.GetUserID(c)
	err = tradeTaskService.CreateTradeTask(ctx, &tradeTask)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteTradeTask 删除交易任务
// @Tags TradeTask
// @Summary 删除交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.TradeTask true "删除交易任务"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /tradeTask/deleteTradeTask [delete]
func (tradeTaskApi *TradeTaskApi) DeleteTradeTask(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ID := c.Query("ID")
	userID := utils.GetUserID(c)
	err := tradeTaskService.DeleteTradeTask(ctx, ID, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败！", zap.Error(err))
		response.FailWithMessage("删除失败！"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteTradeTaskByIds 批量删除交易任务
// @Tags TradeTask
// @Summary 批量删除交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /tradeTask/deleteTradeTaskByIds [delete]
func (tradeTaskApi *TradeTaskApi) DeleteTradeTaskByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := tradeTaskService.DeleteTradeTaskByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败！", zap.Error(err))
		response.FailWithMessage("批量删除失败！"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateTradeTask 更新交易任务
// @Tags TradeTask
// @Summary 更新交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.TradeTask true "更新交易任务"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /tradeTask/updateTradeTask [put]
func (tradeTaskApi *TradeTaskApi) UpdateTradeTask(c *gin.Context) {
	ctx := c.Request.Context()

	var tradeTask quant.TradeTask
	err := c.ShouldBindJSON(&tradeTask)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	tradeTask.UpdatedBy = utils.GetUserID(c)
	err = tradeTaskService.UpdateTradeTask(ctx, tradeTask)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)

		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindTradeTask 用id查询交易任务
// @Tags TradeTask
// @Summary 用id查询交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询交易任务"
// @Success 200 {object} response.Response{data=quant.TradeTask,msg=string} "查询成功"
// @Router /tradeTask/findTradeTask [get]
func (tradeTaskApi *TradeTaskApi) FindTradeTask(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ID := c.Query("ID")
	retradeTask, err := tradeTaskService.GetTradeTask(ctx, ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(retradeTask, c)
}

// GetTradeTaskList 分页获取交易任务列表
// @Tags TradeTask
// @Summary 分页获取交易任务列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.TradeTaskSearch true "分页获取交易任务列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /tradeTask/getTradeTaskList [get]
func (tradeTaskApi *TradeTaskApi) GetTradeTaskList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.TradeTaskSearch
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

	list, total, err := tradeTaskService.GetTradeTaskInfoList(ctx, pageInfo)
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

// GetTradeTaskPublic 不需要鉴权的交易任务接口
// @Tags TradeTask
// @Summary 不需要鉴权的交易任务接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /tradeTask/getTradeTaskPublic [get]
func (tradeTaskApi *TradeTaskApi) GetTradeTaskPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	tradeTaskService.GetTradeTaskPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的交易任务接口信息",
	}, "获取成功", c)
}
