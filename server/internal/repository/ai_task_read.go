package repository

import (
	"context"
	"database/sql"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ task.ReadStore = (*AITaskRepo)(nil)

type aiTaskReadTx struct{ tx *sql.Tx }

func (r *AITaskRepo) WithinTaskRead(ctx context.Context, fn func(task.ReadTx) error) error {
	if r == nil || r.db == nil || fn == nil {
		return points.ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return pointsDBError("开启任务只读事务", err)
	}
	// 提交后回滚的 ErrTxDone 不改变结果；任意读取失败回滚。
	defer func() { _ = tx.Rollback() }()
	if err := fn(&aiTaskReadTx{tx}); err != nil {
		return err
	}
	return pointsDBError("结束任务只读事务", tx.Commit())
}

func (t *aiTaskReadTx) Task(ctx context.Context, uid, id int64) (task.Task, error) {
	return scanAITask(t.tx.QueryRowContext(ctx, `SELECT `+aiTaskColumns+` FROM tm_ai_tasks WHERE user_id=? AND id=?`, uid, id))
}

func (t *aiTaskReadTx) History(ctx context.Context, uid, before int64, limit int) ([]task.Task, error) {
	// (user_id,id) 索引倒序游标；不联表、不 COUNT、不接受任意排序表达式。
	rows, err := t.tx.QueryContext(ctx, `SELECT `+aiTaskColumns+` FROM tm_ai_tasks WHERE user_id=? AND id<? ORDER BY id DESC LIMIT ?`, uid, before, limit)
	if err != nil {
		return nil, pointsDBError("读取会员任务历史", err)
	}
	defer rows.Close()
	out := make([]task.Task, 0)
	for rows.Next() {
		v, err := scanAITask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, pointsDBError("读取会员任务历史", rows.Err())
}
