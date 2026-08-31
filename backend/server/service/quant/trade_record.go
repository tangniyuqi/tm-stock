package quant

import (
	"context"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type TradeRecordService struct{}

// CreateTradeRecord 创建交易记录记录
// Author [yourname](https://github.com/yourname)
func (tradeRecordService *TradeRecordService) CreateTradeRecord(ctx context.Context, tradeRecord *quant.TradeRecord) (err error) {
	err = global.GVA_DB.Create(tradeRecord).Error
	return err
}

// DeleteTradeRecord 删除交易记录记录
// Author [yourname](https://github.com/yourname)
func (tradeRecordService *TradeRecordService) DeleteTradeRecord(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.TradeRecord{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.TradeRecord{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteTradeRecordByIds 批量删除交易记录记录
// Author [yourname](https://github.com/yourname)
func (tradeRecordService *TradeRecordService) DeleteTradeRecordByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.TradeRecord{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.TradeRecord{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateTradeRecord 更新交易记录记录
// Author [yourname](https://github.com/yourname)
func (tradeRecordService *TradeRecordService) UpdateTradeRecord(ctx context.Context, tradeRecord quant.TradeRecord) (err error) {
	err = global.GVA_DB.Model(&quant.TradeRecord{}).Where("id = ?", tradeRecord.ID).Updates(&tradeRecord).Error
	return err
}

// GetTradeRecord 根据ID获取交易记录记录
// Author [yourname](https://github.com/yourname)
func (tradeRecordService *TradeRecordService) GetTradeRecord(ctx context.Context, id string) (tradeRecord quant.TradeRecord, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&tradeRecord).Error
	return
}

// GetTradeRecordInfoList 分页获取交易记录记录
// Author [yourname](https://github.com/yourname)
func (tradeRecordService *TradeRecordService) GetTradeRecordInfoList(ctx context.Context, info quantReq.TradeRecordSearch) (list []quant.TradeRecord, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&quant.TradeRecord{})
	var tradeRecords []quant.TradeRecord

	if info.MemberId != nil {
		db = db.Where("member_id = ?", *info.MemberId)
	}

	if info.AccountId != nil {
		db = db.Where("account_id = ?", *info.AccountId)
	}

	if info.TaskId != nil {
		db = db.Where("task_id = ?", *info.TaskId)
	}

	if info.Action != nil && *info.Action != "" {
		db = db.Where("action = ?", *info.Action)
	}

	if info.Direction != nil {
		db = db.Where("direction = ?", *info.Direction)
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Symbol != nil && *info.Symbol != "" {
		db = db.Where("symbol LIKE ?", "%"+*info.Symbol+"%")
	}

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}

	if info.Keyword != "" {
		keyword := "%" + info.Keyword + "%"
		if _, err := strconv.Atoi(info.Keyword); err == nil {
			db = db.Where("name LIKE ? OR symbol LIKE ? OR id = ?", keyword, keyword, info.Keyword)
		} else {
			db = db.Where("name LIKE ? OR symbol LIKE ?", keyword, keyword)
		}
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

	err = db.Preload("TradeTask").Order("id desc").Find(&tradeRecords).Error
	return tradeRecords, total, err
}

func (tradeRecordService *TradeRecordService) GetTradeRecordPublic(ctx context.Context) {

}

// CreatePublic 创建交易记录（不鉴权）
func (tradeRecordService *TradeRecordService) CreatePublic(ctx context.Context, tradeRecord *quant.TradeRecord) (err error) {
	db := global.GVA_DB.Create(tradeRecord)
	return db.Error
}
