package model

import "github.com/flipped-aurora/gin-vue-admin/server/global"

// PluginFileRecord 文件记录表
type PluginFileRecord struct {
	global.GVA_MODEL
	Name       string `json:"name" gorm:"comment:原始文件名"`
	Key        string `json:"key" gorm:"comment:存储名或路径;uniqueIndex"`
	Url        string `json:"url" gorm:"comment:文件访问URL"`
	Path       string `json:"path" gorm:"default:common;comment:业务定义的子目录"`
	Tag        string `json:"tag" gorm:"comment:文件后缀"`
	Size       int64  `json:"size" gorm:"comment:文件大小(Bytes)"`
	MimeType   string `json:"mimeType" gorm:"comment:MIME类型"`
	CategoryId uint   `json:"categoryId" gorm:"default:0;comment:文件分类ID"`
	UploaderId uint   `json:"uploaderId" gorm:"comment:上传者ID"`
	Remark     string `json:"remark" gorm:"type:text;comment:备注信息"`
}

func (PluginFileRecord) TableName() string {
	return "plugin_file_records"
}

// PluginFileCategory 文件分类表
type PluginFileCategory struct {
	global.GVA_MODEL
	Name      string               `json:"name" gorm:"comment:分类名称"`
	ParentId  uint                 `json:"parentId" gorm:"default:0;comment:父分类ID"`
	Sort      int                  `json:"sort" gorm:"default:0;comment:排序"`
	Path      string               `json:"path" gorm:"comment:分类路径"`
	Level     int                  `json:"level" gorm:"default:1;comment:层级"`
	CreatedBy uint                 `json:"createdBy" gorm:"comment:创建者ID"`
	Children  []PluginFileCategory `json:"children" gorm:"-"`
}

func (PluginFileCategory) TableName() string {
	return "plugin_file_categories"
}
