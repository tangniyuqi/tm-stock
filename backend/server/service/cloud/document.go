package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	cloudReq "github.com/flipped-aurora/gin-vue-admin/server/model/cloud/request"
	"gorm.io/gorm"
)

type DocumentService struct{}

// CreateDocument 创建文档记录
// Author [yourname](https://github.com/yourname)
func (documentService *DocumentService) CreateDocument(document *cloud.Document) (err error) {
	err = global.GVA_DB.Create(document).Error
	return err
}

// DeleteDocument 删除文档记录
// Author [yourname](https://github.com/yourname)
func (documentService *DocumentService) DeleteDocument(id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Document{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cloud.Document{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteDocumentByIds 批量删除文档记录
// Author [yourname](https://github.com/yourname)
func (documentService *DocumentService) DeleteDocumentByIds(ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Document{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cloud.Document{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateDocument 更新文档记录
// Author [yourname](https://github.com/yourname)
func (documentService *DocumentService) UpdateDocument(document cloud.Document) (err error) {
	err = global.GVA_DB.Model(&cloud.Document{}).Where("id = ?", document.ID).Save(&document).Error
	return err
}

// GetDocument 根据ID获取文档记录
// Author [yourname](https://github.com/yourname)
func (documentService *DocumentService) GetDocument(id string) (document cloud.Document, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&document).Error
	return
}

// GetDocumentInfoList 分页获取文档记录
// Author [yourname](https://github.com/yourname)
func (documentService *DocumentService) GetDocumentInfoList(info cloudReq.DocumentSearch) (list []cloud.Document, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&cloud.Document{})
	var documents []cloud.Document

	// log.Printf("获取用户列表，参数：%+v", info)
	// global.GVA_LOG.Error("获取失败!", info)
	// return

	if info.CreatedBy != 1 {
		db = db.Where("created_by = ?", info.CreatedBy)
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Title != nil && *info.Title != "" {
		db = db.Where("title LIKE ?", "%"+*info.Title+"%")
	}

	if info.Content != nil && *info.Content != "" {
		db = db.Where("content LIKE ?", "%"+*info.Content+"%")
	}

	if info.Tags != nil && *info.Tags != "" {
		db = db.Where("tags LIKE ?", "%"+*info.Tags+"%")
	}

	if info.StartCreatedAt != nil && info.EndCreatedAt != nil {
		db = db.Where("created_at BETWEEN ? AND ?", info.StartCreatedAt, info.EndCreatedAt)
	}
	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Order("sort desc, updated_at desc, id desc").Find(&documents).Error
	return documents, total, err
}
func (documentService *DocumentService) GetDocumentPublic() {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
