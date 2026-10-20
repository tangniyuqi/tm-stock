//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func memberQueue(t *testing.T, e *submissionHTTPEnv, w *service.AIL1ReportWorker, enabled bool) *service.AIL1ReportQueue {
	t.Helper()
	q, err := service.NewAIL1ReportQueue(w, e.repo, enabled)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// 测试服务/批准/来源均显著为示例；队列只接收扫描位置，不接任务编号、金额或模型判定。
func TestIntegrationAIReportQueueLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"success", "reject", "cancel", "expired"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, mode)
					if mode == "expired" {
						e.ctl.p.Budget.MaxDurationMS = 1000
					}
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID) // 仅重新绑定回环测试规划器，不向队列传编号。
					q := memberQueue(t, e, admissionWorker(t, e), true)
					if mode == "cancel" {
						if _, err := e.tasks.Cancel(e.ctx, 1, created.TaskID); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "expired" {
						timer := time.NewTimer(time.Until(e.v.Deadline) + 10*time.Millisecond)
						defer timer.Stop()
						select {
						case <-timer.C:
						case <-e.ctx.Done():
							t.Fatal(e.ctx.Err())
						}
					}
					first, err := q.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 1})
					if err != nil || first.Complete || first.Visited != 1 || first.Skipped != 1 || e.submits.Load() != 0 {
						t.Fatal("旧内部任务没有被跳过", first, err)
					}
					out, err := q.RunBatch(e.ctx, task.MemberQueueOptions{AfterID: first.LastID, MaxTasks: 1})
					if err != nil || !out.Complete || out.Visited != 1 || out.LastID != created.TaskID {
						t.Fatal(out, err)
					}
					want, calls := task.Succeeded, 3
					switch mode {
					case "reject":
						want, calls = task.Failed, 2
					case "cancel":
						want, calls = task.Cancelled, 0
					case "expired":
						want, calls = task.Expired, 0
					}
					v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
					if err != nil || v.Status != want || len(e.server.Requests()) != calls || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
						t.Fatal("队列未闭合原状态/账务", out, err)
					}
					if mode == "cancel" {
						if out.Skipped != 1 || out.TerminalObserved != 0 {
							t.Fatal(out)
						}
					} else if out.TerminalObserved != 1 || len(out.Items) != 1 || out.Items[0].Status != want {
						t.Fatal(out)
					}
					path := "/api/v1/ai/tasks/" + jsonNumber(created.TaskID) + "/report"
					status := 422
					if want == task.Succeeded {
						status = 200
					}
					body := e.mustRequest(t, "GET", path, "", "", token, status)
					if status == 200 {
						assertPublicReport(t, body, e.reportEnv)
					}
					e.mustRequest(t, "GET", path, "", "", e.web.token(t, 2, "client"), 404)
					txns := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
					if replay, err := q.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 100}); err != nil || !replay.Complete || replay.TerminalObserved != 0 || replay.Skipped != 2 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != txns || len(e.server.Requests()) != calls {
						t.Fatal("重扫补调用或写账", replay, err)
					}
					settles := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`)
					if (want == task.Succeeded && settles != 1) || (want != task.Succeeded && settles != 0) {
						t.Fatal("结算数量错误", settles)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportQueueGuards(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"off", "withdrawn", "missing", "storage", "active", "context"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, "")
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID)
					q := memberQueue(t, e, admissionWorker(t, e), mode != "off")
					ctx := e.ctx
					switch mode {
					case "withdrawn":
						if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
							t.Fatal(err)
						}
					case "missing", "storage":
						if err := breakTestAdmission(e.ctx, e, mode); err != nil {
							t.Fatal(err)
						}
					case "active":
						if _, err := e.tasks.Claim(e.ctx, 1, created.TaskID, e.owner, time.Minute); err != nil {
							t.Fatal(err)
						}
					case "context":
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(e.ctx)
						cancel()
					}
					out, err := q.RunBatch(ctx, task.MemberQueueOptions{MaxTasks: 100})
					if mode == "active" {
						if err != nil || !out.Complete || out.Skipped != 2 {
							t.Fatal(out, err)
						}
					} else if mode == "withdrawn" {
						if err != nil || !out.Complete || out.TerminalObserved != 1 || len(out.Items) != 1 || out.Items[0].Status != task.Failed {
							t.Fatal("来源撤回仍阻塞队列或没有失败释放", out, err)
						}
					} else {
						if err == nil || out.Complete || out.TerminalObserved != 0 || len(out.Items) != 0 {
							t.Fatal("缺依赖仍放行", out, err)
						}
						if mode == "off" && !errors.Is(err, finalreport.ErrDisabled) {
							t.Fatal(err)
						}
						if mode == "missing" || mode == "storage" {
							if out.Visited != 1 || out.LastID != created.TaskID-1 {
								t.Fatal("故障跨过原行游标", out)
							}
						}
					}
					wantFrozen := 7
					if mode == "withdrawn" {
						wantFrozen = 0
					}
					if e.submits.Load() != 0 || len(e.server.Requests()) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != wantFrozen {
						t.Fatal("拒绝路径发生调用/交付/资金变化")
					}
					if mode == "storage" {
						e.mustExec(`RENAME TABLE test_only_delivery_admission_hidden TO tm_ai_task_admissions`)
					}
					if mode != "active" {
						if _, err := e.tasks.Cancel(e.ctx, 1, created.TaskID); err != nil {
							t.Fatal(err)
						}
						if e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
							t.Fatal("故障后不能取消释放")
						}
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportQueueUnknownCommit(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "")
			token := e.web.token(t, 1, "client")
			created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
			e.bindNewTask(t, created.TaskID)
			e.workflowEnv.store.phase = "final"
			w, err := service.NewAIL1ReportWorker(e.tasks, e.workflowEnv.store, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			q := memberQueue(t, e, w, true)
			out, err := q.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 100})
			if !errors.Is(err, errCheckTest) || out.Complete || out.LastID != created.TaskID-1 || out.Visited != 1 || out.TerminalObserved != 0 || len(out.Items) != 0 {
				t.Fatal("未知结算提交冒充交付或跨游标", out, err)
			}
			if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 {
				t.Fatal("未命中真实提交后未知边界")
			}
			again, err := q.RunBatch(e.ctx, task.MemberQueueOptions{AfterID: out.LastID, MaxTasks: 100})
			if err != nil || !again.Complete || again.Skipped != 1 || again.TerminalObserved != 0 || e.submits.Load() != 1 || len(e.server.Requests()) != 3 {
				t.Fatal("恢复重发或伪造新交付", again, err)
			}
			assertPublicReport(t, e.mustRequest(t, "GET", "/api/v1/ai/tasks/"+jsonNumber(created.TaskID)+"/report", "", "", token, 200), e.reportEnv)
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

func TestIntegrationAIReportQueueConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"same_instance", "independent_workers"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, "")
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID)
					q := memberQueue(t, e, admissionWorker(t, e), true)
					var wg sync.WaitGroup
					results := make(chan error, 12)
					for i := 0; i < 12; i++ {
						currentQueue := q
						if mode == "independent_workers" {
							worker, err := service.NewAIL1ReportWorker(e.tasks, e.repo, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, "test_queue_worker_"+strconv.Itoa(i), agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
							if err != nil {
								t.Fatal(err)
							}
							currentQueue = memberQueue(t, e, worker, true)
						}
						wg.Add(1)
						go func() {
							defer wg.Done()
							_, err := currentQueue.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 100})
							results <- err
						}()
					}
					wg.Wait()
					close(results)
					for err := range results {
						if err != nil && !errors.Is(err, task.ErrNotReady) {
							t.Fatal(err)
						}
					}
					out, err := q.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 100})
					if err != nil || !out.Complete || out.Skipped != 2 || e.submits.Load() != 1 || len(e.server.Requests()) != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 {
						t.Fatal("队列竞争未保持单次调用/结算", out, err)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
