package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type AITaskRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAITaskRepo(db *sql.DB) *AITaskRepo {
	return &AITaskRepo{db: db, isolation: sql.LevelReadCommitted}
}
func NewAITaskRepoWithIsolation(db *sql.DB, level sql.IsolationLevel) *AITaskRepo {
	return &AITaskRepo{db: db, isolation: level}
}

var _ task.Store = (*AITaskRepo)(nil)

func (r *AITaskRepo) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	if r == nil || r.db == nil || fn == nil || (r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead) {
		return points.ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return pointsDBError("开启 AI 任务事务", err)
	}
	// 所有失败回滚；提交后的 ErrTxDone 不改变已提交结果。
	defer func() { _ = tx.Rollback() }()
	if err := fn(AITaskTx(tx)); err != nil {
		return err
	}
	return pointsDBError("提交 AI 任务事务（未知结果须按原键核查）", tx.Commit())
}

type aiTaskTx struct{ tx *sql.Tx }

func AITaskTx(tx *sql.Tx) task.Tx {
	if tx == nil {
		return nil
	}
	return &aiTaskTx{tx}
}
func (t *aiTaskTx) Points() points.Tx { return PointsTx(t.tx) }

const aiTaskColumns = `id,user_id,task_no,idem_key,req_hash,analysis_type,subject_type,subject_id,hold_id,price_ver,price_points,budget_json,status,attempt,current_step,CAST(not_before AS CHAR),CAST(deadline_at AS CHAR),lease_owner,CAST(lease_until AS CHAR),fence,event_seq,cancel_req,error_code`

func scanAITask(s pointsScanner) (task.Task, error) {
	var out task.Task
	var budget []byte
	var before, deadline string
	var owner, until sql.NullString
	err := s.Scan(&out.ID, &out.UserID, &out.No, &out.Key, &out.Hash, &out.AnalysisType, &out.SubjectType, &out.SubjectID, &out.HoldID, &out.PriceVersion, &out.PricePoints, &budget, &out.Status, &out.Attempt, &out.CurrentStep, &before, &deadline, &owner, &until, &out.Fence, &out.EventSeq, &out.CancelRequested, &out.ErrorCode)
	if err != nil {
		return task.Task{}, pointsDBError("读取 AI 任务", err)
	}
	if err := json.Unmarshal(budget, &out.Budget); err != nil {
		return task.Task{}, fmt.Errorf("AI 任务预算无法解码: %w", err)
	}
	if err := out.Budget.Validate(); err != nil {
		return task.Task{}, points.ErrInvariant
	}
	if out.NotBefore, err = parsePointsUTC(before); err != nil {
		return task.Task{}, err
	}
	if out.Deadline, err = parsePointsUTC(deadline); err != nil {
		return task.Task{}, err
	}
	out.LeaseOwner = owner.String
	if until.Valid {
		value, err := parsePointsUTC(until.String)
		if err != nil {
			return task.Task{}, err
		}
		out.LeaseUntil = &value
	}
	return out, nil
}
func (t *aiTaskTx) FindByKey(ctx context.Context, uid int64, key string) (task.Task, error) {
	return scanAITask(t.tx.QueryRowContext(ctx, `SELECT `+aiTaskColumns+` FROM tm_ai_tasks WHERE user_id=? AND idem_key=? FOR UPDATE`, uid, key))
}
func (t *aiTaskTx) LockTask(ctx context.Context, uid, id int64) (task.Task, error) {
	return scanAITask(t.tx.QueryRowContext(ctx, `SELECT `+aiTaskColumns+` FROM tm_ai_tasks WHERE id=? AND user_id=? FOR UPDATE`, id, uid))
}
func (t *aiTaskTx) InsertTask(ctx context.Context, v task.Task) (int64, error) {
	budget, err := json.Marshal(v.Budget)
	if err != nil {
		return 0, fmt.Errorf("编码任务预算失败: %w", err)
	}
	r, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_tasks(user_id,task_no,idem_key,req_hash,analysis_type,subject_type,subject_id,hold_id,price_ver,price_points,budget_json,status,not_before,deadline_at,event_seq,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.No, v.Key, v.Hash, v.AnalysisType, v.SubjectType, v.SubjectID, v.HoldID, v.PriceVersion, v.PricePoints, budget, v.Status, pointsUTC(v.NotBefore), pointsUTC(v.Deadline), v.EventSeq)
	if err != nil {
		return 0, pointsDBError("插入 AI 任务", err)
	}
	return r.LastInsertId()
}
func taskLeaseValues(v task.Task) (any, any) {
	var owner, until any
	if v.LeaseOwner != "" {
		owner = v.LeaseOwner
	}
	if v.LeaseUntil != nil {
		until = pointsUTC(*v.LeaseUntil)
	}
	return owner, until
}
func (t *aiTaskTx) SetTask(ctx context.Context, old, next task.Task) error {
	owner, until := taskLeaseValues(next)
	oldOwner, oldUntil := taskLeaseValues(old)
	return onePointsRow(t.tx.ExecContext(ctx, `UPDATE tm_ai_tasks SET status=?,attempt=?,current_step=?,not_before=?,lease_owner=?,lease_until=?,fence=?,event_seq=?,cancel_req=?,error_code=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND user_id=? AND status=? AND fence=? AND event_seq=? AND lease_owner<=>? AND lease_until<=>? AND cancel_req=?`, next.Status, next.Attempt, next.CurrentStep, pointsUTC(next.NotBefore), owner, until, next.Fence, next.EventSeq, next.CancelRequested, next.ErrorCode, old.ID, old.UserID, old.Status, old.Fence, old.EventSeq, oldOwner, oldUntil, old.CancelRequested))
}
func (t *aiTaskTx) InsertEvent(ctx context.Context, v task.Event) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_task_events(task_id,seq,type,status,step_no,error_code,created_at,updated_at) VALUES(?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.TaskID, v.Seq, v.Type, v.Status, v.Step, v.ErrorCode)
	return pointsDBError("写入 AI 任务事件", err)
}

// ClaimCandidates 只提供提示候选，不授予租约；服务拿账户/任务当前锁后重新验证。
// 过期或取消待处理任务也被列出，避免它们永远占用冻结积分。
func (r *AITaskRepo) ClaimCandidates(ctx context.Context, limit int) ([]task.Task, error) {
	if r == nil || r.db == nil || limit < 1 || limit > 100 {
		return nil, points.ErrInvalid
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+aiTaskColumns+` FROM tm_ai_tasks WHERE (status=1 AND not_before<=UTC_TIMESTAMP(3)) OR (status=2 AND lease_until<=UTC_TIMESTAMP(3)) OR (status IN (1,2) AND deadline_at<=UTC_TIMESTAMP(3)) ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, pointsDBError("读取 AI 任务候选", err)
	}
	defer rows.Close()
	var out []task.Task
	for rows.Next() {
		v, err := scanAITask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, pointsDBError("读取 AI 任务候选列表", rows.Err())
}
