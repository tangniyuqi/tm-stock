package quant

import (
	"context"
	"errors"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradeTaskService struct{}

// CreateTradeTask 创建交易任务记录
func (tradeTaskService *TradeTaskService) CreateTradeTask(ctx context.Context, tradeTask *quant.TradeTask) (err error) {
	var account quant.Account
	if err = global.GVA_DB.First(&account, tradeTask.AccountId).Error; err != nil {
		return err
	}
	if account.ExpirationDate != nil && time.Now().Format("2006-01-02") > account.ExpirationDate.Format("2006-01-02") {
		return errors.New("账户已到期，无法创建新任务")
	}

	err = global.GVA_DB.Create(tradeTask).Error
	return err
}

// DeleteTradeTask 删除交易任务记录
func (tradeTaskService *TradeTaskService) DeleteTradeTask(ctx context.Context, id string, userID uint) (err error) {
	var tradeTask quant.TradeTask
	if err = global.GVA_DB.Where("id = ?", id).First(&tradeTask).Error; err != nil {
		return err
	}
	if tradeTask.Status != nil && *tradeTask.Status == 1 {
		return errors.New("任务正在运行中，无法删除")
	}

	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.TradeTask{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.TradeTask{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteTradeTaskByIds 批量删除交易任务记录
func (tradeTaskService *TradeTaskService) DeleteTradeTaskByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	var tradeTasks []quant.TradeTask
	if err = global.GVA_DB.Where("id in ?", ids).Find(&tradeTasks).Error; err != nil {
		return err
	}
	for _, task := range tradeTasks {
		if task.Status != nil && *task.Status == 1 {
			return errors.New("包含正在运行的任务，无法批量删除")
		}
	}

	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.TradeTask{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.TradeTask{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateTradeTask 更新交易任务记录
func (tradeTaskService *TradeTaskService) UpdateTradeTask(ctx context.Context, tradeTask quant.TradeTask) (err error) {
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if tradeTask.Status != nil && *tradeTask.Status == 1 {
			var existingTask quant.TradeTask

			if err = tx.Where("id = ?", tradeTask.ID).First(&existingTask).Error; err != nil {
				return err
			}

			// 只有在任务从非运行状态切换到运行状态时才检查过期
			if existingTask.Status == nil || *existingTask.Status != 1 {
				var accountId uint32
				if tradeTask.AccountId != nil {
					accountId = *tradeTask.AccountId
				} else if existingTask.AccountId != nil {
					accountId = *existingTask.AccountId
				}

				if accountId == 0 {
					return errors.New("未关联账户，无法启动任务")
				}

				var account quant.Account
				// 加锁防止并发突破最大任务数限制
				if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountId).Error; err != nil {
					return errors.New("无法获取账户信息")
				}

				if account.ExpirationDate != nil && time.Now().Format("2006-01-02") > account.ExpirationDate.Format("2006-01-02") {
					return errors.New("账户已到期，无法启动任务")
				}

				var count int64
				// 使用当前读(FOR UPDATE)来确保读取到最新的运行任务数量，避免RR隔离级别下的快照读导致并发限制失效
				tx.Model(&quant.TradeTask{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("account_id = ? AND status = 1", accountId).Count(&count)
				if account.MaxRunningTask != nil && count >= int64(*account.MaxRunningTask) {
					return errors.New("交易任务运行数量已达最大上限")
				}
			}
		}
		err = tx.Model(&quant.TradeTask{}).Where("id = ?", tradeTask.ID).Updates(&tradeTask).Error
		return err
	})
}

// GetTradeTask 根据ID获取交易任务记录
func (tradeTaskService *TradeTaskService) GetTradeTask(ctx context.Context, ID string) (tradeTask quant.TradeTask, err error) {
	err = global.GVA_DB.Preload("Strategy").Preload("Account").Where("id = ?", ID).First(&tradeTask).Error
	return
}

// GetTradeTaskInfoList 分页获取交易任务记录
func (tradeTaskService *TradeTaskService) GetTradeTaskInfoList(ctx context.Context, info quantReq.TradeTaskSearch) (list []quant.TradeTask, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&quant.TradeTask{})
	var tradeTasks []quant.TradeTask

	if info.MemberId != nil {
		db = db.Where("member_id = ?", *info.MemberId)
	}

	if info.AccountId != nil {
		db = db.Where("account_id = ?", *info.AccountId)
	}

	if info.StrategyId != nil {
		db = db.Where("strategy_id = ?", *info.StrategyId)
	}

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}

	if info.Keyword != "" {
		keyword := "%" + info.Keyword + "%"
		db = db.Where("name LIKE ? OR CAST(stock AS CHAR) LIKE ? OR remark LIKE ?", keyword, keyword, keyword)
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+*info.Remark+"%")
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if info.CreatedBy != nil {
		db = db.Where("created_by = ?", *info.CreatedBy)
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Preload("Strategy").Preload("Account").Order("updated_at desc, id desc").Find(&tradeTasks).Error
	return tradeTasks, total, err
}

func (tradeTaskService *TradeTaskService) GetTradeTaskPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
