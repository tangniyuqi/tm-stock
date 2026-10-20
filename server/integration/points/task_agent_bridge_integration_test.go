//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 测试专用规划器：仅绑定明确的测试题材/股票，不能作为真实授权或远端硬预算实现。
type bridgeTestPlanner struct {
	p       agenttask.Plan
	allowed atomic.Bool
}

func (p *bridgeTestPlanner) Plan(ctx context.Context, v task.Task) (agenttask.Plan, error) {
	return p.p, ctx.Err()
}
func (p *bridgeTestPlanner) Recheck(ctx context.Context, v task.Task, q agenttask.Plan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.allowed.Load() || q != p.p || q.Validate(v) != nil {
		return agents.ErrDisabled
	}
	return nil
}

type bridgeStore interface {
	task.StageStore
	task.BudgetStore
	task.AdmissionStore
}
type bridgeAgent interface {
	Show(context.Context, int64, string) (agents.Record, error)
	Submit(context.Context, int64, agents.Request) (agents.Record, error)
	Status(context.Context, int64, string) (agents.Record, error)
	Result(context.Context, int64, string) (agents.Record, error)
}
type taskAgentBridgeEnv struct {
	*pointsTestEnv
	db                         *sql.DB
	repo                       *repository.AITaskRepo
	tasks                      *service.AITaskService
	ledger                     *service.PointsService
	agents                     *service.AIAgentService
	planner                    *bridgeTestPlanner
	created                    task.Created
	submits, statuses, results atomic.Int32
	unknown                    atomic.Bool
	onSubmit, onStatus         func(context.Context) error
	transportFault             atomic.Bool
}

func newTaskAgentBridgeEnv(t *testing.T, level sql.IsolationLevel) *taskAgentBridgeEnv {
	t.Helper()
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
	reportDDL(t, e, "20261018_create_tm_ai_task_admissions.sql")
	ddl, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261015_create_tm_ai_agent_runs.sql"))
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(string(ddl))
	if _, err := ledger.Grant(e.ctx, pointGrant(now, 100)); err != nil {
		t.Fatal(err)
	}
	b := taskBudget()
	b.MaxDurationMS = 300000
	b.MaxAttempts = 20
	created, err := tasks.Create(e.ctx, taskRequest(1), points.Price{Version: "v1", Points: 7}, b)
	if err != nil {
		t.Fatal(err)
	}
	v, err := tasks.Inspect(e.ctx, 1, created.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	out := &taskAgentBridgeEnv{pointsTestEnv: e, db: db, repo: repo, tasks: tasks, ledger: ledger, created: created}
	bearer := strings.Repeat("x", 40)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+bearer {
			out.transportFault.Store(true)
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/analysis/single":
			out.submits.Add(1)
			var body struct {
				Symbol     string `json:"symbol"`
				Parameters struct {
					Engine string `json:"engine"`
				} `json:"parameters"`
			}
			if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body) != nil || body.Symbol != "600001" || body.Parameters.Engine != "v2" {
				out.transportFault.Store(true)
				w.WriteHeader(400)
				return
			}
			// 独立连接可锁到任务，证明没有持任务/预算事务等待 HTTP。
			check, stop := context.WithTimeout(r.Context(), time.Second)
			_, err := tasks.Inspect(check, 1, created.TaskID)
			stop()
			if err != nil {
				out.transportFault.Store(true)
				w.WriteHeader(500)
				return
			}
			if out.onSubmit != nil {
				if err := out.onSubmit(r.Context()); err != nil {
					w.WriteHeader(500)
					return
				}
			}
			if out.unknown.Load() {
				w.WriteHeader(400)
				io.WriteString(w, `{"success":false}`)
				return
			}
			io.WriteString(w, `{"success":true,"data":{"task_id":"bridge_remote_test","status":"queued"}}`)
		case r.Method == "GET" && r.URL.Path == "/api/analysis/tasks/bridge_remote_test/status":
			out.statuses.Add(1)
			if out.onStatus != nil {
				if err := out.onStatus(r.Context()); err != nil {
					w.WriteHeader(500)
					return
				}
			}
			io.WriteString(w, `{"success":true,"data":{"task_id":"bridge_remote_test","status":"completed"}}`)
		case r.Method == "GET" && r.URL.Path == "/api/analysis/tasks/bridge_remote_test/result":
			out.results.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"analysis_id": "bridge_remote_test", "reports": map[string]string{"news": "未核验的隔离测试正文 " + bearer}, "tokens_used": 0}})
		default:
			out.transportFault.Store(true)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	cfg := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: srv.URL, ApprovedHost: "127.0.0.1", Bearer: bearer, ApprovalRef: "test-only:bridge-approval", LicenseRef: "test-only:bridge-license", QuickModel: "test-quick", DeepModel: "test-deep", Timeout: 6 * time.Second, MaxSubmissions: 10}
	client, err := tradingcn.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	agentRepo := repository.NewAIAgentRepo(db)
	if err := agentRepo.VerifySchema(e.ctx); err != nil {
		t.Fatal("Agent 健康基线", err)
	}
	if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
		t.Fatal("任务健康基线", err)
	}
	out.agents, err = service.NewAIAgentService(agentRepo, client, cfg.Profile())
	if err != nil {
		t.Fatal(err)
	}
	out.planner = &bridgeTestPlanner{p: agenttask.Plan{UserID: 1, TaskID: created.TaskID, TaskNo: v.No, SubjectType: task.Theme, SubjectID: 1, ActorID: 7, OperationKey: "test_bridge_operation", Symbol: "600001", ProfileDigest: cfg.Profile().Digest, Units: task.Units{Calls: 1, Tokens: 100}}}
	out.planner.p.OperationKey = out.planner.p.RequestID()
	out.planner.allowed.Store(true)
	return out
}

