package repository

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ task.ExpiryScanner = (*AITaskRepo)(nil)

func (r *AITaskRepo) ExpiryRange(ctx context.Context) (task.ExpiryRange, error) {
	if r == nil || r.db == nil {
		return task.ExpiryRange{}, points.ErrInvalid
	}
	return scanExpiryRange(r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0),CAST(UTC_TIMESTAMP(3) AS CHAR) FROM tm_ai_tasks`))
}

func scanExpiryRange(row pointsScanner) (task.ExpiryRange, error) {
	var out task.ExpiryRange
	var at string
	if err := row.Scan(&out.UpperID, &at); err != nil {
		return task.ExpiryRange{}, pointsDBError("读取过期清扫范围", err)
	}
	var err error
	out.AsOf, err = parsePointsUTC(at)
	if err != nil {
		return task.ExpiryRange{}, err
	}
	return out, nil
}

func scanExpiryCandidate(row pointsScanner) (task.ExpiryCandidate, error) {
	var out task.ExpiryCandidate
	var deadline string
	if err := row.Scan(&out.ID, &out.UserID, &out.Status, &deadline); err != nil {
		return task.ExpiryCandidate{}, pointsDBError("读取过期清扫任务", err)
	}
	var err error
	out.Deadline, err = parsePointsUTC(deadline)
	if err != nil {
		return task.ExpiryCandidate{}, err
	}
	return out, nil
}

func (r *AITaskRepo) ExpiryPage(ctx context.Context, after, upper int64, limit int) ([]task.ExpiryCandidate, error) {
	if r == nil || r.db == nil || after < 0 || upper < after || limit < 1 || limit > 1000 {
		return nil, points.ErrInvalid
	}
	// 不按期限过滤后再 LIMIT：主键游标限制实际读取行数，避免稀疏过期任务扫完整历史。
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,status,CAST(deadline_at AS CHAR) FROM tm_ai_tasks WHERE id>? AND id<=? ORDER BY id LIMIT ?`, after, upper, limit)
	if err != nil {
		return nil, pointsDBError("读取过期清扫分页", err)
	}
	defer rows.Close()
	page := make([]task.ExpiryCandidate, 0, limit)
	for rows.Next() {
		v, err := scanExpiryCandidate(rows)
		if err != nil {
			return nil, err
		}
		page = append(page, v)
	}
	if err := rows.Err(); err != nil {
		return nil, pointsDBError("读取过期清扫分页游标", err)
	}
	return page, nil
}
