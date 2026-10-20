//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅测试：原终态回执实际捕获提交后丢失回执，不改数据库的确认结果。
type terminalCaptureUnknownStore struct {
	*repository.AITaskRepo
	e    *submissionHTTPEnv
	on   bool
	once atomic.Bool
}

func (s *terminalCaptureUnknownStore) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	if err := s.AITaskRepo.WithinStageTx(ctx, fn); err != nil {
		return err
	}
	var count int
	if s.on {
		if err := s.e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=1`, s.e.v.ID).Scan(&count); err != nil {
			return err
		}
	}
	if count == 1 && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return nil
}

// HTTP新建/冻结→真实CN适配器与明确测试服务→封闭终态回执→失败释放。
// 上游失败不是模型判定通过；不访问选择/蕴含/语义模型或生成报告。
func TestIntegrationAIReportUpstreamTerminal(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"failed", "cancelled", "capture", "event", "points", "commit_unknown", "old_fence"} {
				t.Run(mode, func(t *testing.T) {
					serverMode := "agent_failed"
					if mode == "cancelled" {
						serverMode = "agent_cancelled"
					}
					e := newSubmissionHTTPEnv(t, level, serverMode)
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID)
					if mode == "old_fence" {
						// 先让原预算及远端终态确认，再真实等待原租约失效；不能补造旧围栏回执。
						current, err := e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, 3*time.Second)
						if err != nil {
							t.Fatal(err)
						}
						reserveAttemptCall(t, e, current, false)
						p := e.planner.p.Agent
						if _, err := e.agent.Submit(e.ctx, p.ActorID, agents.Request{RequestID: p.RequestID(), Symbol: p.Symbol}); err != nil {
							t.Fatal(err)
						}
						if r, err := e.agent.Status(e.ctx, p.ActorID, p.RequestID()); err != nil || r.State != agents.Failed {
							t.Fatal("测试远端失败未持久确认", err)
						}
						if current.LeaseUntil == nil {
							t.Fatal("缺少原实际租约")
						}
						waitTaskTime(t, e.pointsTestEnv, *current.LeaseUntil)
					}
					store := &terminalCaptureUnknownStore{AITaskRepo: e.repo, e: e, on: mode == "capture"}
					if mode == "commit_unknown" {
						core, err := service.NewAITaskService(&failureCommitUnknownStore{AITaskRepo: e.repo, e: e}, e.ledger)
						if err != nil {
							t.Fatal(err)
						}
						e.tasks = core
					}
					switch mode {
					case "event":
						e.mustExec(`CREATE TRIGGER test_terminal_event BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='failed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only terminal event'; END IF; END`)
					case "points":
						e.mustExec(`CREATE TRIGGER test_terminal_points BEFORE INSERT ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only terminal points'`)
					}
					w, err := service.NewAIL1ReportWorker(e.tasks, store, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := w.Run(e.ctx, 2, e.v.ID); err == nil {
						t.Fatal("跨会员失败任务可执行")
					}
					out, err := w.Run(e.ctx, 1, e.v.ID)
					fault := mode == "capture" || mode == "event" || mode == "points" || mode == "commit_unknown"
					if fault {
						if err == nil || out != (service.AIL1ReportReceipt{}) {
							t.Fatal("故障或提交未知返回候选终态", out, err)
						}
						if mode != "commit_unknown" && (e.queryInt(`SELECT status FROM tm_ai_tasks WHERE id=?`, e.v.ID) != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0) {
							t.Fatal("未确认或回滚却终结释放")
						}
						if mode == "event" {
							e.mustExec(`DROP TRIGGER test_terminal_event`)
						} else if mode == "points" {
							e.mustExec(`DROP TRIGGER test_terminal_points`)
						}
					} else if err != nil {
						t.Fatal(out, err)
					} else if mode != "old_fence" && (out.Task.TaskStatus != task.Failed || out.Task.Outcome != "terminal" || out.Task.ReportAvailable) {
						t.Fatal("确认失败的首次执行没有闭环", out)
					}
					// 原任务重放只查询已确认的Agent记录，不重新提交、不补发模型。
					out, err = w.Run(e.ctx, 1, e.v.ID)
					if err != nil || e.submits.Load() != 1 || e.server.Count() != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 {
						t.Fatal("上游失败恢复重复调用或生成报告", out, err)
					}
					if mode == "old_fence" {
						if out.Task.Outcome != "response_unknown" || out.Task.TaskStatus != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=?`, e.v.ID) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 0 {
							t.Fatal("旧围栏缺回执被补造或退款", out)
						}
					} else {
						if out.Task.TaskStatus != task.Failed || out.Task.ReportAvailable || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_steps WHERE task_id=?`, e.v.ID) != 1 {
							t.Fatal("已确认上游失败没有恰好一次释放", out)
						}
						var payload []byte
						if err := e.db.QueryRowContext(e.ctx, `SELECT payload FROM tm_ai_task_step_results WHERE task_id=? AND step_no=1`, e.v.ID).Scan(&payload); err != nil || strings.Contains(string(payload), "untrusted-marker") || !strings.Contains(string(payload), `"State":"upstream_`) {
							t.Fatal("终态回执缺失或混入上游研究正文", err)
						}
					}
					e.mustRequest(t, "GET", "/api/v1/ai/tasks/"+jsonNumber(e.v.ID)+"/report", "", "", token, 422)
					if got := decodeCreation(t, e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)); got.TaskID != e.v.ID {
						t.Fatal("失败后原创建收据无法核查")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
