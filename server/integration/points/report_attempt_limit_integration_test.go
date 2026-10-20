//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 只有 Capture，不生成检查点：证明“响应已知”与“报告通过”是不同的门禁。
func reserveAttemptCall(t *testing.T, e *submissionHTTPEnv, current task.Task, captured bool) {
	t.Helper()
	plan := e.planner.p.Agent
	request := task.ReserveRequest{UserID: 1, TaskID: current.ID, Owner: e.owner, Fence: current.Fence, Key: plan.OperationKey, InputDigest: plan.InputDigest(current), Units: plan.Units}
	budget, err := service.NewAITaskBudgetService(e.repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := budget.ReserveStep(e.ctx, request, 1); err != nil {
		t.Fatal(err)
	}
	if !captured {
		return
	}
	stages, err := service.NewAITaskStageService(e.repo, map[string]task.StagePolicy{"agent_call": {Digest: task.StageDigest([]byte("test-only-captured-response")), Check: func(ctx context.Context, _ task.StageValidationInput) error { return ctx.Err() }}})
	if err != nil {
		t.Fatal(err)
	}
	r := task.StageRequest{UserID: 1, TaskID: current.ID, Owner: e.owner, Fence: current.Fence, Stage: "agent_call", StepNo: 1, OperationKey: request.Key, InputDigest: request.InputDigest}
	if _, err := stages.Capture(e.ctx, r, []byte(`{"test_only":"已持久确认的测试响应，不是模型报告"}`)); err != nil {
		t.Fatal(err)
	}
}

// 真实任务/预算/隔离表证明：正常恢复不能用尝试上限覆盖未知调用。
// 仅用已标注的测试来源和响应；不调用模拟模型，更不发起真实调用。
func TestIntegrationAIReportAttemptLimit(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"unknown", "captured", "empty"} {
				t.Run(mode, func(t *testing.T) {
					e := newRejectionEnv(t, level)
					// 此处直接验证已有内部任务API；不改变L1 worker至少一分钟的租约约束。
					current, err := e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, 3*time.Second)
					if err != nil {
						t.Fatal(err)
					}
					if mode != "empty" {
						reserveAttemptCall(t, e, current, mode == "captured")
					}
					for {
						if current.LeaseUntil == nil {
							t.Fatal("缺少实际租约")
						}
						wait := time.Until(*current.LeaseUntil) + 100*time.Millisecond
						if wait < 0 {
							wait = 100 * time.Millisecond
						}
						select {
						case <-e.ctx.Done():
							t.Fatal("真实租约等待超时", e.ctx.Err())
						case <-time.After(wait):
						}
						if current.Attempt == current.Budget.MaxAttempts {
							break
						}
						current, err = e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, time.Second)
						if err != nil {
							t.Fatal(err)
						}
					}
					before, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
					if err != nil {
						t.Fatal(err)
					}
					w := admissionWorker(t, e)
					if _, err := w.Run(e.ctx, 2, e.v.ID); err == nil {
						t.Fatal("他人任务被恢复")
					}
					out, err := w.Run(e.ctx, 1, e.v.ID)
					if err != nil {
						t.Fatal(out, err)
					}
					if mode == "unknown" {
						after, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
						if err != nil || !reflect.DeepEqual(after, before) || out.Task.Outcome != "response_unknown" || out.Task.TaskStatus != task.Running || out.Task.ReportAvailable || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0 {
							t.Fatal("正常恢复终结或重写未知调用现场", out, err)
						}
						if again, err := w.Run(e.ctx, 1, e.v.ID); err != nil || again != out {
							t.Fatal("未知恢复重放不稳定", again, err)
						}
					} else if out.Task.TaskStatus != task.Failed || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 1 {
						t.Fatal("已确认或无调用任务未正常终结", out)
					}
					if e.submits.Load() != 0 || e.server.Count() != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 {
						t.Fatal("恢复补调用或伪造报告结算")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportUnknownFailure(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, reason := range []string{"failed", "retry_exhausted", "retry_past_deadline"} {
				for _, mode := range []string{"unknown", "captured"} {
					t.Run(reason+"_"+mode, func(t *testing.T) {
						e := newRejectionEnv(t, level)
						lease := time.Minute
						if reason == "retry_exhausted" {
							lease = time.Second
						}
						current, err := e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, lease)
						if err != nil {
							t.Fatal(err)
						}
						if reason == "retry_exhausted" {
							for current.Attempt < current.Budget.MaxAttempts {
								if current.LeaseUntil == nil {
									t.Fatal("缺少真实租约")
								}
								waitTaskTime(t, e.pointsTestEnv, *current.LeaseUntil)
								if current.Attempt+1 == current.Budget.MaxAttempts {
									lease = time.Minute
								}
								current, err = e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, lease)
								if err != nil {
									t.Fatal(err)
								}
							}
						}
						reserveAttemptCall(t, e, current, mode == "captured")
						backoff, code := time.Duration(0), "provider_unavailable"
						if reason == "retry_exhausted" {
							backoff, code = time.Second, "attempts_exhausted"
						}
						if reason == "retry_past_deadline" {
							backoff, code = time.Hour, "budget_exhausted"
						}
						before, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
						if err != nil {
							t.Fatal(err)
						}
						// 同一有效围栏同时失败；未知场景全部拒绝，已知场景恰好一次释放。
						var wg sync.WaitGroup
						var successes atomic.Int32
						errs := make(chan error, 8)
						for i := 0; i < 8; i++ {
							wg.Add(1)
							go func() {
								defer wg.Done()
								out, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, e.owner, current.Fence, "provider_unavailable", backoff)
								if mode == "unknown" {
									if !errors.Is(err, task.ErrUnconfirmed) || out != (task.Task{}) {
										errs <- points.ErrInvariant
									}
								} else if err == nil && out.Status == task.Failed && out.ErrorCode == code {
									successes.Add(1)
								} else if !errors.Is(err, task.ErrLease) {
									errs <- points.ErrInvariant
								}
							}()
						}
						wg.Wait()
						close(errs)
						for err := range errs {
							t.Fatal("并发失败门禁错误", err)
						}
						after, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
						if err != nil {
							t.Fatal(err)
						}
						if mode == "unknown" {
							if successes.Load() != 0 || !reflect.DeepEqual(before, after) || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0 {
								t.Fatal("未知调用失败改写任务或释放")
							}
						} else if successes.Load() != 1 || after.Status != task.Failed || after.ErrorCode != code || after.EventSeq != before.EventSeq+1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 1 {
							t.Fatal("已知响应失败未恰好一次终结释放")
						}
						if e.submits.Load() != 0 || e.server.Count() != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 {
							t.Fatal("失败入口产生模型调用或报告结算")
						}
						requirePointAudit(t, e.pointsTestEnv, e.ledger)
					})
				}
			}
		})
	}
}

