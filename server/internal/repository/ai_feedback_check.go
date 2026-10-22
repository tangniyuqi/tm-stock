package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
)

type AIFeedbackSchemaVerifier struct{ db *sql.DB }

func NewAIFeedbackSchemaVerifier(db *sql.DB) *AIFeedbackSchemaVerifier {
	return &AIFeedbackSchemaVerifier{db}
}
func (v *AIFeedbackSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil {
		return feedback.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if err := NewAIReportSchemaVerifier(v.db).Verify(ctx); err != nil {
		return err
	}
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // 探针永不提交，仅消耗允许的自增编号。
	if err = verifyFeedbackMetadata(ctx, tx); err != nil {
		return err
	}
	if err = probeReportConstraintsWith(ctx, tx, probeFeedbackConstraints); err != nil {
		return err
	}
	return tx.Rollback()
}
func verifyFeedbackMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []struct {
		name, check string
		columns     []struct{ name, kind, collation string }
	}{
		{"tm_ai_report_feedbacks", "chk_tm_ai_feedback", []struct{ name, kind, collation string }{
			{"id", "bigint unsigned", ""}, {"task_id", "bigint unsigned", ""}, {"user_id", "bigint unsigned", ""}, {"category", "varchar(7)", "ascii_bin"}, {"report_digest", "char(64)", "ascii_bin"}, {"status", "varchar(10)", "ascii_bin"}, {"resolution", "varchar(14)", "ascii_bin"}, {"version", "bigint", ""}, {"created_at", "datetime(3)", ""}, {"updated_at", "datetime(3)", ""}}},
		{"tm_ai_report_feedback_events", "chk_tm_ai_feedback_event", []struct{ name, kind, collation string }{
			{"feedback_id", "bigint unsigned", ""}, {"seq", "bigint", ""}, {"actor_id", "bigint unsigned", ""}, {"status", "varchar(10)", "ascii_bin"}, {"resolution", "varchar(14)", "ascii_bin"}, {"created_at", "datetime(3)", ""}, {"updated_at", "datetime(3)", ""}}},
	} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, table.name, table.check).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return feedback.ErrInvariant
		}
		for _, column := range table.columns {
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable='NO' AND COALESCE(collation_name,'')=?`, table.name, column.name, column.kind, column.collation).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return feedback.ErrInvariant
			}
		}
	}
	for _, index := range []struct{ table, name, columns string }{
		{"tm_ai_report_feedbacks", "PRIMARY", "id"}, {"tm_ai_report_feedbacks", "uq_tm_ai_feedback_category", "user_id,task_id,category"}, {"tm_ai_report_feedback_events", "PRIMARY", "feedback_id,seq"},
	} {
		var columns string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=0 AND is_visible='YES' AND sub_part IS NULL AND collation='A'`, index.table, index.name).Scan(&columns); err != nil {
			return err
		}
		if columns != index.columns {
			return feedback.ErrInvariant
		}
	}
	for _, fk := range []struct{ table, name, parent, columns string }{
		{"tm_ai_report_feedbacks", "fk_tm_ai_feedback_task", "tm_ai_tasks", "task_id:id,user_id:user_id"}, {"tm_ai_report_feedback_events", "fk_tm_ai_feedback_event", "tm_ai_report_feedbacks", "feedback_id:id"},
	} {
		var columns string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, fk.table, fk.name, fk.parent).Scan(&columns); err != nil {
			return err
		}
		if columns != fk.columns {
			return feedback.ErrInvariant
		}
	}
	return nil
}
func probeFeedbackConstraints(ctx context.Context, tx *sql.Tx, c finalreport.Commit) error {
	t := &aiFeedbackTx{tx}
	now, err := t.Now(ctx)
	if err != nil {
		return err
	}
	r, err := t.Insert(ctx, feedback.Record{TaskID: c.TaskID, UserID: c.UserID, Category: "fact", ReportDigest: c.ContentDigest, Status: "received", Version: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return err
	}
	if !r.Valid() {
		return feedback.ErrInvariant
	}
	for _, bad := range []struct {
		column, value string
		code          uint16
	}{
		{"category", "source", 1644}, {"report_digest", strings.Repeat("b", 64), 1644}, {"status", "resolved", 1644},
	} {
		var err error
		switch bad.column {
		case "category":
			_, err = tx.ExecContext(ctx, `UPDATE tm_ai_report_feedbacks SET category=? WHERE id=?`, bad.value, r.ID)
		case "report_digest":
			_, err = tx.ExecContext(ctx, `UPDATE tm_ai_report_feedbacks SET report_digest=? WHERE id=?`, bad.value, r.ID)
		case "status":
			_, err = tx.ExecContext(ctx, `UPDATE tm_ai_report_feedbacks SET status=? WHERE id=?`, bad.value, r.ID)
		}
		if !pointsProbeError(err, bad.code) {
			return fmt.Errorf("反馈身份或跳级保护失效: %w", errOrMissing(err))
		}
	}
	duplicate := r
	duplicate.ID = 0
	if _, err := t.Insert(ctx, duplicate); !pointsProbeError(err, 1062) {
		return fmt.Errorf("反馈唯一保护失效: %w", errOrMissing(err))
	}
	for _, change := range []func(*feedback.Record){func(r *feedback.Record) { r.UserID++ }, func(r *feedback.Record) { r.ReportDigest = strings.Repeat("e", 64) }, func(r *feedback.Record) { r.Status = "processing"; r.Version = 2 }} {
		bad := r
		change(&bad)
		if _, err := t.Insert(ctx, bad); !pointsProbeError(err, 1644) {
			return fmt.Errorf("反馈报告绑定失效: %w", errOrMissing(err))
		}
	}
	bad := r
	bad.Category = "invalid"
	if _, err := t.Insert(ctx, bad); !pointsProbeError(err, 3819) {
		return fmt.Errorf("反馈分类CHECK失效: %w", errOrMissing(err))
	}
	e := feedback.Event{FeedbackID: r.ID, Version: 1, Status: "received", At: now}
	wrong := e
	wrong.ActorID = 1
	if err := t.Append(ctx, wrong); !pointsProbeError(err, 3819) {
		return fmt.Errorf("反馈事件CHECK失效: %w", errOrMissing(err))
	}
	wrong = e
	wrong.Version = 2
	wrong.Status = "processing"
	wrong.ActorID = 7
	if err := t.Append(ctx, wrong); !pointsProbeError(err, 1644) {
		return fmt.Errorf("反馈事件绑定失效: %w", errOrMissing(err))
	}
	if err = t.Append(ctx, e); err != nil {
		return err
	}
	for _, q := range []string{`DELETE FROM tm_ai_report_feedbacks WHERE id=?`, `UPDATE tm_ai_report_feedback_events SET actor_id=actor_id WHERE feedback_id=?`, `DELETE FROM tm_ai_report_feedback_events WHERE feedback_id=?`} {
		if _, err := tx.ExecContext(ctx, q, r.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("反馈不可变保护失效: %w", errOrMissing(err))
		}
	}
	for _, state := range []string{"processing", "resolved"} {
		next := r
		next.Status = state
		next.Version++
		if state == "resolved" {
			next.Resolution = "reviewed"
		}
		if err = t.Update(ctx, r, next); err != nil {
			return err
		}
		if err = t.Append(ctx, feedback.Event{FeedbackID: r.ID, Version: next.Version, ActorID: 7, Status: next.Status, Resolution: next.Resolution, At: now}); err != nil {
			return err
		}
		r = next
	}
	if err := validateFeedbackProbe(ctx, t, r); err != nil {
		return err
	}
	if err = t.Update(ctx, r, r); !pointsProbeError(err, 1644) {
		return fmt.Errorf("反馈终态保护失效: %w", errOrMissing(err))
	}
	return nil
}
func validateFeedbackProbe(ctx context.Context, t *aiFeedbackTx, r feedback.Record) error {
	events, err := t.History(ctx, r.ID)
	if err != nil {
		return err
	}
	return feedback.ValidateHistory(r, events)
}
