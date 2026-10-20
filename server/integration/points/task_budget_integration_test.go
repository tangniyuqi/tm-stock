//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func runningBudgetTask(t *testing.T, level sql.IsolationLevel) (*pointsTestEnv, *sql.DB, *repository.AITaskRepo, *service.AITaskService, *service.AITaskBudgetService, task.ReserveRequest) {
	t.Helper()
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
	if _, err := ledger.Grant(e.ctx, pointGrant(now, 20)); err != nil {
		t.Fatal(err)
	}
	created, err := tasks.Create(e.ctx, taskRequest(1), points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 4, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 120000})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := tasks.Claim(e.ctx, 1, created.TaskID, "worker_budget_00001", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := service.NewAITaskBudgetService(repo)
	if err != nil {
		t.Fatal(err)
	}
	return e, db, repo, tasks, budget, task.ReserveRequest{UserID: 1, TaskID: created.TaskID, Fence: claimed.Fence, Owner: claimed.LeaseOwner, Key: "budget_operation_00001", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}}
}

func TestIntegrationAITaskBudgetLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, repo, tasks, s, r := runningBudgetTask(t, level)
			before := taskProbeCounts(e)
			if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
				t.Fatal("预算联合自检健康基线", err)
			}
			if !reflect.DeepEqual(before, taskProbeCounts(e)) {
				t.Fatal("联合探针遗留数据")
			}
			first, err := s.Reserve(e.ctx, r)
			if err != nil || !first.NewlyReserved || first.Reservation.Seq != 1 {
				t.Fatal(first, err)
			}
			again, err := s.Reserve(e.ctx, r)
			if err != nil || again.NewlyReserved || again.Reservation != first.Reservation {
				t.Fatal(again, err)
			}
			r.InputDigest = strings.Repeat("b", 64)
			if _, err := s.Reserve(e.ctx, r); !errors.Is(err, points.ErrIdempotency) {
				t.Fatal(err)
			}
			r.InputDigest = strings.Repeat("a", 64)
			for _, units := range []task.Units{{Calls: 4, Tokens: 100}, {Calls: 1, Tokens: 1000}, {Pages: 4}} {
				bad := r
				bad.Key = "budget_limit_rejected"
				bad.Units = units
				if _, err := s.Reserve(e.ctx, bad); !errors.Is(err, task.ErrBudget) {
					t.Fatal(units, err)
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 || e.queryInt("SELECT reserved_tokens FROM tm_ai_task_budgets") != 100 {
				t.Fatal("重复/拒绝请求改变预算")
			}
			queued, err := tasks.FailOrRetry(e.ctx, 1, r.TaskID, r.Owner, r.Fence, "provider_unavailable", time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, e, queued.NotBefore)
			claimed, err := tasks.Claim(e.ctx, 1, r.TaskID, "worker_budget_00002", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reserve(e.ctx, r); !errors.Is(err, task.ErrLease) {
				t.Fatal("旧围栏预留未拒绝", err)
			}
			r.Fence, r.Owner = claimed.Fence, claimed.LeaseOwner
			again, err = s.Reserve(e.ctx, r)
			if err != nil || again.NewlyReserved || again.Reservation.Fence != first.Reservation.Fence {
				t.Fatal("接管重置预留", again, err)
			}
			r.Key = "budget_page_operation"
			r.Units = task.Units{Pages: 1}
			if got, err := s.Reserve(e.ctx, r); err != nil || !got.NewlyReserved {
				t.Fatal(got, err)
			}
			if _, err := tasks.Cancel(e.ctx, 1, r.TaskID); err != nil {
				t.Fatal(err)
			}
			r.Key = "budget_cancel_rejected"
			if _, err := s.Reserve(e.ctx, r); !errors.Is(err, task.ErrCancelled) {
				t.Fatal(err)
			}
			if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 || e.queryInt("SELECT reservation_count FROM tm_ai_task_budgets") != 2 {
				t.Fatal("预算预留改变积分或清零")
			}
			if err := repo.WithinBudgetTx(e.ctx, func(tx task.BudgetTx) error {
				v, err := tx.LockTask(e.ctx, 1, r.TaskID)
				if err != nil {
					return err
				}
				s, err := tx.LockBudget(e.ctx, 1, r.TaskID)
				if err != nil {
					return err
				}
				rows, err := tx.Reservations(e.ctx, 1, r.TaskID)
				if err != nil {
					return err
				}
				return task.CheckBudget(v, s, rows)
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIntegrationAITaskBudgetConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, _, _, s, r := runningBudgetTask(t, level)
			db.SetMaxOpenConns(12)
			var fresh atomic.Int32
			var wg sync.WaitGroup
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					got, err := s.Reserve(e.ctx, r)
					if err != nil {
						t.Error(err)
					} else if got.NewlyReserved {
						fresh.Add(1)
					}
				}()
			}
			wg.Wait()
			if fresh.Load() != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 {
				t.Fatal("同键并发重复预留", fresh.Load())
			}
			fresh.Store(0)
			var limited atomic.Int32
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					c := r
					c.Key = fmt.Sprintf("budget_concurrent_%06d", i)
					got, err := s.Reserve(e.ctx, c)
					if errors.Is(err, task.ErrBudget) {
						limited.Add(1)
					} else if err != nil {
						t.Error(err)
					} else if got.NewlyReserved {
						fresh.Add(1)
					}
				}(i)
			}
			wg.Wait()
			if fresh.Load() != 3 || limited.Load() != 37 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 4 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 4 {
				t.Fatal("并发突破预算", fresh.Load(), limited.Load())
			}
		})
	}
}

