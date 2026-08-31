import service from '@/utils/request'
// @Tags Document
// @Summary 创建文档表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Document true "创建文档表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /cloud/document/createDocument [post]
export const createDocument = (data) => {
  return service({
    url: '/cloud/document/createDocument',
    method: 'post',
    data
  })
}

// @Tags Document
// @Summary 删除文档表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Document true "删除文档表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/document/deleteDocument [delete]
export const deleteDocument = (params) => {
  return service({
    url: '/cloud/document/deleteDocument',
    method: 'delete',
    params
  })
}

// @Tags Document
// @Summary 批量删除文档表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "批量删除文档表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /cloud/document/deleteDocument [delete]
export const deleteDocumentByIds = (params) => {
  return service({
    url: '/cloud/document/deleteDocumentByIds',
    method: 'delete',
    params
  })
}

// @Tags Document
// @Summary 更新文档表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body model.Document true "更新文档表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /cloud/document/updateDocument [put]
export const updateDocument = (data) => {
  return service({
    url: '/cloud/document/updateDocument',
    method: 'put',
    data
  })
}

// @Tags Document
// @Summary 用id查询文档表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query model.Document true "用id查询文档表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /cloud/document/findDocument [get]
export const findDocument = (params) => {
  return service({
    url: '/cloud/document/findDocument',
    method: 'get',
    params
  })
}

// @Tags Document
// @Summary 分页获取文档表列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query request.PageInfo true "分页获取文档表列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /cloud/document/getDocumentList [get]
export const getDocumentList = (params) => {
  return service({
    url: '/cloud/document/getDocumentList',
    method: 'get',
    params
  })
}

// @Tags Document
// @Summary 不需要鉴权的文档表接口
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.DocumentSearch true "分页获取文档表列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /cloud/document/getDocumentPublic [get]
export const getDocumentPublic = () => {
  return service({
    url: '/cloud/document/getDocumentPublic',
    method: 'get',
  })
}
