import service from '@/utils/request'
// @Tags Msg
// @Summary 创建消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Msg true "创建消息"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/msg/createMsg [post]
export const createMsg = (data) => {
  return service({
    url: '/bi/msg/createMsg',
    method: 'post',
    data
  })
}

// @Tags Msg
// @Summary 删除消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Msg true "删除消息"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/msg/deleteMsg [delete]
export const deleteMsg = (params) => {
  return service({
    url: '/bi/msg/deleteMsg',
    method: 'delete',
    params
  })
}

// @Tags Msg
// @Summary 批量删除消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除消息"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/msg/deleteMsg [delete]
export const deleteMsgByIds = (params) => {
  return service({
    url: '/bi/msg/deleteMsgByIds',
    method: 'delete',
    params
  })
}

// @Tags Msg
// @Summary 更新消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Msg true "更新消息"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/msg/updateMsg [put]
export const updateMsg = (data) => {
  return service({
    url: '/bi/msg/updateMsg',
    method: 'put',
    data
  })
}

// @Tags Msg
// @Summary 用id查询消息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Msg true "用id查询消息"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/msg/findMsg [get]
export const findMsg = (params) => {
  return service({
    url: '/bi/msg/findMsg',
    method: 'get',
    params
  })
}

// @Tags Msg
// @Summary 分页获取消息列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取消息列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/msg/getMsgList [get]
export const getMsgList = (params) => {
  return service({
    url: '/bi/msg/getMsgList',
    method: 'get',
    params
  })
}

// @Tags Msg
// @Summary 不需要鉴权的消息接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.MsgSearch true "分页获取消息列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/msg/getMsgPublic [get]
export const getMsgPublic = () => {
  return service({
    url: '/bi/msg/getMsgPublic',
    method: 'get',
  })
}

// @Tags Msg
// @Summary 删除重复
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Msg true "删除消息"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/msg/deleteDuplicatePublic [delete]
export const deleteDuplicatePublic = (params) => {
  return service({
    url: '/bi/msg/deleteDuplicatePublic',
    method: 'delete',
    params
  })
}
