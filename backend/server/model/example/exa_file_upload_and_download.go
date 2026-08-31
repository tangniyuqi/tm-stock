package example

import "github.com/flipped-aurora/gin-vue-admin/server/global"

// ExaFileUploadAndDownload 文件上传与下载记录
type ExaFileUploadAndDownload struct {
	global.GVA_MODEL
	Name    string `json:"name" gorm:"comment:文件名"`
	Url     string `json:"url" gorm:"comment:文件地址"`
	Tag     string `json:"tag" gorm:"comment:文件标签"`
	Key     string `json:"key" gorm:"comment:编号"`
	ClassId int    `json:"classId" gorm:"comment:文件分类ID"`
}

func (ExaFileUploadAndDownload) TableName() string {
	return "exa_file_upload_and_downloads"
}