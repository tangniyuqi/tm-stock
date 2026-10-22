package feedbackconsole

import (
	"context"
	"gorm.io/gorm"
)

type AdminIdentity struct{ db *gorm.DB }

func NewAdminIdentity(db *gorm.DB) *AdminIdentity { return &AdminIdentity{db} }

// 每次操作复核当前后台用户，不能只信未到期JWT和启动时登记。
func (s *AdminIdentity) Active(ctx context.Context, uid int64) (bool, error) {
	if s == nil || s.db == nil || !ID(uid) {
		return false, ErrForbidden
	}
	var n int64
	err := s.db.WithContext(ctx).Table("sys_users").Where("id = ? AND enable = ? AND deleted_at IS NULL", uid, 1).Count(&n).Error
	return n == 1, err
}
