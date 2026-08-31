import service from '@/utils/request'
// @Tags NewsKeyword
// @Summary 创建关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.NewsKeyword true "创建关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /quant/newsKeyword/createNewsKeyword [post]
export const createNewsKeyword = (data) => {
  return service({
    url: '/quant/newsKeyword/createNewsKeyword',
    method: 'post',
    data
  })
}

// @Tags NewsKeyword
// @Summary 删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.NewsKeyword true "删除关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/newsKeyword/deleteNewsKeyword [delete]
export const deleteNewsKeyword = (params) => {
  return service({
    url: '/quant/newsKeyword/deleteNewsKeyword',
    method: 'delete',
    params
  })
}

// @Tags NewsKeyword
// @Summary 批量删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /quant/newsKeyword/deleteNewsKeyword [delete]
export const deleteNewsKeywordByIds = (params) => {
  return service({
    url: '/quant/newsKeyword/deleteNewsKeywordByIds',
    method: 'delete',
    params
  })
}

// @Tags NewsKeyword
// @Summary 更新关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.NewsKeyword true "更新关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/newsKeyword/updateNewsKeyword [put]
export const updateNewsKeyword = (data) => {
  return service({
    url: '/quant/newsKeyword/updateNewsKeyword',
    method: 'put',
    data
  })
}

// @Tags NewsKeyword
// @Summary 用id查询关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.NewsKeyword true "用id查询关键词"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /quant/newsKeyword/findNewsKeyword [get]
export const findNewsKeyword = (params) => {
  return service({
    url: '/quant/newsKeyword/findNewsKeyword',
    method: 'get',
    params
  })
}

// @Tags NewsKeyword
// @Summary 分页获取关键词列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取关键词列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /quant/newsKeyword/getNewsKeywordList [get]
export const getNewsKeywordList = (params) => {
  return service({
    url: '/quant/newsKeyword/getNewsKeywordList',
    method: 'get',
    params
  })
}

// @Tags NewsKeyword
// @Summary 不需要鉴权的关键词接口
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.NewsKeywordSearch true "分页获取关键词列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /quant/newsKeyword/getNewsKeywordPublic [get]
export const getNewsKeywordPublic = () => {
  return service({
    url: '/quant/newsKeyword/getNewsKeywordPublic',
    method: 'get',
  })
}

// @Tags NewsKeyword
// @Summary 更新关键词搜索次数
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.NewsKeyword true "更新关键词搜索次数"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /quant/newsKeyword/incrementTimes [put]
export const incrementTimes = (data) => {
  return service({
    url: '/quant/newsKeyword/incrementTimes',
    method: 'put',
    data
  })
}
