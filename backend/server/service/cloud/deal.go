package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	cloudReq "github.com/flipped-aurora/gin-vue-admin/server/model/cloud/request"
	"gorm.io/gorm"
)

type DealService struct{}

// CreateDeal 创建商机记录
// Author [yourname](https://github.com/yourname)
func (dealService *DealService) CreateDeal(deal *cloud.Deal) (err error) {
	err = global.GVA_DB.Create(deal).Error
	return err
}

// DeleteDeal 删除商机记录
// Author [yourname](https://github.com/yourname)
func (dealService *DealService) DeleteDeal(id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Deal{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cloud.Deal{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteDealByIds 批量删除商机记录
// Author [yourname](https://github.com/yourname)
func (dealService *DealService) DeleteDealByIds(ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Deal{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cloud.Deal{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateDeal 更新商机记录
// Author [yourname](https://github.com/yourname)
func (dealService *DealService) UpdateDeal(deal cloud.Deal) (err error) {
	err = global.GVA_DB.Model(&cloud.Deal{}).Where("id = ?", deal.ID).Save(&deal).Error
	return err
}

// GetDeal 根据ID获取商机记录
// Author [yourname](https://github.com/yourname)
func (dealService *DealService) GetDeal(id string) (deal cloud.Deal, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&deal).Error
	return
}

// GetDealInfoList 分页获取商机记录
// Author [yourname](https://github.com/yourname)
func (dealService *DealService) GetDealInfoList(info cloudReq.DealSearch) (list []cloud.Deal, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&cloud.Deal{})
	var deals []cloud.Deal

	if info.CreatedBy != 1 {
		db = db.Where("created_by = ?", info.CreatedBy)
	}

	if info.Title != nil && *info.Title != "" {
		db = db.Where("title LIKE ?", "%"+*info.Title+"%")
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Stage != nil {
		db = db.Where("stage = ?", *info.Stage)
	}

	if info.Source != nil {
		db = db.Where("source = ?", *info.Source)
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

	err = db.Order("id desc").Find(&deals).Error
	return deals, total, err
}
func (dealService *DealService) GetDealPublic() {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
