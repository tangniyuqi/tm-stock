import service from '@/utils/request'
// @Tags Weishi
// @Summary 创建微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Weishi true "创建微视账号"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /weishi/createWeishi [post]
export const createWeishi = (data) => {
  return service({
    url: '/weishi/createWeishi',
    method: 'post',
    data
  })
}

// @Tags Weishi
// @Summary 删除微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Weishi true "删除微视账号"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /weishi/deleteWeishi [delete]
export const deleteWeishi = (params) => {
  return service({
    url: '/weishi/deleteWeishi',
    method: 'delete',
    params
  })
}

// @Tags Weishi
// @Summary 批量删除微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除微视账号"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /weishi/deleteWeishi [delete]
export const deleteWeishiByIds = (params) => {
  return service({
    url: '/weishi/deleteWeishiByIds',
    method: 'delete',
    params
  })
}

// @Tags Weishi
// @Summary 更新微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Weishi true "更新微视账号"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /weishi/updateWeishi [put]
export const updateWeishi = (data) => {
  return service({
    url: '/weishi/updateWeishi',
    method: 'put',
    data
  })
}

// @Tags Weishi
// @Summary 用id查询微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Weishi true "用id查询微视账号"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /weishi/findWeishi [get]
export const findWeishi = (params) => {
  return service({
    url: '/weishi/findWeishi',
    method: 'get',
    params
  })
}

// @Tags Weishi
// @Summary 分页获取微视账号列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取微视账号列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /weishi/getWeishiList [get]
export const getWeishiList = (params) => {
  return service({
    url: '/weishi/getWeishiList',
    method: 'get',
    params
  })
}

// @Tags Weishi
// @Summary 不需要鉴权的微视账号接口
// @Accept application/json
// @Produce application/json
// @Param data query addonReq.WeishiSearch true "分页获取微视账号列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /weishi/getWeishiPublic [get]
export const getWeishiPublic = () => {
  return service({
    url: '/weishi/getWeishiPublic',
    method: 'get',
  })
}
// BatchCreateWeishi 用于批量创建
// @Tags Weishi
// @Summary 用于批量创建
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /weishi/batchCreateWeishi [POST]
export const batchCreateWeishi = (data) => {
  return service({
    url: '/weishi/batchCreateWeishi',
    method: 'POST',
    data
  })
}
// CancelWeishi 注销微视账号
// @Tags Weishi
// @Summary 注销微视账号
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /weishi/cancelWeishi [POST]
export const cancelWeishi = (data) => {
  return service({
    url: '/weishi/cancelWeishi',
    method: 'POST',
    data
  })
}
