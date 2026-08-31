import service from '@/utils/request';
// @Tags News
// @Summary 创建快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.News true "创建快讯"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/news/createNews [post]
export const createNews = data => {
  return service({
    url: '/quant/news/createNews',
    method: 'post',
    data,
  });
};

// @Tags News
// @Summary 删除快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.News true "删除快讯"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/news/deleteNews [delete]
export const deleteNews = params => {
  return service({
    url: '/quant/news/deleteNews',
    method: 'delete',
    params,
  });
};

// @Tags News
// @Summary 批量删除快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除快讯"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/news/deleteNews [delete]
export const deleteNewsByIds = params => {
  return service({
    url: '/quant/news/deleteNewsByIds',
    method: 'delete',
    params,
  });
};

// @Tags News
// @Summary 更新快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.News true "更新快讯"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/news/updateNews [put]
export const updateNews = data => {
  return service({
    url: '/quant/news/updateNews',
    method: 'put',
    data,
  });
};

// @Tags News
// @Summary 用id查询快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.News true "用id查询快讯"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/news/findNews [get]
export const findNews = params => {
  return service({
    url: '/quant/news/findNews',
    method: 'get',
    params,
  });
};

// @Tags News
// @Summary 分页获取快讯列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取快讯列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/news/getNewsList [get]
export const getNewsList = params => {
  return service({
    url: '/quant/news/getNewsList',
    method: 'get',
    params,
  });
};

// @Tags News
// @Summary 不需要鉴权的快讯接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.NewsSearch true "分页获取快讯列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/news/getNewsPublic [get]
export const getNewsPublic = () => {
  return service({
    url: '/quant/news/getNewsPublic',
    method: 'get',
  });
};
// CreatePublic 创建快讯（不鉴权）
// @Tags News
// @Summary 创建快讯（不鉴权）
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /quant/news/createPublic [POST]
export const createPublic = () => {
  return service({
    url: '/quant/news/createPublic',
    method: 'POST',
  });
};

// @Tags News
// @Summary 使用 Meilisearch 搜索快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "搜索快讯"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"搜索成功"}"
// @Router /quant/news/meilisearch [get]
export const meilisearch = params => {
  return service({
    url: '/quant/news/meilisearch',
    method: 'get',
    params,
  });
};

// @Tags News
// @Summary 全量同步历史数据到 Meilisearch
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {string} string "{"success":true,"data":{},"msg":"同步成功"}"
// @Router /quant/news/fullSyncToMeilisearch [post]
export const fullSyncToMeilisearch = () => {
  return service({
    url: '/quant/news/fullSyncToMeilisearch',
    method: 'post',
  });
};

// @Tags News
// @Summary 检查 MySQL 和 Meilisearch 数据一致性
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {string} string "{"success":true,"data":{},"msg":"检查成功"}"
// @Router /quant/news/checkMeilisearchConsistency [get]
export const checkMeilisearchConsistency = () => {
  return service({
    url: '/quant/news/checkMeilisearchConsistency',
    method: 'get',
  });
};
