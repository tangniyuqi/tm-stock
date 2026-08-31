import service from '@/utils/request'
// @Tags Account
// @Summary 创建财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Account true "创建财务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cloud/account/createAccount [post]
export const createAccount = (data) => {
  return service({
    url: '/cloud/account/createAccount',
    method: 'post',
    data
  })
}

// @Tags Account
// @Summary 删除财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Account true "删除财务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/account/deleteAccount [delete]
export const deleteAccount = (params) => {
  return service({
    url: '/cloud/account/deleteAccount',
    method: 'delete',
    params
  })
}

// @Tags Account
// @Summary 批量删除财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除财务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/account/deleteAccount [delete]
export const deleteAccountByIds = (params) => {
  return service({
    url: '/cloud/account/deleteAccountByIds',
    method: 'delete',
    params
  })
}

// @Tags Account
// @Summary 更新财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Account true "更新财务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cloud/account/updateAccount [put]
export const updateAccount = (data) => {
  return service({
    url: '/cloud/account/updateAccount',
    method: 'put',
    data
  })
}

// @Tags Account
// @Summary 用id查询财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Account true "用id查询财务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cloud/account/findAccount [get]
export const findAccount = (params) => {
  return service({
    url: '/cloud/account/findAccount',
    method: 'get',
    params
  })
}

// @Tags Account
// @Summary 分页获取财务列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取财务列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cloud/account/getAccountList [get]
export const getAccountList = (params) => {
  return service({
    url: '/cloud/account/getAccountList',
    method: 'get',
    params
  })
}

// @Tags Account
// @Summary 不需要鉴权的财务接口
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.AccountSearch true "分页获取财务列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cloud/account/getAccountPublic [get]
export const getAccountPublic = () => {
  return service({
    url: '/cloud/account/getAccountPublic',
    method: 'get',
  })
}
