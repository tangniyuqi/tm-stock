import service from '@/utils/request';

// @Tags Article
// @Summary 创建文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Article true "创建文章"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cms/article/createArticle [post]
export const createArticle = data => {
  return service({
    url: '/cms/article/createArticle',
    method: 'post',
    data,
  });
};

// @Tags Article
// @Summary 删除文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Article true "删除文章"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/article/deleteArticle [delete]
export const deleteArticle = params => {
  return service({
    url: '/cms/article/deleteArticle',
    method: 'delete',
    params,
  });
};

// @Tags Article
// @Summary 批量删除文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除文章"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cms/article/deleteArticleByIds [delete]
export const deleteArticleByIds = params => {
  return service({
    url: '/cms/article/deleteArticleByIds',
    method: 'delete',
    params,
  });
};

// @Tags Article
// @Summary 更新文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Article true "更新文章"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cms/article/updateArticle [put]
export const updateArticle = data => {
  return service({
    url: '/cms/article/updateArticle',
    method: 'put',
    data,
  });
};

// @Tags Article
// @Summary 用id查询文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Article true "用id查询文章"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cms/article/findArticle [get]
export const findArticle = params => {
  return service({
    url: '/cms/article/findArticle',
    method: 'get',
    params,
  });
};

// @Tags Article
// @Summary 分页获取文章列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取文章列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cms/article/getArticleList [get]
export const getArticleList = params => {
  return service({
    url: '/cms/article/getArticleList',
    method: 'get',
    params,
  });
};

// @Tags Article
// @Summary 不需要鉴权的文章接口
// @Accept application/json
// @Produce application/json
// @Param data query request.ArticleSearch true "分页获取文章列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cms/article/getArticlePublic [get]
export const getArticlePublic = () => {
  return service({
    url: '/cms/article/getArticlePublic',
    method: 'get',
  });
};
