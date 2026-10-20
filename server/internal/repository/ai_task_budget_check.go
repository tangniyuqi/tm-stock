package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func verifyAITaskBudgetMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, c := range []struct{ table, name string }{{"tm_ai_task_budgets", "chk_tm_ai_budget"}, {"tm_ai_task_reservations", "chk_tm_ai_reservation"}} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, c.table, c.name).Scan(&n); err != nil {
			return fmt.Errorf("核对预算表保护失败: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("预算表 %s 缺失或保护未强制执行", c.table)
		}
	}
	columns := []struct{ table, name, kind, collation string }{}
	for _, table := range []string{"tm_ai_task_budgets", "tm_ai_task_reservations"} {
		for _, name := range []string{"task_id", "user_id"} {
			columns = append(columns, struct{ table, name, kind, collation string }{table, name, "bigint unsigned", ""})
		}
		for _, name := range []string{"created_at", "updated_at"} {
			columns = append(columns, struct{ table, name, kind, collation string }{table, name, "datetime(3)", ""})
		}
	}
	for _, name := range []string{"limit_calls", "limit_tokens", "limit_pages", "reserved_calls", "reserved_tokens", "reserved_pages"} {
		columns = append(columns, struct{ table, name, kind, collation string }{"tm_ai_task_budgets", name, "bigint", ""})
	}
	columns = append(columns, struct{ table, name, kind, collation string }{"tm_ai_task_budgets", "reservation_count", "int", ""})
	for _, name := range []string{"fence", "calls", "tokens", "pages"} {
		columns = append(columns, struct{ table, name, kind, collation string }{"tm_ai_task_reservations", name, "bigint", ""})
	}
	for _, c := range []struct{ name, kind, collation string }{{"seq", "int", ""}, {"reservation_key", "varchar(80)", "ascii_bin"}, {"input_digest", "char(64)", "ascii_bin"}, {"owner", "varchar(64)", "ascii_bin"}} {
		columns = append(columns, struct{ table, name, kind, collation string }{"tm_ai_task_reservations", c.name, c.kind, c.collation})
	}
	for _, c := range columns {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable='NO' AND COALESCE(collation_name,'')=?`, c.table, c.name, c.kind, c.collation).Scan(&n); err != nil {
			return fmt.Errorf("核对预算列失败: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("预算列 %s.%s 不兼容", c.table, c.name)
		}
	}
	for _, i := range []struct {
		table, name, cols string
		nonUnique         int
	}{{"tm_ai_task_budgets", "PRIMARY", "task_id", 0}, {"tm_ai_task_budgets", "uk_tm_ai_budget_owner", "task_id,user_id", 0}, {"tm_ai_task_reservations", "PRIMARY", "task_id,seq", 0}, {"tm_ai_task_reservations", "uk_tm_ai_reservation_key", "task_id,reservation_key", 0}, {"tm_ai_task_reservations", "idx_tm_ai_reservation_owner", "task_id,user_id,seq", 1}} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=? AND is_visible='YES'`, i.table, i.name, i.nonUnique).Scan(&actual); err != nil {
			return fmt.Errorf("核对预算索引失败: %w", err)
		}
		if actual != i.cols {
			return fmt.Errorf("预算索引 %s 不兼容", i.name)
		}
	}
	for _, f := range []struct{ table, name, parent string }{{"tm_ai_task_budgets", "fk_tm_ai_budget_task", "tm_ai_tasks"}, {"tm_ai_task_reservations", "fk_tm_ai_reservation_budget", "tm_ai_task_budgets"}} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, f.table, f.name, f.parent).Scan(&actual); err != nil {
			return fmt.Errorf("核对预算归属失败: %w", err)
		}
		want := "task_id:task_id,user_id:user_id"
		if f.parent == "tm_ai_tasks" {
			want = "task_id:id,user_id:user_id"
		}
		if actual != want {
			return fmt.Errorf("预算外键 %s 不兼容", f.name)
		}
	}
	return nil
}

func budgetProbeTask(ctx context.Context, tx *sql.Tx) (task.Task, error) {
	return budgetProbeTaskAtStep(ctx, tx, 0)
}

