import service from '@/utils/request';

// @Tags Page
// @Summary 创建单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Page true "创建单页"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cms/page/createPage [post]
export const createPage = data => {
  return service({
    url: '/cms/page/createPage',
    method: 'post',
    data,
  });
};

// @Tags Page
// @Summary 删除单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Page true "删除单页"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/page/deletePage [delete]
export const deletePage = params => {
  return service({
    url: '/cms/page/deletePage',
    method: 'delete',
    params,
  });
};

// @Tags Page
// @Summary 批量删除单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除单页"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/page/deletePageByIds [delete]
export const deletePageByIds = params => {
  return service({
    url: '/cms/page/deletePageByIds',
    method: 'delete',
    params,
  });
};

// @Tags Page
// @Summary 更新单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Page true "更新单页"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cms/page/updatePage [put]
export const updatePage = data => {
  return service({
    url: '/cms/page/updatePage',
    method: 'put',
    data,
  });
};

// @Tags Page
// @Summary 用id查询单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Page true "用id查询单页"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cms/page/findPage [get]
export const findPage = params => {
  return service({
    url: '/cms/page/findPage',
    method: 'get',
    params,
  });
};

// @Tags Page
// @Summary 分页获取单页列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取单页列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cms/page/getPageList [get]
export const getPageList = params => {
  return service({
    url: '/cms/page/getPageList',
    method: 'get',
    params,
  });
};

// @Tags Page
// @Summary 不需要鉴权的单页接口
// @Accept application/json
// @Produce application/json
// @Param data query request.PageSearch true "分页获取单页列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cms/page/getPagePublic [get]
export const getPagePublic = () => {
  return service({
    url: '/cms/page/getPagePublic',
    method: 'get',
  });
};
