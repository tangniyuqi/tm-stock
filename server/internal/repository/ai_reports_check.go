package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// AIReportSchemaVerifier 在同一回滚事务核对任务/账本、准入与报告提交保护。
// 报告读取启动复用该关口，不迁移、不为历史任务补造批准。
type AIReportSchemaVerifier struct{ db *sql.DB }

func NewAIReportSchemaVerifier(db *sql.DB) *AIReportSchemaVerifier {
	return &AIReportSchemaVerifier{db}
}
func (v *AIReportSchemaVerifier) Verify(ctx context.Context) error {
	if v == nil || v.db == nil {
		return fmt.Errorf("报告自检数据库未初始化")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启报告约束探针失败: %w", err)
	}
	// 所有探针均回滚，不提交探针数据，不运行 DDL。
	defer func() { _ = tx.Rollback() }()
	for _, f := range []func(context.Context, *sql.Tx) error{verifyPointsSession, verifyPointsSchemaMetadata, verifyAITaskMetadata, verifyAITaskBudgetMetadata, verifyAITaskStageMetadata, verifyAdmissionMetadata, verifyReportMetadata, probePointsConstraints, probeAITaskConstraints, probeAITaskBudgetConstraints, probeAITaskStageConstraints, probeAdmissionConstraints, probeReportConstraints} {
		if err := f(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("回滚报告约束探针失败: %w", err)
	}
	return nil
}

func verifyReportMetadata(ctx context.Context, tx *sql.Tx) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name='tm_ai_report_commits' AND t.engine='InnoDB' AND c.constraint_name='chk_tm_ai_report' AND c.constraint_type='CHECK' AND c.enforced='YES'`).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("报告表或强制 CHECK 不兼容")
	}
	for _, c := range []struct{ name, kind, nullable, collation string }{
		{"task_id", "bigint unsigned", "NO", ""}, {"user_id", "bigint unsigned", "NO", ""}, {"task_no", "char(32)", "NO", "ascii_bin"},
		{"step_no", "int", "NO", ""}, {"fence", "bigint", "NO", ""}, {"event_seq", "bigint", "NO", ""}, {"terminal_txn_id", "bigint unsigned", "NO", ""},
		{"operation_key", "varchar(80)", "NO", "ascii_bin"}, {"input_digest", "char(64)", "NO", "ascii_bin"}, {"content_digest", "char(64)", "NO", "ascii_bin"}, {"policy_digest", "char(64)", "NO", "ascii_bin"},
		{"status", "tinyint", "NO", ""}, {"entailment", "varchar(9)", "NO", "ascii_bin"}, {"semantic", "varchar(9)", "NO", "ascii_bin"}, {"reason", "varchar(16)", "NO", "ascii_bin"},
		{"payload", "mediumblob", "YES", ""}, {"created_at", "datetime(3)", "NO", ""}, {"updated_at", "datetime(3)", "NO", ""},
	} {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_report_commits' AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=?`, c.name, c.kind, c.nullable, c.collation).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("报告列 %s 不兼容", c.name)
		}
	}
	var actual string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tm_ai_report_commits' AND index_name='PRIMARY' AND non_unique=0 AND is_visible='YES' AND sub_part IS NULL AND collation='A'`).Scan(&actual); err != nil {
		return err
	}
	if actual != "task_id" {
		return fmt.Errorf("报告唯一索引不兼容")
	}
	for _, f := range []struct{ name, parent, columns string }{
		{"fk_tm_ai_report_task", "tm_ai_tasks", "task_id:id,user_id:user_id"}, {"fk_tm_ai_report_step", "tm_ai_task_steps", "task_id:task_id,step_no:step_no"},
		{"fk_tm_ai_report_event", "tm_ai_task_events", "task_id:task_id,event_seq:seq"}, {"fk_tm_ai_report_txn", "tm_points_txns", "terminal_txn_id:id,user_id:user_id"},
	} {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name='tm_ai_report_commits' AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, f.name, f.parent).Scan(&actual); err != nil {
			return err
		}
		if actual != f.columns {
			return fmt.Errorf("报告外键 %s 不兼容", f.name)
		}
	}
	return nil
}

