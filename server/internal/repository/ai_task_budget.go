package repository

import (
	"context"
	"errors"
	"fmt"

	mysqldrv "github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ task.BudgetStore = (*AITaskRepo)(nil)
var _ task.BudgetTx = (*aiTaskTx)(nil)

func (r *AITaskRepo) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	if fn == nil {
		return points.ErrInvalid
	}
	return r.WithinTaskTx(ctx, func(tx task.Tx) error { return fn(tx.(*aiTaskTx)) })
}
func (t *aiTaskTx) LockBudget(ctx context.Context, uid, id int64) (task.BudgetState, error) {
	var s task.BudgetState
	err := t.tx.QueryRowContext(ctx, `SELECT task_id,user_id,limit_calls,limit_tokens,limit_pages,reserved_calls,reserved_tokens,reserved_pages,reservation_count FROM tm_ai_task_budgets WHERE task_id=? AND user_id=? FOR UPDATE`, id, uid).Scan(&s.TaskID, &s.UserID, &s.Limits.Calls, &s.Limits.Tokens, &s.Limits.Pages, &s.Reserved.Calls, &s.Reserved.Tokens, &s.Reserved.Pages, &s.Count)
	if err != nil {
		return task.BudgetState{}, pointsDBError("读取任务预算", err)
	}
	return s, nil
}
func (t *aiTaskTx) InitBudget(ctx context.Context, s task.BudgetState) (bool, error) {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_task_budgets(task_id,user_id,limit_calls,limit_tokens,limit_pages,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, s.TaskID, s.UserID, s.Limits.Calls, s.Limits.Tokens, s.Limits.Pages)
	// 不先锁不存在的行，不用 IGNORE/无条件 upsert 掩盖触发器或数据错误。
	var duplicate *mysqldrv.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return false, nil
	}
	if err != nil {
		return false, pointsDBError("初始化任务预算", err)
	}
	return true, nil
}
func (t *aiTaskTx) Reservations(ctx context.Context, uid, id int64) ([]task.Reservation, error) {
	// FOR UPDATE 是当前读；父事务的旧 RR 快照不能漏读其他已提交预留。
	rows, err := t.tx.QueryContext(ctx, `SELECT task_id,user_id,fence,seq,owner,reservation_key,input_digest,calls,tokens,pages,CAST(created_at AS CHAR) FROM tm_ai_task_reservations WHERE task_id=? AND user_id=? ORDER BY seq LIMIT 1001 FOR UPDATE`, id, uid)
	if err != nil {
		return nil, pointsDBError("读取任务预留", err)
	}
	defer rows.Close()
	out := make([]task.Reservation, 0)
	for rows.Next() {
		var v task.Reservation
		var raw string
		if err := rows.Scan(&v.TaskID, &v.UserID, &v.Fence, &v.Seq, &v.Owner, &v.Key, &v.InputDigest, &v.Units.Calls, &v.Units.Tokens, &v.Units.Pages, &raw); err != nil {
			return nil, pointsDBError("读取任务预留", err)
		}
		v.CreatedAt, err = parsePointsUTC(raw)
		if err != nil {
			return nil, fmt.Errorf("任务预留时点损坏: %w", err)
		}
		out = append(out, v)
	}
	return out, pointsDBError("读取任务预留", rows.Err())
}
func (t *aiTaskTx) InsertReservation(ctx context.Context, v task.Reservation) error {
	return pointsDBError("写入任务预算预留", t.insertReservation(ctx, v))
}

// insertReservation 保留原始 SQL 判据，供同包回滚探针区分唯一冲突与其他故障。
func (t *aiTaskTx) insertReservation(ctx context.Context, v task.Reservation) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_task_reservations(task_id,user_id,seq,reservation_key,input_digest,owner,fence,calls,tokens,pages,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, v.TaskID, v.UserID, v.Seq, v.Key, v.InputDigest, v.Owner, v.Fence, v.Units.Calls, v.Units.Tokens, v.Units.Pages, pointsUTC(v.CreatedAt), pointsUTC(v.CreatedAt))
	return err
}
