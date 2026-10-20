//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅测试：在第一步已经持久完成、第二步尚未预留时撤回示例来源。
type rejectionSelectionControl struct {
	workflowSelectionControl
	e    *submissionHTTPEnv
	once atomic.Bool
}

func (c *rejectionSelectionControl) Recheck(ctx context.Context, v task.Task, p *finalreport.SelectionModelPlan) error {
	if c.once.CompareAndSwap(false, true) {
		if err := repository.NewAISourceRepo(c.e.db).SetSourceEnabled(ctx, c.e.source.ID, false); err != nil {
			return err
		}
	}
	return c.workflowSelectionControl.Recheck(ctx, v, p)
}

type rejectionUnknownStore struct {
	*repository.AITaskRepo
	e    *submissionHTTPEnv
	once atomic.Bool
}

func (s *rejectionUnknownStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	err := s.AITaskRepo.WithinReportTx(ctx, fn)
	if err != nil {
		return err
	}
	var status int
	if err := s.e.db.QueryRowContext(ctx, `SELECT status FROM tm_ai_tasks WHERE id=?`, s.e.v.ID).Scan(&status); err != nil {
		return err
	}
	if status == task.Failed && s.once.CompareAndSwap(false, true) {
		return errCheckTest // 真实COMMIT后才模拟丢失回执，不重写数据库结果。
	}
	return nil
}

