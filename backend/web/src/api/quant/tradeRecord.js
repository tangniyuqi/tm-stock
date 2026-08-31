import service from '@/utils/request'
// @Tags TradeRecord
// @Summary 创建交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.TradeRecord true "创建交易记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/tradeRecord/createTradeRecord [post]
export const createTradeRecord = (data) => {
  return service({
    url: '/quant/tradeRecord/createTradeRecord',
    method: 'post',
    data
  })
}

// @Tags TradeRecord
// @Summary 删除交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.TradeRecord true "删除交易记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/tradeRecord/deleteTradeRecord [delete]
export const deleteTradeRecord = (params) => {
  return service({
    url: '/quant/tradeRecord/deleteTradeRecord',
    method: 'delete',
    params
  })
}

// @Tags TradeRecord
// @Summary 批量删除交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除交易记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/tradeRecord/deleteTradeRecord [delete]
export const deleteTradeRecordByIds = (params) => {
  return service({
    url: '/quant/tradeRecord/deleteTradeRecordByIds',
    method: 'delete',
    params
  })
}

// @Tags TradeRecord
// @Summary 更新交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.TradeRecord true "更新交易记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/tradeRecord/updateTradeRecord [put]
export const updateTradeRecord = (data) => {
  return service({
    url: '/quant/tradeRecord/updateTradeRecord',
    method: 'put',
    data
  })
}

// @Tags TradeRecord
// @Summary 用id查询交易记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.TradeRecord true "用id查询交易记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/tradeRecord/findTradeRecord [get]
export const findTradeRecord = (params) => {
  return service({
    url: '/quant/tradeRecord/findTradeRecord',
    method: 'get',
    params
  })
}

// @Tags TradeRecord
// @Summary 分页获取交易记录列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取交易记录列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/tradeRecord/getTradeRecordList [get]
export const getTradeRecordList = (params) => {
  return service({
    url: '/quant/tradeRecord/getTradeRecordList',
    method: 'get',
    params
  })
}

// @Tags TradeRecord
// @Summary 不需要鉴权的交易记录接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.TradeRecordSearch true "分页获取交易记录列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/tradeRecord/getTradeRecordPublic [get]
export const getTradeRecordPublic = () => {
  return service({
    url: '/quant/tradeRecord/getTradeRecordPublic',
    method: 'get',
  })
}
