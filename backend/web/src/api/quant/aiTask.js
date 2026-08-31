import service from '@/utils/request';

// @Tags ThemeStock
// @Summary 根据ID获取AI执行任务（进度/日志/结果）
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询AI执行任务"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/aiTask/getAiTask [get]
export const getAiTask = params => {
  return service({
    url: '/quant/aiTask/getAiTask',
    method: 'get',
    params,
  });
};

// @Tags ThemeStock
// @Summary 分页获取AI执行任务列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.AiTaskSearch true "分页获取AI执行任务列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/aiTask/getAiTaskList [get]
export const getAiTaskList = params => {
  return service({
    url: '/quant/aiTask/getAiTaskList',
    method: 'get',
    params,
  });
};

// @Tags ThemeStock
// @Summary 停止运行中的AI执行任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.AiTaskActionReq true "任务ID"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"停止成功"}"
// @Router /quant/aiTask/stopAiTask [post]
export const stopAiTask = data => {
  return service({
    url: '/quant/aiTask/stopAiTask',
    method: 'post',
    data,
  });
};

// @Tags ThemeStock
// @Summary 重启已取消的AI执行任务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.AiTaskActionReq true "任务ID"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"重启成功"}"
// @Router /quant/aiTask/restartAiTask [post]
export const restartAiTask = data => {
  return service({
    url: '/quant/aiTask/restartAiTask',
    method: 'post',
    data,
  });
};
