//go:build integration

package points_test

import (
	"bufio"
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
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type recoveryEnv struct {
	*pointsTestEnv
	db       *sql.DB
	repo     *repository.AITaskRepo
	tasks    *service.AITaskService
	planner  *service.AIExtractionRecoveryPlanner
	specs    []service.ExtractionRecoverySpec
	plan     task.RecoveryPlan
	old      task.Task
	sourceID int64
}

func newRecoveryEnv(t *testing.T, level sql.IsolationLevel, count int, capture bool, payload string) *recoveryEnv {
	t.Helper()
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
	ddl, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261010_create_ai_source_snapshots.sql"))
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(string(ddl))
	e.mustExec("CREATE TABLE addon_quant_theme(id bigint PRIMARY KEY,name varchar(64),status tinyint,deleted_at datetime)")
	e.mustExec("INSERT INTO addon_quant_theme(id,name,status) VALUES(1,'算力',1)")
	sourceRepo := repository.NewAISourceRepo(db)
	src, err := sourceRepo.RegisterSource(e.ctx, sources.Source{Domain: "recovery.example", Category: sources.CategoryAnnualReport, LicenseTag: "fake-recovery-test", LicenseRef: "fake-written-approval"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceRepo.SetSourceEnabled(e.ctx, src.ID, true); err != nil {
		t.Fatal(err)
	}
	src.Enabled = true
	var specs []service.ExtractionRecoverySpec
	for i := 1; i <= count; i++ {
		snap, err := sources.BuildSnapshot(src, fmt.Sprintf("https://recovery.example/test/%d", i), sources.MediaPlain, []byte(fmt.Sprintf("测试星河公司主营算力服务。第%d份明确的测试资料。", i)), now, 4096)
		if err != nil {
			t.Fatal(err)
		}
		snap, err = sourceRepo.SaveSnapshot(e.ctx, snap)
		if err != nil {
			t.Fatal(err)
		}
		specs = append(specs, service.ExtractionRecoverySpec{SnapshotID: snap.ID, OperationKey: fmt.Sprintf("recovery_operation_%04d", i)})
	}
	if _, err := ledger.Grant(e.ctx, pointGrant(now, 20)); err != nil {
		t.Fatal(err)
	}
	created, err := tasks.Create(e.ctx, taskRequest(1), points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 8, MaxCalls: 10, MaxTokens: 10000, MaxPages: 3, MaxDurationMS: 120000})
	if err != nil {
		t.Fatal(err)
	}
	old, err := tasks.Claim(e.ctx, 1, created.TaskID, "worker_original_0001", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	planner, err := service.NewAIExtractionRecoveryPlanner(sourceRepo, repository.NewAIExtractionCatalog(db), specs)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planner.Plan(e.ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := service.NewAITaskBudgetService(repo)
	if err != nil {
		t.Fatal(err)
	}
	for index, step := range plan.Steps {
		if _, err := budget.Reserve(e.ctx, task.ReserveRequest{UserID: 1, TaskID: old.ID, Fence: old.Fence, Owner: old.LeaseOwner, Key: step.OperationKey, InputDigest: step.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
			t.Fatal(err)
		}
		if capture {
			stages, err := service.NewAITaskStageService(repo, map[string]task.StagePolicy{step.Stage: step.Policy})
			if err != nil {
				t.Fatal(err)
			}
			r := task.StageRequest{UserID: 1, TaskID: old.ID, Fence: old.Fence, Owner: old.LeaseOwner, OperationKey: step.OperationKey, Stage: step.Stage, InputDigest: step.InputDigest, StepNo: index + 1}
			if _, err := stages.Capture(e.ctx, r, []byte(payload)); err != nil {
				t.Fatal(err)
			}
			if index < count-1 {
				if _, err := stages.Complete(e.ctx, r); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
		t.Fatal("恢复用例健康基线失败", err)
	}
	return &recoveryEnv{pointsTestEnv: e, db: db, repo: repo, tasks: tasks, planner: planner, specs: specs, plan: plan, old: old, sourceID: src.ID}
}
func (e *recoveryEnv) queue(t *testing.T) {
	t.Helper()
	v, err := e.tasks.FailOrRetry(e.ctx, 1, e.old.ID, e.old.LeaseOwner, e.old.Fence, "provider_unavailable", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	waitTaskTime(t, e.pointsTestEnv, v.NotBefore)
}
func (e *recoveryEnv) worker(t *testing.T, planner service.RecoveryPlanner, owner string) *service.AIRecoveryWorker {
	t.Helper()
	w, err := service.NewAIRecoveryWorker(e.tasks, e.repo, planner, owner, task.RecoveryOptions{Lease: time.Second, Heartbeat: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

type recoveryPlanFunc func(context.Context, task.Task) (task.RecoveryPlan, error)

func (f recoveryPlanFunc) Plan(ctx context.Context, v task.Task) (task.RecoveryPlan, error) {
	return f(ctx, v)
}

func TestIntegrationAITaskRecoveryLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			t.Run("多阶段恢复", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 2, true, extractionStageFact)
				e.queue(t)
				w := e.worker(t, e.planner, "worker_recovery_0002")
				out, err := w.Recover(e.ctx, 1, e.old.ID)
				if err != nil || out.Outcome != "report_pending" || out.CurrentStep != 2 || out.ReportAvailable {
					t.Fatal(out, err)
				}
				if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 2 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 2 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 2 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
					t.Fatal("恢复重复执行/记账")
				}
				if _, err := w.Recover(e.ctx, 1, e.old.ID); !errors.Is(err, task.ErrNotReady) {
					t.Fatal("活跃租约被重抢", err)
				}
				v, err := e.tasks.Inspect(e.ctx, 1, e.old.ID)
				if err != nil {
					t.Fatal(err)
				}
				waitTaskTime(t, e.pointsTestEnv, *v.LeaseUntil)
				if out, err := w.Recover(e.ctx, 1, e.old.ID); err != nil || out.Outcome != "report_pending" || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 2 {
					t.Fatal(out, err)
				}
				step := e.plan.Steps[1]
				s, _ := service.NewAITaskStageService(e.repo, map[string]task.StagePolicy{"extract": step.Policy})
				if _, err := s.Complete(e.ctx, task.StageRequest{UserID: 1, TaskID: e.old.ID, Fence: e.old.Fence, Owner: e.old.LeaseOwner, OperationKey: step.OperationKey, Stage: step.Stage, InputDigest: step.InputDigest, StepNo: 2}); !errors.Is(err, task.ErrLease) {
					t.Fatal("旧围栏恢复被接受", err)
				}
			})
			t.Run("未知响应", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, false, "")
				e.queue(t)
				out, err := e.worker(t, e.planner, "worker_unknown_0001").Recover(e.ctx, 1, e.old.ID)
				if err != nil || out.Outcome != "response_unknown" || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
					t.Fatal(out, err)
				}
			})
			t.Run("校验拒绝", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, true, `{"claims":[]}`)
				e.queue(t)
				out, err := e.worker(t, e.planner, "worker_rejected_001").Recover(e.ctx, 1, e.old.ID)
				if err != nil || out.TaskStatus != task.Failed || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 3 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 {
					t.Fatal(out, err)
				}
			})
			t.Run("用户取消", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, true, extractionStageFact)
				if _, err := e.tasks.Cancel(e.ctx, 1, e.old.ID); err != nil {
					t.Fatal(err)
				}
				if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.sourceID, false); err != nil {
					t.Fatal(err)
				}
				waitTaskTime(t, e.pointsTestEnv, *e.old.LeaseUntil)
				out, err := e.worker(t, e.planner, "worker_cancelled_01").Recover(e.ctx, 1, e.old.ID)
				if err != nil || out.TaskStatus != task.Cancelled || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
					t.Fatal(out, err)
				}
			})
			t.Run("来源撤回", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, true, extractionStageFact)
				e.queue(t)
				if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.sourceID, false); err != nil {
					t.Fatal(err)
				}
				out, err := e.worker(t, e.planner, "worker_revoked_0001").Recover(e.ctx, 1, e.old.ID)
				if !errors.Is(err, sources.ErrUnauthorized) || out != (task.RecoveryResult{}) || e.queryInt("SELECT attempt FROM tm_ai_tasks") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
					t.Fatal(out, err)
				}
			})
			t.Run("主数据改变", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, true, extractionStageFact)
				e.queue(t)
				e.mustExec("UPDATE addon_quant_theme SET name='数据中心' WHERE id=1")
				out, err := e.worker(t, e.planner, "worker_changed_0001").Recover(e.ctx, 1, e.old.ID)
				if !errors.Is(err, points.ErrIdempotency) || out != (task.RecoveryResult{}) || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
					t.Fatal(out, err)
				}
			})
			t.Run("心跳取消", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, true, extractionStageFact)
				e.queue(t)
				entered := make(chan struct{})
				plan := e.plan
				plan.Steps = append([]task.RecoveryStep(nil), plan.Steps...)
				plan.Steps[0].Policy.Check = func(ctx context.Context, _ task.StageValidationInput) error {
					close(entered)
					<-ctx.Done()
					return ctx.Err()
				}
				w := e.worker(t, recoveryPlanFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return plan, nil }), "worker_heartbeat_01")
				done := make(chan struct {
					out task.RecoveryResult
					err error
				}, 1)
				go func() {
					out, err := w.Recover(e.ctx, 1, e.old.ID)
					done <- struct {
						out task.RecoveryResult
						err error
					}{out, err}
				}()
				select {
				case <-entered:
				case <-e.ctx.Done():
					t.Fatal("未进入测试阻塞校验")
				}
				if _, err := e.tasks.Cancel(e.ctx, 1, e.old.ID); err != nil {
					t.Fatal(err)
				}
				select {
				case result := <-done:
					if result.err != nil || result.out.TaskStatus != task.Cancelled {
						t.Fatal(result.out, result.err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("心跳未取消并结束校验")
				}
				if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
					t.Fatal("取消仍推进或未原子释放")
				}
			})
			t.Run("事件故障回滚", func(t *testing.T) {
				e := newRecoveryEnv(t, level, 1, true, extractionStageFact)
				e.queue(t)
				e.mustExec("CREATE TRIGGER test_recovery_event_failure BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='step_completed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test recovery rollback'; END IF; END")
				w := e.worker(t, e.planner, "worker_rollback_001")
				out, err := w.Recover(e.ctx, 1, e.old.ID)
				if err == nil || out != (task.RecoveryResult{}) || e.queryInt("SELECT current_step FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
					t.Fatal(out, err)
				}
				e.mustExec("DROP TRIGGER test_recovery_event_failure")
				v, err := e.tasks.Inspect(e.ctx, 1, e.old.ID)
				if err != nil {
					t.Fatal(err)
				}
				waitTaskTime(t, e.pointsTestEnv, *v.LeaseUntil)
				if out, err := w.Recover(e.ctx, 1, e.old.ID); err != nil || out.Outcome != "report_pending" {
					t.Fatal(out, err)
				}
			})
		})
	}
}

