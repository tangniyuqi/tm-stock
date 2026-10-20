//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type expiryEnv struct {
	*pointsTestEnv
	db                               *sql.DB
	repo                             *repository.AITaskRepo
	tasks                            *service.AITaskService
	ledger                           *service.PointsService
	first, running, terminal, future task.Created
	lease                            task.Task
}

// 只用于测试：在读到真实上界后并发创建任务，不改写期限或生产保护。
type expiryRangeHook struct {
	task.ExpiryScanner
	after func()
}

func (s expiryRangeHook) ExpiryRange(ctx context.Context) (task.ExpiryRange, error) {
	v, err := s.ExpiryScanner.ExpiryRange(ctx)
	if err == nil {
		s.after()
	}
	return v, err
}

func newExpiryEnv(t *testing.T, level sql.IsolationLevel) *expiryEnv {
	t.Helper()
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
	for _, uid := range []int64{1, 2} {
		r := pointGrant(now, 100)
		r.UserID, r.Key, r.BizID = uid, fmt.Sprintf("test_expiry_grant_%d", uid), fmt.Sprintf("test-expiry-grant-%d", uid)
		if _, err := ledger.Grant(e.ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	v := &expiryEnv{pointsTestEnv: e, db: db, repo: repo, tasks: tasks, ledger: ledger}
	create := func(n int, uid, duration int64) task.Created {
		r, b := taskRequest(n), taskBudget()
		r.UserID, b.MaxDurationMS = uid, duration
		c, err := tasks.Create(e.ctx, r, points.Price{Version: "v1", Points: 7}, b)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	v.first = create(1, 1, 3000)
	v.running = create(2, 2, 3000)
	var err error
	v.lease, err = tasks.Claim(e.ctx, 2, v.running.TaskID, "worker_expiry_original", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	budgets, err := service.NewAITaskBudgetService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := budgets.Reserve(e.ctx, task.ReserveRequest{UserID: 2, TaskID: v.lease.ID, Fence: v.lease.Fence, Owner: v.lease.LeaseOwner, Key: "expiry_unknown_operation", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
		t.Fatal(err)
	}
	v.terminal, v.future = create(3, 1, 300000), create(4, 2, 300000)
	if _, err := tasks.Cancel(e.ctx, 1, v.terminal.TaskID); err != nil {
		t.Fatal(err)
	}
	if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
		t.Fatal("清扫健康基线", err)
	}
	waitTaskTime(t, e, v.lease.Deadline)
	return v
}

func (e *expiryEnv) sweep(t *testing.T, finisher task.Expirer, opts task.ExpiryOptions) (task.ExpirySweep, error) {
	t.Helper()
	s, err := service.NewAITaskExpiryService(finisher, e.repo)
	if err != nil {
		t.Fatal(err)
	}
	return s.Sweep(e.ctx, opts)
}

func (e *expiryEnv) check(t *testing.T) {
	t.Helper()
	for _, c := range []task.Created{e.first, e.running} {
		if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", c.TaskID) != task.Expired || e.queryInt("SELECT state FROM tm_points_holds WHERE id=?", c.HoldID) != points.Expired || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE task_id=? AND type='expired'", c.TaskID) != 1 {
			t.Fatal("未恰好一次过期")
		}
	}
	if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", e.future.TaskID) != task.Queued || e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", e.terminal.TaskID) != task.Cancelled || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=2") != 7 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 0 {
		t.Fatal("清扫改变未到期任务/预算/响应")
	}
	for _, uid := range []int64{1, 2} {
		v, err := e.ledger.Reconcile(e.ctx, uid)
		if err != nil || !v.Healthy() {
			t.Fatal(v, err)
		}
	}
	if _, err := e.tasks.Heartbeat(e.ctx, 2, e.lease.ID, e.lease.LeaseOwner, e.lease.Fence, time.Second); !errors.Is(err, task.ErrLease) {
		t.Fatal("旧 worker 仍可写", err)
	}
}

func TestIntegrationAITaskExpiryLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newExpiryEnv(t, level)
			a, err := e.sweep(t, e.tasks, task.ExpiryOptions{PageSize: 1, MaxTasks: 1})
			if err != nil || a.Complete || a.LastID != e.first.TaskID || a.ExpiredObserved != 1 || a.Visited != 1 {
				t.Fatal(a, err)
			}
			b, err := e.sweep(t, e.tasks, task.ExpiryOptions{AfterID: a.LastID, PageSize: 2, MaxTasks: 10})
			if err != nil || !b.Complete || b.Visited != 3 || b.ExpiredObserved != 1 || b.Skipped != 2 {
				t.Fatal(b, err)
			}
			e.check(t)
			before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			c, err := e.sweep(t, e.tasks, task.ExpiryOptions{PageSize: 2, MaxTasks: 10})
			if err != nil || !c.Complete || c.ExpiredObserved != 0 || c.Skipped != 4 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
				t.Fatal("重扫产生重复账务", c, err)
			}
			cfg, err := mysqldrv.ParseDSN(expiryDSN(t, e, false))
			if err != nil {
				t.Fatal(err)
			}
			cfg.Loc, err = time.LoadLocation("Asia/Shanghai")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Params == nil {
				cfg.Params = map[string]string{}
			}
			cfg.Params["time_zone"] = "'+08:00'"
			db, err := sql.Open("mysql", cfg.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repo := repository.NewAITaskRepo(db)
			r, err := repo.ExpiryRange(e.ctx)
			if err != nil || r.AsOf.Location() != time.UTC || r.UpperID != e.future.TaskID || r.AsOf.UnixMilli() < c.AsOf || r.AsOf.Sub(time.Now()) > time.Minute {
				t.Fatal(r, err)
			}
			page, err := repo.ExpiryPage(e.ctx, 0, r.UpperID, 10)
			if err != nil || len(page) != 4 || !page[1].Deadline.Equal(e.lease.Deadline) {
				t.Fatal(page, err)
			}
			var plan string
			rows, err := db.QueryContext(e.ctx, "EXPLAIN FORMAT=TREE SELECT id,user_id,status,CAST(deadline_at AS CHAR) FROM tm_ai_tasks WHERE id>? AND id<=? ORDER BY id LIMIT ?", 0, r.UpperID, 2)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if !rows.Next() || rows.Scan(&plan) != nil || !strings.Contains(plan, "PRIMARY") || rows.Err() != nil {
				t.Fatal("游标未走主键", plan)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			var created task.Created
			hook := expiryRangeHook{ExpiryScanner: e.repo, after: func() {
				b := taskBudget()
				b.MaxDurationMS = 1000
				var err error
				created, err = e.tasks.Create(e.ctx, taskRequest(5), points.Price{Version: "v1", Points: 7}, b)
				if err != nil {
					t.Fatal(err)
				}
				v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
				if err != nil {
					t.Fatal(err)
				}
				waitTaskTime(t, e.pointsTestEnv, v.Deadline)
			}}
			svc, err := service.NewAITaskExpiryService(e.tasks, hook)
			if err != nil {
				t.Fatal(err)
			}
			bounded, err := svc.Sweep(e.ctx, task.ExpiryOptions{PageSize: 1, MaxTasks: 10})
			if err != nil || !bounded.Complete || bounded.UpperID != e.future.TaskID || bounded.Visited != 4 || e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", created.TaskID) != task.Queued {
				t.Fatal("追逐新任务或越过上界", bounded, err)
			}
			next, err := e.sweep(t, e.tasks, task.ExpiryOptions{PageSize: 1, MaxTasks: 10})
			if err != nil || !next.Complete || next.UpperID != created.TaskID || next.ExpiredObserved != 1 {
				t.Fatal("下一轮未补扫新任务", next, err)
			}
			e.check(t)
		})
	}
}