func budgetProbeTaskAtStep(ctx context.Context, tx *sql.Tx, step int) (task.Task, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return task.Task{}, err
	}
	uid := int64(binary.BigEndian.Uint64(random[:])&((1<<62)-2)) | (1 << 62)
	key := fmt.Sprintf("budget_probe_%x", uid)
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_points_accounts(user_id,created_at,updated_at) VALUES(?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid); err != nil {
		return task.Task{}, err
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,created_at,updated_at) VALUES(?,?,?,'freeze','ai_task',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, key, strings.Repeat("0", 64), key)
	if err != nil {
		return task.Task{}, err
	}
	txn, err := r.LastInsertId()
	if err != nil {
		return task.Task{}, err
	}
	r, err = tx.ExecContext(ctx, `INSERT INTO tm_points_holds(user_id,biz_type,biz_id,freeze_txn_id,amount,price_ver,expire_at,created_at,updated_at) VALUES(?,'ai_task',?,?,7,'probe',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, key, txn)
	if err != nil {
		return task.Task{}, err
	}
	hold, err := r.LastInsertId()
	if err != nil {
		return task.Task{}, err
	}
	now, err := PointsTx(tx).Now(ctx)
	if err != nil {
		return task.Task{}, err
	}
	until := now.Add(time.Hour)
	v := task.Task{UserID: uid, No: fmt.Sprintf("%032x", uid), Key: key, Hash: strings.Repeat("0", 64), AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, HoldID: hold, PriceVersion: "probe", PricePoints: 7, Budget: task.Budget{MaxAttempts: 3, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 3600000}, Status: task.Running, Attempt: 1, Fence: 1, LeaseOwner: key, LeaseUntil: &until, NotBefore: now, Deadline: until, EventSeq: 1}
	v.CurrentStep = step
	v.ID, err = insertAITaskProbe(ctx, tx, v, "")
	return v, err
}

func probeAITaskBudgetConstraints(ctx context.Context, tx *sql.Tx) error {
	v, err := budgetProbeTask(ctx, tx)
	if err != nil {
		return fmt.Errorf("预算依赖探针失败: %w", err)
	}
	a := &aiTaskTx{tx}
	state := task.BudgetState{TaskID: v.ID, UserID: v.UserID, Limits: v.Budget.Limits()}
	badState := state
	badState.Limits.Tokens++
	if _, err := a.InitBudget(ctx, badState); !pointsProbeError(err, 1644) {
		return fmt.Errorf("预算限额初始化未拒绝: %w", errOrMissing(err))
	}
	if _, err := a.InitBudget(ctx, state); err != nil {
		return fmt.Errorf("预算初始化正例失败: %w", err)
	}
	now, err := a.Points().Now(ctx)
	if err != nil {
		return err
	}
	row := task.Reservation{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Seq: 1, Owner: v.LeaseOwner, Key: "budget_probe_operation", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100, Pages: 1}, CreatedAt: now}
	for _, change := range []func(*task.Reservation){func(r *task.Reservation) { r.Fence++ }, func(r *task.Reservation) { r.Owner = strings.ToUpper(r.Owner) }, func(r *task.Reservation) { r.Seq++ }, func(r *task.Reservation) { r.Units = task.Units{Calls: 5, Tokens: 100} }, func(r *task.Reservation) { r.Units.Tokens = 1001 }, func(r *task.Reservation) { r.Units.Pages = 4 }} {
		bad := row
		change(&bad)
		if err := a.InsertReservation(ctx, bad); !pointsProbeError(err, 1644) {
			return fmt.Errorf("预算预留反例未拒绝: %w", errOrMissing(err))
		}
	}
	bad := row
	bad.Key = "short"
	if err := a.InsertReservation(ctx, bad); !pointsProbeError(err, 3819) {
		return fmt.Errorf("预算明细 CHECK 未拒绝: %w", errOrMissing(err))
	}
	if err := a.InsertReservation(ctx, row); err != nil {
		return fmt.Errorf("预算预留正例失败: %w", err)
	}
	actual, err := a.LockBudget(ctx, v.UserID, v.ID)
	if err != nil {
		return err
	}
	state.Reserved = row.Units
	state.Count = 1
	if actual != state {
		return fmt.Errorf("预算预留未原子累计")
	}
	for _, p := range []struct{ name, q string }{{"预算身份", `UPDATE tm_ai_task_budgets SET limit_tokens=limit_tokens+1 WHERE task_id=?`}, {"无明细增量", `UPDATE tm_ai_task_budgets SET reservation_count=reservation_count+1,reserved_tokens=reserved_tokens+1 WHERE task_id=?`}, {"预算删除", `DELETE FROM tm_ai_task_budgets WHERE task_id=?`}, {"预留修改", `UPDATE tm_ai_task_reservations SET tokens=tokens+1 WHERE task_id=?`}, {"预留删除", `DELETE FROM tm_ai_task_reservations WHERE task_id=?`}} {
		if _, err := tx.ExecContext(ctx, p.q, v.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("%s 保护未拒绝: %w", p.name, errOrMissing(err))
		}
	}
	bad = row
	bad.Seq = 2
	if err := a.insertReservation(ctx, bad); !pointsProbeError(err, 1062) {
		return fmt.Errorf("预算操作键唯一保护未拒绝: %w", errOrMissing(err))
	}
	rows, err := a.Reservations(ctx, v.UserID, v.ID)
	if err != nil {
		return err
	}
	return task.CheckBudget(v, state, rows)
}
