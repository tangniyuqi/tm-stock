import service from '@/utils/request';
// @Tags TradeTask
// @Summary 创建交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.TradeTask true "创建交易任务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/tradeTask/createTradeTask [post]
export const createTradeTask = (data) => {
  return service({
    url: '/quant/tradeTask/createTradeTask',
    method: 'post',
    data
  });
};

// @Tags TradeTask
// @Summary 删除交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.TradeTask true "删除交易任务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/tradeTask/deleteTradeTask [delete]
export const deleteTradeTask = (params) => {
  return service({
    url: '/quant/tradeTask/deleteTradeTask',
    method: 'delete',
    params
  });
};

// @Tags TradeTask
// @Summary 批量删除交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除交易任务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/tradeTask/deleteTradeTask [delete]
export const deleteTradeTaskByIds = (params) => {
  return service({
    url: '/quant/tradeTask/deleteTradeTaskByIds',
    method: 'delete',
    params
  });
};

// @Tags TradeTask
// @Summary 更新交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.TradeTask true "更新交易任务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/tradeTask/updateTradeTask [put]
export const updateTradeTask = (data) => {
  return service({
    url: '/quant/tradeTask/updateTradeTask',
    method: 'put',
    data
  });
};

// @Tags TradeTask
// @Summary 用id查询交易任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.TradeTask true "用id查询交易任务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/tradeTask/findTradeTask [get]
export const findTradeTask = (params) => {
  return service({
    url: '/quant/tradeTask/findTradeTask',
    method: 'get',
    params
  });
};

// @Tags TradeTask
// @Summary 分页获取交易任务列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取交易任务列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/tradeTask/getTradeTaskList [get]
export const getTradeTaskList = (params) => {
  return service({
    url: '/quant/tradeTask/getTradeTaskList',
    method: 'get',
    params
  });
};

// @Tags TradeTask
// @Summary 不需要鉴权的交易任务接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.TradeTaskSearch true "分页获取交易任务列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/tradeTask/getTradeTaskPublic [get]
export const getTradeTaskPublic = () => {
  return service({
    url: '/quant/tradeTask/getTradeTaskPublic',
    method: 'get'
  });
};
