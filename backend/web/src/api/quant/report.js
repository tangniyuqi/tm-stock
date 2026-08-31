import service from '@/utils/request'
// @Tags Report
// @Summary 创建研报
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Report true "创建研报"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/report/createReport [post]
export const createReport = (data) => {
  return service({
    url: '/quant/report/createReport',
    method: 'post',
    data
  })
}

// @Tags Report
// @Summary 删除研报
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Report true "删除研报"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/report/deleteReport [delete]
export const deleteReport = (params) => {
  return service({
    url: '/quant/report/deleteReport',
    method: 'delete',
    params
  })
}

// @Tags Report
// @Summary 批量删除研报
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除研报"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/report/deleteReport [delete]
export const deleteReportByIds = (params) => {
  return service({
    url: '/quant/report/deleteReportByIds',
    method: 'delete',
    params
  })
}

// @Tags Report
// @Summary 更新研报
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Report true "更新研报"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/report/updateReport [put]
export const updateReport = (data) => {
  return service({
    url: '/quant/report/updateReport',
    method: 'put',
    data
  })
}

// @Tags Report
// @Summary 用id查询研报
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Report true "用id查询研报"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/report/findReport [get]
export const findReport = (params) => {
  return service({
    url: '/quant/report/findReport',
    method: 'get',
    params
  })
}

// @Tags Report
// @Summary 分页获取研报列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取研报列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/report/getReportList [get]
export const getReportList = (params) => {
  return service({
    url: '/quant/report/getReportList',
    method: 'get',
    params
  })
}

// @Tags Report
// @Summary 不需要鉴权的研报接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ReportSearch true "分页获取研报列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/report/getReportPublic [get]
export const getReportPublic = () => {
  return service({
    url: '/quant/report/getReportPublic',
    method: 'get',
  })
}
