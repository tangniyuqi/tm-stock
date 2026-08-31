import service from '@/utils/request'
// @Tags Config
// @Summary 创建配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Config true "创建配置"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/config/createConfig [post]
export const createConfig = (data) => {
  return service({
    url: '/quant/config/createConfig',
    method: 'post',
    data
  })
}

// @Tags Config
// @Summary 删除配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Config true "删除配置"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/config/deleteConfig [delete]
export const deleteConfig = (params) => {
  return service({
    url: '/quant/config/deleteConfig',
    method: 'delete',
    params
  })
}

// @Tags Config
// @Summary 批量删除配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除配置"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/config/deleteConfig [delete]
export const deleteConfigByIds = (params) => {
  return service({
    url: '/quant/config/deleteConfigByIds',
    method: 'delete',
    params
  })
}

// @Tags Config
// @Summary 更新配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Config true "更新配置"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/config/updateConfig [put]
export const updateConfig = (data) => {
  return service({
    url: '/quant/config/updateConfig',
    method: 'put',
    data
  })
}

// @Tags Config
// @Summary 用id查询配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Config true "用id查询配置"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/config/findConfig [get]
export const findConfig = (params) => {
  return service({
    url: '/quant/config/findConfig',
    method: 'get',
    params
  })
}

// @Tags Config
// @Summary 分页获取配置列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取配置列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/config/getConfigList [get]
export const getConfigList = (params) => {
  return service({
    url: '/quant/config/getConfigList',
    method: 'get',
    params
  })
}

// @Tags Config
// @Summary 不需要鉴权的配置接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ConfigSearch true "分页获取配置列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/config/getConfigPublic [get]
export const getConfigPublic = () => {
  return service({
    url: '/quant/config/getConfigPublic',
    method: 'get',
  })
}