func (e *taskAgentBridgeEnv) worker(t *testing.T, store bridgeStore, client bridgeAgent, enabled bool) *service.AITaskAgentWorker {
	t.Helper()
	w, err := service.NewAITaskAgentWorker(e.tasks, store, client, e.planner, "worker_bridge_test_01", agenttask.Options{Enabled: enabled, Lease: task.RecoveryOptions{Lease: 2 * time.Second, Heartbeat: 250 * time.Millisecond, Timeout: 15 * time.Second}, PollInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func (e *taskAgentBridgeEnv) waitLease(t *testing.T) {
	t.Helper()
	v, err := e.tasks.Inspect(e.ctx, 1, e.created.TaskID)
	if err != nil || v.LeaseUntil == nil {
		t.Fatal(v, err)
	}
	waitTaskTime(t, e.pointsTestEnv, *v.LeaseUntil)
}
func (e *taskAgentBridgeEnv) checkFrozen(t *testing.T, step int) {
	t.Helper()
	if e.transportFault.Load() || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 || e.queryInt("SELECT current_step FROM tm_ai_tasks") != step || e.queryInt("SELECT status FROM tm_ai_tasks") != task.Running || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 {
		t.Fatal("接线改变积分、预算或错误推进")
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
}

func TestIntegrationAITaskAgentBridgeLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newTaskAgentBridgeEnv(t, level)
			w := e.worker(t, e.repo, e.agents, true)
			out, err := w.Run(e.ctx, 1, e.created.TaskID)
			if err != nil || out.Outcome != "evidence_pending" || out.ReportAvailable || out.CurrentStep != 1 {
				t.Fatal(out, err)
			}
			e.checkFrozen(t, 1)
			var raw, receipt []byte
			if err := e.db.QueryRowContext(e.ctx, "SELECT result_payload FROM tm_ai_agent_runs").Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), strings.Repeat("x", 40)) || !strings.Contains(string(raw), "[已隐藏]") {
				t.Fatal("隔离结果未脱敏")
			}
			if err := e.db.QueryRowContext(e.ctx, "SELECT payload FROM tm_ai_task_step_results").Scan(&receipt); err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if json.Unmarshal(receipt, &fields) != nil || len(fields) != 4 || strings.Contains(string(receipt), "未核验") || strings.Contains(string(receipt), "tokens_used") {
				t.Fatal("任务回执混入正文或虚构用量")
			}
			e.waitLease(t)
			out, err = w.Run(e.ctx, 1, e.created.TaskID)
			if err != nil || out.Outcome != "evidence_pending" || e.submits.Load() != 1 || e.results.Load() != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 1 {
				t.Fatal("接管重复提交/推进", out, err)
			}
			e.checkFrozen(t, 1)
			before := e.submits.Load()
			e.planner.p.Symbol = "600002"
			e.waitLease(t)
			if _, err := w.Run(e.ctx, 1, e.created.TaskID); !errors.Is(err, points.ErrIdempotency) || e.submits.Load() != before {
				t.Fatal("计划变更规避旧预留", err)
			}
		})
	}
}

