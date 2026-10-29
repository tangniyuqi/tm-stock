package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 只在回滚事务内验证原依赖及调用审计，不迁移、不联网、不提交探针。
type AIReportCallAuditSchemaVerifier struct{ db *sql.DB }

func NewAIReportCallAuditSchemaVerifier(db *sql.DB) *AIReportCallAuditSchemaVerifier {
	return &AIReportCallAuditSchemaVerifier{db}
}
func (v *AIReportCallAuditSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启调用审计自检失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for i, check := range []func(context.Context, *sql.Tx) error{verifyPointsSession, verifyPointsSchemaMetadata, verifyAITaskMetadata, verifyAITaskBudgetMetadata, verifyAITaskStageMetadata, verifyAdmissionMetadata, verifyReportCallAuditMetadata, probePointsConstraints, probeAITaskConstraints, probeAITaskBudgetConstraints, probeAITaskStageConstraints, probeReportCallAuditConstraints} {
		if err := check(ctx, tx); err != nil {
			return fmt.Errorf("调用审计自检阶段%d失败: %w", i+1, err)
		}
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("回滚调用审计探针失败: %w", err)
	}
	return nil
}

func verifyReportCallAuditMetadata(ctx context.Context, tx *sql.Tx) error {
	var n int
	for _, c := range []struct{ name, kind, nullable, collation string }{
		{"task_id", "bigint unsigned", "NO", ""}, {"user_id", "bigint unsigned", "NO", ""}, {"step_no", "int", "NO", ""},
		{"stage", "varchar(7)", "NO", "ascii_bin"}, {"owner", "varchar(64)", "NO", "ascii_bin"}, {"fence", "bigint", "NO", ""},
		{"operation_key", "varchar(80)", "NO", "ascii_bin"}, {"input_digest", "char(64)", "NO", "ascii_bin"},
		{"phase", "varchar(6)", "NO", "ascii_bin"}, {"task_no", "char(32)", "NO", "ascii_bin"},
		{"admission_digest", "char(64)", "NO", "ascii_bin"}, {"profile_digest", "char(64)", "NO", "ascii_bin"},
		{"request_digest", "char(64)", "NO", "ascii_bin"}, {"result_digest", "char(64)", "YES", "ascii_bin"},
		{"outcome", "varchar(8)", "NO", "ascii_bin"}, {"created_at", "datetime(3)", "NO", ""}, {"updated_at", "datetime(3)", "NO", ""},
	} {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_report_call_audits' AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=?`, c.name, c.kind, c.nullable, c.collation).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("调用审计列不兼容")
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name='tm_ai_report_call_audits' AND t.engine='InnoDB' AND c.constraint_type='CHECK' AND c.enforced='YES' AND c.constraint_name IN ('chk_tm_ai_report_call_identity','chk_tm_ai_report_call_step','chk_tm_ai_report_call_phase')`).Scan(&n); err != nil {
		return err
	}
	if n != 3 {
		return fmt.Errorf("调用审计CHECK未强制")
	}
	var columns string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tm_ai_report_call_audits' AND index_name='PRIMARY' AND non_unique=0 AND is_visible='YES' AND sub_part IS NULL`).Scan(&columns); err != nil {
		return err
	}
	if columns != "task_id,step_no,phase" {
		return fmt.Errorf("调用审计唯一索引不兼容")
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name='tm_ai_report_call_audits' AND k.constraint_name='fk_tm_ai_report_call_task' AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name='tm_ai_tasks' AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`).Scan(&columns); err != nil {
		return err
	}
	if columns != "task_id:id,user_id:user_id" {
		return fmt.Errorf("调用审计归属外键不兼容")
	}
	return nil
}

func probeReportCallAuditConstraints(ctx context.Context, tx *sql.Tx) error {
	return probeAdmissionTask(ctx, tx, func(v task.Task, admission task.AdmissionRecord) error {
		a := &aiTaskTx{tx}
		now, err := a.Points().Now(ctx)
		if err != nil {
			return err
		}
		next := v
		until := now.Add(time.Minute)
		next.Status, next.Attempt, next.Fence, next.LeaseOwner, next.LeaseUntil, next.EventSeq = task.Running, 1, 1, v.Key, &until, v.EventSeq+1
		if err := a.SetTask(ctx, v, next); err != nil {
			return err
		}
		if err := a.InsertEvent(ctx, task.Event{TaskID: v.ID, Seq: next.EventSeq, Type: "claimed", Status: task.Running}); err != nil {
			return err
		}
		v = next
		if _, err := a.InitBudget(ctx, task.BudgetState{TaskID: v.ID, UserID: v.UserID, Limits: v.Budget.Limits()}); err != nil {
			return err
		}
		res := task.Reservation{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Seq: 1, Owner: v.LeaseOwner, Key: "audit_probe_agent_operation", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 10}, CreatedAt: now}
		if err := a.InsertReservation(ctx, res); err != nil {
			return err
		}
		body := []byte(`{}`)
		row := task.StageResult{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, ReservationSeq: 1, StepNo: 1, Stage: "agent_call", Owner: v.LeaseOwner, InputDigest: res.InputDigest, OutputDigest: task.StageDigest(body), Payload: body, CreatedAt: now}
		if err := a.insertStageResult(ctx, row); err != nil {
			return err
		}
		cp := task.StageCheckpoint{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, StepNo: 1, ReservationSeq: 1, Stage: row.Stage, Owner: v.LeaseOwner, InputDigest: row.InputDigest, OutputDigest: row.OutputDigest, PolicyDigest: strings.Repeat("b", 64), EventSeq: v.EventSeq + 1, CreatedAt: now}
		if err := a.insertStageCheckpoint(ctx, cp); err != nil {
			return err
		}
		res.Seq, res.Key = 2, "audit_probe_model_operation"
		if err := a.InsertReservation(ctx, res); err != nil {
			return err
		}
		now, err = a.Points().Now(ctx)
		if err != nil {
			return err
		}
		start := finalreport.CallAudit{Request: task.StageRequest{UserID: v.UserID, TaskID: v.ID, Fence: v.Fence, Owner: v.LeaseOwner, StepNo: 2, Stage: "extract", OperationKey: res.Key, InputDigest: res.InputDigest}, Phase: "start", TaskNo: v.No, AdmissionDigest: admission.Digest, ProfileDigest: admission.Plan.ProfileDigest, RequestDigest: strings.Repeat("c", 64), Outcome: "started", CreatedAt: now}
		for i, change := range []func(*finalreport.CallAudit){
			func(b *finalreport.CallAudit) { b.Request.Fence++ }, func(b *finalreport.CallAudit) { b.Request.Owner = strings.ToUpper(b.Request.Owner) },
			func(b *finalreport.CallAudit) { b.Request.OperationKey = "audit_probe_other_operation" }, func(b *finalreport.CallAudit) { b.Request.InputDigest = strings.Repeat("d", 64) },
			func(b *finalreport.CallAudit) { b.AdmissionDigest = strings.Repeat("d", 64) }, func(b *finalreport.CallAudit) { b.ProfileDigest = strings.Repeat("d", 64) },
			func(b *finalreport.CallAudit) { b.TaskNo = strings.Repeat("e", 32) }, func(b *finalreport.CallAudit) { b.Request.UserID++ },
		} {
			bad := start
			change(&bad)
			if err := a.insertCallAudit(ctx, bad); !pointsProbeError(err, 1644) {
				return fmt.Errorf("调用前绑定反例%d未拒绝: %w", i+1, errOrMissing(err))
			}
		}
		for _, change := range []func(*finalreport.CallAudit){func(b *finalreport.CallAudit) { b.RequestDigest = strings.Repeat("z", 64) }, func(b *finalreport.CallAudit) { b.Request.Stage = "judge" }, func(b *finalreport.CallAudit) { b.Outcome = "returned" }} {
			bad := start
			change(&bad)
			if err := a.insertCallAudit(ctx, bad); !pointsProbeError(err, 3819) {
				return fmt.Errorf("调用审计CHECK反例未拒绝: %w", errOrMissing(err))
			}
		}
		if err := a.InsertCallAudit(ctx, start); err != nil {
			return fmt.Errorf("调用前审计正例失败: %w", err)
		}
		if err := a.insertCallAudit(ctx, start); !pointsProbeError(err, 1062) {
			return fmt.Errorf("调用前审计唯一保护未拒绝: %w", errOrMissing(err))
		}
		finish := start
		finish.Phase, finish.Outcome, finish.ResultDigest = "finish", "returned", strings.Repeat("f", 64)
		for _, change := range []func(*finalreport.CallAudit){func(b *finalreport.CallAudit) { b.Request.Fence++ }, func(b *finalreport.CallAudit) { b.RequestDigest = strings.Repeat("d", 64) }, func(b *finalreport.CallAudit) { b.AdmissionDigest = strings.Repeat("d", 64) }, func(b *finalreport.CallAudit) { b.ProfileDigest = strings.Repeat("d", 64) }} {
			bad := finish
			change(&bad)
			if err := a.insertCallAudit(ctx, bad); !pointsProbeError(err, 1644) {
				return fmt.Errorf("结果绑定反例未拒绝: %w", errOrMissing(err))
			}
		}
		bad := finish
		bad.ResultDigest = strings.Repeat("z", 64)
		if err := a.insertCallAudit(ctx, bad); !pointsProbeError(err, 3819) {
			return fmt.Errorf("结果摘要CHECK未拒绝: %w", errOrMissing(err))
		}
		current, err := a.LockTask(ctx, v.UserID, v.ID)
		if err != nil {
			return err
		}
		next = current
		next.CancelRequested = true
		next.EventSeq++
		if err := a.SetTask(ctx, current, next); err != nil {
			return err
		}
		if err := a.InsertEvent(ctx, task.Event{TaskID: v.ID, Seq: next.EventSeq, Type: "cancel_requested", Status: task.Running, Step: 1}); err != nil {
			return err
		}
		if err := a.InsertCallAudit(ctx, finish); err != nil {
			return fmt.Errorf("取消后结果审计正例失败: %w", err)
		}
		if err := a.insertCallAudit(ctx, finish); !pointsProbeError(err, 1062) {
			return fmt.Errorf("结果审计唯一保护未拒绝: %w", errOrMissing(err))
		}
		for _, q := range []string{`UPDATE tm_ai_report_call_audits SET updated_at=updated_at WHERE task_id=?`, `DELETE FROM tm_ai_report_call_audits WHERE task_id=?`} {
			if _, err := tx.ExecContext(ctx, q, v.ID); !pointsProbeError(err, 1644) {
				return fmt.Errorf("调用审计不可变保护未拒绝: %w", errOrMissing(err))
			}
		}
		stored, err := a.ReadCallAudit(ctx, v.UserID, v.ID, 2, "finish")
		if err != nil || stored != finish {
			return fmt.Errorf("调用结果审计正例回读失败: %w", errOrMissing(err))
		}
		return nil
	})
}
