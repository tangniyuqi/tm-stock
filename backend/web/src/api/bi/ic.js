import service from '@/utils/request'
// @Tags Ic
// @Summary 创建IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Ic true "创建IC"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/ic/createIc [post]
export const createIc = (data) => {
  return service({
    url: '/bi/ic/createIc',
    method: 'post',
    data
  })
}

// @Tags Ic
// @Summary 删除IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Ic true "删除IC"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/ic/deleteIc [delete]
export const deleteIc = (params) => {
  return service({
    url: '/bi/ic/deleteIc',
    method: 'delete',
    params
  })
}

// @Tags Ic
// @Summary 批量删除IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除IC"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/ic/deleteIc [delete]
export const deleteIcByIds = (params) => {
  return service({
    url: '/bi/ic/deleteIcByIds',
    method: 'delete',
    params
  })
}

// @Tags Ic
// @Summary 更新IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Ic true "更新IC"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/ic/updateIc [put]
export const updateIc = (data) => {
  return service({
    url: '/bi/ic/updateIc',
    method: 'put',
    data
  })
}

// @Tags Ic
// @Summary 用id查询IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Ic true "用id查询IC"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/ic/findIc [get]
export const findIc = (params) => {
  return service({
    url: '/bi/ic/findIc',
    method: 'get',
    params
  })
}

// @Tags Ic
// @Summary 分页获取IC列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取IC列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/ic/getIcList [get]
export const getIcList = (params) => {
  return service({
    url: '/bi/ic/getIcList',
    method: 'get',
    params
  })
}

// @Tags Ic
// @Summary 不需要鉴权的IC接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.IcSearch true "分页获取IC列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/ic/getIcPublic [get]
export const getIcPublic = () => {
  return service({
    url: '/bi/ic/getIcPublic',
    method: 'get',
  })
}
