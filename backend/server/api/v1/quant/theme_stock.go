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

type ThemeStockApi struct{}

// CreateThemeStock 创建题材股票
// @Tags ThemeStock
// @Summary 创建题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ThemeStock true "创建题材股票"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /themeStock/createThemeStock [post]
func (themeStockApi *ThemeStockApi) CreateThemeStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var themeStock quant.ThemeStock
	err := c.ShouldBindJSON(&themeStock)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	themeStock.CreatedBy = utils.GetUserID(c)
	err = themeStockService.CreateThemeStock(ctx, &themeStock)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteThemeStock 删除题材股票
// @Tags ThemeStock
// @Summary 删除题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ThemeStock true "删除题材股票"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /themeStock/deleteThemeStock [delete]
func (themeStockApi *ThemeStockApi) DeleteThemeStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ID := c.Query("ID")
	userID := utils.GetUserID(c)
	err := themeStockService.DeleteThemeStock(ctx, ID, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteThemeStockByIds 批量删除题材股票
// @Tags ThemeStock
// @Summary 批量删除题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /themeStock/deleteThemeStockByIds [delete]
func (themeStockApi *ThemeStockApi) DeleteThemeStockByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	IDs := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := themeStockService.DeleteThemeStockByIds(ctx, IDs, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateThemeStock 更新题材股票
// @Tags ThemeStock
// @Summary 更新题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ThemeStock true "更新题材股票"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /themeStock/updateThemeStock [put]
func (themeStockApi *ThemeStockApi) UpdateThemeStock(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var themeStock quant.ThemeStock
	err := c.ShouldBindJSON(&themeStock)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	themeStock.UpdatedBy = utils.GetUserID(c)
	err = themeStockService.UpdateThemeStock(ctx, themeStock)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindThemeStock 用id查询题材股票
// @Tags ThemeStock
// @Summary 用id查询题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询题材股票"
// @Success 200 {object} response.Response{data=quant.ThemeStock,msg=string} "查询成功"
// @Router /themeStock/findThemeStock [get]
func (themeStockApi *ThemeStockApi) FindThemeStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ID := c.Query("ID")
	rethemeStock, err := themeStockService.GetThemeStock(ctx, ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(rethemeStock, c)
}

// GetThemeStockList 分页获取题材股票列表
// @Tags ThemeStock
// @Summary 分页获取题材股票列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ThemeStockSearch true "分页获取题材股票列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /themeStock/getThemeStockList [get]
func (themeStockApi *ThemeStockApi) GetThemeStockList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.ThemeStockSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := themeStockService.GetThemeStockInfoList(ctx, pageInfo)
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

// GetThemeStockPublic 不需要鉴权的题材股票接口
// @Tags ThemeStock
// @Summary 不需要鉴权的题材股票接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /themeStock/getThemeStockPublic [get]
func (themeStockApi *ThemeStockApi) GetThemeStockPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	themeStockService.GetThemeStockPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的题材股票接口信息",
	}, "获取成功", c)
}

// AiAddThemeStocks AI智能添加题材股票（异步任务化）
// @Tags ThemeStock
// @Summary AI智能为题材添加股票（已存在股票可按配置覆盖或跳过；存量暴雷股自动标记下架），提交后立即返回任务ID，执行进度可通过任务接口查询
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiAddThemeStockReq true "AI智能添加题材股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "任务已创建"
// @Router /themeStock/aiAddThemeStocks [post]
func (themeStockApi *ThemeStockApi) AiAddThemeStocks(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var req quantReq.AiAddThemeStockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	taskID, err := themeStockService.AiAddThemeStocks(ctx, req, userID)
	if err != nil {
		global.GVA_LOG.Error("AI添加题材股票任务创建失败!", zap.Error(err))
		response.FailWithMessage("AI添加题材股票失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{
		"task_id": taskID,
	}, "任务已创建，可前往执行记录查看进度", c)
}

// AiUpdateThemeStock AI智能更新题材股票（异步任务化）
// @Tags ThemeStock
// @Summary AI一键更新个股的入选逻辑、梯队、相关度，提交后立即返回任务ID
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiUpdateThemeStockReq true "AI更新题材股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "任务已创建"
// @Router /themeStock/aiUpdateThemeStock [post]
func (themeStockApi *ThemeStockApi) AiUpdateThemeStock(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var req quantReq.AiUpdateThemeStockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	taskID, err := themeStockService.AiUpdateThemeStock(ctx, req, userID)
	if err != nil {
		global.GVA_LOG.Error("AI更新题材股票任务创建失败!", zap.Error(err))
		response.FailWithMessage("AI更新题材股票失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{
		"task_id": taskID,
	}, "任务已创建，可前往执行记录查看进度", c)
}

// AiUpdateThemeStocks AI批量更新题材股票（异步任务化）
// @Tags ThemeStock
// @Summary AI批量更新所选个股的入选逻辑、梯队、相关度，提交后立即返回任务ID
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiUpdateThemeStocksReq true "AI批量更新题材股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "任务已创建"
// @Router /themeStock/aiUpdateThemeStocks [post]
func (themeStockApi *ThemeStockApi) AiUpdateThemeStocks(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var req quantReq.AiUpdateThemeStocksReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	userID := utils.GetUserID(c)
	taskID, err := themeStockService.AiUpdateThemeStocks(ctx, req, userID)
	if err != nil {
		global.GVA_LOG.Error("AI批量更新题材股票任务创建失败!", zap.Error(err))
		response.FailWithMessage("AI批量更新题材股票失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{
		"task_id": taskID,
	}, "任务已创建，可前往执行记录查看进度", c)
}