func workerTestDSN(t *testing.T, e *recoveryEnv) string {
	t.Helper()
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试 DSN 无效")
	}
	cfg.DBName = e.database
	return cfg.FormatDSN()
}

func restrictedWorkerDSN(t *testing.T, e *recoveryEnv) string {
	t.Helper()
	cfg, err := mysqldrv.ParseDSN(workerTestDSN(t, e))
	if err != nil {
		t.Fatal("测试受限连接配置失败")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	user := "worker_it_" + hex.EncodeToString(random[:6])
	cfg.User = user
	cfg.Passwd = hex.EncodeToString(random[:])
	// 仅本轮隔离实例中的随机十六进制账号；不输出口令或 DSN。
	e.mustExec("CREATE USER '" + user + "'@'%' IDENTIFIED BY '" + cfg.Passwd + "'")
	t.Cleanup(func() { e.mustExec("DROP USER '" + user + "'@'%'") })
	for _, table := range taskProbeTables {
		e.mustExec("GRANT SELECT, INSERT, UPDATE, DELETE ON `" + e.database + "`." + table + " TO '" + user + "'@'%'")
	}
	for _, table := range []string{"tm_ai_sources", "tm_ai_source_bodies", "tm_ai_source_snapshots", "addon_quant_theme"} {
		e.mustExec("GRANT SELECT ON `" + e.database + "`." + table + " TO '" + user + "'@'%'")
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("受限连接失败")
	}
	defer db.Close()
	if _, err := db.ExecContext(e.ctx, "CREATE TABLE recovery_forbidden_ddl(id INT)"); err == nil {
		t.Fatal("恢复账号不应有 DDL 权限")
	} else {
		var permission *mysqldrv.MySQLError
		if !errors.As(err, &permission) || permission.Number != 1142 {
			t.Fatal("不是预期的 DDL 权限拒绝")
		}
	}
	return cfg.FormatDSN()
}

func TestAITaskRecoveryProcessHelper(t *testing.T) {
	dsn := os.Getenv("TM_WORKER_TEST_DSN")
	if dsn == "" {
		return
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("测试子进程数据库配置失败")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	level := sql.LevelReadCommitted
	if os.Getenv("TM_WORKER_TEST_LEVEL") == "RR" {
		level = sql.LevelRepeatableRead
	}
	repo := repository.NewAITaskRepoWithIsolation(db, level)
	ledger, _ := service.NewPointsService(repository.NewPointsRepoWithIsolation(db, level))
	tasks, _ := service.NewAITaskService(repo, ledger)
	var request struct {
		ID    int64
		Specs []service.ExtractionRecoverySpec
	}
	if err := json.Unmarshal([]byte(os.Getenv("TM_WORKER_TEST_REQUEST")), &request); err != nil {
		t.Fatal("测试子进程请求无效")
	}
	planner, err := service.NewAIExtractionRecoveryPlanner(repository.NewAISourceRepo(db), repository.NewAIExtractionCatalog(db), request.Specs)
	if err != nil {
		t.Fatal(err)
	}
	blocking := recoveryPlanFunc(func(ctx context.Context, v task.Task) (task.RecoveryPlan, error) {
		p, err := planner.Plan(ctx, v)
		if err != nil {
			return p, err
		}
		p.Steps[0].Policy.Check = func(ctx context.Context, _ task.StageValidationInput) error {
			fmt.Fprintln(os.Stdout, "recovery_validator_entered")
			<-ctx.Done()
			return ctx.Err()
		}
		return p, nil
	})
	w, err := service.NewAIRecoveryWorker(tasks, repo, blocking, "worker_process_old", task.RecoveryOptions{Lease: time.Second, Heartbeat: 100 * time.Millisecond, Timeout: 20 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.NewAITaskSchemaVerifier(db).Verify(ctx); err != nil {
		t.Fatal("子进程自检失败")
	}
	if _, err := w.Recover(ctx, 1, request.ID); err != nil {
		t.Fatal("测试子进程未完成恢复")
	}
}

func TestIntegrationAITaskRecoveryProcessCrash(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "recovery-test.exe")
	buildCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "test", "-p", "1", "-race", "-tags=integration", "-c", "-o", executable, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("构建恢复测试进程失败: %v；%s", err, output)
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newRecoveryEnv(t, level, 1, true, extractionStageFact)
			e.queue(t)
			request, err := json.Marshal(struct {
				ID    int64
				Specs []service.ExtractionRecoverySpec
			}{e.old.ID, e.specs})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(e.ctx, executable, "-test.run=^TestAITaskRecoveryProcessHelper$")
			for _, item := range os.Environ() {
				if !strings.HasPrefix(item, "TM_WORKER_TEST_") {
					cmd.Env = append(cmd.Env, item)
				}
			}
			iso := "RC"
			if level == sql.LevelRepeatableRead {
				iso = "RR"
			}
			cmd.Env = append(cmd.Env, "TM_WORKER_TEST_DSN="+workerTestDSN(t, e), "TM_WORKER_TEST_REQUEST="+string(request), "TM_WORKER_TEST_LEVEL="+iso)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			finished := false
			defer func() {
				if !finished {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}() // 只清理并回收本用例明确启动的进程。
			entered := make(chan bool, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					if scanner.Text() == "recovery_validator_entered" {
						entered <- true
						return
					}
				}
				entered <- false
			}()
			select {
			case ok := <-entered:
				if !ok {
					t.Fatal("子进程未进入隔离校验")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("等待子进程进入校验超时")
			}
			before, err := e.tasks.Inspect(e.ctx, 1, e.old.ID)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-time.After(1200 * time.Millisecond):
			case <-e.ctx.Done():
				t.Fatal("测试上下文结束")
			}
			after, err := e.tasks.Inspect(e.ctx, 1, e.old.ID)
			if err != nil || !after.LeaseUntil.After(*before.LeaseUntil) {
				t.Fatal("独立心跳未续租", err)
			}
			if _, err := e.tasks.Claim(e.ctx, 1, e.old.ID, "worker_competitor_01", time.Second); !errors.Is(err, task.ErrNotReady) {
				t.Fatal("校验期间租约被抢占", err)
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			waitErr := cmd.Wait()
			finished = true
			if waitErr == nil {
				t.Fatal("测试进程未被杀死")
			}
			if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
				t.Fatal("杀进程造成伪完成/退款")
			}
			last, err := e.tasks.Inspect(e.ctx, 1, e.old.ID)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, e.pointsTestEnv, *last.LeaseUntil)
			out, err := e.worker(t, e.planner, "worker_process_new").Recover(e.ctx, 1, e.old.ID)
			if err != nil || out.Outcome != "report_pending" || out.CurrentStep != 1 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
				t.Fatal(out, err)
			}
			step := e.plan.Steps[0]
			stages, _ := service.NewAITaskStageService(e.repo, map[string]task.StagePolicy{"extract": step.Policy})
			if _, err := stages.Complete(e.ctx, task.StageRequest{UserID: 1, TaskID: e.old.ID, Fence: last.Fence, Owner: last.LeaseOwner, OperationKey: step.OperationKey, Stage: "extract", InputDigest: step.InputDigest, StepNo: 1}); !errors.Is(err, task.ErrLease) {
				t.Fatal("被杀旧进程的围栏仍可写", err)
			}
		})
	}
}

