import service from '@/utils/request';
// @Tags ThemeStock
// @Summary 创建题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ThemeStock true "创建题材股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/themeStock/createThemeStock [post]
export const createThemeStock = data => {
  return service({
    url: '/quant/themeStock/createThemeStock',
    method: 'post',
    data,
  });
};

// @Tags ThemeStock
// @Summary 删除题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ThemeStock true "删除题材股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/themeStock/deleteThemeStock [delete]
export const deleteThemeStock = params => {
  return service({
    url: '/quant/themeStock/deleteThemeStock',
    method: 'delete',
    params,
  });
};

// @Tags ThemeStock
// @Summary 批量删除题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除题材股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/themeStock/deleteThemeStock [delete]
export const deleteThemeStockByIds = params => {
  return service({
    url: '/quant/themeStock/deleteThemeStockByIds',
    method: 'delete',
    params,
  });
};

// @Tags ThemeStock
// @Summary 更新题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ThemeStock true "更新题材股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/themeStock/updateThemeStock [put]
export const updateThemeStock = data => {
  return service({
    url: '/quant/themeStock/updateThemeStock',
    method: 'put',
    data,
  });
};

// @Tags ThemeStock
// @Summary 用id查询题材股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.ThemeStock true "用id查询题材股票"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/themeStock/findThemeStock [get]
export const findThemeStock = params => {
  return service({
    url: '/quant/themeStock/findThemeStock',
    method: 'get',
    params,
  });
};

// @Tags ThemeStock
// @Summary 分页获取题材股票列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取题材股票列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/themeStock/getThemeStockList [get]
export const getThemeStockList = params => {
  return service({
    url: '/quant/themeStock/getThemeStockList',
    method: 'get',
    params,
  });
};

// @Tags ThemeStock
// @Summary 不需要鉴权的题材股票接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ThemeStockSearch true "分页获取题材股票列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/themeStock/getThemeStockPublic [get]
export const getThemeStockPublic = () => {
  return service({
    url: '/quant/themeStock/getThemeStockPublic',
    method: 'get',
  });
};

// @Tags ThemeStock
// @Summary AI智能为题材添加股票
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiAddThemeStockReq true "AI智能添加题材股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "添加成功"
// @Router /quant/themeStock/aiAddThemeStocks [post]
export const aiAddThemeStocks = (data, config = {}) => {
  return service({
    url: '/quant/themeStock/aiAddThemeStocks',
    method: 'post',
    data,
    // AI 选股耗时长，不显示全局 loading，避免整个页面被遮罩
    donNotShowLoading: true,
    // 支持透传 axios 配置（如 signal 用于取消请求）
    ...config,
  });
};

// @Tags ThemeStock
// @Summary AI一键更新个股的入选逻辑、梯队、相关度
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiUpdateThemeStockReq true "AI更新题材股票请求"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /quant/themeStock/aiUpdateThemeStock [post]
export const aiUpdateThemeStock = (data, config = {}) => {
  return service({
    url: '/quant/themeStock/aiUpdateThemeStock',
    method: 'post',
    data,
    // AI 更新耗时长，不显示全局 loading，避免整个页面被遮罩
    donNotShowLoading: true,
    // 支持透传 axios 配置（如 signal 用于取消请求）
    ...config,
  });
};

// @Tags ThemeStock
// @Summary AI批量更新所选个股的入选逻辑、梯队、相关度
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quantReq.AiUpdateThemeStocksReq true "AI批量更新题材股票请求"
// @Success 200 {object} response.Response{data=object,msg=string} "批量更新完成"
// @Router /quant/themeStock/aiUpdateThemeStocks [post]
export const aiUpdateThemeStocks = (data, config = {}) => {
  return service({
    url: '/quant/themeStock/aiUpdateThemeStocks',
    method: 'post',
    data,
    // AI 批量更新耗时长，不显示全局 loading，避免整个页面被遮罩
    donNotShowLoading: true,
    // 支持透传 axios 配置（如 signal 用于取消请求）
    ...config,
  });
};
