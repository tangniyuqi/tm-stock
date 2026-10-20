package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type MemberIdentityRepo struct{ db *sql.DB }

func NewMemberIdentityRepo(db *sql.DB) *MemberIdentityRepo { return &MemberIdentityRepo{db} }

// EnabledMember 每次请求查询，不读积分、手机号或密码，也不缓存启用状态。
func (r *MemberIdentityRepo) EnabledMember(ctx context.Context, uid int64) (bool, error) {
	if r == nil || r.db == nil || uid <= 0 {
		return false, memberauth.ErrUnavailable
	}
	var status int
	err := r.db.QueryRowContext(ctx, `SELECT status FROM addon_member WHERE id=? AND deleted_at IS NULL`, uid).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, memberauth.ErrUnavailable
	}
	return status == 1, nil
}
