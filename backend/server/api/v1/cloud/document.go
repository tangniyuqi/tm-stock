package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	cloudReq "github.com/flipped-aurora/gin-vue-admin/server/model/cloud/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DocumentApi struct{}

// CreateDocument 创建文档
// @Tags Document
// @Summary 创建文档
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Document true "创建文档"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /document/createDocument [post]
func (documentApi *DocumentApi) CreateDocument(c *gin.Context) {
	var document cloud.Document
	err := c.ShouldBindJSON(&document)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	document.CreatedBy = utils.GetUserID(c)
	err = documentService.CreateDocument(&document)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteDocument 删除文档
// @Tags Document
// @Summary 删除文档
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Document true "删除文档"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /document/deleteDocument [delete]
func (documentApi *DocumentApi) DeleteDocument(c *gin.Context) {
	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := documentService.DeleteDocument(id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteDocumentByIds 批量删除文档
// @Tags Document
// @Summary 批量删除文档
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /document/deleteDocumentByIds [delete]
func (documentApi *DocumentApi) DeleteDocumentByIds(c *gin.Context) {
	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := documentService.DeleteDocumentByIds(ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateDocument 更新文档
// @Tags Document
// @Summary 更新文档
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Document true "更新文档"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /document/updateDocument [put]
func (documentApi *DocumentApi) UpdateDocument(c *gin.Context) {
	var document cloud.Document
	err := c.ShouldBindJSON(&document)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	document.UpdatedBy = utils.GetUserID(c)
	err = documentService.UpdateDocument(document)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindDocument 用ID查询文档
// @Tags Document
// @Summary 用ID查询文档
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询文档"
// @Success 200 {object} response.Response{data=cloud.Document,msg=string} "查询成功"
// @Router /document/findDocument [get]
func (documentApi *DocumentApi) FindDocument(c *gin.Context) {
	id := c.Query("id")
	redocument, err := documentService.GetDocument(id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(redocument, c)
}

// GetDocumentList 分页获取文档列表
// @Tags Document
// @Summary 分页获取文档列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.DocumentSearch true "分页获取文档列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /document/getDocumentList [get]
func (documentApi *DocumentApi) GetDocumentList(c *gin.Context) {
	var pageInfo cloudReq.DocumentSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	pageInfo.CreatedBy = utils.GetUserID(c)
	list, total, err := documentService.GetDocumentInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetDocumentPublic 不需要鉴权的文档接口
// @Tags Document
// @Summary 不需要鉴权的文档接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /document/getDocumentPublic [get]
func (documentApi *DocumentApi) GetDocumentPublic(c *gin.Context) {
	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	documentService.GetDocumentPublic()
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的文档接口信息",
	}, "获取成功", c)
}
