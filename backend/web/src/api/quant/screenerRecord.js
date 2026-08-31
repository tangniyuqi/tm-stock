import service from '@/utils/request';

// @Tags ScreenerRecord
// @Summary 创建AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ScreenerRecord true "创建AI选股器查询记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/screenerRecord/createScreenerRecord [post]
export const createScreenerRecord = (data) => {
  return service({
    url: '/quant/screenerRecord/createScreenerRecord',
    method: 'post',
    data
  });
};

// @Tags ScreenerRecord
// @Summary 删除AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ScreenerRecord true "删除AI选股器查询记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/screenerRecord/deleteScreenerRecord [delete]
export const deleteScreenerRecord = (params) => {
  return service({
    url: '/quant/screenerRecord/deleteScreenerRecord',
    method: 'delete',
    params
  });
};

// @Tags ScreenerRecord
// @Summary 批量删除AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除AI选股器查询记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/screenerRecord/deleteScreenerRecordByIds [delete]
export const deleteScreenerRecordByIds = (params) => {
  return service({
    url: '/quant/screenerRecord/deleteScreenerRecordByIds',
    method: 'delete',
    params
  });
};

// @Tags ScreenerRecord
// @Summary 更新AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ScreenerRecord true "更新AI选股器查询记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/screenerRecord/updateScreenerRecord [put]
export const updateScreenerRecord = (data) => {
  return service({
    url: '/quant/screenerRecord/updateScreenerRecord',
    method: 'put',
    data
  });
};

// @Tags ScreenerRecord
// @Summary 用id查询AI选股器查询记录
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.ScreenerRecord true "用id查询AI选股器查询记录"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/screenerRecord/findScreenerRecord [get]
export const findScreenerRecord = (params) => {
  return service({
    url: '/quant/screenerRecord/findScreenerRecord',
    method: 'get',
    params
  });
};

// @Tags ScreenerRecord
// @Summary 分页获取AI选股器查询记录列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取AI选股器查询记录列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/screenerRecord/getScreenerRecordList [get]
export const getScreenerRecordList = (params) => {
  return service({
    url: '/quant/screenerRecord/getScreenerRecordList',
    method: 'get',
    params
  });
};
