import service from '@/utils/request';

// @Tags Feedback
// @Summary 创建反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Feedback true "创建反馈"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cms/feedback/createFeedback [post]
export const createFeedback = data => {
  return service({
    url: '/cms/feedback/createFeedback',
    method: 'post',
    data,
  });
};

// @Tags Feedback
// @Summary 删除反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Feedback true "删除反馈"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/feedback/deleteFeedback [delete]
export const deleteFeedback = params => {
  return service({
    url: '/cms/feedback/deleteFeedback',
    method: 'delete',
    params,
  });
};

// @Tags Feedback
// @Summary 批量删除反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除反馈"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/feedback/deleteFeedbackByIds [delete]
export const deleteFeedbackByIds = params => {
  return service({
    url: '/cms/feedback/deleteFeedbackByIds',
    method: 'delete',
    params,
  });
};

// @Tags Feedback
// @Summary 更新反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Feedback true "更新反馈"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cms/feedback/updateFeedback [put]
export const updateFeedback = data => {
  return service({
    url: '/cms/feedback/updateFeedback',
    method: 'put',
    data,
  });
};

// @Tags Feedback
// @Summary 用id查询反馈
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Feedback true "用id查询反馈"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cms/feedback/findFeedback [get]
export const findFeedback = params => {
  return service({
    url: '/cms/feedback/findFeedback',
    method: 'get',
    params,
  });
};

// @Tags Feedback
// @Summary 分页获取反馈列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取反馈列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cms/feedback/getFeedbackList [get]
export const getFeedbackList = params => {
  return service({
    url: '/cms/feedback/getFeedbackList',
    method: 'get',
    params,
  });
};

// @Tags Feedback
// @Summary 不需要鉴权的反馈接口
// @Accept application/json
// @Produce application/json
// @Param data query request.FeedbackSearch true "分页获取反馈列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cms/feedback/getFeedbackPublic [get]
export const getFeedbackPublic = () => {
  return service({
    url: '/cms/feedback/getFeedbackPublic',
    method: 'get',
  });
};
