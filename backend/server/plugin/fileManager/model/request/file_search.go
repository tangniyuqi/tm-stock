package request

import "github.com/flipped-aurora/gin-vue-admin/server/model/common/request"

// FileSearch 文件搜索条件
type FileSearch struct {
	request.PageInfo
	Name       string `json:"name" form:"name"`
	Path       string `json:"path" form:"path"`
	CategoryId uint   `json:"categoryId" form:"categoryId"`
	Tag        string `json:"tag" form:"tag"`
	StartTime  string `json:"startTime" form:"startTime"`
	EndTime    string `json:"endTime" form:"endTime"`
}

// FileUpload 文件上传参数
type FileUpload struct {
	Path       string `json:"path" form:"path" binding:"required"`
	CategoryId uint   `json:"categoryId" form:"categoryId"`
	Remark     string `json:"remark" form:"remark"`
}

// FileUpdate 文件更新参数
type FileUpdate struct {
	ID     uint   `json:"id" binding:"required"`
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

// FileBatchDelete 批量删除参数
type FileBatchDelete struct {
	IDs []uint `json:"ids" binding:"required"`
}

// CategoryCreate 新建分类参数
type CategoryCreate struct {
	Name     string `json:"name" binding:"required"`
	ParentId uint   `json:"parentId"`
}

// CategoryUpdate 更新分类参数
type CategoryUpdate struct {
	ID       uint   `json:"id" binding:"required"`
	Name     string `json:"name" binding:"required"`
	ParentId uint   `json:"parentId"`
}
