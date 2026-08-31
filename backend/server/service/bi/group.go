package bi

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type GroupService struct{}

// CreateGroup 创建群组记录
// Author [yourname](https://github.com/yourname)
func (groupService *GroupService) CreateGroup(ctx context.Context, group *bi.Group) (err error) {
	err = global.GVA_DB.Create(group).Error
	return err
}

// DeleteGroup 删除群组记录
// Author [yourname](https://github.com/yourname)
func (groupService *GroupService) DeleteGroup(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Group{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Group{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteGroupByIds 批量删除群组记录
// Author [yourname](https://github.com/yourname)
func (groupService *GroupService) DeleteGroupByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Group{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Group{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateGroup 更新群组记录
// Author [yourname](https://github.com/yourname)
func (groupService *GroupService) UpdateGroup(ctx context.Context, group bi.Group) (err error) {
	err = global.GVA_DB.Model(&bi.Group{}).Where("id = ?", group.ID).Updates(&group).Error
	return err
}

// GetGroup 根据ID获取群组记录
// Author [yourname](https://github.com/yourname)
func (groupService *GroupService) GetGroup(ctx context.Context, id string) (group bi.Group, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&group).Error
	return
}

// GetGroupInfoList 分页获取群组记录
// Author [yourname](https://github.com/yourname)
func (groupService *GroupService) GetGroupInfoList(ctx context.Context, info biReq.GroupSearch) (list []bi.Group, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&bi.Group{})
	var groups []bi.Group
	// 如果有条件搜索 下方会自动创建搜索语句
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

	err = db.Order("id desc").Find(&groups).Error
	return groups, total, err
}
func (groupService *GroupService) GetGroupPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
