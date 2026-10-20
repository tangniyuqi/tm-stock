package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
)

type AIL0RunRepo struct{ db *sql.DB }

func NewAIL0RunRepo(db *sql.DB) *AIL0RunRepo { return &AIL0RunRepo{db: db} }

var _ pipeline.RunStore = (*AIL0RunRepo)(nil)

const runSelect = `SELECT id,request_id,snapshot_id,actor_id,provider,model,approval_ref,state,result_json,error_code,
  DATE_FORMAT(created_at,'` + databaseUTCSelect + `'),DATE_FORMAT(finished_at,'` + databaseUTCSelect + `') FROM tm_ai_l0_extraction_runs`

func (r *AIL0RunRepo) RunByRequest(ctx context.Context, id string) (pipeline.RunRecord, error) {
	record, err := scanL0Run(r.db.QueryRowContext(ctx, runSelect+` WHERE request_id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return pipeline.RunRecord{}, pipeline.ErrRunNotFound
	}
	return record, err
}

func (r *AIL0RunRepo) BeginRun(ctx context.Context, request pipeline.RunRequest, actorID int64, policy pipeline.RunPolicy) (pipeline.RunRecord, bool, error) {
	if err := request.Validate(); err != nil {
		return pipeline.RunRecord{}, false, err
	}
	if actorID <= 0 || !validL0Policy(policy) {
		return pipeline.RunRecord{}, false, pipeline.ErrInvalidRun
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return pipeline.RunRecord{}, false, fmt.Errorf("开启抽取登记事务失败: %w", err)
	}
	// 成功提交后回滚是空操作，其他路径确保释放锁。
	defer func() { _ = tx.Rollback() }()
	record, err := scanL0Run(tx.QueryRowContext(ctx, runSelect+` WHERE request_id=?`, request.RequestID))
	if err == nil {
		if record.ActorID != actorID || record.Request != request {
			return pipeline.RunRecord{}, false, pipeline.ErrRunConflict
		}
		return record, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return pipeline.RunRecord{}, false, err
	}
	if _, err := lockAndLoadSnapshotForCandidate(ctx, tx, candidate.Candidate{SnapshotID: request.SnapshotID}); err != nil {
		return pipeline.RunRecord{}, false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT IGNORE INTO tm_ai_l0_extraction_runs
	  (request_id,snapshot_id,actor_id,provider,model,approval_ref,state,created_at,updated_at)
	  VALUES (?,?,?,?,?,?,'running',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, request.RequestID, request.SnapshotID, actorID, policy.Provider, policy.Model, policy.ApprovalRef)
	if err != nil {
		return pipeline.RunRecord{}, false, fmt.Errorf("登记抽取请求失败: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return pipeline.RunRecord{}, false, fmt.Errorf("读取抽取登记状态失败: %w", err)
	}
	record, err = scanL0Run(tx.QueryRowContext(ctx, runSelect+` WHERE request_id=?`, request.RequestID))
	if err != nil {
		return pipeline.RunRecord{}, false, err
	}
	if record.ActorID != actorID || record.Request != request {
		return pipeline.RunRecord{}, false, pipeline.ErrRunConflict
	}
	if err := tx.Commit(); err != nil {
		return pipeline.RunRecord{}, false, fmt.Errorf("提交抽取登记失败: %w", err)
	}
	return record, affected == 1, nil
}

func (r *AIL0RunRepo) FinishRun(ctx context.Context, id int64, result pipeline.ExtractionResult, failure string) (pipeline.RunRecord, error) {
	if id <= 0 || (failure != "" && failure != "extraction_failed" && failure != "audit_unavailable") {
		return pipeline.RunRecord{}, pipeline.ErrInvalidRun
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return pipeline.RunRecord{}, fmt.Errorf("开启抽取终态事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	record, err := scanL0Run(tx.QueryRowContext(ctx, runSelect+` WHERE id=? FOR UPDATE`, id))
	if err != nil {
		return pipeline.RunRecord{}, err
	}
	if record.State != "running" {
		return pipeline.RunRecord{}, pipeline.ErrRunConflict
	}
	state := "failed"
	var raw any
	if failure == "" {
		if !validL0Result(result, record) {
			return pipeline.RunRecord{}, pipeline.ErrInvalidRun
		}
		// 即使没有候选，也必须确认模型执行期间来源授权没有失效。
		if _, err := lockAndLoadSnapshotForCandidate(ctx, tx, candidate.Candidate{SnapshotID: record.Request.SnapshotID}); err != nil {
			return pipeline.RunRecord{}, err
		}
		var eventCount, attemptCount, minAttempt, maxAttempt int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(DISTINCT attempt),IFNULL(MIN(attempt),0),IFNULL(MAX(attempt),0) FROM tm_ai_l0_call_events WHERE run_id=?`, id).Scan(&eventCount, &attemptCount, &minAttempt, &maxAttempt); err != nil {
			return pipeline.RunRecord{}, fmt.Errorf("复核抽取调用审计失败: %w", err)
		}
		if eventCount != 2*result.Attempts || attemptCount != result.Attempts || minAttempt != 1 || maxAttempt != result.Attempts {
			return pipeline.RunRecord{}, pipeline.ErrAuditUnavailable
		}
		for _, item := range result.Candidates {
			if item.SnapshotID != record.Request.SnapshotID || item.Provider != record.Policy.Provider || item.Model != record.Policy.Model {
				return pipeline.RunRecord{}, pipeline.ErrRunConflict
			}
		}
		stored, err := savePendingCandidatesTx(ctx, tx, result.Candidates)
		if err != nil {
			return pipeline.RunRecord{}, err
		}
		result.Candidates = stored
		body, err := json.Marshal(result)
		if err != nil {
			return pipeline.RunRecord{}, fmt.Errorf("编码抽取终态失败: %w", err)
		}
		raw = string(body)
		state = "succeeded"
	}
	_, err = tx.ExecContext(ctx, `UPDATE tm_ai_l0_extraction_runs SET state=?,result_json=?,error_code=?,finished_at=UTC_TIMESTAMP(3),updated_at=UTC_TIMESTAMP(3) WHERE id=? AND state='running'`, state, raw, failure, id)
	if err != nil {
		return pipeline.RunRecord{}, fmt.Errorf("写入抽取终态失败: %w", err)
	}
	record, err = scanL0Run(tx.QueryRowContext(ctx, runSelect+` WHERE id=?`, id))
	if err != nil {
		return pipeline.RunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return pipeline.RunRecord{}, fmt.Errorf("提交抽取终态失败: %w", err)
	}
	return record, nil
}

