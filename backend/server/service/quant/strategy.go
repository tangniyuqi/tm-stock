package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type StrategyService struct{}

// CreateStrategy 创建策略记录
// Author [yourname](https://github.com/yourname)
func (strategyService *StrategyService) CreateStrategy(ctx context.Context, strategy *quant.Strategy) (err error) {
	err = global.GVA_DB.Create(strategy).Error
	return err
}

// DeleteStrategy 删除策略记录
// Author [yourname](https://github.com/yourname)
func (strategyService *StrategyService) DeleteStrategy(ctx context.Context, id string, userid uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Strategy{}).Where("id = ?", id).Update("deleted_by", userid).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.Strategy{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteStrategyByIds 批量删除策略记录
// Author [yourname](https://github.com/yourname)
func (strategyService *StrategyService) DeleteStrategyByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Strategy{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.Strategy{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateStrategy 更新策略记录
// Author [yourname](https://github.com/yourname)
func (strategyService *StrategyService) UpdateStrategy(ctx context.Context, strategy quant.Strategy) (err error) {
	err = global.GVA_DB.Model(&quant.Strategy{}).Where("id = ?", strategy.ID).Updates(&strategy).Error
	return err
}

// GetStrategy 根据ID获取策略记录
// Author [yourname](https://github.com/yourname)
func (strategyService *StrategyService) GetStrategy(ctx context.Context, id string) (strategy quant.Strategy, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&strategy).Error
	return
}

// GetStrategyInfoList 分页获取策略记录
// Author [yourname](https://github.com/yourname)
func (strategyService *StrategyService) GetStrategyInfoList(ctx context.Context, info quantReq.StrategySearch) (list []quant.Strategy, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.Strategy{})
	var strategys []quant.Strategy

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

	// 动态排序：如果指定了排序字段则使用，否则默认按 sort 倒序
	if info.OrderKey != "" {
		orderClause := info.OrderKey
		if info.Desc {
			orderClause += " DESC"
		} else {
			orderClause += " ASC"
		}
		err = db.Order(orderClause).Find(&strategys).Error
	} else {
		// 默认按 sort 字段倒序排序
		err = db.Order("sort DESC").Find(&strategys).Error
	}
	return strategys, total, err
}
func (strategyService *StrategyService) GetStrategyPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
