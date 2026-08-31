package quant

import (
	"context"
	"fmt"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ScreenerRecordService struct{}

// CreateScreenerRecord 创建AI选股器查询记录
func (screenerRecordService *ScreenerRecordService) CreateScreenerRecord(ctx context.Context, screenerRecord *quant.ScreenerRecord) (err error) {
	err = global.GVA_DB.Create(screenerRecord).Error
	return err
}

// DeleteScreenerRecord 删除AI选股器查询记录
func (screenerRecordService *ScreenerRecordService) DeleteScreenerRecord(ctx context.Context, ID string, userid uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.ScreenerRecord{}).Where("id = ?", ID).Update("deleted_by", userid).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.ScreenerRecord{}, "id = ?", ID).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteScreenerRecordByIds 批量删除AI选股器查询记录
func (screenerRecordService *ScreenerRecordService) DeleteScreenerRecordByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.ScreenerRecord{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.ScreenerRecord{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateScreenerRecord 更新AI选股器查询记录
func (screenerRecordService *ScreenerRecordService) UpdateScreenerRecord(ctx context.Context, screenerRecord quant.ScreenerRecord) (err error) {
	err = global.GVA_DB.Model(&quant.ScreenerRecord{}).Where("id = ?", screenerRecord.ID).Updates(&screenerRecord).Error
	return err
}

// GetScreenerRecord 根据ID获取AI选股器查询记录
func (screenerRecordService *ScreenerRecordService) GetScreenerRecord(ctx context.Context, id string) (screenerRecord quant.ScreenerRecord, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&screenerRecord).Error
	return
}

// GetScreenerRecordInfoList 分页获取AI选股器查询记录
func (screenerRecordService *ScreenerRecordService) GetScreenerRecordInfoList(ctx context.Context, info quantReq.ScreenerRecordSearch) (list []quant.ScreenerRecord, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.ScreenerRecord{})
	var screenerRecords []quant.ScreenerRecord

	if info.MemberId != nil && *info.MemberId != 0 {
		db = db.Where("member_id = ?", *info.MemberId)
	}

	if info.Prompt != nil && *info.Prompt != "" {
		db = db.Where("prompt LIKE ?", "%"+*info.Prompt+"%")
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
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

	err = db.Order("id desc").Find(&screenerRecords).Error
	return screenerRecords, total, err
}

// RecordQuery 记录查询或增加次数
func (screenerRecordService *ScreenerRecordService) RecordQuery(ctx context.Context, memberId *uint32, prompt string) error {
	// 清理 prompt 字符串（去除首尾空格）
	prompt = strings.TrimSpace(prompt)
	
	// 查找是否存在相同用户和相同prompt的记录（只查询未删除的记录）
	var existingRecord quant.ScreenerRecord
	
	// 添加详细日志
	global.GVA_LOG.Info("开始查询记录", 
		zap.Uint32("member_id", *memberId), 
		zap.String("prompt", prompt),
		zap.Int("prompt_length", len(prompt)),
		zap.String("prompt_hex", fmt.Sprintf("%x", prompt)))
	
	// 先查询所有该用户的记录，用于调试
	var allRecords []quant.ScreenerRecord
	global.GVA_DB.Where("member_id = ?", *memberId).Find(&allRecords)
	global.GVA_LOG.Info("该用户的所有记录", 
		zap.Int("count", len(allRecords)))
	for i, rec := range allRecords {
		if rec.Prompt != nil {
			global.GVA_LOG.Info("记录详情", 
				zap.Int("index", i),
				zap.Uint("id", rec.ID),
				zap.String("prompt", *rec.Prompt),
				zap.Int("prompt_length", len(*rec.Prompt)),
				zap.String("prompt_hex", fmt.Sprintf("%x", *rec.Prompt)),
				zap.Bool("match", *rec.Prompt == prompt))
		}
	}
	
	err := global.GVA_DB.Where("member_id = ? AND prompt = ?", *memberId, prompt).First(&existingRecord).Error
	
	if err == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		times := 1
		status := int8(1)
		newRecord := quant.ScreenerRecord{
			MemberId: memberId,
			Prompt:   &prompt,
			Times:    &times,
			Status:   &status,
		}
		global.GVA_LOG.Info("创建新的查询记录", 
			zap.Uint32("member_id", *memberId), 
			zap.String("prompt", prompt))
		return global.GVA_DB.Create(&newRecord).Error
	} else if err != nil {
		global.GVA_LOG.Error("查询记录时出错", zap.Error(err))
		return err
	}
	
	// 存在，增加次数
	global.GVA_LOG.Info("找到已存在记录，更新查询次数", 
		zap.Uint("record_id", existingRecord.ID), 
		zap.Int("current_times", *existingRecord.Times),
		zap.String("existing_prompt", *existingRecord.Prompt))
	return global.GVA_DB.Model(&quant.ScreenerRecord{}).
		Where("id = ?", existingRecord.ID).
		Update("times", gorm.Expr("times + 1")).Error
}
