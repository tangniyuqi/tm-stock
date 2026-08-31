package bi

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type ClientService struct{}

// CreateClient 创建客户端记录
// Author [yourname](https://github.com/yourname)
func (clientService *ClientService) CreateClient(ctx context.Context, client *bi.Client) (err error) {
	err = global.GVA_DB.Create(client).Error
	return err
}

// DeleteClient 删除客户端记录
// Author [yourname](https://github.com/yourname)
func (clientService *ClientService) DeleteClient(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Client{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Client{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteClientByIds 批量删除客户端记录
// Author [yourname](https://github.com/yourname)
func (clientService *ClientService) DeleteClientByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Client{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Client{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateClient 更新客户端记录
// Author [yourname](https://github.com/yourname)
func (clientService *ClientService) UpdateClient(ctx context.Context, client bi.Client) (err error) {
	err = global.GVA_DB.Model(&bi.Client{}).Where("id = ?", client.ID).Updates(&client).Error
	return err
}

// GetClient 根据ID获取客户端记录
// Author [yourname](https://github.com/yourname)
func (clientService *ClientService) GetClient(ctx context.Context, id string) (client bi.Client, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&client).Error
	return
}

// GetClientInfoList 分页获取客户端记录
// Author [yourname](https://github.com/yourname)
func (clientService *ClientService) GetClientInfoList(ctx context.Context, info biReq.ClientSearch) (list []bi.Client, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&bi.Client{})
	var clients []bi.Client

	if info.Name != nil && *info.Name != "" {
		db = db.Where("`name` LIKE ?", "%"+*info.Name+"%")
	}

	if info.Server != nil && *info.Server != "" {
		db = db.Where("`server` LIKE ?", "%"+*info.Server+"%")
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

	err = db.Order("id desc").Find(&clients).Error
	return clients, total, err
}
func (clientService *ClientService) GetClientPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
