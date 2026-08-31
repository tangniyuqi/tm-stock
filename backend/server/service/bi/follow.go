package bi

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type FollowService struct{}

// CreateFollow 创建跟进记录
// Author [yourname](https://github.com/yourname)
func (followService *FollowService) CreateFollow(ctx context.Context, follow *bi.Follow) (err error) {
	err = global.GVA_DB.Create(follow).Error
	return err
}

// DeleteFollow 删除跟进记录
// Author [yourname](https://github.com/yourname)
func (followService *FollowService) DeleteFollow(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Follow{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Follow{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteFollowByIds 批量删除跟进记录
// Author [yourname](https://github.com/yourname)
func (followService *FollowService) DeleteFollowByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Follow{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Follow{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateFollow 更新跟进记录
// Author [yourname](https://github.com/yourname)
func (followService *FollowService) UpdateFollow(ctx context.Context, follow bi.Follow) (err error) {
	err = global.GVA_DB.Model(&bi.Follow{}).Where("id = ?", follow.ID).Updates(&follow).Error
	return err
}

// GetFollow 根据ID获取跟进记录
// Author [yourname](https://github.com/yourname)
func (followService *FollowService) GetFollow(ctx context.Context, id string) (follow bi.Follow, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&follow).Error
	return
}

// GetFollowInfoList 分页获取跟进记录
// Author [yourname](https://github.com/yourname)
func (followService *FollowService) GetFollowInfoList(ctx context.Context, info biReq.FollowSearch) (list []bi.Follow, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&bi.Follow{})
	var follows []bi.Follow
	// 如果有条件搜索 下方会自动创建搜索语句
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if info.DealId != nil {
		db = db.Where("deal_id = ?", *info.DealId)
	}
	if info.MsgId != nil {
		db = db.Where("msg_id = ?", *info.MsgId)
	}
	if info.MemberId != nil {
		db = db.Where("member_id = ?", *info.MemberId)
	}
	if info.CustomerId != nil {
		db = db.Where("customer_id = ?", *info.CustomerId)
	}
	if len(info.FollowTimeRange) == 2 {
		db = db.Where("follow_time BETWEEN ? AND ? ", info.FollowTimeRange[0], info.FollowTimeRange[1])
	}
	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}
	if info.Content != nil && *info.Content != "" {
		db = db.Where("content LIKE ?", "%"+*info.Content+"%")
	}
	if info.Plan != nil && *info.Plan != "" {
		db = db.Where("plan LIKE ?", "%"+*info.Plan+"%")
	}
	if info.Feedback != nil {
		db = db.Where("feedback = ?", *info.Feedback)
	}
	if info.Reason != nil && *info.Reason != "" {
		db = db.Where("reason LIKE ?", "%"+*info.Reason+"%")
	}
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Order("id desc").Find(&follows).Error
	return follows, total, err
}
func (followService *FollowService) GetFollowPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