func (r *AIL0RunRepo) AppendCallEvent(ctx context.Context, event pipeline.CallEvent) error {
	if event.RunID <= 0 || event.Attempt < 1 || event.Attempt > 2 || (event.Phase != "start" && event.Phase != "finish") || len(event.Payload) == 0 || len(event.Payload) > 8<<20 {
		return pipeline.ErrInvalidRun
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO tm_ai_l0_call_events (run_id,attempt,phase,payload,created_at)
	  SELECT id,?,?,?,UTC_TIMESTAMP(3) FROM tm_ai_l0_extraction_runs WHERE id=? AND state='running'`, event.Attempt, event.Phase, event.Payload, event.RunID)
	if err != nil {
		return fmt.Errorf("追加抽取调用审计失败: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取调用审计写入状态失败: %w", err)
	}
	if affected != 1 {
		return pipeline.ErrRunConflict
	}
	return nil
}

func scanL0Run(row candidateScanner) (pipeline.RunRecord, error) {
	var record pipeline.RunRecord
	var raw []byte
	var created string
	var finished sql.NullString
	err := row.Scan(&record.ID, &record.Request.RequestID, &record.Request.SnapshotID, &record.ActorID, &record.Policy.Provider, &record.Policy.Model, &record.Policy.ApprovalRef, &record.State, &raw, &record.ErrorCode, &created, &finished)
	if err != nil {
		return pipeline.RunRecord{}, err
	}
	record.CreatedAt, err = time.Parse(databaseUTCLayout, created)
	if err != nil {
		return pipeline.RunRecord{}, fmt.Errorf("读取抽取登记时点失败: %w", err)
	}
	record.FinishedAt, err = parseDatabaseUTC(finished)
	if err != nil {
		return pipeline.RunRecord{}, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &record.Result); err != nil {
			return pipeline.RunRecord{}, fmt.Errorf("读取抽取终态内容失败: %w", err)
		}
	}
	if err := record.Request.Validate(); err != nil {
		return pipeline.RunRecord{}, err
	}
	if record.ID <= 0 || record.ActorID <= 0 || !validL0Policy(record.Policy) || record.CreatedAt.IsZero() {
		return pipeline.RunRecord{}, pipeline.ErrInvalidRun
	}
	switch record.State {
	case "running":
		if raw != nil || record.ErrorCode != "" || record.FinishedAt != nil {
			return pipeline.RunRecord{}, pipeline.ErrInvalidRun
		}
	case "failed":
		if raw != nil || (record.ErrorCode != "extraction_failed" && record.ErrorCode != "audit_unavailable") || record.FinishedAt == nil {
			return pipeline.RunRecord{}, pipeline.ErrInvalidRun
		}
	case "succeeded":
		if raw == nil || record.ErrorCode != "" || record.FinishedAt == nil || !validL0Result(record.Result, record) {
			return pipeline.RunRecord{}, pipeline.ErrInvalidRun
		}
		for _, item := range record.Result.Candidates {
			if item.ID <= 0 {
				return pipeline.RunRecord{}, pipeline.ErrInvalidRun
			}
		}
	default:
		return pipeline.RunRecord{}, pipeline.ErrInvalidRun
	}
	return record, nil
}

func validL0Policy(p pipeline.RunPolicy) bool {
	for _, value := range []struct {
		text  string
		limit int
	}{{p.Provider, 64}, {p.Model, 128}, {p.ApprovalRef, 512}} {
		if value.text == "" || len(value.text) > value.limit || !utf8.ValidString(value.text) || strings.TrimSpace(value.text) != value.text || strings.ContainsAny(value.text, "\r\n\x00") {
			return false
		}
	}
	return true
}

func validL0Result(result pipeline.ExtractionResult, record pipeline.RunRecord) bool {
	u := result.Usage
	if result.Attempts < 1 || result.Attempts > 2 || result.Model != record.Policy.Model || u.Total < 0 || u.Prompt < 0 || u.Completion < 0 || u.Cached < 0 || u.Reasoning < 0 || len(result.Candidates)+len(result.Rejected) > 50 {
		return false
	}
	for _, item := range result.Candidates {
		// 相同依据复用首次持久化的候选，候选保留原模型追溯；本次模型由运行记录固化。
		if item.SnapshotID != record.Request.SnapshotID || item.Validate() != nil {
			return false
		}
	}
	for _, rejected := range result.Rejected {
		if rejected.Index < 0 || rejected.Index >= 50 || rejected.Reason == "" || len(rejected.Reason) > 64 {
			return false
		}
		for _, c := range rejected.Reason {
			if !((c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
		}
	}
	return true
}