func TestIntegrationAITaskExpiryAtomicity(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"事件失败", "提交未知"} {
				t.Run(mode, func(t *testing.T) {
					e := newExpiryEnv(t, level)
					var finisher task.Expirer = e.tasks
					before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
					if mode == "事件失败" {
						e.mustExec("CREATE TRIGGER test_expiry_event_failure BEFORE INSERT ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fake expiry event failure'")
					} else {
						var err error
						finisher, err = service.NewAITaskService(abortTaskStore{db: e.db, level: level, commit: true, after: errors.New("fake unknown expiry commit")}, e.ledger)
						if err != nil {
							t.Fatal(err)
						}
					}
					out, err := e.sweep(t, finisher, task.ExpiryOptions{PageSize: 2, MaxTasks: 10})
					if err == nil || out.Complete || out.LastID != 0 || out.Visited != 0 {
						t.Fatal("失败项越过游标", out, err)
					}
					if mode == "事件失败" {
						if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", e.first.TaskID) != task.Queued || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
							t.Fatal("事件失败未整体回滚")
						}
						e.mustExec("DROP TRIGGER test_expiry_event_failure")
					} else if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", e.first.TaskID) != task.Expired || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before+1 {
						t.Fatal("未模拟真实已提交")
					}
					out, err = e.sweep(t, e.tasks, task.ExpiryOptions{PageSize: 2, MaxTasks: 10})
					if err != nil || !out.Complete || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before+2 {
						t.Fatal(out, err)
					}
					e.check(t)
				})
			}
		})
	}
}

