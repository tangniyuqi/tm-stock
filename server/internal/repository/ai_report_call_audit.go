package repository

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ finalreport.CallAuditStore = (*AITaskRepo)(nil)

func (r *AITaskRepo) WithinCallAuditTx(ctx context.Context, fn func(finalreport.CallAuditTx) error) error {
	if fn == nil {
		return points.ErrInvalid
	}
	return r.WithinTaskTx(ctx, func(tx task.Tx) error { return fn(tx.(*aiTaskTx)) })
}
func (t *aiTaskTx) ReadCallAudit(ctx context.Context, uid, id int64, step int, phase string) (finalreport.CallAudit, error) {
	var out finalreport.CallAudit
	var raw string
	r := &out.Request
	err := t.tx.QueryRowContext(ctx, `SELECT task_id,user_id,step_no,stage,owner,fence,operation_key,input_digest,phase,task_no,admission_digest,profile_digest,request_digest,COALESCE(result_digest,''),outcome,CAST(created_at AS CHAR) FROM tm_ai_report_call_audits WHERE task_id=? AND user_id=? AND step_no=? AND phase=? FOR UPDATE`, id, uid, step, phase).Scan(&r.TaskID, &r.UserID, &r.StepNo, &r.Stage, &r.Owner, &r.Fence, &r.OperationKey, &r.InputDigest, &out.Phase, &out.TaskNo, &out.AdmissionDigest, &out.ProfileDigest, &out.RequestDigest, &out.ResultDigest, &out.Outcome, &raw)
	if err != nil {
		return out, pointsDBError("读取报告调用审计", err)
	}
	out.CreatedAt, err = parsePointsUTC(raw)
	if err != nil {
		return finalreport.CallAudit{}, points.ErrInvariant
	}
	if err := out.Validate(); err != nil {
		return finalreport.CallAudit{}, err
	}
	return out, nil
}
func (t *aiTaskTx) InsertCallAudit(ctx context.Context, v finalreport.CallAudit) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return pointsDBError("保存不可变报告调用审计", t.insertCallAudit(ctx, v))
}

// 保留原SQL错误码供回滚探针验证，业务入口仍先做领域校验。
func (t *aiTaskTx) insertCallAudit(ctx context.Context, v finalreport.CallAudit) error {
	r := v.Request
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_report_call_audits(task_id,user_id,step_no,stage,owner,fence,operation_key,input_digest,phase,task_no,admission_digest,profile_digest,request_digest,result_digest,outcome,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,NULLIF(?,''),?,?,?)`, r.TaskID, r.UserID, r.StepNo, r.Stage, r.Owner, r.Fence, r.OperationKey, r.InputDigest, v.Phase, v.TaskNo, v.AdmissionDigest, v.ProfileDigest, v.RequestDigest, v.ResultDigest, v.Outcome, pointsUTC(v.CreatedAt), pointsUTC(v.CreatedAt))
	return err
}
