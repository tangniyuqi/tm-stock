package cloud

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	cloudReq "github.com/flipped-aurora/gin-vue-admin/server/model/cloud/request"
	"gorm.io/gorm"
)

type AccountService struct{}

// CreateAccount 创建财务记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) CreateAccount(ctx context.Context, account *cloud.Account) (err error) {
	err = global.GVA_DB.Create(account).Error
	return err
}

// DeleteAccount 删除财务记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) DeleteAccount(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Account{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cloud.Account{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteAccountByIds 批量删除财务记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) DeleteAccountByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Account{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cloud.Account{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateAccount 更新财务记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) UpdateAccount(ctx context.Context, account cloud.Account) (err error) {
	err = global.GVA_DB.Model(&cloud.Account{}).Where("id = ?", account.ID).Save(&account).Error
	return err
}

// GetAccount 根据ID获取财务记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) GetAccount(ctx context.Context, id string) (account cloud.Account, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&account).Error
	return
}

// GetAccountInfoList 分页获取财务记录
// Author [yourname](https://github.com/yourname)
func (accountService *AccountService) GetAccountInfoList(ctx context.Context, info cloudReq.AccountSearch) (list []cloud.Account, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&cloud.Account{})
	var accounts []cloud.Account

	if info.CreatedBy != 1 {
		db = db.Where("created_by = ?", info.CreatedBy)
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Title != nil && *info.Title != "" {
		db = db.Where("title LIKE ?", "%"+*info.Title+"%")
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+*info.Remark+"%")
	}

	if info.Balance != nil {
		db = db.Where("balance = ?", *info.Balance)
	}

	if info.Review != nil {
		db = db.Where("review = ?", *info.Review)
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

	err = db.Order("handling_date desc, id desc").Find(&accounts).Error
	return accounts, total, err
}

func (accountService *AccountService) GetAccountPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
