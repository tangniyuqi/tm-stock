package repository

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const taskEventReadSQL = `SELECT e.task_id,e.seq,e.type,e.status,e.step_no,e.error_code,CAST(e.created_at AS CHAR) FROM tm_ai_task_events e JOIN tm_ai_tasks t ON t.id=e.task_id AND t.user_id=? WHERE e.task_id=? AND e.seq>? AND e.seq<=? ORDER BY e.seq LIMIT ?`

func scanTaskEventRecord(s pointsScanner) (task.EventRecord, error) {
	var out task.EventRecord
	var raw string
	if err := s.Scan(&out.TaskID, &out.Seq, &out.Type, &out.Status, &out.Step, &out.ErrorCode, &raw); err != nil {
		return task.EventRecord{}, pointsDBError("读取会员任务事件", err)
	}
	var err error
	out.CreatedAt, err = parsePointsUTC(raw)
	if err != nil {
		return task.EventRecord{}, err
	}
	return out, nil
}

func (t *aiTaskReadTx) Events(ctx context.Context, uid, id, after, through int64, limit int) ([]task.EventRecord, error) {
	if t == nil || t.tx == nil || uid <= 0 || id <= 0 || after < 0 || through < after || limit < 1 || limit > 101 {
		return nil, points.ErrInvalid
	}
	// 归属过滤 + (task_id,seq) 主键范围；沿用同一只读 RR 快照，不读取新上界。
	rows, err := t.tx.QueryContext(ctx, taskEventReadSQL, uid, id, after, through, limit)
	if err != nil {
		return nil, pointsDBError("读取会员任务事件页", err)
	}
	defer rows.Close()
	out := make([]task.EventRecord, 0)
	for rows.Next() {
		row, err := scanTaskEventRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, pointsDBError("读取会员任务事件页", rows.Err())
}
