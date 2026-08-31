import service from '@/utils/request'
// @Tags Deal
// @Summary 创建商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Deal true "创建商机"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cloud/deal/createDeal [post]
export const createDeal = (data) => {
  return service({
    url: '/cloud/deal/createDeal',
    method: 'post',
    data
  })
}

// @Tags Deal
// @Summary 删除商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Deal true "删除商机"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/deal/deleteDeal [delete]
export const deleteDeal = (params) => {
  return service({
    url: '/cloud/deal/deleteDeal',
    method: 'delete',
    params
  })
}

// @Tags Deal
// @Summary 批量删除商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除商机"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/deal/deleteDeal [delete]
export const deleteDealByIds = (params) => {
  return service({
    url: '/cloud/deal/deleteDealByIds',
    method: 'delete',
    params
  })
}

// @Tags Deal
// @Summary 更新商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Deal true "更新商机"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cloud/deal/updateDeal [put]
export const updateDeal = (data) => {
  return service({
    url: '/cloud/deal/updateDeal',
    method: 'put',
    data
  })
}

// @Tags Deal
// @Summary 用id查询商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Deal true "用id查询商机"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cloud/deal/findDeal [get]
export const findDeal = (params) => {
  return service({
    url: '/cloud/deal/findDeal',
    method: 'get',
    params
  })
}

// @Tags Deal
// @Summary 分页获取商机列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取商机列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cloud/deal/getDealList [get]
export const getDealList = (params) => {
  return service({
    url: '/cloud/deal/getDealList',
    method: 'get',
    params
  })
}

// @Tags Deal
// @Summary 不需要鉴权的商机接口
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.DealSearch true "分页获取商机列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cloud/deal/getDealPublic [get]
export const getDealPublic = () => {
  return service({
    url: '/cloud/deal/getDealPublic',
    method: 'get',
  })
}
