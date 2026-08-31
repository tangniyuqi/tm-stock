package api

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/fileManager/model/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type FileManagerApi struct{}

// UploadFile 上传文件
// @Tags FileManager
// @Summary 上传文件
// @Security ApiKeyAuth
// @accept multipart/form-data
// @Produce application/json
// @Param file formData file true "文件"
// @Param path formData string true "存储路径"
// @Param categoryId formData int false "分类ID"
// @Param remark formData string false "备注"
// @Success 200 {object} response.Response{data=model.PluginFileRecord,msg=string} "上传成功"
// @Router /fileManager/upload [post]
func (a *FileManagerApi) UploadFile(c *gin.Context) {
	var params request.FileUpload
	_ = c.ShouldBind(&params)

	// 参数验证
	if err := utils.Verify(params, utils.Rules{"Path": {utils.NotEmpty()}}); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		global.GVA_LOG.Error("获取文件失败!", zap.Error(err))
		response.FailWithMessage("获取文件失败", c)
		return
	}

	// 限制文件大小（100MB）
	if file.Size > 100*1024*1024 {
		response.FailWithMessage("文件大小不能超过100MB", c)
		return
	}

	userId := utils.GetUserID(c)
	record, err := serviceFileManager.UploadFile(file, params, userId)
	if err != nil {
		global.GVA_LOG.Error("上传失败!", zap.Error(err))
		response.FailWithMessage("上传失败: "+err.Error(), c)
		return
	}

	response.OkWithData(record, c)
}

// GetFileList 获取文件列表
// @Tags FileManager
// @Summary 获取文件列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data query request.FileSearch true "搜索条件"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /fileManager/list [get]
func (a *FileManagerApi) GetFileList(c *gin.Context) {
	var params request.FileSearch
	_ = c.ShouldBindQuery(&params)

	if err := utils.Verify(params.PageInfo, utils.Rules{"Page": {utils.NotEmpty()}, "PageSize": {utils.NotEmpty()}}); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	list, total, err := serviceFileManager.GetFileList(params)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}

	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, "获取成功", c)
}

// GetFile 获取单个文件信息
// @Tags FileManager
// @Summary 获取文件信息
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param id path int true "文件ID"
// @Success 200 {object} response.Response{data=model.PluginFileRecord,msg=string} "获取成功"
// @Router /fileManager/file/{id} [get]
func (a *FileManagerApi) GetFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.FailWithMessage("ID格式错误", c)
		return
	}

	record, err := serviceFileManager.GetFileById(uint(id))
	if err != nil {
		response.FailWithMessage("文件不存在", c)
		return
	}

	response.OkWithData(record, c)
}

// UpdateFile 更新文件信息
// @Tags FileManager
// @Summary 更新文件
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.FileUpdate true "更新参数"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /fileManager/update [put]
func (a *FileManagerApi) UpdateFile(c *gin.Context) {
	var params request.FileUpdate
	_ = c.ShouldBindJSON(&params)

	if err := utils.Verify(params, utils.Rules{"ID": {utils.NotEmpty()}}); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := serviceFileManager.UpdateFile(params); err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败", c)
		return
	}

	response.OkWithMessage("更新成功", c)
}

// DeleteFiles 批量删除文件
// @Tags FileManager
// @Summary 批量删除文件
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.FileBatchDelete true "ID列表"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /fileManager/delete [delete]
func (a *FileManagerApi) DeleteFiles(c *gin.Context) {
	var params request.FileBatchDelete
	_ = c.ShouldBindJSON(&params)

	if err := utils.Verify(params, utils.Rules{"IDs": {utils.NotEmpty()}}); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := serviceFileManager.DeleteFiles(params.IDs); err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}

	response.OkWithMessage("删除成功", c)
}

// DownloadFile 下载文件
// @Tags FileManager
// @Summary 下载文件
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/octet-stream
// @Param id path int true "文件ID"
// @Success 200
// @Router /fileManager/download/{id} [get]
func (a *FileManagerApi) DownloadFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.FailWithMessage("ID格式错误", c)
		return
	}
	disposition := c.DefaultQuery("disposition", "attachment")

	if err := serviceFileManager.DownloadFile(c, uint(id), disposition); err != nil {
		global.GVA_LOG.Error("下载失败!", zap.Error(err))
		response.FailWithMessage("下载失败: "+err.Error(), c)
	}
}

// GetCategoryList 获取分类列表
// @Tags FileManager
// @Summary 获取分类列表
// @Security ApiKeyAuth
// @Produce application/json
// @Success 200 {object} response.Response{data=[]model.PluginFileCategory,msg=string} "获取成功"
// @Router /fileManager/category/list [get]
func (a *FileManagerApi) GetCategoryList(c *gin.Context) {
	tree, err := serviceFileManager.GetCategoryTree()
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithData(tree, c)
}

// CreateCategory 新建分类
// @Tags FileManager
// @Summary 新建分类
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.CategoryCreate true "分类信息"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /fileManager/category [post]
func (a *FileManagerApi) CreateCategory(c *gin.Context) {
	var params request.CategoryCreate
	_ = c.ShouldBindJSON(&params)

	if err := utils.Verify(params, utils.Rules{"Name": {utils.NotEmpty()}}); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	userId := utils.GetUserID(c)
	if err := serviceFileManager.CreateCategory(params, userId); err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败: "+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// UpdateCategory 更新分类
// @Tags FileManager
// @Summary 更新分类
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.CategoryUpdate true "分类信息"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /fileManager/category [put]
func (a *FileManagerApi) UpdateCategory(c *gin.Context) {
	var params request.CategoryUpdate
	_ = c.ShouldBindJSON(&params)

	if err := utils.Verify(params, utils.Rules{"ID": {utils.NotEmpty()}, "Name": {utils.NotEmpty()}}); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := serviceFileManager.UpdateCategory(params); err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// DeleteCategory 删除分类
// @Tags FileManager
// @Summary 删除分类
// @Security ApiKeyAuth
// @Param id path int true "分类ID"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /fileManager/category/{id} [delete]
func (a *FileManagerApi) DeleteCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.FailWithMessage("ID格式错误", c)
		return
	}

	if err := serviceFileManager.DeleteCategory(uint(id)); err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}
