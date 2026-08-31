import service from '@/utils/request'
// @Tags Event
// @Summary 创建事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Event true "创建事件"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/event/createEvent [post]
export const createEvent = (data) => {
  return service({
    url: '/quant/event/createEvent',
    method: 'post',
    data
  })
}

// @Tags Event
// @Summary 删除事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Event true "删除事件"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/event/deleteEvent [delete]
export const deleteEvent = (params) => {
  return service({
    url: '/quant/event/deleteEvent',
    method: 'delete',
    params
  })
}

// @Tags Event
// @Summary 批量删除事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除事件"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/event/deleteEvent [delete]
export const deleteEventByIds = (params) => {
  return service({
    url: '/quant/event/deleteEventByIds',
    method: 'delete',
    params
  })
}

// @Tags Event
// @Summary 更新事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Event true "更新事件"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/event/updateEvent [put]
export const updateEvent = (data) => {
  return service({
    url: '/quant/event/updateEvent',
    method: 'put',
    data
  })
}

// @Tags Event
// @Summary 用id查询事件
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Event true "用id查询事件"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/event/findEvent [get]
export const findEvent = (params) => {
  return service({
    url: '/quant/event/findEvent',
    method: 'get',
    params
  })
}

// @Tags Event
// @Summary 分页获取事件列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取事件列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/event/getEventList [get]
export const getEventList = (params) => {
  return service({
    url: '/quant/event/getEventList',
    method: 'get',
    params
  })
}

// @Tags Event
// @Summary 不需要鉴权的事件接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.EventSearch true "分页获取事件列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/event/getEventPublic [get]
export const getEventPublic = () => {
  return service({
    url: '/quant/event/getEventPublic',
    method: 'get',
  })
}
