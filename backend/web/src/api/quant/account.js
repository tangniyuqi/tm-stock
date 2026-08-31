import service from '@/utils/request';
// @Tags Account
// @Summary 创建账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Account true "创建账户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/account/createAccount [post]
export const createAccount = (data) => {
  return service({
    url: '/quant/account/createAccount',
    method: 'post',
    data
  });
};

// @Tags Account
// @Summary 删除账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Account true "删除账户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/account/deleteAccount [delete]
export const deleteAccount = (params) => {
  return service({
    url: '/quant/account/deleteAccount',
    method: 'delete',
    params
  });
};

// @Tags Account
// @Summary 批量删除账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除账户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/account/deleteAccount [delete]
export const deleteAccountByIds = (params) => {
  return service({
    url: '/quant/account/deleteAccountByIds',
    method: 'delete',
    params
  });
};

// @Tags Account
// @Summary 更新账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Account true "更新账户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/account/updateAccount [put]
export const updateAccount = (data) => {
  return service({
    url: '/quant/account/updateAccount',
    method: 'put',
    data
  });
};

// @Tags Account
// @Summary 用id查询账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Account true "用id查询账户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/account/findAccount [get]
export const findAccount = (params) => {
  return service({
    url: '/quant/account/findAccount',
    method: 'get',
    params
  });
};

// @Tags Account
// @Summary 分页获取账户列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取账户列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/account/getAccountList [get]
export const getAccountList = (params) => {
  return service({
    url: '/quant/account/getAccountList',
    method: 'get',
    params
  });
};

// @Tags Account
// @Summary 不需要鉴权的账户接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.AccountSearch true "分页获取账户列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/account/getAccountPublic [get]
export const getAccountPublic = () => {
  return service({
    url: '/quant/account/getAccountPublic',
    method: 'get'
  });
};

// @Tags Account
// @Summary 获取我的账户
// @Security ApiKeyAuth
// @Produce application/json
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/account/getMyAccount [get]
export const getMyAccount = () => {
  return service({
    url: '/quant/account/getMyAccount',
    method: 'get'
  });
};
