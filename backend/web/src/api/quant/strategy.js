import service from '@/utils/request'
// @Tags Strategy
// @Summary 创建策略
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Strategy true "创建策略"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/strategy/createStrategy [post]
export const createStrategy = (data) => {
  return service({
    url: '/quant/strategy/createStrategy',
    method: 'post',
    data
  })
}

// @Tags Strategy
// @Summary 删除策略
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Strategy true "删除策略"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/strategy/deleteStrategy [delete]
export const deleteStrategy = (params) => {
  return service({
    url: '/quant/strategy/deleteStrategy',
    method: 'delete',
    params
  })
}

// @Tags Strategy
// @Summary 批量删除策略
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除策略"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/strategy/deleteStrategy [delete]
export const deleteStrategyByIds = (params) => {
  return service({
    url: '/quant/strategy/deleteStrategyByIds',
    method: 'delete',
    params
  })
}

// @Tags Strategy
// @Summary 更新策略
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Strategy true "更新策略"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/strategy/updateStrategy [put]
export const updateStrategy = (data) => {
  return service({
    url: '/quant/strategy/updateStrategy',
    method: 'put',
    data
  })
}

// @Tags Strategy
// @Summary 用id查询策略
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Strategy true "用id查询策略"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/strategy/findStrategy [get]
export const findStrategy = (params) => {
  return service({
    url: '/quant/strategy/findStrategy',
    method: 'get',
    params
  })
}

// @Tags Strategy
// @Summary 分页获取策略列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取策略列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/strategy/getStrategyList [get]
export const getStrategyList = (params) => {
  return service({
    url: '/quant/strategy/getStrategyList',
    method: 'get',
    params
  })
}

// @Tags Strategy
// @Summary 不需要鉴权的策略接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.StrategySearch true "分页获取策略列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/strategy/getStrategyPublic [get]
export const getStrategyPublic = () => {
  return service({
    url: '/quant/strategy/getStrategyPublic',
    method: 'get',
  })
}
