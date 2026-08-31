import service from '@/utils/request';
// @Tags ThemeTopic
// @Summary 创建动态
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ThemeTopic true "创建动态"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/themeTopic/createThemeTopic [post]
export const createThemeTopic = data => {
  return service({
    url: '/quant/themeTopic/createThemeTopic',
    method: 'post',
    data,
  });
};

// @Tags ThemeTopic
// @Summary 删除动态
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ThemeTopic true "删除动态"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/themeTopic/deleteThemeTopic [delete]
export const deleteThemeTopic = params => {
  return service({
    url: '/quant/themeTopic/deleteThemeTopic',
    method: 'delete',
    params,
  });
};

// @Tags ThemeTopic
// @Summary 批量删除动态
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除动态"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/themeTopic/deleteThemeTopic [delete]
export const deleteThemeTopicByIds = params => {
  return service({
    url: '/quant/themeTopic/deleteThemeTopicByIds',
    method: 'delete',
    params,
  });
};

// @Tags ThemeTopic
// @Summary 更新动态
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.ThemeTopic true "更新动态"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/themeTopic/updateThemeTopic [put]
export const updateThemeTopic = data => {
  return service({
    url: '/quant/themeTopic/updateThemeTopic',
    method: 'put',
    data,
  });
};

// @Tags ThemeTopic
// @Summary 用id查询动态
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.ThemeTopic true "用id查询动态"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/themeTopic/findThemeTopic [get]
export const findThemeTopic = params => {
  return service({
    url: '/quant/themeTopic/findThemeTopic',
    method: 'get',
    params,
  });
};

// @Tags ThemeTopic
// @Summary 分页获取动态列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取动态列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/themeTopic/getThemeTopicList [get]
export const getThemeTopicList = params => {
  return service({
    url: '/quant/themeTopic/getThemeTopicList',
    method: 'get',
    params,
  });
};

// @Tags ThemeTopic
// @Summary 不需要鉴权的动态接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ThemeTopicSearch true "分页获取动态列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/themeTopic/getThemeTopicPublic [get]
export const getThemeTopicPublic = () => {
  return service({
    url: '/quant/themeTopic/getThemeTopicPublic',
    method: 'get',
  });
};
