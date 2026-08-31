package bi

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type KeywordService struct{}

// CreateKeyword 创建关键词记录
// Author [yourname](https://github.com/yourname)
func (keywordService *KeywordService) CreateKeyword(ctx context.Context, keyword *bi.Keyword) (err error) {
	err = global.GVA_DB.Create(keyword).Error
	return err
}

// DeleteKeyword 删除关键词记录
// Author [yourname](https://github.com/yourname)
func (keywordService *KeywordService) DeleteKeyword(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Keyword{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Keyword{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteKeywordByIds 批量删除关键词记录
// Author [yourname](https://github.com/yourname)
func (keywordService *KeywordService) DeleteKeywordByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Keyword{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Keyword{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateKeyword 更新关键词记录
// Author [yourname](https://github.com/yourname)
func (keywordService *KeywordService) UpdateKeyword(ctx context.Context, keyword bi.Keyword) (err error) {
	err = global.GVA_DB.Model(&bi.Keyword{}).Where("id = ?", keyword.ID).Updates(&keyword).Error
	return err
}

// GetKeyword 根据ID获取关键词记录
// Author [yourname](https://github.com/yourname)
func (keywordService *KeywordService) GetKeyword(ctx context.Context, id string) (keyword bi.Keyword, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&keyword).Error
	return
}

// GetKeywordInfoList 分页获取关键词记录
// Author [yourname](https://github.com/yourname)
func (keywordService *KeywordService) GetKeywordInfoList(ctx context.Context, info biReq.KeywordSearch) (list []bi.Keyword, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)

	db := global.GVA_DB.Model(&bi.Keyword{})
	var keywords []bi.Keyword

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

	err = db.Find(&keywords).Error

	return keywords, total, err
}

func (keywordService *KeywordService) GetKeywordPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}

// IncrementTimes 增加关键词的times计数（+1）
func (keywordService *KeywordService) IncrementTimes(ctx context.Context, name *string) (err error) {
	res := global.GVA_DB.Model(&bi.Keyword{}).
		Where("name = ?", name).
		Update("times", gorm.Expr("times + ?", 1))

	if res.RowsAffected == 0 {
		times := 1
		keyword := &bi.Keyword{
			Name:  name,
			Times: &times,
		}
		err = global.GVA_DB.Create(keyword).Error
	} else {
		err = res.Error
	}

	return err
}
