package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type ConfigService struct{}

// CreateConfig 创建配置记录
// Author [yourname](https://github.com/yourname)
func (configService *ConfigService) CreateConfig(ctx context.Context, config *quant.Config) (err error) {
	err = global.GVA_DB.Create(config).Error
	return err
}

// DeleteConfig 删除配置记录
// Author [yourname](https://github.com/yourname)
func (configService *ConfigService) DeleteConfig(ctx context.Context, id string, userid uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Config{}).Where("id = ?", id).Update("deleted_by", userid).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.Config{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteConfigByIds 批量删除配置记录
// Author [yourname](https://github.com/yourname)
func (configService *ConfigService) DeleteConfigByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Config{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.Config{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateConfig 更新配置记录
// Author [yourname](https://github.com/yourname)
func (configService *ConfigService) UpdateConfig(ctx context.Context, config quant.Config) (err error) {
	err = global.GVA_DB.Model(&quant.Config{}).Where("id = ?", config.ID).Updates(&config).Error
	return err
}

// GetConfig 根据ID获取配置记录
// Author [yourname](https://github.com/yourname)
func (configService *ConfigService) GetConfig(ctx context.Context, id string) (config quant.Config, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&config).Error
	return
}

// GetConfigInfoList 分页获取配置记录
// Author [yourname](https://github.com/yourname)
func (configService *ConfigService) GetConfigInfoList(ctx context.Context, info quantReq.ConfigSearch) (list []quant.Config, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.Config{})
	var configs []quant.Config

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
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

	err = db.Find(&configs).Error
	return configs, total, err
}
func (configService *ConfigService) GetConfigPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
