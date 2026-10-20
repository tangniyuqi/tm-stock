package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ task.MemberQueueScanner = (*AITaskRepo)(nil)

func (r *AITaskRepo) MemberQueueRange(ctx context.Context) (task.MemberQueueRange, error) {
	if r == nil || r.db == nil {
		return task.MemberQueueRange{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var out task.MemberQueueRange
	var at string
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0),CAST(UTC_TIMESTAMP(3) AS CHAR) FROM tm_ai_tasks`).Scan(&out.UpperID, &at); err != nil {
		return out, pointsDBError("读取会员编排范围", err)
	}
	var err error
	out.AsOf, err = parsePointsUTC(at)
	return out, err
}

func scanMemberQueueCandidate(row pointsScanner) (task.MemberQueueCandidate, error) {
	var out task.MemberQueueCandidate
	var before, deadline string
	var lease sql.NullString
	if err := row.Scan(&out.ID, &out.UserID, &out.AnalysisType, &out.SubjectType, &out.Status, &before, &deadline, &lease, &out.HasAdmission); err != nil {
		return out, pointsDBError("读取会员编排候选", err)
	}
	var err error
	if out.NotBefore, err = parsePointsUTC(before); err != nil {
		return task.MemberQueueCandidate{}, err
	}
	if out.Deadline, err = parsePointsUTC(deadline); err != nil {
		return task.MemberQueueCandidate{}, err
	}
	if lease.Valid {
		at, err := parsePointsUTC(lease.String)
		if err != nil {
			return task.MemberQueueCandidate{}, err
		}
		out.LeaseUntil = &at
	}
	return out, nil
}

func (r *AITaskRepo) MemberQueuePage(ctx context.Context, after, upper int64, limit int) ([]task.MemberQueueCandidate, error) {
	if r == nil || r.db == nil || after < 0 || upper < after || limit < 1 || limit > task.MaxMemberQueueTasks+1 {
		return nil, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// 主键范围限制实际访问行数；资格不先放到WHERE，避免稀疏候选无界扫历史。
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,analysis_type,subject_type,status,CAST(not_before AS CHAR),CAST(deadline_at AS CHAR),CAST(lease_until AS CHAR),admission_digest IS NOT NULL FROM tm_ai_tasks WHERE id>? AND id<=? ORDER BY id LIMIT ?`, after, upper, limit)
	if err != nil {
		return nil, pointsDBError("读取会员编排分页", err)
	}
	defer rows.Close()
	page := make([]task.MemberQueueCandidate, 0, limit)
	for rows.Next() {
		v, err := scanMemberQueueCandidate(rows)
		if err != nil {
			return nil, err
		}
		page = append(page, v)
	}
	if err := rows.Err(); err != nil {
		return nil, pointsDBError("读取会员编排分页游标", err)
	}
	return page, nil
}
