package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const insertAdmissionSQL = `INSERT INTO tm_ai_task_admissions(task_id,user_id,task_no,request_hash,plan_digest,payload,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`

func (r *AITaskRepo) WithinAdmissionTx(ctx context.Context, fn func(task.AdmissionReadTx) error) error {
	if fn == nil {
		return points.ErrInvalid
	}
	return r.WithinTaskTx(ctx, func(tx task.Tx) error {
		a, ok := tx.(task.AdmissionReadTx)
		if !ok {
			return points.ErrInvariant
		}
		return fn(a)
	})
}
func (t *aiTaskTx) InsertAdmittedTask(ctx context.Context, v task.Task, digest string) (int64, error) {
	if t == nil || t.tx == nil || len(digest) != 64 {
		return 0, points.ErrInvalid
	}
	budget, err := json.Marshal(v.Budget)
	if err != nil {
		return 0, err
	}
	r, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_tasks(user_id,task_no,idem_key,req_hash,analysis_type,subject_type,subject_id,hold_id,price_ver,price_points,budget_json,status,not_before,deadline_at,event_seq,admission_digest,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.No, v.Key, v.Hash, v.AnalysisType, v.SubjectType, v.SubjectID, v.HoldID, v.PriceVersion, v.PricePoints, budget, v.Status, pointsUTC(v.NotBefore), pointsUTC(v.Deadline), v.EventSeq, digest)
	if err != nil {
		return 0, pointsDBError("写入带准入任务", err)
	}
	return r.LastInsertId()
}
func (t *aiTaskTx) InsertAdmission(ctx context.Context, r task.AdmissionRecord) error {
	if t == nil || t.tx == nil {
		return points.ErrInvalid
	}
	b, digest, err := r.Plan.Encode()
	if err != nil || digest != r.Digest {
		return points.ErrInvalid
	}
	_, err = t.tx.ExecContext(ctx, insertAdmissionSQL, r.TaskID, r.UserID, r.TaskNo, r.RequestHash, r.Digest, b, pointsUTC(r.CreatedAt), pointsUTC(r.CreatedAt))
	return pointsDBError("写入首次准入计划", err)
}
func (t *aiTaskTx) ReadAdmission(ctx context.Context, v task.Task) (task.AdmissionRecord, bool, error) {
	if t == nil || t.tx == nil || v.ID <= 0 || v.UserID <= 0 {
		return task.AdmissionRecord{}, false, points.ErrInvalid
	}
	var marker, no, hash, digest, created sql.NullString
	var payload []byte
	err := t.tx.QueryRowContext(ctx, `SELECT t.admission_digest,a.task_no,a.request_hash,a.plan_digest,a.payload,CAST(a.created_at AS CHAR) FROM tm_ai_tasks t LEFT JOIN tm_ai_task_admissions a ON a.task_id=t.id AND a.user_id=t.user_id WHERE t.id=? AND t.user_id=? FOR UPDATE`, v.ID, v.UserID).Scan(&marker, &no, &hash, &digest, &payload, &created)
	if err != nil {
		return task.AdmissionRecord{}, false, pointsDBError("读取原准入计划", err)
	}
	if !marker.Valid && !digest.Valid {
		return task.AdmissionRecord{}, false, nil
	}
	if !marker.Valid || !no.Valid || !hash.Valid || !digest.Valid || !created.Valid || marker.String != digest.String || task.StageDigest(payload) != digest.String {
		return task.AdmissionRecord{}, false, points.ErrInvariant
	}
	p, err := task.DecodeAdmission(payload)
	if err != nil {
		return task.AdmissionRecord{}, false, points.ErrInvariant
	}
	at, err := parsePointsUTC(created.String)
	if err != nil {
		return task.AdmissionRecord{}, false, err
	}
	r := task.AdmissionRecord{TaskID: v.ID, UserID: v.UserID, TaskNo: no.String, RequestHash: hash.String, Digest: digest.String, Plan: p, CreatedAt: at}
	if err := r.Bind(v); err != nil {
		return task.AdmissionRecord{}, false, err
	}
	return r, true, nil
}
