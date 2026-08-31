import service from '@/utils/request'
// @Tags Member
// @Summary 创建用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Member true "创建用户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/member/createMember [post]
export const createMember = (data) => {
  return service({
    url: '/bi/member/createMember',
    method: 'post',
    data
  })
}

// @Tags Member
// @Summary 删除用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Member true "删除用户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/member/deleteMember [delete]
export const deleteMember = (params) => {
  return service({
    url: '/bi/member/deleteMember',
    method: 'delete',
    params
  })
}

// @Tags Member
// @Summary 批量删除用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除用户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/member/deleteMember [delete]
export const deleteMemberByIds = (params) => {
  return service({
    url: '/bi/member/deleteMemberByIds',
    method: 'delete',
    params
  })
}

// @Tags Member
// @Summary 更新用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Member true "更新用户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/member/updateMember [put]
export const updateMember = (data) => {
  return service({
    url: '/bi/member/updateMember',
    method: 'put',
    data
  })
}

// @Tags Member
// @Summary 用id查询用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Member true "用id查询用户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/member/findMember [get]
export const findMember = (params) => {
  return service({
    url: '/bi/member/findMember',
    method: 'get',
    params
  })
}

// @Tags Member
// @Summary 分页获取用户列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取用户列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/member/getMemberList [get]
export const getMemberList = (params) => {
  return service({
    url: '/bi/member/getMemberList',
    method: 'get',
    params
  })
}

// @Tags Member
// @Summary 不需要鉴权的用户接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.MemberSearch true "分页获取用户列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/member/getMemberPublic [get]
export const getMemberPublic = () => {
  return service({
    url: '/bi/member/getMemberPublic',
    method: 'get',
  })
}