// 仅测试：真实失败事务已经提交之后丢失回执；不回滚或伪造已提交数据。
type failureCommitUnknownStore struct {
	*repository.AITaskRepo
	e    *submissionHTTPEnv
	once atomic.Bool
}

func (s *failureCommitUnknownStore) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	if err := s.AITaskRepo.WithinTaskTx(ctx, fn); err != nil {
		return err
	}
	var status int
	if err := s.e.db.QueryRowContext(ctx, `SELECT status FROM tm_ai_tasks WHERE id=?`, s.e.v.ID).Scan(&status); err != nil {
		return err
	}
	if status == task.Failed && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return nil
}

func TestIntegrationAIReportUnknownFailureAtomicity(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"event", "points", "commit_unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := newRejectionEnv(t, level)
					current, err := e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, time.Minute)
					if err != nil {
						t.Fatal(err)
					}
					reserveAttemptCall(t, e, current, true)
					store := &failureCommitUnknownStore{AITaskRepo: e.repo, e: e}
					if mode == "event" {
						e.mustExec(`CREATE TRIGGER test_call_failure_event BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='failed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only call failure event'; END IF; END`)
					} else if mode == "points" {
						e.mustExec(`CREATE TRIGGER test_call_failure_points BEFORE INSERT ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only call failure points'`)
					}
					core, err := service.NewAITaskService(store, e.ledger)
					if err != nil {
						t.Fatal(err)
					}
					out, err := core.FailOrRetry(e.ctx, 1, e.v.ID, e.owner, current.Fence, "provider_unavailable", 0)
					if err == nil || out != (task.Task{}) {
						t.Fatal("失败或提交未知仍返回成功任务", out, err)
					}
					if mode != "commit_unknown" {
						after, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
						if err != nil || !reflect.DeepEqual(current, after) || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0 {
							t.Fatal("确认后失败释放未整体回滚", err)
						}
						store.once.Store(true)
						if mode == "event" {
							e.mustExec(`DROP TRIGGER test_call_failure_event`)
						} else {
							e.mustExec(`DROP TRIGGER test_call_failure_points`)
						}
						if _, err := core.FailOrRetry(e.ctx, 1, e.v.ID, e.owner, current.Fence, "provider_unavailable", 0); err != nil {
							t.Fatal(err)
						}
					}
					after, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
					if err != nil || after.Status != task.Failed || after.EventSeq != current.EventSeq+1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 1 {
						t.Fatal("原任务核查未确认恰好一次终结", err)
					}
					if e.submits.Load() != 0 || e.server.Count() != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 {
						t.Fatal("失败恢复伪造报告或调用")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
