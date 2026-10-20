package repository

import (
	"context"
	"errors"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ task.StageStore = (*AITaskRepo)(nil)
var _ task.StageTx = (*aiTaskTx)(nil)
var _ task.ExecutionReadTx = (*aiTaskTx)(nil)

func (r *AITaskRepo) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	if fn == nil {
		return points.ErrInvalid
	}
	return r.WithinTaskTx(ctx, func(tx task.Tx) error { return fn(tx.(*aiTaskTx)) })
}
func (t *aiTaskTx) InsertStageResult(ctx context.Context, v task.StageResult) (bool, error) {
	err := t.insertStageResult(ctx, v)
	var duplicate *mysqldrv.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return false, nil
	}
	if err != nil {
		return false, pointsDBError("保存隔离阶段结果", err)
	}
	return true, nil
}
func (t *aiTaskTx) insertStageResult(ctx context.Context, v task.StageResult) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_task_step_results(task_id,user_id,reservation_seq,step_no,stage,owner,fence,input_digest,output_digest,payload,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, v.TaskID, v.UserID, v.ReservationSeq, v.StepNo, v.Stage, v.Owner, v.Fence, v.InputDigest, v.OutputDigest, v.Payload, pointsUTC(v.CreatedAt), pointsUTC(v.CreatedAt))
	return err
}
func (t *aiTaskTx) StageResult(ctx context.Context, uid, id int64, seq int) (task.StageResult, error) {
	var v task.StageResult
	var raw string
	err := t.tx.QueryRowContext(ctx, `SELECT task_id,user_id,reservation_seq,step_no,stage,owner,fence,input_digest,output_digest,payload,CAST(created_at AS CHAR) FROM tm_ai_task_step_results WHERE task_id=? AND reservation_seq=? AND user_id=? FOR UPDATE`, id, seq, uid).Scan(&v.TaskID, &v.UserID, &v.ReservationSeq, &v.StepNo, &v.Stage, &v.Owner, &v.Fence, &v.InputDigest, &v.OutputDigest, &v.Payload, &raw)
	if err != nil {
		return task.StageResult{}, pointsDBError("读取隔离阶段结果", err)
	}
	v.CreatedAt, err = parsePointsUTC(raw)
	if err != nil {
		return task.StageResult{}, err
	}
	return v, nil
}
func (t *aiTaskTx) InsertStageCheckpoint(ctx context.Context, c task.StageCheckpoint) error {
	return pointsDBError("保存阶段检查点", t.insertStageCheckpoint(ctx, c))
}
func (t *aiTaskTx) insertStageCheckpoint(ctx context.Context, c task.StageCheckpoint) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_task_steps(task_id,user_id,step_no,reservation_seq,stage,owner,fence,input_digest,output_digest,policy_digest,event_seq,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.TaskID, c.UserID, c.StepNo, c.ReservationSeq, c.Stage, c.Owner, c.Fence, c.InputDigest, c.OutputDigest, c.PolicyDigest, c.EventSeq, pointsUTC(c.CreatedAt), pointsUTC(c.CreatedAt))
	return err
}
func (t *aiTaskTx) StageCheckpoint(ctx context.Context, uid, id int64, step int) (task.StageCheckpoint, error) {
	var c task.StageCheckpoint
	var raw string
	err := t.tx.QueryRowContext(ctx, `SELECT task_id,user_id,step_no,reservation_seq,stage,owner,fence,input_digest,output_digest,policy_digest,event_seq,CAST(created_at AS CHAR) FROM tm_ai_task_steps WHERE task_id=? AND step_no=? AND user_id=? FOR UPDATE`, id, step, uid).Scan(&c.TaskID, &c.UserID, &c.StepNo, &c.ReservationSeq, &c.Stage, &c.Owner, &c.Fence, &c.InputDigest, &c.OutputDigest, &c.PolicyDigest, &c.EventSeq, &raw)
	if err != nil {
		return task.StageCheckpoint{}, pointsDBError("读取阶段检查点", err)
	}
	c.CreatedAt, err = parsePointsUTC(raw)
	if err != nil {
		return task.StageCheckpoint{}, err
	}
	return c, nil
}
func (t *aiTaskTx) StageEvent(ctx context.Context, id, seq int64) (task.Event, error) {
	var e task.Event
	err := t.tx.QueryRowContext(ctx, `SELECT task_id,seq,type,status,step_no,error_code FROM tm_ai_task_events WHERE task_id=? AND seq=? FOR UPDATE`, id, seq).Scan(&e.TaskID, &e.Seq, &e.Type, &e.Status, &e.Step, &e.ErrorCode)
	if err != nil {
		return task.Event{}, pointsDBError("读取阶段事件", err)
	}
	return e, nil
}
