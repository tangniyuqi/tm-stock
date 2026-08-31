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

type EventApi struct{}

// CreateEvent 创建事件
// @Tags Event
// @Summary 创建事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Event true "创建事件"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /event/createEvent [post]
func (eventApi *EventApi) CreateEvent(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var event quant.Event
	err := c.ShouldBindJSON(&event)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	event.CreatedBy = utils.GetUserID(c)
	err = eventService.CreateEvent(ctx, &event)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteEvent 删除事件
// @Tags Event
// @Summary 删除事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Event true "删除事件"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /event/deleteEvent [delete]
func (eventApi *EventApi) DeleteEvent(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := eventService.DeleteEvent(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteEventByIds 批量删除事件
// @Tags Event
// @Summary 批量删除事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /event/deleteEventByIds [delete]
func (eventApi *EventApi) DeleteEventByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := eventService.DeleteEventByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateEvent 更新事件
// @Tags Event
// @Summary 更新事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Event true "更新事件"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /event/updateEvent [put]
func (eventApi *EventApi) UpdateEvent(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var event quant.Event
	err := c.ShouldBindJSON(&event)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	event.UpdatedBy = utils.GetUserID(c)
	err = eventService.UpdateEvent(ctx, event)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindEvent 用id查询事件
// @Tags Event
// @Summary 用id查询事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询事件"
// @Success 200 {object} response.Response{data=quant.Event,msg=string} "查询成功"
// @Router /event/findEvent [get]
func (eventApi *EventApi) FindEvent(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	reevent, err := eventService.GetEvent(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reevent, c)
}

// GetEventList 分页获取事件列表
// @Tags Event
// @Summary 分页获取事件列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.EventSearch true "分页获取事件列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /event/getEventList [get]
func (eventApi *EventApi) GetEventList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.EventSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := eventService.GetEventInfoList(ctx, pageInfo)
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

// GetEventPublic 不需要鉴权的事件接口
// @Tags Event
// @Summary 不需要鉴权的事件接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /event/getEventPublic [get]
func (eventApi *EventApi) GetEventPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	eventService.GetEventPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的事件接口信息",
	}, "获取成功", c)
}
