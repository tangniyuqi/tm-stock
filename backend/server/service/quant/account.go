package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type AccountService struct{}

// CreateAccount 创建账户记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) CreateAccount(ctx context.Context, account *quant.Account) (err error) {
	err = global.GVA_DB.Create(account).Error
	return err
}

// DeleteAccount 删除账户记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) DeleteAccount(ctx context.Context, ID string, userid uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Account{}).Where("id = ?", ID).Update("deleted_by", userid).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.Account{}, "id = ?", ID).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteAccountByIds 批量删除账户记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) DeleteAccountByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Account{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.Account{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateAccount 更新账户记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) UpdateAccount(ctx context.Context, account quant.Account) (err error) {
	err = global.GVA_DB.Model(&quant.Account{}).Where("id = ?", account.ID).Updates(&account).Error
	return err
}

// GetAccount 根据ID获取账户记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) GetAccount(ctx context.Context, id string) (account quant.Account, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&account).Error
	return
}

// GetAccountInfoList 分页获取账户记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) GetAccountInfoList(ctx context.Context, info quantReq.AccountSearch) (list []quant.Account, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.Account{})
	var accounts []quant.Account

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}

	if info.MemberId != nil && *info.MemberId != 0 {
		db = db.Where("member_id = ?", *info.MemberId)
	}

	if info.Broker != nil && *info.Broker != "" {
		db = db.Where("broker LIKE ?", "%"+*info.Broker+"%")
	}

	if info.AccountNo != nil && *info.AccountNo != "" {
		db = db.Where("account_no LIKE ?", "%"+*info.AccountNo+"%")
	}

	// 到期时间筛选
	if info.ExpirationDay != nil && *info.ExpirationDay != 0 {
		now := global.GVA_DB.NowFunc()
		days := *info.ExpirationDay

		if days == -1 {
			db = db.Where("expiration_date < ?", now)
		} else if days > 0 {
			db = db.Where("expiration_date BETWEEN ? AND DATE_ADD(?, INTERVAL ? DAY)", now, now, days)
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

	// 预加载用户信息
	err = db.Preload("Member").Find(&accounts).Error
	return accounts, total, err
}

func (accountService *AccountService) GetAccountPublic(ctx context.Context, id string) (account quant.Account, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&account).Error
	return
}

// GetMyAccount 根据当前用户获取最新的账户记录
func (accountService *AccountService) GetMyAccount(ctx context.Context, userID uint) (account quant.Account, err error) {
	err = global.GVA_DB.Where("member_id = ?", userID).Order("id desc").First(&account).Error
	if err == nil {
		global.GVA_DB.Model(&quant.TradeTask{}).Where("account_id = ? AND status = 1", account.ID).Count(&account.RunningTaskCount)
	}
	return
}
