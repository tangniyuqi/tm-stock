package addon

import (
	"errors"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/addon"
	addonReq "github.com/flipped-aurora/gin-vue-admin/server/model/addon/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WeishiService struct{}

// CreateWeishi 创建微视账号记录
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) CreateWeishi(weishi *addon.Weishi) (err error) {
	err = global.GVA_DB.Create(weishi).Error
	return err
}

// DeleteWeishi 删除微视账号记录
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) DeleteWeishi(ID string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&addon.Weishi{}).Where("id = ?", ID).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&addon.Weishi{}, "id = ?", ID).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteWeishiByIds 批量删除微视账号记录
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) DeleteWeishiByIds(IDs []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&addon.Weishi{}).Where("id in ?", IDs).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", IDs).Delete(&addon.Weishi{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateWeishi 更新微视账号记录
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) UpdateWeishi(weishi addon.Weishi) (err error) {
	err = global.GVA_DB.Model(&addon.Weishi{}).Where("id = ?", weishi.ID).Updates(&weishi).Error
	return err
}

// GetWeishi 根据ID获取微视账号记录
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) GetWeishi(ID string) (weishi addon.Weishi, err error) {
	err = global.GVA_DB.Where("id = ?", ID).First(&weishi).Error
	return
}

// GetWeishiInfoList 分页获取微视账号记录
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) GetWeishiInfoList(info addonReq.WeishiSearch, order string, desc string) (list []addon.Weishi, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&addon.Weishi{})
	var weishis []addon.Weishi
	// 如果有条件搜索 下方会自动创建搜索语句
	if info.StartCreatedAt != nil && info.EndCreatedAt != nil {
		db = db.Where("created_at BETWEEN ? AND ?", info.StartCreatedAt, info.EndCreatedAt)
	}
	if info.Account != nil && *info.Account != "" {
		db = db.Where("account LIKE ?", "%"+*info.Account+"%")
	}
	if info.IAuthType != nil {
		db = db.Where("iAuthType = ?", *info.IAuthType)
	}
	if info.Main_login != nil && *info.Main_login != "" {
		db = db.Where("main_login = ?", *info.Main_login)
	}
	if info.Openid != nil && *info.Openid != "" {
		db = db.Where("openid = ?", *info.Openid)
	}
	if info.Person_id != nil && *info.Person_id != "" {
		db = db.Where("person_id = ?", *info.Person_id)
	}
	if info.StartCancelled_at != nil && info.EndCancelled_at != nil {
		db = db.Where("cancelled_at BETWEEN ? AND ? ", info.StartCancelled_at, info.EndCancelled_at)
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

	OrderStr := "id desc"
	if order != "" {
		OrderStr = order
		if desc != "true" {
			OrderStr = order + " asc"
		}
	}

	err = db.Order(OrderStr).Find(&weishis).Error
	return weishis, total, err
}
func (weishiService *WeishiService) GetWeishiPublic() {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}

// BatchCreateWeishi 批量创建微视数据
func (weishiService *WeishiService) BatchCreateWeishi(weishiList []addon.Weishi) (err error) {
	// 如果传入的数据为空，直接返回错误
	if len(weishiList) == 0 {
		return errors.New("微视数据不能为空")
	}

	// 使用事务确保批量插入的原子性
	tx := global.GVA_DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback() // 回滚事务
			global.GVA_LOG.Error("批量创建微视失败: 事务回滚", zap.Any("recover", r))
			err = fmt.Errorf("批量创建微视失败: %v", r)
		}
	}()

	// 批量插入数据
	if err := tx.Create(&weishiList).Error; err != nil {
		tx.Rollback() // 插入失败，回滚事务
		global.GVA_LOG.Error("批量创建微视失败: 数据库插入错误", zap.Error(err))
		return fmt.Errorf("批量创建微视失败: %v", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		global.GVA_LOG.Error("提交事务失败", zap.Error(err))
		return fmt.Errorf("提交事务失败: %v", err)
	}

	global.GVA_LOG.Info("批量创建微视成功", zap.Int("数量", len(weishiList)))
	return nil
}

// CancelWeishi 注销微视账号
// Author [yourname](https://github.com/yourname)
func (weishiService *WeishiService) CancelWeishi() (err error) {
	// 请在这里实现自己的业务逻辑
	db := global.GVA_DB.Model(&addon.Weishi{})
	return db.Error
}