func TestIntegrationAITaskBudgetIndependentTasks(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
			if _, err := ledger.Grant(e.ctx, pointGrant(now, 1000)); err != nil {
				t.Fatal(err)
			}
			var requests []task.ReserveRequest
			for i := 0; i < 40; i++ {
				created, err := tasks.Create(e.ctx, taskRequest(100+i), points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 2, MaxCalls: 2, MaxTokens: 200, MaxPages: 1, MaxDurationMS: 120000})
				if err != nil {
					t.Fatal(err)
				}
				v, err := tasks.Claim(e.ctx, 1, created.TaskID, "worker_budget_00001", time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				requests = append(requests, task.ReserveRequest{UserID: 1, TaskID: v.ID, Fence: v.Fence, Owner: v.LeaseOwner, Key: "budget_independent_01", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}})
			}
			db.SetMaxOpenConns(12)
			s, err := service.NewAITaskBudgetService(repo)
			if err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 2; round++ {
				var wg sync.WaitGroup
				for _, r := range requests {
					wg.Add(1)
					go func(r task.ReserveRequest) {
						defer wg.Done()
						if round == 1 {
							r.Key = "budget_independent_02"
						}
						got, err := s.Reserve(e.ctx, r)
						if err != nil || !got.NewlyReserved {
							t.Error(got, err)
						}
					}(r)
				}
				wg.Wait()
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 80 || e.queryInt("SELECT SUM(reserved_calls) FROM tm_ai_task_budgets") != 80 {
				t.Fatal("不同任务并发丢失预留")
			}
		})
	}
}

type budgetStoreFunc func(context.Context, func(task.BudgetTx) error) error

func (f budgetStoreFunc) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	return f(ctx, fn)
}

