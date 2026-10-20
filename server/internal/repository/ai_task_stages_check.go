package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func verifyAITaskStageMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, c := range []struct{ table, check string }{{"tm_ai_task_step_results", "chk_tm_ai_step_result"}, {"tm_ai_task_steps", "chk_tm_ai_step"}} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, c.table, c.check).Scan(&n); err != nil {
			return fmt.Errorf("核对阶段表保护失败: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("阶段表 %s 缺失或保护无效", c.table)
		}
		columns := []struct{ name, kind, collation string }{{"task_id", "bigint unsigned", ""}, {"user_id", "bigint unsigned", ""}, {"reservation_seq", "int", ""}, {"step_no", "int", ""}, {"stage", "varchar(16)", "ascii_bin"}, {"owner", "varchar(64)", "ascii_bin"}, {"fence", "bigint", ""}, {"input_digest", "char(64)", "ascii_bin"}, {"output_digest", "char(64)", "ascii_bin"}, {"created_at", "datetime(3)", ""}, {"updated_at", "datetime(3)", ""}}
		if c.table == "tm_ai_task_step_results" {
			columns = append(columns, struct{ name, kind, collation string }{"payload", "mediumblob", ""})
		} else {
			columns = append(columns, struct{ name, kind, collation string }{"policy_digest", "char(64)", "ascii_bin"}, struct{ name, kind, collation string }{"event_seq", "bigint", ""})
		}
		for _, col := range columns {
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable='NO' AND COALESCE(collation_name,'')=?`, c.table, col.name, col.kind, col.collation).Scan(&n); err != nil {
				return fmt.Errorf("核对阶段列失败: %w", err)
			}
			if n != 1 {
				return fmt.Errorf("阶段列 %s.%s 不兼容", c.table, col.name)
			}
		}
	}
	for _, i := range []struct{ table, name, columns string }{{"tm_ai_task_step_results", "PRIMARY", "task_id,reservation_seq"}, {"tm_ai_task_step_results", "uk_tm_ai_result_owner", "task_id,reservation_seq,user_id"}, {"tm_ai_task_steps", "PRIMARY", "task_id,step_no"}, {"tm_ai_task_steps", "uk_tm_ai_step_operation", "task_id,reservation_seq"}, {"tm_ai_task_steps", "uk_tm_ai_step_event", "task_id,event_seq"}} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=0 AND is_visible='YES'`, i.table, i.name).Scan(&actual); err != nil {
			return fmt.Errorf("核对阶段索引失败: %w", err)
		}
		if actual != i.columns {
			return fmt.Errorf("阶段索引 %s 不兼容", i.name)
		}
	}
	for _, f := range []struct{ table, name, parent, columns string }{{"tm_ai_task_step_results", "fk_tm_ai_result_reservation", "tm_ai_task_reservations", "task_id:task_id,reservation_seq:seq"}, {"tm_ai_task_step_results", "fk_tm_ai_result_budget", "tm_ai_task_budgets", "task_id:task_id,user_id:user_id"}, {"tm_ai_task_steps", "fk_tm_ai_step_result", "tm_ai_task_step_results", "task_id:task_id,reservation_seq:reservation_seq,user_id:user_id"}} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, f.table, f.name, f.parent).Scan(&actual); err != nil {
			return fmt.Errorf("核对阶段归属失败: %w", err)
		}
		if actual != f.columns {
			return fmt.Errorf("阶段外键 %s 不兼容", f.name)
		}
	}
	return nil
}