func rejectionWorker(t *testing.T, e *submissionHTTPEnv, store interface {
	task.Store
	task.BudgetStore
	task.StageStore
	task.AdmissionStore
	finalreport.Store
}, selection service.AIReportSelectionControl) *service.AIL1ReportWorker {
	t.Helper()
	w, err := service.NewAIL1ReportWorker(e.tasks, store, e.ledger, e.agent, e.payloads, e.model, e.planner, selection, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func newRejectionEnv(t *testing.T, level sql.IsolationLevel) *submissionHTTPEnv {
	t.Helper()
	e := newSubmissionHTTPEnv(t, level, "")
	token := e.web.token(t, 1, "client")
	created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
	e.bindNewTask(t, created.TaskID)
	return e
}

func assertRejectedTask(t *testing.T, e *submissionHTTPEnv) {
	t.Helper()
	if e.queryInt(`SELECT status FROM tm_ai_tasks WHERE id=?`, e.v.ID) != task.Failed || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_events WHERE task_id=? AND type='failed' AND error_code='guard_rejected'`, e.v.ID) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 {
		t.Fatal("已知拒绝没有原子失败/释放，或伪造报告")
	}
	token := e.web.token(t, 1, "client")
	e.mustRequest(t, "GET", "/api/v1/ai/tasks/"+jsonNumber(e.v.ID)+"/report", "", "", token, 422)
	if got := decodeCreation(t, e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)); got.TaskID != e.v.ID {
		t.Fatal("失败后原创建键不能核查原收据")
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
}

func TestIntegrationAIReportRejectionLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"before", "completed_prefix", "unknown", "unknown_exhausted", "concurrent"} {
				t.Run(mode, func(t *testing.T) {
					e := newRejectionEnv(t, level)
					selection := service.AIReportSelectionControl(workflowSelectionControl{e.control, e.planner})
					if mode == "completed_prefix" {
						selection = &rejectionSelectionControl{workflowSelectionControl: workflowSelectionControl{e.control, e.planner}, e: e}
					}
					store := e.workflowEnv.store
					w := rejectionWorker(t, e, store, selection)
					if mode == "unknown" || mode == "unknown_exhausted" {
						store.phase = "budget"
						if _, err := w.Run(e.ctx, 1, e.v.ID); err == nil || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_reservations WHERE task_id=?`, e.v.ID) != 2 {
							t.Fatal("没有实际制造未完成预留", err)
						}
						if mode == "unknown_exhausted" {
							// 用既有内部租约API真实等待接管，不改表/时钟/生产worker租约。
							for {
								current, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
								if err != nil {
									t.Fatal(err)
								}
								if current.LeaseUntil == nil {
									t.Fatal("测试任务缺少真实租约")
								}
								// 心跳明确禁止缩短既有租约，等实际期限，不以短参数伪造过期。
								wait := time.Until(*current.LeaseUntil) + 100*time.Millisecond
								if wait < 0 {
									wait = 100 * time.Millisecond
								}
								select {
								case <-e.ctx.Done():
									t.Fatal("等待真实租约到期失败", e.ctx.Err())
								case <-time.After(wait):
								}
								if current.Attempt == current.Budget.MaxAttempts {
									break
								}
								if _, err := e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, time.Second); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
					if mode != "completed_prefix" {
						if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "concurrent" {
						var wg sync.WaitGroup
						for i := 0; i < 8; i++ {
							wg.Add(1)
							go func() {
								defer wg.Done()
								out, err := w.Run(e.ctx, 1, e.v.ID)
								if err != nil && !errors.Is(err, task.ErrNotReady) && !errors.Is(err, task.ErrLease) && !errors.Is(err, points.ErrClosed) || err == nil && out.Task.TaskStatus != task.Failed {
									t.Error("竞争拒绝结果异常", out, err)
								}
							}()
						}
						wg.Wait()
					} else {
						out, err := w.Run(e.ctx, 1, e.v.ID)
						if mode == "unknown" || mode == "unknown_exhausted" {
							if err != nil || out.Task.Outcome != "response_unknown" || out.Task.TaskStatus != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_reservations WHERE task_id=?`, e.v.ID) != 2 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0 || e.submits.Load() != 1 || e.server.Count() != 0 {
								t.Fatal("来源拒绝掩盖未知调用或补发", out, err)
							}
							if mode == "unknown_exhausted" && e.queryInt(`SELECT attempt FROM tm_ai_tasks WHERE id=?`, e.v.ID) != e.v.Budget.MaxAttempts {
								t.Fatal("未知预留被再次领取")
							}
							return
						}
						if err != nil || out.Task.TaskStatus != task.Failed || out.Task.Outcome != "terminal" {
							t.Fatal("已知来源拒绝未终结", out, err)
						}
					}
					wantAgent := int32(0)
					if mode == "completed_prefix" {
						wantAgent = 1
					}
					if e.submits.Load() != wantAgent || e.server.Count() != 0 {
						t.Fatal("拒绝后额外外呼")
					}
					assertRejectedTask(t, e)
					if out, err := w.Run(e.ctx, 1, e.v.ID); err != nil || out.Task.TaskStatus != task.Failed {
						t.Fatal("失败重放异常", out, err)
					}
					assertRejectedTask(t, e)
				})
			}
		})
	}
}

func TestIntegrationAIReportRejectionAtomicity(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"event", "points", "unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := newRejectionEnv(t, level)
					if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
						t.Fatal(err)
					}
					store := &rejectionUnknownStore{AITaskRepo: e.repo, e: e}
					switch mode {
					case "event":
						e.mustExec(`CREATE TRIGGER test_rejection_event_abort BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='failed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only rejection event failure'; END IF; END`)
					case "points":
						e.mustExec(`CREATE TRIGGER test_rejection_points_abort BEFORE INSERT ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only rejection points failure'`)
					}
					w := rejectionWorker(t, e, store, workflowSelectionControl{e.control, e.planner})
					out, err := w.Run(e.ctx, 1, e.v.ID)
					if err == nil || out != (service.AIL1ReportReceipt{}) {
						t.Fatal("回滚或未知提交仍返回候选收据", out, err)
					}
					if mode != "unknown" {
						if e.queryInt(`SELECT status FROM tm_ai_tasks WHERE id=?`, e.v.ID) != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_events WHERE task_id=? AND type='failed'`, e.v.ID) != 0 {
							t.Fatal("失败/释放/事件没有整体回滚")
						}
						store.once.Store(true) // 回滚恢复不再额外模拟未知回执。
						if mode == "event" {
							e.mustExec(`DROP TRIGGER test_rejection_event_abort`)
						} else {
							e.mustExec(`DROP TRIGGER test_rejection_points_abort`)
						}
					}
					if out, err := w.Run(e.ctx, 1, e.v.ID); err != nil || out.Task.TaskStatus != task.Failed || e.submits.Load() != 0 || e.server.Count() != 0 {
						t.Fatal("原任务恢复重复调用或未终结", out, err)
					}
					assertRejectedTask(t, e)
				})
			}
		})
	}
}
