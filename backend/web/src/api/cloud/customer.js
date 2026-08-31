import service from '@/utils/request'
// @Tags Customer
// @Summary 创建客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Customer true "创建客户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cloud/customer/createCustomer [post]
export const createCustomer = (data) => {
  return service({
    url: '/cloud/customer/createCustomer',
    method: 'post',
    data
  })
}

// @Tags Customer
// @Summary 删除客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Customer true "删除客户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/customer/deleteCustomer [delete]
export const deleteCustomer = (params) => {
  return service({
    url: '/cloud/customer/deleteCustomer',
    method: 'delete',
    params
  })
}

// @Tags Customer
// @Summary 批量删除客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除客户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/customer/deleteCustomer [delete]
export const deleteCustomerByIds = (params) => {
  return service({
    url: '/cloud/customer/deleteCustomerByIds',
    method: 'delete',
    params
  })
}

// @Tags Customer
// @Summary 更新客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Customer true "更新客户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cloud/customer/updateCustomer [put]
export const updateCustomer = (data) => {
  return service({
    url: '/cloud/customer/updateCustomer',
    method: 'put',
    data
  })
}

// @Tags Customer
// @Summary 用id查询客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Customer true "用id查询客户"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cloud/customer/findCustomer [get]
export const findCustomer = (params) => {
  return service({
    url: '/cloud/customer/findCustomer',
    method: 'get',
    params
  })
}

// @Tags Customer
// @Summary 分页获取客户列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取客户列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cloud/customer/getCustomerList [get]
export const getCustomerList = (params) => {
  return service({
    url: '/cloud/customer/getCustomerList',
    method: 'get',
    params
  })
}

// @Tags Customer
// @Summary 不需要鉴权的客户接口
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.CustomerSearch true "分页获取客户列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cloud/customer/getCustomerPublic [get]
export const getCustomerPublic = () => {
  return service({
    url: '/cloud/customer/getCustomerPublic',
    method: 'get',
  })
}
