package cms

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"gorm.io/gorm"
)

type PageService struct{}

// CreatePage 创建单页记录
// Author [yourname](https://github.com/yourname)
func (pageService *PageService) CreatePage(ctx context.Context, page *cms.Page) (err error) {
	err = global.GVA_DB.WithContext(ctx).Create(page).Error
	return err
}

// DeletePage 删除单页记录
// Author [yourname](https://github.com/yourname)
func (pageService *PageService) DeletePage(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Page{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cms.Page{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeletePageByIds 批量删除单页记录
// Author [yourname](https://github.com/yourname)
func (pageService *PageService) DeletePageByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Page{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cms.Page{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdatePage 更新单页记录
// Author [yourname](https://github.com/yourname)
func (pageService *PageService) UpdatePage(ctx context.Context, page cms.Page) (err error) {
	err = global.GVA_DB.WithContext(ctx).Model(&cms.Page{}).Where("id = ?", page.ID).Updates(&page).Error
	return err
}

// GetPage 根据ID获取单页记录
// Author [yourname](https://github.com/yourname)
func (pageService *PageService) GetPage(ctx context.Context, id string) (page cms.Page, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&page).Error
	return
}

// GetPageInfoList 分页获取单页记录
// Author [yourname](https://github.com/yourname)
func (pageService *PageService) GetPageInfoList(ctx context.Context, info cmsReq.PageSearch) (list []cms.Page, total int64, err error) {
	limit, offset := info.LimitOffset()
	// 创建db
	db := global.GVA_DB.WithContext(ctx).Model(&cms.Page{})
	var pages []cms.Page
	// 如果有条件搜索 下方会自动创建搜索语句
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Find(&pages).Error
	return pages, total, err
}
// GetPagePublic 按 name 标识获取单页内容（C 端公开接口，可用于"关于我们"/"联系方式"等）
func (pageService *PageService) GetPagePublic(ctx context.Context, name string) (page cms.Page, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("name = ? AND status = ?", name, true).First(&page).Error
	return
}
