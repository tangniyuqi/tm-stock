package quant

import (
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type BaseStockApi struct{}

// CreateBaseStock 创建基础股票
// @Tags BaseStock
// @Summary 创建基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.BaseStock true "创建基础股票"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /baseStock/createBaseStock [post]
func (baseStockApi *BaseStockApi) CreateBaseStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var baseStock quant.BaseStock
	err := c.ShouldBindJSON(&baseStock)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	baseStock.CreatedBy = utils.GetUserID(c)
	err = baseStockService.CreateBaseStock(ctx, &baseStock)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteBaseStock 删除基础股票
// @Tags BaseStock
// @Summary 删除基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.BaseStock true "删除基础股票"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /baseStock/deleteBaseStock [delete]
func (baseStockApi *BaseStockApi) DeleteBaseStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := baseStockService.DeleteBaseStock(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteBaseStockByIds 批量删除基础股票
// @Tags BaseStock
// @Summary 批量删除基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /baseStock/deleteBaseStockByIds [delete]
func (baseStockApi *BaseStockApi) DeleteBaseStockByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := baseStockService.DeleteBaseStockByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateBaseStock 更新基础股票
// @Tags BaseStock
// @Summary 更新基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.BaseStock true "更新基础股票"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /baseStock/updateBaseStock [put]
func (baseStockApi *BaseStockApi) UpdateBaseStock(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var baseStock quant.BaseStock
	err := c.ShouldBindJSON(&baseStock)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	baseStock.UpdatedBy = utils.GetUserID(c)
	err = baseStockService.UpdateBaseStock(ctx, baseStock)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindBaseStock 用id查询基础股票
// @Tags BaseStock
// @Summary 用id查询基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询基础股票"
// @Success 200 {object} response.Response{data=quant.BaseStock,msg=string} "查询成功"
// @Router /baseStock/findBaseStock [get]
func (baseStockApi *BaseStockApi) FindBaseStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	rebaseStock, err := baseStockService.GetBaseStock(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(rebaseStock, c)
}

// GetBaseStockList 分页获取基础股票列表
// @Tags BaseStock
// @Summary 分页获取基础股票列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.BaseStockSearch true "分页获取基础股票列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /baseStock/getBaseStockList [get]
func (baseStockApi *BaseStockApi) GetBaseStockList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.BaseStockSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := baseStockService.GetBaseStockInfoList(ctx, pageInfo)
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

// GetBaseStockPublic 不需要鉴权的基础股票接口
// @Tags BaseStock
// @Summary 不需要鉴权的基础股票接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /baseStock/getBaseStockPublic [get]
func (baseStockApi *BaseStockApi) GetBaseStockPublic(c *gin.Context) {
	baseStockApi.GetBaseStockList(c)
}

// Sync 增量同步基础股票数据
// @Tags BaseStock
// @Summary 增量同步基础股票数据（已存在的按 ts_code 更新基础信息，不存在的执行新增，保留本地行情与AI分析结果）
// @Security ApiKeyAuth
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "同步成功"
// @Router /baseStock/sync [get]
func (baseStockApi *BaseStockApi) Sync(c *gin.Context) {
	ctx := c.Request.Context()
	err := baseStockService.Sync(ctx)
	if err != nil {
		global.GVA_LOG.Error("同步失败!", zap.Error(err))
		response.FailWithMessage("同步失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("同步成功", c)
}

// Clear 清除全部基础股票数据
// @Tags BaseStock
// @Summary 清除全部基础股票数据
// @Security ApiKeyAuth
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "清除成功"
// @Router /baseStock/clear [delete]
func (baseStockApi *BaseStockApi) Clear(c *gin.Context) {
	ctx := c.Request.Context()
	err := baseStockService.Clear(ctx)
	if err != nil {
		global.GVA_LOG.Error("清除失败!", zap.Error(err))
		response.FailWithMessage("清除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("清除成功", c)
}

// AiAnalyzeStocks AI自动分析股票（异步任务化）
// @Tags BaseStock
// @Summary AI自动分析股票基本面，提交后立即返回任务ID，执行进度可通过任务接口查询
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiAnalyzeStockReq true "AI自动分析股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "任务已创建"
// @Router /baseStock/aiAnalyzeStocks [post]
func (baseStockApi *BaseStockApi) AiAnalyzeStocks(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var req quantReq.AiAnalyzeStockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	taskID, err := baseStockService.AiAnalyzeStocks(ctx, req, userID)
	if err != nil {
		global.GVA_LOG.Error("AI自动分析股票失败!", zap.Error(err))
		response.FailWithMessage("AI自动分析股票失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{
		"task_id": taskID,
	}, "任务已创建，可前往执行记录查看进度", c)
}

// UpdateAllChangePct 一键更新全部股票涨跌幅
// @Tags BaseStock
// @Summary 一键更新全部股票涨跌幅（通过 Tushare 获取最近交易日行情并批量更新）
// @Security ApiKeyAuth
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "更新成功"
// @Router /baseStock/updateAllChangePct [post]
func (baseStockApi *BaseStockApi) UpdateAllChangePct(c *gin.Context) {
	ctx := c.Request.Context()
	updatedCount, err := baseStockService.UpdateAllChangePct(ctx)
	if err != nil {
		global.GVA_LOG.Error("更新全部涨跌幅失败!", zap.Error(err))
		response.FailWithMessage("更新涨跌幅失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{
		"updated": updatedCount,
	}, fmt.Sprintf("更新成功，共更新 %d 只股票涨跌幅", updatedCount), c)
}
