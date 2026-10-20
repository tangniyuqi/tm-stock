//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

var testStageBody = []byte(`{"test":"隔离测试数据"}`)

func testStagePolicies() map[string]task.StagePolicy {
	return map[string]task.StagePolicy{"extract": {Digest: strings.Repeat("c", 64), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if in.Stage != "extract" || !strings.Contains(in.Payload, "隔离测试数据") {
			return errors.New("仅接受明确的隔离测试夹具")
		}
		return nil
	}}}
}
func runningStageTask(t *testing.T, level sql.IsolationLevel) (*pointsTestEnv, *sql.DB, *repository.AITaskRepo, *service.AITaskService, *service.AITaskStageService, task.StageRequest) {
	t.Helper()
	e, db, repo, tasks, budget, r := runningBudgetTask(t, level)
	if _, err := budget.Reserve(e.ctx, r); err != nil {
		t.Fatal(err)
	}
	stages, err := service.NewAITaskStageService(repo, testStagePolicies())
	if err != nil {
		t.Fatal(err)
	}
	return e, db, repo, tasks, stages, task.StageRequest{UserID: r.UserID, TaskID: r.TaskID, Fence: r.Fence, Owner: r.Owner, OperationKey: r.Key, Stage: "extract", InputDigest: r.InputDigest, StepNo: 1}
}

func TestIntegrationAITaskStageLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, repo, _, s, r := runningStageTask(t, level)
			before := taskProbeCounts(e)
			if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
				t.Fatal("阶段联合自检基线", err)
			}
			if !reflect.DeepEqual(before, taskProbeCounts(e)) {
				t.Fatal("探针遗留数据")
			}
			if _, err := s.Complete(e.ctx, r); !errors.Is(err, points.ErrNotFound) {
				t.Fatal("未知响应完成", err)
			}
			first, err := s.Capture(e.ctx, r, testStageBody)
			if err != nil || !first.NewlyCaptured {
				t.Fatal(first, err)
			}
			if got, err := s.Capture(e.ctx, r, testStageBody); err != nil || got.NewlyCaptured || got.OutputDigest != first.OutputDigest {
				t.Fatal(got, err)
			}
			if _, err := s.Capture(e.ctx, r, []byte(`{"test":"修改结果"}`)); !errors.Is(err, points.ErrIdempotency) {
				t.Fatal(err)
			}
			if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 {
				t.Fatal("捕获就推进")
			}
			if _, err := s.Load(e.ctx, r); !errors.Is(err, task.ErrNotReady) {
				t.Fatal(err)
			}
			done, err := s.Complete(e.ctx, r)
			if err != nil || !done.NewlyCompleted {
				t.Fatal(done, err)
			}
			if again, err := s.Complete(e.ctx, r); err != nil || again.NewlyCompleted || again.Checkpoint != done.Checkpoint {
				t.Fatal(again, err)
			}
			loaded, err := s.Load(e.ctx, r)
			if err != nil || string(loaded.Result.Payload) != string(testStageBody) {
				t.Fatal(err)
			}
			b, err := json.Marshal(loaded)
			if err != nil || strings.Contains(string(b), "隔离测试数据") {
				t.Fatal("JSON 泄漏隔离正文", err)
			}
			reads, err := service.NewAITaskReadService(repo)
			if err != nil {
				t.Fatal(err)
			}
			progress, err := reads.Progress(e.ctx, 1, r.TaskID)
			if err != nil || progress.CurrentStep != 1 || progress.Status != task.Running || progress.ReportAvailable {
				t.Fatal(progress, err)
			}
			policies := testStagePolicies()
			p := policies["extract"]
			p.Digest = strings.Repeat("d", 64)
			policies["extract"] = p
			changed, err := service.NewAITaskStageService(repo, policies)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := changed.Load(e.ctx, r); !errors.Is(err, task.ErrStagePolicy) {
				t.Fatal("变化策略复用旧结果", err)
			}
			r.StepNo = 2
			r.OperationKey = "stage_second_operation"
			r.InputDigest = strings.Repeat("b", 64)
			budget, err := service.NewAITaskBudgetService(repo)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := budget.Reserve(e.ctx, task.ReserveRequest{UserID: 1, TaskID: r.TaskID, Fence: r.Fence, Owner: r.Owner, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Capture(e.ctx, r, testStageBody); err != nil {
				t.Fatal(err)
			}
			if got, err := s.Complete(e.ctx, r); err != nil || !got.NewlyCompleted {
				t.Fatal(got, err)
			}
			if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 2 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 2 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
				t.Fatal("步骤/事件/积分不一致")
			}
		})
	}
}

func TestIntegrationAITaskStageConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, _, _, s, r := runningStageTask(t, level)
			db.SetMaxOpenConns(12)
			var fresh atomic.Int32
			var wg sync.WaitGroup
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					got, err := s.Capture(e.ctx, r, testStageBody)
					if err != nil {
						t.Error(err)
					} else if got.NewlyCaptured {
						fresh.Add(1)
					}
				}()
			}
			wg.Wait()
			if fresh.Load() != 1 {
				t.Fatal("重复捕获", fresh.Load())
			}
			fresh.Store(0)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					got, err := s.Complete(e.ctx, r)
					if err != nil {
						t.Error(err)
					} else if got.NewlyCompleted {
						fresh.Add(1)
					}
				}()
			}
			wg.Wait()
			if fresh.Load() != 1 || e.queryInt("SELECT current_step FROM tm_ai_tasks") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 1 || e.queryInt("SELECT reservation_count FROM tm_ai_task_budgets") != 1 {
				t.Fatal("重复阶段推进", fresh.Load())
			}
		})
	}
}

func TestIntegrationAITaskStageIndependentTasks(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
			if _, err := ledger.Grant(e.ctx, pointGrant(now, 1000)); err != nil {
				t.Fatal(err)
			}
			budget, err := service.NewAITaskBudgetService(repo)
			if err != nil {
				t.Fatal(err)
			}
			s, err := service.NewAITaskStageService(repo, testStagePolicies())
			if err != nil {
				t.Fatal(err)
			}
			var requests []task.StageRequest
			for i := 0; i < 40; i++ {
				created, err := tasks.Create(e.ctx, taskRequest(200+i), points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 2, MaxCalls: 2, MaxTokens: 200, MaxPages: 1, MaxDurationMS: 120000})
				if err != nil {
					t.Fatal(err)
				}
				v, err := tasks.Claim(e.ctx, 1, created.TaskID, "worker_stage_00001", time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				r := task.StageRequest{UserID: 1, TaskID: v.ID, Fence: v.Fence, Owner: v.LeaseOwner, OperationKey: "stage_independent_01", Stage: "extract", InputDigest: strings.Repeat("a", 64), StepNo: 1}
				if _, err := budget.Reserve(e.ctx, task.ReserveRequest{UserID: 1, TaskID: r.TaskID, Fence: r.Fence, Owner: r.Owner, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
					t.Fatal(err)
				}
				requests = append(requests, r)
			}
			db.SetMaxOpenConns(12)
			var wg sync.WaitGroup
			for _, r := range requests {
				wg.Add(1)
				go func(r task.StageRequest) {
					defer wg.Done()
					if _, err := s.Capture(e.ctx, r, testStageBody); err != nil {
						t.Error(err)
						return
					}
					if _, err := s.Complete(e.ctx, r); err != nil {
						t.Error(err)
					}
				}(r)
			}
			wg.Wait()
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 40 || e.queryInt("SELECT SUM(current_step) FROM tm_ai_tasks") != 40 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 40 {
				t.Fatal("独立任务阶段丢失")
			}
		})
	}
}

type stageStoreFunc func(context.Context, func(task.StageTx) error) error

func (f stageStoreFunc) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	return f(ctx, fn)
}

