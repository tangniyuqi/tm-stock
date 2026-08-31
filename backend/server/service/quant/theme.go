package quant

import (
	"context"
	"errors"
	"sort"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"gorm.io/gorm"
)

type ThemeService struct{}

// CreateTheme 创建题材记录
// Author [yourname](https://github.com/yourname)
func (themeService *ThemeService) CreateTheme(ctx context.Context, theme *quant.Theme) (err error) {
	err = global.GVA_DB.Create(theme).Error
	return err
}

// DeleteTheme 删除题材记录
// Author [yourname](https://github.com/yourname)
func (themeService *ThemeService) DeleteTheme(ctx context.Context, id string, userID uint) (err error) {
	var count int64
	err = global.GVA_DB.Find(&quant.Theme{}, "parent_id = ?", id).Count(&count).Error
	if count > 0 {
		return errors.New("此节点存在子节点不允许删除")
	}
	if err != nil {
		return err
	}
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Theme{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.Theme{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteThemeByIds 批量删除题材记录
// Author [yourname](https://github.com/yourname)
func (themeService *ThemeService) DeleteThemeByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Theme{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.Theme{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateTheme 更新题材记录
// Author [yourname](https://github.com/yourname)
func (themeService *ThemeService) UpdateTheme(ctx context.Context, theme quant.Theme) (err error) {
	err = global.GVA_DB.Model(&quant.Theme{}).Where("id = ?", theme.ID).Updates(&theme).Error
	return err
}

// GetTheme 根id获取题材记录
// Author [yourname](https://github.com/yourname)
func (themeService *ThemeService) GetTheme(ctx context.Context, id string) (theme quant.Theme, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&theme).Error
	return
}

// GetThemeList 分页获取题材记录,Tree模式下不添加分页和搜索
// Author [yourname](https://github.com/yourname)
func (themeService *ThemeService) GetThemeList(ctx context.Context) (list []*quant.Theme, err error) {
	// 创建db
	db := global.GVA_DB.Model(&quant.Theme{})
	var themes []*quant.Theme

	err = db.Order("sort DESC, id ASC").Find(&themes).Error

	list = utils.BuildTree(themes)
	sortThemeTree(list)
	return list, err
}

// GetThemeListWithoutChildren 根据层级获取题材列表，不返回子节点
func (themeService *ThemeService) GetThemeListWithoutChildren(ctx context.Context, level int8) (list []*quant.Theme, err error) {
	err = global.GVA_DB.Model(&quant.Theme{}).
		Where("level = ?", level).
		Order("sort DESC, id ASC").
		Find(&list).Error
	return list, err
}

// GetThemeWithChildren 根据ID获取题材信息以及全部子节点数据
func (themeService *ThemeService) GetThemeWithChildren(ctx context.Context, id string) (theme *quant.Theme, err error) {
	theme = &quant.Theme{}
	err = global.GVA_DB.Where("id = ?", id).First(theme).Error
	if err != nil {
		return nil, err
	}

	err = themeService.loadThemeChildren(theme)
	return theme, err
}

func (themeService *ThemeService) loadThemeChildren(theme *quant.Theme) error {
	var children []*quant.Theme
	err := global.GVA_DB.Where("parent_id = ?", theme.ID).
		Order("sort DESC, id ASC").
		Find(&children).Error
	if err != nil {
		return err
	}

	theme.Children = children
	for _, child := range children {
		if err = themeService.loadThemeChildren(child); err != nil {
			return err
		}
	}
	return nil
}

func sortThemeTree(themes []*quant.Theme) {
	sort.SliceStable(themes, func(i, j int) bool {
		leftSort, rightSort := int32(0), int32(0)
		if themes[i].Sort != nil {
			leftSort = *themes[i].Sort
		}
		if themes[j].Sort != nil {
			rightSort = *themes[j].Sort
		}
		if leftSort != rightSort {
			return leftSort > rightSort
		}
		return themes[i].ID < themes[j].ID
	})

	for _, theme := range themes {
		sortThemeTree(theme.Children)
	}
}

func (themeService *ThemeService) GetThemePublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
