package cms

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"gorm.io/gorm"
)

type FeedbackService struct{}

// CreateFeedback 创建反馈记录
// Author [yourname](https://github.com/yourname)
func (feedbackService *FeedbackService) CreateFeedback(ctx context.Context, feedback *cms.Feedback) (err error) {
	err = global.GVA_DB.WithContext(ctx).Create(feedback).Error
	return err
}

// DeleteFeedback 删除反馈记录
// Author [yourname](https://github.com/yourname)
func (feedbackService *FeedbackService) DeleteFeedback(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Feedback{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cms.Feedback{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteFeedbackByIds 批量删除反馈记录
// Author [yourname](https://github.com/yourname)
func (feedbackService *FeedbackService) DeleteFeedbackByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Feedback{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cms.Feedback{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateFeedback 更新反馈记录
// Author [yourname](https://github.com/yourname)
func (feedbackService *FeedbackService) UpdateFeedback(ctx context.Context, feedback cms.Feedback) (err error) {
	err = global.GVA_DB.WithContext(ctx).Model(&cms.Feedback{}).Where("id = ?", feedback.ID).Updates(&feedback).Error
	return err
}

// GetFeedback 根据ID获取反馈记录
// Author [yourname](https://github.com/yourname)
func (feedbackService *FeedbackService) GetFeedback(ctx context.Context, id string) (feedback cms.Feedback, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&feedback).Error
	return
}

// GetFeedbackInfoList 分页获取反馈记录
// Author [yourname](https://github.com/yourname)
func (feedbackService *FeedbackService) GetFeedbackInfoList(ctx context.Context, info cmsReq.FeedbackSearch) (list []cms.Feedback, total int64, err error) {
	limit, offset := info.LimitOffset()
	// 创建db
	db := global.GVA_DB.WithContext(ctx).Model(&cms.Feedback{})
	var feedbacks []cms.Feedback
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

	err = db.Find(&feedbacks).Error
	return feedbacks, total, err
}

// CreateFeedbackPublic C 端公开提交反馈（无需登录）
func (feedbackService *FeedbackService) CreateFeedbackPublic(ctx context.Context, feedback *cms.Feedback) (err error) {
	err = global.GVA_DB.WithContext(ctx).Create(feedback).Error
	return err
}
