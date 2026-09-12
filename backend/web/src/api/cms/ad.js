import service from '@/utils/request';

// @Tags Ad
// @Summary 创建广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Ad true "创建广告表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cms/ad/createAd [post]
export const createAd = data => {
  return service({
    url: '/cms/ad/createAd',
    method: 'post',
    data,
  });
};

// @Tags Ad
// @Summary 删除广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Ad true "删除广告表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/ad/deleteAd [delete]
export const deleteAd = params => {
  return service({
    url: '/cms/ad/deleteAd',
    method: 'delete',
    params,
  });
};

// @Tags Ad
// @Summary 批量删除广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除广告表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/ad/deleteAdByIds [delete]
export const deleteAdByIds = params => {
  return service({
    url: '/cms/ad/deleteAdByIds',
    method: 'delete',
    params,
  });
};

// @Tags Ad
// @Summary 更新广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Ad true "更新广告表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cms/ad/updateAd [put]
export const updateAd = data => {
  return service({
    url: '/cms/ad/updateAd',
    method: 'put',
    data,
  });
};

// @Tags Ad
// @Summary 用id查询广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Ad true "用id查询广告表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cms/ad/findAd [get]
export const findAd = params => {
  return service({
    url: '/cms/ad/findAd',
    method: 'get',
    params,
  });
};

// @Tags Ad
// @Summary 分页获取广告表列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取广告表列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cms/ad/getAdList [get]
export const getAdList = params => {
  return service({
    url: '/cms/ad/getAdList',
    method: 'get',
    params,
  });
};

// @Tags Ad
// @Summary 不需要鉴权的广告表接口
// @Accept application/json
// @Produce application/json
// @Param data query request.AdSearch true "分页获取广告表列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cms/ad/getAdPublic [get]
export const getAdPublic = () => {
  return service({
    url: '/cms/ad/getAdPublic',
    method: 'get',
  });
};
