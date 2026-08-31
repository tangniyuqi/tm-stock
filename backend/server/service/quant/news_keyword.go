package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type NewsKeywordService struct{}

// CreateNewsKeyword 创建关键词记录
// Author [yourname](https://github.com/yourname)
func (newsKeywordService *NewsKeywordService) CreateNewsKeyword(ctx context.Context, newsKeyword *quant.NewsKeyword) (err error) {
	err = global.GVA_DB.Create(newsKeyword).Error
	return err
}

// DeleteNewsKeyword 删除关键词记录
// Author [yourname](https://github.com/yourname)
func (newsKeywordService *NewsKeywordService) DeleteNewsKeyword(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.NewsKeyword{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.NewsKeyword{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteNewsKeywordByIds 批量删除关键词记录
// Author [yourname](https://github.com/yourname)
func (newsKeywordService *NewsKeywordService) DeleteNewsKeywordByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.NewsKeyword{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.NewsKeyword{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateNewsKeyword 更新关键词记录
// Author [yourname](https://github.com/yourname)
func (newsKeywordService *NewsKeywordService) UpdateNewsKeyword(ctx context.Context, newsKeyword quant.NewsKeyword) (err error) {
	err = global.GVA_DB.Model(&quant.NewsKeyword{}).Where("id = ?", newsKeyword.ID).Updates(&newsKeyword).Error
	return err
}

// GetNewsKeyword 根据id获取关键词记录
// Author [yourname](https://github.com/yourname)
func (newsKeywordService *NewsKeywordService) GetNewsKeyword(ctx context.Context, id string) (newsKeyword quant.NewsKeyword, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&newsKeyword).Error
	return
}

// GetNewsKeywordInfoList 分页获取关键词记录
// Author [yourname](https://github.com/yourname)
func (newsKeywordService *NewsKeywordService) GetNewsKeywordInfoList(ctx context.Context, info quantReq.NewsKeywordSearch) (list []quant.NewsKeyword, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)

	db := global.GVA_DB.Model(&quant.NewsKeyword{})
	var newsKeywords []quant.NewsKeyword

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}

	if info.Times != nil {
		db = db.Where("times = ?", *info.Times)
	}

	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
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

	switch info.SortType {
	case "times_desc":
		db = db.Order("times desc, id asc")
	case "times_asc":
		db = db.Order("times asc, id asc")
	default:
		db = db.Order("id desc")
	}

	err = db.Find(&newsKeywords).Error
	return newsKeywords, total, err
}
func (newsKeywordService *NewsKeywordService) GetNewsKeywordPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}

// IncrementTimes 增加关键词的times计数（+1）
func (newsKeywordService *NewsKeywordService) IncrementTimes(ctx context.Context, name *string) (err error) {
	res := global.GVA_DB.Model(&quant.NewsKeyword{}).
		Where("name = ?", name).
		Update("times", gorm.Expr("times + ?", 1))

	if res.RowsAffected == 0 {
		times := 1
		keyword := &quant.NewsKeyword{
			Name:  name,
			Times: &times,
		}
		err = global.GVA_DB.Create(keyword).Error
	} else {
		err = res.Error
	}

	return err
}
