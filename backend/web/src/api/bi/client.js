import service from '@/utils/request'
// @Tags Client
// @Summary 创建客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Client true "创建客户端"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/client/createClient [post]
export const createClient = (data) => {
  return service({
    url: '/bi/client/createClient',
    method: 'post',
    data
  })
}

// @Tags Client
// @Summary 删除客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Client true "删除客户端"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/client/deleteClient [delete]
export const deleteClient = (params) => {
  return service({
    url: '/bi/client/deleteClient',
    method: 'delete',
    params
  })
}

// @Tags Client
// @Summary 批量删除客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除客户端"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/client/deleteClient [delete]
export const deleteClientByIds = (params) => {
  return service({
    url: '/bi/client/deleteClientByIds',
    method: 'delete',
    params
  })
}

// @Tags Client
// @Summary 更新客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Client true "更新客户端"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/client/updateClient [put]
export const updateClient = (data) => {
  return service({
    url: '/bi/client/updateClient',
    method: 'put',
    data
  })
}

// @Tags Client
// @Summary 用id查询客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Client true "用id查询客户端"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/client/findClient [get]
export const findClient = (params) => {
  return service({
    url: '/bi/client/findClient',
    method: 'get',
    params
  })
}

// @Tags Client
// @Summary 分页获取客户端列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取客户端列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/client/getClientList [get]
export const getClientList = (params) => {
  return service({
    url: '/bi/client/getClientList',
    method: 'get',
    params
  })
}

// @Tags Client
// @Summary 不需要鉴权的客户端接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.ClientSearch true "分页获取客户端列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/client/getClientPublic [get]
export const getClientPublic = () => {
  return service({
    url: '/bi/client/getClientPublic',
    method: 'get',
  })
}
