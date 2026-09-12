package cms

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"gorm.io/gorm"
)

type AdService struct{}

// CreateAd 创建广告表记录
// Author [yourname](https://github.com/yourname)
func (adService *AdService) CreateAd(ctx context.Context, ad *cms.Ad) (err error) {
	err = global.GVA_DB.WithContext(ctx).Create(ad).Error
	return err
}

// DeleteAd 删除广告表记录
// Author [yourname](https://github.com/yourname)
func (adService *AdService) DeleteAd(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Ad{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cms.Ad{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteAdByIds 批量删除广告表记录
// Author [yourname](https://github.com/yourname)
func (adService *AdService) DeleteAdByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Ad{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cms.Ad{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateAd 更新广告表记录
// Author [yourname](https://github.com/yourname)
func (adService *AdService) UpdateAd(ctx context.Context, ad cms.Ad) (err error) {
	err = global.GVA_DB.WithContext(ctx).Model(&cms.Ad{}).Where("id = ?", ad.ID).Updates(&ad).Error
	return err
}

// GetAd 根据ID获取广告表记录
// Author [yourname](https://github.com/yourname)
func (adService *AdService) GetAd(ctx context.Context, id string) (ad cms.Ad, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&ad).Error
	return
}

// GetAdInfoList 分页获取广告表记录
// Author [yourname](https://github.com/yourname)
func (adService *AdService) GetAdInfoList(ctx context.Context, info cmsReq.AdSearch) (list []cms.Ad, total int64, err error) {
	limit, offset := info.LimitOffset()
	// 创建db
	db := global.GVA_DB.WithContext(ctx).Model(&cms.Ad{})
	var ads []cms.Ad
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

	err = db.Find(&ads).Error
	return ads, total, err
}
func (adService *AdService) GetAdPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
