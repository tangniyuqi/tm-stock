import service from '@/utils/request'
// @Tags Keyword
// @Summary 创建关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Keyword true "创建关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /bi/keyword/createKeyword [post]
export const createKeyword = (data) => {
  return service({
    url: '/bi/keyword/createKeyword',
    method: 'post',
    data
  })
}

// @Tags Keyword
// @Summary 删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Keyword true "删除关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/keyword/deleteKeyword [delete]
export const deleteKeyword = (params) => {
  return service({
    url: '/bi/keyword/deleteKeyword',
    method: 'delete',
    params
  })
}

// @Tags Keyword
// @Summary 批量删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /bi/keyword/deleteKeyword [delete]
export const deleteKeywordByIds = (params) => {
  return service({
    url: '/bi/keyword/deleteKeywordByIds',
    method: 'delete',
    params
  })
}

// @Tags Keyword
// @Summary 更新关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Keyword true "更新关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/keyword/updateKeyword [put]
export const updateKeyword = (data) => {
  return service({
    url: '/bi/keyword/updateKeyword',
    method: 'put',
    data
  })
}

// @Tags Keyword
// @Summary 用id查询关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Keyword true "用id查询关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /bi/keyword/findKeyword [get]
export const findKeyword = (params) => {
  return service({
    url: '/bi/keyword/findKeyword',
    method: 'get',
    params
  })
}

// @Tags Keyword
// @Summary 分页获取关键词列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取关键词列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /bi/keyword/getKeywordList [get]
export const getKeywordList = (params) => {
  return service({
    url: '/bi/keyword/getKeywordList',
    method: 'get',
    params
  })
}

// @Tags Keyword
// @Summary 不需要鉴权的关键词接口
// @Accept application/json
// @Produce application/json
// @Param data query biReq.KeywordSearch true "分页获取关键词列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /bi/keyword/getKeywordPublic [get]
export const getKeywordPublic = () => {
  return service({
    url: '/bi/keyword/getKeywordPublic',
    method: 'get',
  })
}

// @Tags Keyword
// @Summary 更新关键词搜索次数
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Keyword true "更新关键词搜索次数"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /bi/keyword/incrementTimes [put]
export const incrementTimes = (data) => {
  return service({
    url: '/bi/keyword/incrementTimes',
    method: 'put',
    data
  })
}
