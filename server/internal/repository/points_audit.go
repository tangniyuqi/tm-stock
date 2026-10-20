package repository

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ points.AuditScanner = (*PointsRepo)(nil)

func (r *PointsRepo) AuditUpperBound(ctx context.Context) (int64, error) {
	if r == nil || r.db == nil {
		return 0, points.ErrInvalid
	}
	var upper int64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(user_id),0) FROM tm_points_accounts`).Scan(&upper)
	return upper, pointsDBError("读取账本对账用户上界", err)
}

func (r *PointsRepo) ListAuditUsers(ctx context.Context, after, upper int64, limit int) ([]int64, error) {
	if r == nil || r.db == nil || after < 0 || upper < after || limit < 1 || limit > 1000 {
		return nil, points.ErrInvalid
	}
	rows, err := r.db.QueryContext(ctx, `SELECT user_id FROM tm_points_accounts WHERE user_id>? AND user_id<=? ORDER BY user_id LIMIT ?`, after, upper, limit)
	if err != nil {
		return nil, pointsDBError("分页读取账本对账用户", err)
	}
	defer rows.Close()
	users := make([]int64, 0, limit)
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			return nil, pointsDBError("读取对账用户编号", err)
		}
		users = append(users, uid)
	}
	return users, pointsDBError("读取账本对账用户列表", rows.Err())
}