func TestIntegrationAITaskStageRecoveryAndRollback(t *testing.T) {
	t.Run("提交未知", func(t *testing.T) {
		e, _, repo, _, s, r := runningStageTask(t, sql.LevelReadCommitted)
		unknown, err := service.NewAITaskStageService(stageStoreFunc(func(ctx context.Context, fn func(task.StageTx) error) error {
			if err := repo.WithinStageTx(ctx, fn); err != nil {
				return err
			}
			return context.DeadlineExceeded
		}), testStagePolicies())
		if err != nil {
			t.Fatal(err)
		}
		if got, err := unknown.Capture(e.ctx, r, testStageBody); !errors.Is(err, context.DeadlineExceeded) || got != (task.CaptureReceipt{}) {
			t.Fatal(got, err)
		}
		if got, err := s.Capture(e.ctx, r, testStageBody); err != nil || got.NewlyCaptured {
			t.Fatal(got, err)
		}
		calls := 0
		unknown, err = service.NewAITaskStageService(stageStoreFunc(func(ctx context.Context, fn func(task.StageTx) error) error {
			calls++
			if err := repo.WithinStageTx(ctx, fn); err != nil {
				return err
			}
			if calls == 2 {
				return context.DeadlineExceeded
			}
			return nil
		}), testStagePolicies())
		if err != nil {
			t.Fatal(err)
		}
		if got, err := unknown.Complete(e.ctx, r); !errors.Is(err, context.DeadlineExceeded) || got != (task.CompletedStage{}) {
			t.Fatal(got, err)
		}
		if got, err := s.Complete(e.ctx, r); err != nil || got.NewlyCompleted {
			t.Fatal(got, err)
		}
	})
	t.Run("事件故障原子回滚", func(t *testing.T) {
		e, _, _, _, s, r := runningStageTask(t, sql.LevelReadCommitted)
		if _, err := s.Capture(e.ctx, r, testStageBody); err != nil {
			t.Fatal(err)
		}
		e.mustExec(`CREATE TRIGGER test_stage_event_fail BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='step_completed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test stage event failure'; END IF; END`)
		if got, err := s.Complete(e.ctx, r); err == nil || got != (task.CompletedStage{}) {
			t.Fatal(got, err)
		}
		if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 1 {
			t.Fatal("事件故障未回滚步骤与进度")
		}
		e.mustExec(`DROP TRIGGER test_stage_event_fail`)
		if got, err := s.Complete(e.ctx, r); err != nil || !got.NewlyCompleted {
			t.Fatal(got, err)
		}
	})
	t.Run("父RR旧快照", func(t *testing.T) {
		e, db, repo, _, s, r := runningStageTask(t, sql.LevelReadCommitted)
		tx, err := db.BeginTx(e.ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		var n int
		if err := tx.QueryRowContext(e.ctx, `SELECT COUNT(*) FROM tm_ai_task_step_results`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Capture(e.ctx, r, testStageBody); err != nil {
			t.Fatal(err)
		}
		calls := 0
		parent, err := service.NewAITaskStageService(stageStoreFunc(func(ctx context.Context, fn func(task.StageTx) error) error {
			calls++
			if calls == 1 {
				return repo.WithinStageTx(ctx, fn)
			}
			if err := fn(repository.AITaskTx(tx).(task.StageTx)); err != nil {
				return err
			}
			return tx.Commit()
		}), testStagePolicies())
		if err != nil {
			t.Fatal(err)
		}
		if got, err := parent.Complete(e.ctx, r); err != nil || !got.NewlyCompleted {
			t.Fatal("旧快照漏读隔离响应", got, err)
		}
	})
}

func TestIntegrationAITaskStageValidationBoundaries(t *testing.T) {
	for _, kind := range []string{"拒绝", "取消", "接管"} {
		t.Run(kind, func(t *testing.T) {
			e, _, repo, tasks, normal, r := runningStageTask(t, sql.LevelReadCommitted)
			if _, err := normal.Capture(e.ctx, r, testStageBody); err != nil {
				t.Fatal(err)
			}
			policies := testStagePolicies()
			p := policies["extract"]
			var claimed task.Task
			p.Check = func(ctx context.Context, _ task.StageValidationInput) error {
				switch kind {
				case "拒绝":
					return errors.New("模拟敏感校验细节")
				case "取消":
					_, err := tasks.Cancel(ctx, 1, r.TaskID)
					return err
				default:
					queued, err := tasks.FailOrRetry(ctx, 1, r.TaskID, r.Owner, r.Fence, "provider_unavailable", time.Millisecond)
					if err != nil {
						return err
					}
					waitTaskTime(t, e, queued.NotBefore)
					claimed, err = tasks.Claim(ctx, 1, r.TaskID, "worker_stage_00002", time.Minute)
					return err
				}
			}
			policies["extract"] = p
			s, err := service.NewAITaskStageService(repo, policies)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.Complete(e.ctx, r)
			if err == nil || got != (task.CompletedStage{}) || strings.Contains(err.Error(), "敏感") {
				t.Fatal(got, err)
			}
			if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 {
				t.Fatal("校验竞态推进了步骤")
			}
			if kind == "接管" {
				r.Fence, r.Owner = claimed.Fence, claimed.LeaseOwner
				if got, err := normal.Complete(e.ctx, r); err != nil || !got.NewlyCompleted || got.Checkpoint.Fence != claimed.Fence {
					t.Fatal("新租约无法复用响应", got, err)
				}
			}
		})
	}
}

func TestIntegrationAITaskStageSchemaCorruption(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"响应绑定", `DROP TRIGGER tm_ai_result_insert`}, {"响应可改", `DROP TRIGGER tm_ai_result_no_update`}, {"响应可删", `DROP TRIGGER tm_ai_result_no_delete`},
		{"检查点绑定", `DROP TRIGGER tm_ai_step_insert`}, {"进度未增长", `DROP TRIGGER tm_ai_step_account`}, {"检查点可改", `DROP TRIGGER tm_ai_step_no_update`}, {"检查点可删", `DROP TRIGGER tm_ai_step_no_delete`},
		{"初始步骤", `DROP TRIGGER tm_ai_task_stage_init`}, {"无检查点推进", `DROP TRIGGER tm_ai_task_stage_guard`},
		{"响应检查恒真", `ALTER TABLE tm_ai_task_step_results DROP CHECK chk_tm_ai_step_result,ADD CONSTRAINT chk_tm_ai_step_result CHECK(TRUE)`}, {"检查点检查恒真", `ALTER TABLE tm_ai_task_steps DROP CHECK chk_tm_ai_step,ADD CONSTRAINT chk_tm_ai_step CHECK(TRUE)`},
		{"响应预留外键", `ALTER TABLE tm_ai_task_step_results DROP FOREIGN KEY fk_tm_ai_result_reservation`}, {"响应用户外键", `ALTER TABLE tm_ai_task_step_results DROP FOREIGN KEY fk_tm_ai_result_budget`}, {"检查点响应外键", `ALTER TABLE tm_ai_task_steps DROP FOREIGN KEY fk_tm_ai_step_result`},
		{"非二进制摘要", `ALTER TABLE tm_ai_task_step_results MODIFY input_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL`}, {"事件唯一索引", `ALTER TABLE tm_ai_task_steps DROP INDEX uk_tm_ai_step_event`},
		{"响应类型", `ALTER TABLE tm_ai_task_step_results MODIFY payload LONGBLOB NOT NULL`}, {"围栏类型", `ALTER TABLE tm_ai_task_steps MODIFY fence DOUBLE NOT NULL`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, db, _, _, _, _ := runningStageTask(t, sql.LevelReadCommitted)
			v := repository.NewAITaskSchemaVerifier(db)
			if err := v.Verify(e.ctx); err != nil {
				t.Fatal("破坏前基线", err)
			}
			e.mustExec(tc.sql)
			if err := v.Verify(e.ctx); err == nil {
				t.Fatal("阶段保护破坏未检出")
			}
		})
	}
}
