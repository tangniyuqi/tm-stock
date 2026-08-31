import service from '@/utils/request';
// @Tags BaseStock
// @Summary 创建基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.BaseStock true "创建基础股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/baseStock/createBaseStock [post]
export const createBaseStock = data => {
  return service({
    url: '/quant/baseStock/createBaseStock',
    method: 'post',
    data,
  });
};

// @Tags BaseStock
// @Summary 删除基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.BaseStock true "删除基础股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/baseStock/deleteBaseStock [delete]
export const deleteBaseStock = params => {
  return service({
    url: '/quant/baseStock/deleteBaseStock',
    method: 'delete',
    params,
  });
};

// @Tags BaseStock
// @Summary 批量删除基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除基础股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/baseStock/deleteBaseStock [delete]
export const deleteBaseStockByIds = params => {
  return service({
    url: '/quant/baseStock/deleteBaseStockByIds',
    method: 'delete',
    params,
  });
};

// @Tags BaseStock
// @Summary 更新基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.BaseStock true "更新基础股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/baseStock/updateBaseStock [put]
export const updateBaseStock = data => {
  return service({
    url: '/quant/baseStock/updateBaseStock',
    method: 'put',
    data,
  });
};

// @Tags BaseStock
// @Summary 用id查询基础股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.BaseStock true "用id查询基础股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/baseStock/findBaseStock [get]
export const findBaseStock = params => {
  return service({
    url: '/quant/baseStock/findBaseStock',
    method: 'get',
    params,
  });
};

// @Tags BaseStock
// @Summary 分页获取基础股票列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取基础股票列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/baseStock/getBaseStockList [get]
export const getBaseStockList = params => {
  return service({
    url: '/quant/baseStock/getBaseStockList',
    method: 'get',
    params,
  });
};

// @Tags BaseStock
// @Summary 不需要鉴权的基础股票接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.BaseStockSearch true "分页获取基础股票列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/baseStock/getBaseStockPublic [get]
export const getBaseStockPublic = params => {
  return service({
    url: '/quant/baseStock/getBaseStockPublic',
    method: 'get',
    params,
  });
};

// @Tags BaseStock
// @Summary 同步基础股票数据
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.SyncReq true "同步基础股票数据参数"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"同步成功"}"
// @Router /quant/baseStock/sync [get]
export const sync = () => {
  return service({
    url: '/quant/baseStock/sync',
    method: 'get',
  });
};

// @Tags BaseStock
// @Summary 清除全部基础股票数据
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param params body model.ClearReq true "清除全部确认参数"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"清除成功"}"
// @Router /quant/baseStock/clear [delete]
export const clear = () => {
  return service({
    url: '/quant/baseStock/clear',
    method: 'delete',
  });
};

// @Tags BaseStock
// @Summary AI自动分析股票基本面
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiAnalyzeStockReq true "AI自动分析股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "分析完成"
// @Router /quant/baseStock/aiAnalyzeStocks [post]
export const aiAnalyzeStocks = (data, config = {}) => {
  return service({
    url: '/quant/baseStock/aiAnalyzeStocks',
    method: 'post',
    data,
    // AI 分析耗时长，默认不显示全局 loading，避免整个页面被遮罩；如需显示可传 donNotShowLoading: false
    donNotShowLoading: true,
    // 支持透传 axios 配置（如 signal 用于取消请求）
    ...config,
  });
};

// @Tags BaseStock
// @Summary 一键更新全部股票涨跌幅（通过 Tushare 获取最近交易日行情）
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/baseStock/updateAllChangePct [post]
export const updateAllChangePct = () => {
  return service({
    url: '/quant/baseStock/updateAllChangePct',
    method: 'post',
  });
};