func TestIntegrationAITaskBudgetRecoveryAndRollback(t *testing.T) {
	e, db, repo, _, s, r := runningBudgetTask(t, sql.LevelReadCommitted)
	// 制造真实已提交但调用方收到未知结果；新实例按原键只能复用，不能再次获得预留。
	unknown, err := service.NewAITaskBudgetService(budgetStoreFunc(func(ctx context.Context, fn func(task.BudgetTx) error) error {
		if err := repo.WithinBudgetTx(ctx, fn); err != nil {
			return err
		}
		return context.DeadlineExceeded
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := unknown.Reserve(e.ctx, r); !errors.Is(err, context.DeadlineExceeded) || got != (task.ReservationReceipt{}) {
		t.Fatal(got, err)
	}
	s, err = service.NewAITaskBudgetService(repository.NewAITaskRepo(db))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Reserve(e.ctx, r); err != nil || got.NewlyReserved {
		t.Fatal(got, err)
	}
	// 父 RR 事务已建立旧快照，另一事务先提交第二次预留；当前读必须计入它。
	parent, err := service.NewAITaskBudgetService(budgetStoreFunc(func(ctx context.Context, fn func(task.BudgetTx) error) error {
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_task_reservations`).Scan(&count); err != nil {
			return err
		}
		other := r
		other.Key = "budget_parent_external"
		if _, err := s.Reserve(ctx, other); err != nil {
			return err
		}
		if err := fn(repository.AITaskTx(tx).(task.BudgetTx)); err != nil {
			return err
		}
		return tx.Commit()
	}))
	if err != nil {
		t.Fatal(err)
	}
	r.Key = "budget_parent_operation"
	if got, err := parent.Reserve(e.ctx, r); err != nil || !got.NewlyReserved || got.Reservation.Seq != 3 {
		t.Fatal("旧快照漏读预算", got, err)
	}
	// 真实触发器故障必须同时回滚明细和计数器。
	e.mustExec(`CREATE TRIGGER test_budget_failure AFTER INSERT ON tm_ai_task_reservations FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test budget failure'`)
	r.Key = "budget_failed_operation"
	if got, err := s.Reserve(e.ctx, r); err == nil || got != (task.ReservationReceipt{}) {
		t.Fatal(got, err)
	}
	if e.queryInt("SELECT reservation_count FROM tm_ai_task_budgets") != 3 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 3 {
		t.Fatal("预留故障未全部回滚")
	}
	e.mustExec(`DROP TRIGGER test_budget_failure`)
	if got, err := s.Reserve(e.ctx, r); err != nil || !got.NewlyReserved || got.Reservation.Seq != 4 {
		t.Fatal(got, err)
	}
}

func TestIntegrationAITaskBudgetSchemaCorruption(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"缺初始化保护", `DROP TRIGGER tm_ai_budget_init`}, {"计数无增长", `DROP TRIGGER tm_ai_reservation_account`}, {"限额身份可改", `DROP TRIGGER tm_ai_budget_update`},
		{"预算可删", `DROP TRIGGER tm_ai_budget_no_delete`}, {"租约失效保护", `DROP TRIGGER tm_ai_reservation_insert`}, {"预留可改", `DROP TRIGGER tm_ai_reservation_no_update`}, {"预留可删", `DROP TRIGGER tm_ai_reservation_no_delete`},
		{"预算检查缺失", `ALTER TABLE tm_ai_task_budgets DROP CHECK chk_tm_ai_budget`}, {"明细检查恒真", `ALTER TABLE tm_ai_task_reservations DROP CHECK chk_tm_ai_reservation, ADD CONSTRAINT chk_tm_ai_reservation CHECK(TRUE)`},
		{"操作键唯一缺失", `ALTER TABLE tm_ai_task_reservations DROP INDEX uk_tm_ai_reservation_key`}, {"预算归属缺失", `ALTER TABLE tm_ai_task_budgets DROP FOREIGN KEY fk_tm_ai_budget_task`},
		{"明细归属缺失", `ALTER TABLE tm_ai_task_reservations DROP FOREIGN KEY fk_tm_ai_reservation_budget`},
		{"浮点计数器", `ALTER TABLE tm_ai_task_budgets MODIFY reserved_tokens DOUBLE NOT NULL DEFAULT 0`},
		{"非二进制操作键", `ALTER TABLE tm_ai_task_reservations MODIFY reservation_key VARCHAR(80) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL`},
		{"错误初值", `ALTER TABLE tm_ai_task_budgets ALTER COLUMN reserved_tokens SET DEFAULT 1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, db, _, _, _, _ := runningBudgetTask(t, sql.LevelReadCommitted)
			v := repository.NewAITaskSchemaVerifier(db)
			if err := v.Verify(e.ctx); err != nil {
				t.Fatal("破坏前基线", err)
			}
			e.mustExec(tc.sql)
			if err := v.Verify(e.ctx); err == nil {
				t.Fatal("预算保护破坏未被检出")
			}
		})
	}
}
