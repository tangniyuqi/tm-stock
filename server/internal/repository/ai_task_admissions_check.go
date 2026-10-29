package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AIAdmissionSchemaVerifier 不迁移、不修复，只在回滚事务实测新增保护。
type AIAdmissionSchemaVerifier struct{ db *sql.DB }

func NewAIAdmissionSchemaVerifier(db *sql.DB) *AIAdmissionSchemaVerifier {
	return &AIAdmissionSchemaVerifier{db}
}
func (v *AIAdmissionSchemaVerifier) Verify(ctx context.Context) error {
	if v == nil || v.db == nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := NewAITaskSchemaVerifier(v.db).Verify(ctx); err != nil {
		return fmt.Errorf("准入原依赖自检失败: %w", err)
	}
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := verifyAdmissionMetadata(ctx, tx); err != nil {
		return fmt.Errorf("准入元数据自检失败: %w", err)
	}
	if err := probeAdmissionConstraints(ctx, tx); err != nil {
		return fmt.Errorf("准入事务探针失败: %w", err)
	}
	return tx.Rollback()
}
func verifyAdmissionMetadata(ctx context.Context, tx *sql.Tx) error {
	var n int
	for _, c := range []struct{ table, name, kind, nullable, collation string }{
		{"tm_ai_tasks", "admission_digest", "char(64)", "YES", "ascii_bin"},
		{"tm_ai_task_admissions", "task_id", "bigint unsigned", "NO", ""}, {"tm_ai_task_admissions", "user_id", "bigint unsigned", "NO", ""},
		{"tm_ai_task_admissions", "task_no", "char(32)", "NO", "ascii_bin"}, {"tm_ai_task_admissions", "request_hash", "char(64)", "NO", "ascii_bin"},
		{"tm_ai_task_admissions", "plan_digest", "char(64)", "NO", "ascii_bin"}, {"tm_ai_task_admissions", "payload", "mediumblob", "NO", ""},
		{"tm_ai_task_admissions", "created_at", "datetime(3)", "NO", ""}, {"tm_ai_task_admissions", "updated_at", "datetime(3)", "NO", ""},
	} {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=?`, c.table, c.name, c.kind, c.nullable, c.collation).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("准入结构列不兼容")
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.engine='InnoDB' AND c.constraint_type='CHECK' AND c.enforced='YES' AND ((t.table_name='tm_ai_tasks' AND c.constraint_name='chk_tm_ai_task_admission') OR (t.table_name='tm_ai_task_admissions' AND c.constraint_name='chk_tm_ai_admission'))`).Scan(&n); err != nil {
		return err
	}
	if n != 2 {
		return fmt.Errorf("准入CHECK未强制")
	}
	var columns string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tm_ai_task_admissions' AND index_name='PRIMARY' AND non_unique=0 AND is_visible='YES' AND sub_part IS NULL`).Scan(&columns); err != nil {
		return err
	}
	if columns != "task_id" {
		return fmt.Errorf("准入唯一索引不兼容")
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name='tm_ai_task_admissions' AND k.constraint_name='fk_tm_ai_admission_task' AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name='tm_ai_tasks' AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`).Scan(&columns); err != nil {
		return err
	}
	if columns != "task_id:id,user_id:user_id" {
		return fmt.Errorf("准入身份外键不兼容")
	}
	return nil
}

func probeAdmissionConstraints(ctx context.Context, tx *sql.Tx) error {
	return probeAdmissionTask(ctx, tx, nil)
}

// 回调仅供同一回滚事务的后续保护探针使用，不返回可供执行的业务批准。
func probeAdmissionTask(ctx context.Context, tx *sql.Tx, action func(task.Task, task.AdmissionRecord) error) error {
	// 这些行仅为保护探针，全部回滚，绝不代表模型/来源/定价批准。
	base, err := budgetProbeTask(ctx, tx)
	if err != nil {
		return err
	}
	a := &aiTaskTx{tx}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_tasks SET admission_digest=? WHERE id=?`, strings.Repeat("a", 64), base.ID); !pointsProbeError(err, 1644) {
		return fmt.Errorf("准入标记可被后补: %w", errOrMissing(err))
	}
	now, err := a.Points().Now(ctx)
	if err != nil {
		return err
	}
	p := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: base.UserID, AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1}, Price: points.Price{Version: "probe", Points: 7}, Budget: base.Budget, SubjectName: "结构探针题材", ModelName: "结构探针模型", FilingNo: "probe-only", ProfileDigest: strings.Repeat("a", 64), ExecutionDigest: strings.Repeat("b", 64), ApprovalRef: "probe-only", QualityRef: "probe-only", HardBudgetRef: "probe-only", PriceRef: "probe-only", SourceRef: "probe-only", Materials: []task.AdmissionMaterial{{SnapshotID: 1, Digest: strings.Repeat("c", 64), Domain: "admission.example.test", Category: sources.CategoryAnnouncement, URL: "https://admission.example.test/probe", CollectedAt: now.Add(-time.Hour)}}}
	p.Budget.MaxCalls = 5
	no := fmt.Sprintf("%032x", base.UserID+1)
	key := "admission_probe_" + no
	txn, err := a.Points().InsertTransaction(ctx, points.Transaction{UserID: base.UserID, Key: key, Hash: strings.Repeat("0", 64), Kind: "freeze", BizType: "ai_task", BizID: "task:" + no})
	if err != nil {
		return err
	}
	hold, err := a.Points().InsertHold(ctx, points.Hold{UserID: base.UserID, BizType: "ai_task", BizID: "task:" + no, FreezeTxnID: txn, Amount: 7, PriceVersion: "probe", ExpiresAt: now.Add(time.Hour), State: points.Holding})
	if err != nil {
		return err
	}
	_, digest, err := p.Encode()
	if err != nil {
		return err
	}
	v := task.Task{UserID: base.UserID, No: no, Key: key, Hash: strings.Repeat("0", 64), AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, HoldID: hold, PriceVersion: "probe", PricePoints: 7, Budget: p.Budget, Status: task.Queued, NotBefore: now, Deadline: now.Add(time.Hour), EventSeq: 1}
	if _, err := a.InsertAdmittedTask(ctx, v, strings.Repeat("g", 64)); !pointsProbeError(err, 3819) {
		return fmt.Errorf("准入标记CHECK未拒绝: %w", errOrMissing(err))
	}
	v.ID, err = a.InsertAdmittedTask(ctx, v, digest)
	if err != nil {
		return err
	}
	if err := a.InsertEvent(ctx, task.Event{TaskID: v.ID, Seq: 1, Type: "created", Status: task.Queued}); err != nil {
		return err
	}
	r := task.AdmissionRecord{TaskID: v.ID, UserID: v.UserID, TaskNo: v.No, RequestHash: v.Hash, Digest: digest, Plan: p, CreatedAt: now}
	for _, change := range []func(*task.AdmissionRecord){func(r *task.AdmissionRecord) { r.TaskNo = strings.Repeat("e", 32) }, func(r *task.AdmissionRecord) { r.RequestHash = strings.Repeat("f", 64) }, func(r *task.AdmissionRecord) { r.UserID++ }} {
		bad := r
		change(&bad)
		if err := a.InsertAdmission(ctx, bad); !pointsProbeError(err, 1644) {
			return fmt.Errorf("准入绑定反例未拒绝: %w", errOrMissing(err))
		}
	}
	b, _, err := p.Encode()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_task_admissions(task_id,user_id,task_no,request_hash,plan_digest,payload,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, v.ID, v.UserID, v.No, v.Hash, digest, append(b, ' '), pointsUTC(now), pointsUTC(now)); !pointsProbeError(err, 3819) {
		return fmt.Errorf("准入摘要CHECK未拒绝: %w", errOrMissing(err))
	}
	if err := a.InsertAdmission(ctx, r); err != nil {
		return err
	}
	if _, found, err := a.ReadAdmission(ctx, v); err != nil || !found {
		return fmt.Errorf("准入正例读取失败: %w", errOrMissing(err))
	}
	for _, q := range []string{`UPDATE tm_ai_task_admissions SET updated_at=updated_at WHERE task_id=?`, `DELETE FROM tm_ai_task_admissions WHERE task_id=?`} {
		if _, err := tx.ExecContext(ctx, q, v.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("准入不可变保护未拒绝: %w", errOrMissing(err))
		}
	}
	for _, value := range []any{nil, strings.Repeat("d", 64)} {
		if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_tasks SET admission_digest=? WHERE id=?`, value, v.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("准入标记可被抹除或替换: %w", errOrMissing(err))
		}
	}
	// 探针须检查数据库原始1062；业务仓储会把它转换为ErrConflict，不能据映射后的错误判保护失效。
	if _, err := tx.ExecContext(ctx, insertAdmissionSQL, r.TaskID, r.UserID, r.TaskNo, r.RequestHash, r.Digest, b, pointsUTC(r.CreatedAt), pointsUTC(r.CreatedAt)); !pointsProbeError(err, 1062) {
		return fmt.Errorf("准入唯一保护未拒绝: %w", errOrMissing(err))
	}
	if action != nil {
		return action(v, r)
	}
	return nil
}
