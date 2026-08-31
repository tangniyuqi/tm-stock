package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type ThemeStockService struct{}

// syncThemeStockCount 重新统计题材下的有效股票数量（status 为 NULL 或不等于 -1，-1 表示已下架）并更新到 addon_quant_theme.stock_count
func syncThemeStockCount(tx *gorm.DB, themeIDs []int32) error {
	if len(themeIDs) == 0 {
		return nil
	}
	var rows []struct {
		ThemeId int32
		Count   int64
	}
	if err := tx.Model(&quant.ThemeStock{}).
		Select("theme_id, COUNT(*) AS count").
		Where("theme_id IN ? AND COALESCE(status, 0) <> -1", themeIDs).
		Group("theme_id").
		Scan(&rows).Error; err != nil {
		return err
	}
	countMap := make(map[int32]int64, len(rows))
	for _, row := range rows {
		countMap[row.ThemeId] = row.Count
	}
	for _, themeID := range themeIDs {
		count := int32(countMap[themeID])
		if err := tx.Model(&quant.Theme{}).Where("id = ?", themeID).Update("stock_count", count).Error; err != nil {
			return err
		}
	}
	return nil
}

// CreateThemeStock 创建题材股票记录
// Author [yourname](https://github.com/yourname)
func (themeStockService *ThemeStockService) CreateThemeStock(ctx context.Context, themeStock *quant.ThemeStock) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(themeStock).Error; err != nil {
			return err
		}
		if themeStock.ThemeId != nil {
			return syncThemeStockCount(tx, []int32{*themeStock.ThemeId})
		}
		return nil
	})
	return err
}

// DeleteThemeStock 删除题材股票记录
// Author [yourname](https://github.com/yourname)
func (themeStockService *ThemeStockService) DeleteThemeStock(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		var ts quant.ThemeStock
		if err := tx.Where("id = ?", id).First(&ts).Error; err != nil {
			return err
		}
		if err := tx.Model(&quant.ThemeStock{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.ThemeStock{}, "id = ?", id).Error; err != nil {
			return err
		}
		if ts.ThemeId != nil {
			return syncThemeStockCount(tx, []int32{*ts.ThemeId})
		}
		return nil
	})
	return err
}

// DeleteThemeStockByIds 批量删除题材股票记录
// Author [yourname](https://github.com/yourname)
func (themeStockService *ThemeStockService) DeleteThemeStockByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		var themeIDs []int32
		if err := tx.Model(&quant.ThemeStock{}).Where("id IN ?", ids).Distinct().Pluck("theme_id", &themeIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&quant.ThemeStock{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.ThemeStock{}).Error; err != nil {
			return err
		}
		return syncThemeStockCount(tx, themeIDs)
	})
	return err
}

// UpdateThemeStock 更新题材股票记录
// Author [yourname](https://github.com/yourname)
func (themeStockService *ThemeStockService) UpdateThemeStock(ctx context.Context, themeStock quant.ThemeStock) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 查询旧记录，以便在 theme_id/status 变更后重新统计旧题材的股票数量
		var old quant.ThemeStock
		if err := tx.Where("id = ?", themeStock.ID).First(&old).Error; err != nil {
			return err
		}
		if err := tx.Model(&quant.ThemeStock{}).Where("id = ?", themeStock.ID).Updates(&themeStock).Error; err != nil {
			return err
		}
		themeIDs := make([]int32, 0, 2)
		if old.ThemeId != nil {
			themeIDs = append(themeIDs, *old.ThemeId)
		}
		if themeStock.ThemeId != nil {
			themeIDs = append(themeIDs, *themeStock.ThemeId)
		}
		return syncThemeStockCount(tx, themeIDs)
	})
	return err
}

// GetThemeStock 根据id获取题材股票记录
// Author [yourname](https://github.com/yourname)
func (themeStockService *ThemeStockService) GetThemeStock(ctx context.Context, id string) (themeStock quant.ThemeStock, err error) {
	err = global.GVA_DB.Preload("Theme").Preload("Stock").Where("id = ?", id).First(&themeStock).Error
	return
}

// GetThemeStockInfoList 分页获取题材股票记录
// Author [yourname](https://github.com/yourname)
func (themeStockService *ThemeStockService) GetThemeStockInfoList(ctx context.Context, info quantReq.ThemeStockSearch) (list []quant.ThemeStock, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db，并加载题材与股票名称/代码
	db := global.GVA_DB.Model(&quant.ThemeStock{}).Preload("Theme")
	// 按涨跌幅排序时需联表查询股票实时涨跌幅
	if info.OrderKey == "change_pct" {
		db = db.Joins("Stock")
	} else {
		db = db.Preload("Stock")
	}
	var themeStocks []quant.ThemeStock
	// 如果有条件搜索 下方会自动创建搜索语句
	// 精确或模糊搜索字段（字段名加表名前缀，避免联表排序时列名歧义）
	if info.ID != nil {
		db = db.Where("addon_quant_theme_stock.id = ?", *info.ID)
	}
	if info.ThemeId != nil {
		db = db.Where("theme_id = ?", *info.ThemeId)
	}
	if info.StockId != nil {
		db = db.Where("stock_id = ?", *info.StockId)
	}
	if info.Reason != nil && *info.Reason != "" {
		db = db.Where("reason LIKE ?", "%"+*info.Reason+"%")
	}
	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("addon_quant_theme_stock.created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	// 支持前端传入 sort 作为筛选项（非排序）
	if info.Sort != nil {
		db = db.Where("addon_quant_theme_stock.sort = ?", *info.Sort)
	}
	if info.Tier != nil {
		db = db.Where("addon_quant_theme_stock.tier = ?", *info.Tier)
	}

	// 默认排序：sort 倒序、id 倒序
	defaultOrder := "addon_quant_theme_stock.sort DESC, addon_quant_theme_stock.tier ASC, addon_quant_theme_stock.relevance DESC"
	// 可排序字段白名单（防止注入非法字段）
	orderFieldMap := map[string]string{
		"id":         "addon_quant_theme_stock.id",
		"tier":       "addon_quant_theme_stock.tier",
		"relevance":  "addon_quant_theme_stock.relevance",
		"sort":       "addon_quant_theme_stock.sort",
		"change_pct": "Stock.change_pct",
	}
	orderStr := defaultOrder
	if info.OrderKey != "" {
		if field, ok := orderFieldMap[info.OrderKey]; ok {
			orderDirect := "ASC"
			if info.Desc {
				orderDirect = "DESC"
			}
			orderStr = field + " " + orderDirect + ", addon_quant_theme_stock.id DESC"
		}
	}

	err = db.Order(orderStr).Find(&themeStocks).Error
	return themeStocks, total, err
}
func (themeStockService *ThemeStockService) GetThemeStockPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
