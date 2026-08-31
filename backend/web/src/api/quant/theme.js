import service from '@/utils/request';
// @Tags Theme
// @Summary 创建题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Theme true "创建题材"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/theme/createTheme [post]
export const createTheme = data => {
  return service({
    url: '/quant/theme/createTheme',
    method: 'post',
    data,
  });
};

// @Tags Theme
// @Summary 删除题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Theme true "删除题材"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/theme/deleteTheme [delete]
export const deleteTheme = params => {
  return service({
    url: '/quant/theme/deleteTheme',
    method: 'delete',
    params,
  });
};

// @Tags Theme
// @Summary 批量删除题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除题材"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/theme/deleteTheme [delete]
export const deleteThemeByIds = params => {
  return service({
    url: '/quant/theme/deleteThemeByIds',
    method: 'delete',
    params,
  });
};

// @Tags Theme
// @Summary 更新题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Theme true "更新题材"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/theme/updateTheme [put]
export const updateTheme = data => {
  return service({
    url: '/quant/theme/updateTheme',
    method: 'put',
    data,
  });
};

// @Tags Theme
// @Summary 用id查询题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Theme true "用id查询题材"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/theme/findTheme [get]
export const findTheme = params => {
  return service({
    url: '/quant/theme/findTheme',
    method: 'get',
    params,
  });
};

// @Tags Theme
// @Summary 分页获取题材列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取题材列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/theme/getThemeList [get]
export const getThemeList = params => {
  return service({
    url: '/quant/theme/getThemeList',
    method: 'get',
    params,
  });
};

// @Tags Theme
// @Summary 不需要鉴权的题材接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ThemeSearch true "分页获取题材列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/theme/getThemePublic [get]
export const getThemePublic = () => {
  return service({
    url: '/quant/theme/getThemePublic',
    method: 'get',
  });
};