func probeReportConstraints(ctx context.Context, tx *sql.Tx) error {
	v, err := budgetProbeTask(ctx, tx)
	if err != nil {
		return err
	}
	a := &aiTaskTx{tx}
	if _, err := a.InitBudget(ctx, task.BudgetState{TaskID: v.ID, UserID: v.UserID, Limits: v.Budget.Limits()}); err != nil {
		return err
	}
	now, err := a.Points().Now(ctx)
	if err != nil {
		return err
	}
	res := task.Reservation{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Seq: 1, Owner: v.LeaseOwner, Key: "report_probe_operation", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}, CreatedAt: now}
	if err := a.InsertReservation(ctx, res); err != nil {
		return err
	}
	raw := task.StageResult{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, ReservationSeq: 1, StepNo: 1, Stage: "judge", Owner: v.LeaseOwner, InputDigest: res.InputDigest, OutputDigest: task.StageDigest([]byte(`{}`)), Payload: []byte(`{}`), CreatedAt: now}
	if _, err := a.InsertStageResult(ctx, raw); err != nil {
		return err
	}
	cp := task.StageCheckpoint{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, StepNo: 1, ReservationSeq: 1, Stage: "judge", Owner: v.LeaseOwner, InputDigest: res.InputDigest, OutputDigest: raw.OutputDigest, PolicyDigest: strings.Repeat("b", 64), EventSeq: v.EventSeq + 1, CreatedAt: now}
	if err := a.InsertStageCheckpoint(ctx, cp); err != nil {
		return err
	}
	v, err = a.LockTask(ctx, v.UserID, v.ID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT report_probe_terminal"); err != nil {
		return err
	}
	for _, status := range []int{task.Succeeded, task.Failed} {
		kind, state, event, code := "settle", 2, "succeeded", ""
		if status == task.Failed {
			kind, state, event, code = "release", 3, "failed", "guard_rejected"
		}
		key := fmt.Sprintf("report_probe_%d_%d", v.UserID, status)
		r, err := tx.ExecContext(ctx, `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,created_at,updated_at) SELECT user_id,?,?,?,biz_type,biz_id,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_points_holds WHERE id=?`, key, strings.Repeat("0", 64), kind, v.HoldID)
		if err != nil {
			return err
		}
		txn, err := r.LastInsertId()
		if err != nil {
			return err
		}
		captured := int64(0)
		if status == task.Succeeded {
			captured = 7
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tm_points_holds SET state=?,captured=?,terminal_txn_id=? WHERE id=?`, state, captured, txn, v.HoldID); err != nil {
			return err
		}
		next := v
		next.Status = status
		next.ErrorCode = code
		next.LeaseOwner = ""
		next.LeaseUntil = nil
		next.EventSeq++
		if err := a.SetTask(ctx, v, next); err != nil {
			return err
		}
		if err := a.InsertEvent(ctx, task.Event{TaskID: v.ID, Seq: next.EventSeq, Type: event, Status: status, Step: 1, ErrorCode: code}); err != nil {
			return err
		}
		c := finalreport.Commit{TaskID: v.ID, UserID: v.UserID, TaskNo: v.No, Fence: v.Fence, EventSeq: next.EventSeq, StepNo: 1, TerminalTxnID: txn, OperationKey: res.Key, InputDigest: res.InputDigest, ContentDigest: raw.OutputDigest, PolicyDigest: cp.PolicyDigest, Status: status, Entailment: finalreport.Pass, Semantic: finalreport.Pass, Payload: raw.Payload, CreatedAt: now}
		if status == task.Failed {
			c.Semantic = finalreport.Block
			c.Reason = "semantic"
			c.Payload = nil
		}
		for _, change := range []func(*finalreport.Commit){func(c *finalreport.Commit) { c.TaskNo = strings.Repeat("f", 32) }, func(c *finalreport.Commit) { c.Fence++ }, func(c *finalreport.Commit) { c.OperationKey = "report_probe_changed_key" }, func(c *finalreport.Commit) { c.PolicyDigest = strings.Repeat("c", 64) }, func(c *finalreport.Commit) { c.EventSeq++ }} {
			bad := c
			change(&bad)
			if err := a.insertReportCommit(ctx, bad); !pointsProbeError(err, 1644) {
				return fmt.Errorf("报告绑定反例未拒绝: %w", errOrMissing(err))
			}
		}
		bad := c
		if status == task.Succeeded {
			bad.Payload = nil
		} else {
			bad.Payload = []byte(`{}`)
		}
		if err := a.insertReportCommit(ctx, bad); !pointsProbeError(err, 3819) {
			return fmt.Errorf("报告正文 CHECK 未拒绝: %w", errOrMissing(err))
		}
		if err := a.InsertReportCommit(ctx, c); err != nil {
			return fmt.Errorf("报告正例失败: %w", err)
		}
		for _, q := range []string{`UPDATE tm_ai_report_commits SET updated_at=updated_at WHERE task_id=?`, `DELETE FROM tm_ai_report_commits WHERE task_id=?`} {
			if _, err := tx.ExecContext(ctx, q, c.TaskID); !pointsProbeError(err, 1644) {
				return fmt.Errorf("报告不可变保护失效: %w", errOrMissing(err))
			}
		}
		if err := a.insertReportCommit(ctx, c); !pointsProbeError(err, 1062) {
			return fmt.Errorf("报告唯一保护失效: %w", errOrMissing(err))
		}
		if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT report_probe_terminal"); err != nil {
			return err
		}
	}
	return nil
}
