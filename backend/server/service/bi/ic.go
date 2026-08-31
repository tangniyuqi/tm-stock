package bi

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type IcService struct{}

// CreateIc 创建IC记录
// Author [yourname](https://github.com/yourname)
func (icService *IcService) CreateIc(ctx context.Context, ic *bi.Ic) (err error) {
	err = global.GVA_DB.Create(ic).Error
	return err
}

// DeleteIc 删除IC记录
// Author [yourname](https://github.com/yourname)
func (icService *IcService) DeleteIc(ctx context.Context, ID string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Ic{}).Where("id = ?", ID).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Ic{}, "id = ?", ID).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteIcByIds 批量删除IC记录
// Author [yourname](https://github.com/yourname)
func (icService *IcService) DeleteIcByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Ic{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Ic{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateIc 更新IC记录
// Author [yourname](https://github.com/yourname)
func (icService *IcService) UpdateIc(ctx context.Context, ic bi.Ic) (err error) {
	err = global.GVA_DB.Model(&bi.Ic{}).Where("id = ?", ic.ID).Updates(&ic).Error
	return err
}

// GetIc 根据ID获取IC记录
// Author [yourname](https://github.com/yourname)
func (icService *IcService) GetIc(ctx context.Context, ID string) (ic bi.Ic, err error) {
	err = global.GVA_DB.Where("id = ?", ID).First(&ic).Error
	return
}

// GetIcInfoList 分页获取IC记录
// Author [yourname](https://github.com/yourname)
func (icService *IcService) GetIcInfoList(ctx context.Context, info biReq.IcSearch) (list []bi.Ic, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&bi.Ic{})
	var ics []bi.Ic

	if info.MsgId != nil {
		db = db.Where("msg_id = ?", *info.MsgId)
	}

	if info.Sender != nil && *info.Sender != "" {
		db = db.Where("sender LIKE ?", "%"+*info.Sender+"%")
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

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

	err = db.Order("id desc").Find(&ics).Error
	return ics, total, err
}

func (icService *IcService) GetIcPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