func TestIntegrationAITaskExpiryConcurrent(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newExpiryEnv(t, level)
			var wg sync.WaitGroup
			faults := make(chan error, 40)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					s, err := service.NewAITaskExpiryService(e.tasks, e.repo)
					if err == nil {
						var out task.ExpirySweep
						out, err = s.Sweep(e.ctx, task.ExpiryOptions{PageSize: 1, MaxTasks: 10})
						if err == nil && !out.Complete {
							err = points.ErrInvariant
						}
					}
					faults <- err
				}()
			}
			wg.Wait()
			close(faults)
			for err := range faults {
				if err != nil {
					t.Fatal(err)
				}
			}
			e.check(t)
		})
	}
}

func expiryDSN(t *testing.T, e *expiryEnv, restricted bool) string {
	t.Helper()
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试连接配置")
	}
	cfg.DBName = e.database
	if restricted {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			t.Fatal(err)
		}
		cfg.User = "expiry_it_" + hex.EncodeToString(random[:6])
		cfg.Passwd = hex.EncodeToString(random[:])
		e.mustExec("CREATE USER '" + cfg.User + "'@'%' IDENTIFIED BY '" + cfg.Passwd + "'")
		t.Cleanup(func() { e.mustExec("DROP USER '" + cfg.User + "'@'%'") })
		for _, table := range taskProbeTables {
			e.mustExec("GRANT SELECT,INSERT,UPDATE,DELETE ON `" + e.database + "`." + table + " TO '" + cfg.User + "'@'%'")
		}
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.ExecContext(e.ctx, "CREATE TABLE expiry_forbidden_ddl(id INT)"); err == nil {
			t.Fatal("受限账号有 DDL 权限")
		} else {
			var denied *mysqldrv.MySQLError
			if !errors.As(err, &denied) || denied.Number != 1142 {
				t.Fatal("不是权限拒绝")
			}
		}
	}
	return cfg.FormatDSN()
}

func TestIntegrationAITaskExpiryCLI(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "ai-task-expire.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if raw, err := exec.CommandContext(ctx, "go", "build", "-p", "1", "-o", executable, "../../cmd/ai-task-expire").CombinedOutput(); err != nil {
		t.Fatalf("构建清扫命令失败: %v %s", err, raw)
	}
	for _, mode := range []string{"完整受限账号", "限量续扫", "保护缺失", "已取消父进程"} {
		t.Run(mode, func(t *testing.T) {
			e := newExpiryEnv(t, sql.LevelReadCommitted)
			if mode == "保护缺失" {
				e.mustExec("DROP TRIGGER tm_ai_task_events_no_update")
			}
			dsn := expiryDSN(t, e, mode == "完整受限账号")
			args := []string{"-page-size", "1", "-max-tasks", "10"}
			if mode == "限量续扫" {
				args = []string{"-page-size", "1", "-max-tasks", "1"}
			}
			runCtx := e.ctx
			if mode == "已取消父进程" {
				var stop context.CancelFunc
				runCtx, stop = context.WithCancel(e.ctx)
				stop()
			}
			cmd := exec.CommandContext(runCtx, executable, args...)
			for _, item := range os.Environ() {
				if !strings.HasPrefix(item, "TM_DB_DSN=") && !strings.HasPrefix(item, "TM_QUOTE_ENABLED=") {
					cmd.Env = append(cmd.Env, item)
				}
			}
			cmd.Env = append(cmd.Env, "TM_DB_DSN="+dsn, "TM_QUOTE_ENABLED=false")
			raw, err := cmd.CombinedOutput()
			if mode == "已取消父进程" {
				if !errors.Is(err, context.Canceled) || len(raw) != 0 {
					t.Fatal("已取消父进程仍启动命令")
				}
				return
			}
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal("命令运行失败")
				}
				code = exit.ExitCode()
			}
			var v struct {
				Event  string            `json:"event"`
				Sweep  *task.ExpirySweep `json:"sweep"`
				Failed bool              `json:"failed"`
			}
			if json.Unmarshal(raw, &v) != nil || strings.Contains(string(raw), dsn) || strings.Contains(string(raw), "expiry_unknown_operation") || strings.Contains(string(raw), "payload") {
				t.Fatal("输出非法或泄漏")
			}
			switch mode {
			case "完整受限账号":
				if code != 0 || v.Failed || v.Sweep == nil || !v.Sweep.Complete || v.Sweep.ExpiredObserved != 2 {
					t.Fatal(code, v)
				}
				e.check(t)
			case "限量续扫":
				if code != 2 || v.Failed || v.Sweep == nil || v.Sweep.Complete || v.Sweep.LastID != e.first.TaskID {
					t.Fatal(code, v)
				}
				out, err := e.sweep(t, e.tasks, task.ExpiryOptions{AfterID: v.Sweep.LastID, PageSize: 1, MaxTasks: 10})
				if err != nil || !out.Complete {
					t.Fatal(out, err)
				}
				e.check(t)
			case "保护缺失":
				if code != 1 || !v.Failed || v.Sweep != nil || e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", e.first.TaskID) != task.Queued {
					t.Fatal("保护缺失仍清扫", code, v)
				}
			}
		})
	}
}
