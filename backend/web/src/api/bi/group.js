import service from '@/utils/request'
// @Tags Group
// @Summary 创建群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Group true "创建群组"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/group/createGroup [post]
export const createGroup = (data) => {
  return service({
    url: '/bi/group/createGroup',
    method: 'post',
    data
  })
}

// @Tags Group
// @Summary 删除群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Group true "删除群组"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/group/deleteGroup [delete]
export const deleteGroup = (params) => {
  return service({
    url: '/bi/group/deleteGroup',
    method: 'delete',
    params
  })
}

// @Tags Group
// @Summary 批量删除群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除群组"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/group/deleteGroup [delete]
export const deleteGroupByIds = (params) => {
  return service({
    url: '/bi/group/deleteGroupByIds',
    method: 'delete',
    params
  })
}

// @Tags Group
// @Summary 更新群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Group true "更新群组"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/group/updateGroup [put]
export const updateGroup = (data) => {
  return service({
    url: '/bi/group/updateGroup',
    method: 'put',
    data
  })
}

// @Tags Group
// @Summary 用id查询群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Group true "用id查询群组"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/group/findGroup [get]
export const findGroup = (params) => {
  return service({
    url: '/bi/group/findGroup',
    method: 'get',
    params
  })
}

// @Tags Group
// @Summary 分页获取群组列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取群组列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/group/getGroupList [get]
export const getGroupList = (params) => {
  return service({
    url: '/bi/group/getGroupList',
    method: 'get',
    params
  })
}

// @Tags Group
// @Summary 不需要鉴权的群组接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.GroupSearch true "分页获取群组列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/group/getGroupPublic [get]
export const getGroupPublic = () => {
  return service({
    url: '/bi/group/getGroupPublic',
    method: 'get',
  })
}