func TestIntegrationAITaskRecoveryCLI(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "ai-worker.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-p", "1", "-o", executable, "../../cmd/ai-worker").CombinedOutput(); err != nil {
		t.Fatalf("构建恢复命令失败: %v；%s", err, output)
	}
	for _, mode := range []string{"已有响应", "未知响应", "来源停用", "保护缺失"} {
		t.Run(mode, func(t *testing.T) {
			e := newRecoveryEnv(t, sql.LevelReadCommitted, 1, mode != "未知响应", extractionStageFact)
			e.queue(t)
			body, err := json.Marshal(struct {
				Steps []service.ExtractionRecoverySpec `json:"steps"`
			}{e.specs})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "来源停用" {
				if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.sourceID, false); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "保护缺失" {
				e.mustExec("DROP TRIGGER tm_ai_step_account")
			}
			cmd := exec.CommandContext(e.ctx, executable, "recover", "-user", "1", "-task", fmt.Sprint(e.old.ID), "-plan", path)
			for _, item := range os.Environ() {
				if !strings.HasPrefix(item, "TM_DB_DSN=") && !strings.HasPrefix(item, "TM_QUOTE_ENABLED=") {
					cmd.Env = append(cmd.Env, item)
				}
			}
			dsn := workerTestDSN(t, e)
			if mode == "已有响应" {
				dsn = restrictedWorkerDSN(t, e)
			}
			cmd.Env = append(cmd.Env, "TM_DB_DSN="+dsn, "TM_QUOTE_ENABLED=false")
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal("命令进程错误")
				}
				code = exit.ExitCode()
			}
			if strings.Contains(string(output), dsn) || strings.Contains(string(output), "测试星河公司") || strings.Contains(string(output), "fake-written-approval") || strings.Contains(string(output), e.specs[0].OperationKey) {
				t.Fatal("真实命令泄漏敏感输入")
			}
			want := 1
			if mode == "已有响应" {
				want = 0
			} else if mode == "未知响应" {
				want = 2
			}
			if code != want {
				t.Fatal("命令退出码不符", code)
			}
			if code == 0 || code == 2 {
				var receipt task.RecoveryResult
				if err := json.Unmarshal(output, &receipt); err != nil || receipt.TaskID != e.old.ID || receipt.ReportAvailable {
					t.Fatal("命令没有安全元数据收据")
				}
			} else if e.queryInt("SELECT attempt FROM tm_ai_tasks") != 1 {
				t.Fatal("启动/授权拒绝后仍抢占任务")
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 {
				t.Fatal("命令重复预算或擅自扣费/退款")
			}
		})
	}
}
