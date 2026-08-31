package cloud

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	cloudReq "github.com/flipped-aurora/gin-vue-admin/server/model/cloud/request"
	"gorm.io/gorm"
)

type CustomerService struct{}

// CreateCustomer 创建客户记录
// Author [yourname](https://github.com/yourname)
func (customerService *CustomerService) CreateCustomer(ctx context.Context, customer *cloud.Customer) (err error) {
	err = global.GVA_DB.Create(customer).Error
	return err
}

// DeleteCustomer 删除客户记录
// Author [yourname](https://github.com/yourname)
func (customerService *CustomerService) DeleteCustomer(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Customer{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cloud.Customer{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteCustomerByIds 批量删除客户记录
// Author [yourname](https://github.com/yourname)
func (customerService *CustomerService) DeleteCustomerByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cloud.Customer{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cloud.Customer{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateCustomer 更新客户记录
// Author [yourname](https://github.com/yourname)
func (customerService *CustomerService) UpdateCustomer(ctx context.Context, customer cloud.Customer) (err error) {
	err = global.GVA_DB.Model(&cloud.Customer{}).Where("id = ?", customer.ID).Save(&customer).Error
	return err
}

// GetCustomer 根据ID获取客户记录
// Author [yourname](https://github.com/yourname)
func (customerService *CustomerService) GetCustomer(ctx context.Context, id string) (customer cloud.Customer, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&customer).Error
	return
}

// GetCustomerInfoList 分页获取客户记录
// Author [yourname](https://github.com/yourname)
func (customerService *CustomerService) GetCustomerInfoList(ctx context.Context, info cloudReq.CustomerSearch) (list []cloud.Customer, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&cloud.Customer{})
	var customers []cloud.Customer

	if info.CreatedBy != 1 {
		db = db.Where("created_by = ?", info.CreatedBy)
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Level != nil {
		db = db.Where("level = ?", *info.Level)
	}

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}

	if info.Tips != nil && *info.Tips != "" {
		db = db.Where("tips LIKE ?", "%"+*info.Tips+"%")
	}

	if info.Mobile != nil && *info.Mobile != "" {
		db = db.Where("mobile LIKE ?", "%"+*info.Mobile+"%")
	}

	if info.Wechat != nil && *info.Wechat != "" {
		db = db.Where("wechat LIKE ?", "%"+*info.Wechat+"%")
	}

	if info.Douyin != nil && *info.Douyin != "" {
		db = db.Where("douyin LIKE ?", "%"+*info.Douyin+"%")
	}

	if info.Qq != nil && *info.Qq != "" {
		db = db.Where("qq LIKE ?", "%"+*info.Qq+"%")
	}

	if info.Industry != nil {
		db = db.Where("industry = ?", *info.Industry)
	}

	if info.Company != nil && *info.Company != "" {
		db = db.Where("company LIKE ?", "%"+*info.Company+"%")
	}

	if info.Business != nil && *info.Business != "" {
		db = db.Where("business LIKE ?", "%"+*info.Business+"%")
	}

	if info.Source != nil {
		db = db.Where("source = ?", *info.Source)
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+*info.Remark+"%")
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

	err = db.Order("id desc").Find(&customers).Error
	return customers, total, err
}
func (customerService *CustomerService) GetCustomerPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