type bridgeBudgetUnknown struct {
	bridgeStore
	once bool
}

func (s *bridgeBudgetUnknown) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	if err := s.bridgeStore.WithinBudgetTx(ctx, fn); err != nil {
		return err
	}
	if !s.once {
		s.once = true
		return errors.New("fake committed budget acknowledgement lost")
	}
	return nil
}

type bridgeResultUnknown struct{ bridgeAgent }

func (s bridgeResultUnknown) Result(ctx context.Context, a int64, id string) (agents.Record, error) {
	if _, err := s.bridgeAgent.Result(ctx, a, id); err != nil {
		return agents.Record{}, err
	}
	return agents.Record{}, errors.New("fake committed quarantine acknowledgement lost")
}

func TestIntegrationAITaskAgentBridgeRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"提交未知", "预算回执未知", "阶段事件失败", "旧租约无回执"} {
				t.Run(mode, func(t *testing.T) {
					e := newTaskAgentBridgeEnv(t, level)
					var store bridgeStore = e.repo
					var client bridgeAgent = e.agents
					switch mode {
					case "提交未知":
						e.unknown.Store(true)
					case "预算回执未知":
						store = &bridgeBudgetUnknown{bridgeStore: e.repo}
					case "阶段事件失败":
						e.mustExec("CREATE TRIGGER test_bridge_step_failure BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='step_completed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fake bridge checkpoint failure'; END IF; END")
					case "旧租约无回执":
						client = bridgeResultUnknown{e.agents}
					}
					out, err := e.worker(t, store, client, true).Run(e.ctx, 1, e.created.TaskID)
					if mode == "提交未知" {
						if err != nil || out.Outcome != "response_unknown" {
							t.Fatal(out, err)
						}
					} else if err == nil || out != (task.RecoveryResult{}) {
						t.Fatal("未知/失败返回候选成功", out, err)
					}
					e.checkFrozen(t, 0)
					if mode == "阶段事件失败" {
						if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 {
							t.Fatal("事件失败未回滚检查点")
						}
						e.mustExec("DROP TRIGGER test_bridge_step_failure")
					}
					e.waitLease(t)
					out, err = e.worker(t, e.repo, e.agents, true).Run(e.ctx, 1, e.created.TaskID)
					want, step := "response_unknown", 0
					if mode == "阶段事件失败" {
						want, step = "evidence_pending", 1
					}
					if mode == "旧租约无回执" {
						want = "capture_pending"
					}
					if err != nil || out.Outcome != want || out.ReportAvailable {
						t.Fatal(out, err)
					}
					calls := int32(1)
					if mode == "预算回执未知" {
						calls = 0
					}
					if e.submits.Load() != calls || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 {
						t.Fatal("未知窗口重发或重复预留")
					}
					e.checkFrozen(t, step)
				})
			}
		})
	}
}

func TestIntegrationAITaskAgentBridgeBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"关闭", "准入拒绝", "混合阶段拒绝", "取消中断", "慢提交心跳"} {
				t.Run(mode, func(t *testing.T) {
					e := newTaskAgentBridgeEnv(t, level)
					enabled := mode != "关闭"
					if mode == "准入拒绝" {
						e.planner.allowed.Store(false)
					}
					if mode == "混合阶段拒绝" {
						leased, err := e.tasks.Claim(e.ctx, 1, e.created.TaskID, "worker_other_stage_01", time.Second)
						if err != nil {
							t.Fatal(err)
						}
						budgets, err := service.NewAITaskBudgetService(e.repo)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := budgets.Reserve(e.ctx, task.ReserveRequest{UserID: 1, TaskID: leased.ID, Fence: leased.Fence, Owner: leased.LeaseOwner, Key: "test_other_extract", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
							t.Fatal(err)
						}
						e.waitLease(t)
					}
					if mode == "取消中断" {
						e.onStatus = func(ctx context.Context) error {
							if _, err := e.tasks.Cancel(ctx, 1, e.created.TaskID); err != nil {
								return err
							}
							<-ctx.Done()
							return ctx.Err()
						}
					}
					if mode == "慢提交心跳" {
						e.onSubmit = func(ctx context.Context) error {
							timer := time.NewTimer(2500 * time.Millisecond)
							defer timer.Stop()
							select {
							case <-ctx.Done():
								return ctx.Err()
							case <-timer.C:
							}
							_, err := e.tasks.Claim(ctx, 1, e.created.TaskID, "worker_bridge_competitor", time.Second)
							if !errors.Is(err, task.ErrNotReady) {
								e.transportFault.Store(true)
								return points.ErrInvariant
							}
							return nil
						}
					}
					out, err := e.worker(t, e.repo, e.agents, enabled).Run(e.ctx, 1, e.created.TaskID)
					if mode == "混合阶段拒绝" {
						if !errors.Is(err, points.ErrIdempotency) || out != (task.RecoveryResult{}) || e.submits.Load() != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 {
							t.Fatal("混合阶段触发外部调用", out, err)
						}
						e.checkFrozen(t, 0)
						return
					}
					if mode == "关闭" || mode == "准入拒绝" {
						if !errors.Is(err, agents.ErrDisabled) || out != (task.RecoveryResult{}) || e.submits.Load() != 0 || e.queryInt("SELECT attempt FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 0 {
							t.Fatal("无准入仍有副作用", out, err)
						}
						return
					}
					if mode == "取消中断" {
						if err == nil || out != (task.RecoveryResult{}) || e.queryInt("SELECT cancel_req FROM tm_ai_tasks") != 1 {
							t.Fatal(out, err)
						}
						e.checkFrozen(t, 0)
					} else {
						if err != nil || out.Outcome != "evidence_pending" {
							t.Fatal(out, err)
						}
						e.checkFrozen(t, 1)
					}
				})
			}
		})
	}
}

func TestIntegrationAITaskAgentBridgeConcurrent(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newTaskAgentBridgeEnv(t, level)
			var wg sync.WaitGroup
			faults := make(chan error, 40)
			var finished atomic.Int32
			w := e.worker(t, e.repo, e.agents, true)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					out, err := w.Run(e.ctx, 1, e.created.TaskID)
					if err == nil {
						if out.Outcome != "evidence_pending" {
							err = points.ErrInvariant
						} else {
							finished.Add(1)
						}
					}
					if errors.Is(err, task.ErrNotReady) {
						err = nil
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
			if finished.Load() < 1 || e.submits.Load() != 1 || e.results.Load() != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 1 {
				t.Fatal("并发重复提交或推进")
			}
			e.checkFrozen(t, 1)
		})
	}
}