func probeAITaskStageConstraints(ctx context.Context, tx *sql.Tx) error {
	if _, err := budgetProbeTaskAtStep(ctx, tx, 1); !pointsProbeError(err, 1644) {
		return fmt.Errorf("阶段初值保护未拒绝: %w", errOrMissing(err))
	}
	v, err := budgetProbeTask(ctx, tx)
	if err != nil {
		return fmt.Errorf("阶段依赖探针失败: %w", err)
	}
	a := &aiTaskTx{tx}
	if _, err := a.InitBudget(ctx, task.BudgetState{TaskID: v.ID, UserID: v.UserID, Limits: v.Budget.Limits()}); err != nil {
		return err
	}
	now, err := a.Points().Now(ctx)
	if err != nil {
		return err
	}
	res := task.Reservation{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Seq: 1, Owner: v.LeaseOwner, Key: "stage_probe_operation", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100, Pages: 1}, CreatedAt: now}
	if err := a.InsertReservation(ctx, res); err != nil {
		return err
	}
	row := task.StageResult{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, ReservationSeq: 1, StepNo: 1, Stage: "extract", Owner: v.LeaseOwner, InputDigest: res.InputDigest, Payload: []byte(`{}`), OutputDigest: task.StageDigest([]byte(`{}`)), CreatedAt: now}
	for _, change := range []func(*task.StageResult){func(r *task.StageResult) { r.Fence++ }, func(r *task.StageResult) { r.Owner = strings.ToUpper(r.Owner) }, func(r *task.StageResult) { r.StepNo++ }, func(r *task.StageResult) { r.ReservationSeq++ }, func(r *task.StageResult) { r.InputDigest = strings.Repeat("b", 64) }, func(r *task.StageResult) { r.UserID++ }} {
		bad := row
		change(&bad)
		if err := a.insertStageResult(ctx, bad); !pointsProbeError(err, 1644) {
			return fmt.Errorf("隔离响应绑定反例未拒绝: %w", errOrMissing(err))
		}
	}
	for _, body := range [][]byte{[]byte(`[]`), []byte(`null`)} {
		bad := row
		bad.Payload = body
		bad.OutputDigest = task.StageDigest(body)
		if err := a.insertStageResult(ctx, bad); !pointsProbeError(err, 3819) {
			return fmt.Errorf("隔离响应 CHECK 未拒绝: %w", errOrMissing(err))
		}
	}
	bad := row
	bad.OutputDigest = strings.Repeat("b", 64)
	if err := a.insertStageResult(ctx, bad); !pointsProbeError(err, 3819) {
		return fmt.Errorf("隔离响应摘要未拒绝: %w", errOrMissing(err))
	}
	if err := a.insertStageResult(ctx, row); err != nil {
		return fmt.Errorf("隔离响应正例失败: %w", err)
	}
	if err := a.insertStageResult(ctx, row); !pointsProbeError(err, 1062) {
		return fmt.Errorf("隔离响应唯一保护未拒绝: %w", errOrMissing(err))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_tasks SET current_step=1,event_seq=event_seq+1 WHERE id=?`, v.ID); !pointsProbeError(err, 1644) {
		return fmt.Errorf("无检查点推进未拒绝: %w", errOrMissing(err))
	}
	c := task.StageCheckpoint{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, StepNo: 1, ReservationSeq: 1, Stage: row.Stage, Owner: v.LeaseOwner, InputDigest: row.InputDigest, OutputDigest: row.OutputDigest, PolicyDigest: strings.Repeat("c", 64), EventSeq: v.EventSeq + 1, CreatedAt: now}
	for _, change := range []func(*task.StageCheckpoint){func(c *task.StageCheckpoint) { c.Fence++ }, func(c *task.StageCheckpoint) { c.Owner = strings.ToUpper(c.Owner) }, func(c *task.StageCheckpoint) { c.StepNo++ }, func(c *task.StageCheckpoint) { c.EventSeq++ }, func(c *task.StageCheckpoint) { c.ReservationSeq++ }, func(c *task.StageCheckpoint) { c.InputDigest = strings.Repeat("b", 64) }, func(c *task.StageCheckpoint) { c.OutputDigest = strings.Repeat("b", 64) }, func(c *task.StageCheckpoint) { c.Stage = "judge" }} {
		bad := c
		change(&bad)
		if err := a.insertStageCheckpoint(ctx, bad); !pointsProbeError(err, 1644) {
			return fmt.Errorf("检查点绑定反例未拒绝: %w", errOrMissing(err))
		}
	}
	badCheckpoint := c
	badCheckpoint.PolicyDigest = "bad"
	if err := a.insertStageCheckpoint(ctx, badCheckpoint); !pointsProbeError(err, 3819) {
		return fmt.Errorf("检查点策略 CHECK 未拒绝: %w", errOrMissing(err))
	}
	if err := a.insertStageCheckpoint(ctx, c); err != nil {
		return fmt.Errorf("检查点正例失败: %w", err)
	}
	current, err := a.LockTask(ctx, v.UserID, v.ID)
	if err != nil {
		return err
	}
	stored, err := a.StageCheckpoint(ctx, v.UserID, v.ID, 1)
	if err != nil {
		return err
	}
	event, err := a.StageEvent(ctx, v.ID, c.EventSeq)
	if err != nil {
		return err
	}
	if current.CurrentStep != 1 || current.EventSeq != c.EventSeq || stored != c || event.Type != "step_completed" || event.Status != task.Running || event.Step != 1 || event.ErrorCode != "" {
		return fmt.Errorf("阶段/步骤/事件未原子推进")
	}
	for _, q := range []string{`UPDATE tm_ai_task_step_results SET updated_at=updated_at WHERE task_id=?`, `DELETE FROM tm_ai_task_step_results WHERE task_id=?`, `UPDATE tm_ai_task_steps SET updated_at=updated_at WHERE task_id=?`, `DELETE FROM tm_ai_task_steps WHERE task_id=?`} {
		if _, err := tx.ExecContext(ctx, q, v.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("阶段不可变保护未拒绝: %w", errOrMissing(err))
		}
	}
	return nil
}
