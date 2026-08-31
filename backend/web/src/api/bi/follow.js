import service from '@/utils/request'
// @Tags Follow
// @Summary 创建跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Follow true "创建跟进"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/follow/createFollow [post]
export const createFollow = (data) => {
  return service({
    url: '/bi/follow/createFollow',
    method: 'post',
    data
  })
}

// @Tags Follow
// @Summary 删除跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Follow true "删除跟进"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/follow/deleteFollow [delete]
export const deleteFollow = (params) => {
  return service({
    url: '/bi/follow/deleteFollow',
    method: 'delete',
    params
  })
}

// @Tags Follow
// @Summary 批量删除跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除跟进"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/follow/deleteFollow [delete]
export const deleteFollowByIds = (params) => {
  return service({
    url: '/bi/follow/deleteFollowByIds',
    method: 'delete',
    params
  })
}

// @Tags Follow
// @Summary 更新跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Follow true "更新跟进"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/follow/updateFollow [put]
export const updateFollow = (data) => {
  return service({
    url: '/bi/follow/updateFollow',
    method: 'put',
    data
  })
}

// @Tags Follow
// @Summary 用id查询跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Follow true "用id查询跟进"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/follow/findFollow [get]
export const findFollow = (params) => {
  return service({
    url: '/bi/follow/findFollow',
    method: 'get',
    params
  })
}

// @Tags Follow
// @Summary 分页获取跟进列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取跟进列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/follow/getFollowList [get]
export const getFollowList = (params) => {
  return service({
    url: '/bi/follow/getFollowList',
    method: 'get',
    params
  })
}

// @Tags Follow
// @Summary 不需要鉴权的跟进接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.FollowSearch true "分页获取跟进列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/follow/getFollowPublic [get]
export const getFollowPublic = () => {
  return service({
    url: '/bi/follow/getFollowPublic',
    method: 'get',
  })
}
