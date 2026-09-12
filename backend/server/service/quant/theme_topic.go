package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type ThemeTopicService struct{}

// CreateThemeTopic 创建题材话题记录
// Author [yourname](https://github.com/yourname)
func (themeTopicService *ThemeTopicService) CreateThemeTopic(ctx context.Context, themeTopic *quant.ThemeTopic) (err error) {
	err = global.GVA_DB.Create(themeTopic).Error
	return err
}

// DeleteThemeTopic 删除题材话题记录
// Author [yourname](https://github.com/yourname)
func (themeTopicService *ThemeTopicService) DeleteThemeTopic(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.ThemeTopic{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.ThemeTopic{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteThemeTopicByIds 批量删除题材话题记录
// Author [yourname](https://github.com/yourname)
func (themeTopicService *ThemeTopicService) DeleteThemeTopicByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.ThemeTopic{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.ThemeTopic{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateThemeTopic 更新题材话题记录
// Author [yourname](https://github.com/yourname)
func (themeTopicService *ThemeTopicService) UpdateThemeTopic(ctx context.Context, themeTopic quant.ThemeTopic) (err error) {
	err = global.GVA_DB.Model(&quant.ThemeTopic{}).Where("id = ?", themeTopic.ID).Updates(&themeTopic).Error
	return err
}

// GetThemeTopic 根据ID获取题材话题记录
// Author [yourname](https://github.com/yourname)
func (themeTopicService *ThemeTopicService) GetThemeTopic(ctx context.Context, id string) (themeTopic quant.ThemeTopic, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&themeTopic).Error
	return
}

// GetThemeTopicInfoList 分页获取题材话题记录
// Author [yourname](https://github.com/yourname)
func (themeTopicService *ThemeTopicService) GetThemeTopicInfoList(ctx context.Context, info quantReq.ThemeTopicSearch) (list []quant.ThemeTopic, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.ThemeTopic{})
	var themeTopics []quant.ThemeTopic

	if info.Title != nil && *info.Title != "" {
		db = db.Where("title LIKE ?", "%"+*info.Title+"%")
	}

	if info.ThemeId != nil {
		db = db.Where("theme_id = ?", *info.ThemeId)
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Sentiment != nil {
		db = db.Where("sentiment = ?", *info.Sentiment)
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	// 按发布日过滤（publish_date=YYYY-MM-DD）
	if info.PublishDate != nil && *info.PublishDate != "" {
		db = db.Where("publish_date = ?", *info.PublishDate)
	}

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

	err = db.Order("id DESC").Find(&themeTopics).Error
	return themeTopics, total, err
}

func (themeTopicService *ThemeTopicService) GetThemeTopicPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
