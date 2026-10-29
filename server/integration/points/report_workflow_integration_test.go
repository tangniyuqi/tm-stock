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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 仅测试：明确示例来源+模拟网络，绝非生产来源许可/质量/远端硬预算控制器。
type workflowPlanner struct {
	x       *reportEnv
	p       service.AIL1ReportPlan
	enabled atomic.Bool
}

func (p *workflowPlanner) Plan(ctx context.Context, _ task.Task) (service.AIL1ReportPlan, error) {
	return p.p, ctx.Err()
}
func (p *workflowPlanner) Recheck(ctx context.Context, v task.Task, q service.AIL1ReportPlan) error {
	if !p.enabled.Load() || q.Agent != p.p.Agent || q.Model != p.p.Model || q.Agent.ValidateScope(v) != nil {
		return agents.ErrDisabled
	}
	return p.x.repo.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		ids := make([]int64, 0, len(q.Snapshots))
		for _, snap := range q.Snapshots {
			ids = append(ids, snap.ID)
		}
		slices.Sort(ids)
		snaps, theme, err := tx.ReportEvidence(ctx, ids, v.SubjectID)
		if err != nil {
			return err
		}
		sel, err := finalreport.NewSelectionStage(p.x.policy, v.No, v.SubjectID, q.ThemeName, q.Snapshots)
		if err != nil {
			return err
		}
		if !sel.MatchesEvidence(snaps, theme) {
			return sources.ErrUnauthorized
		}
		return nil
	})
}

type workflowSelectionControl struct {
	*checkSQLControl
	planner *workflowPlanner
}

func (c workflowSelectionControl) Recheck(ctx context.Context, v task.Task, p *finalreport.SelectionModelPlan) error {
	if p.Config() != c.cfg || !p.MatchesTask(v) {
		return errCheckTest
	}
	return c.planner.Recheck(ctx, v, c.planner.p)
}

type workflowStore struct {
	*repository.AITaskRepo
	x     *reportEnv
	phase string
	once  atomic.Bool
}

func (s *workflowStore) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	err := s.AITaskRepo.WithinBudgetTx(ctx, fn)
	if err == nil && s.phase == "budget" && s.x.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations WHERE seq=2") == 1 && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return err
}
func (s *workflowStore) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	err := s.AITaskRepo.WithinStageTx(ctx, fn)
	if err == nil && s.phase == "capture" && s.x.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results WHERE step_no=2") == 1 && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return err
}
func (s *workflowStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	err := s.AITaskRepo.WithinReportTx(ctx, fn)
	if err == nil && s.phase == "final" && s.x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") == 1 && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return err
}

type workflowEnv struct {
	*reportEnv
	planner       *workflowPlanner
	control       *checkSQLControl
	model         *provider.OpenAICompat
	server        *providertest.Server
	agent         *service.AIAgentService
	payloads      *repository.AIAgentRepo
	store         *workflowStore
	submits       atomic.Int32
	agentPolls    atomic.Int32
	agentComplete atomic.Bool // 仅浏览器流续读夹具显式放行回环Agent；普通运行模式保持原行为。
}

func workflowSelectionJSON(t *testing.T, id int64) string {
	t.Helper()
	out := finalreport.SelectionResponse{}
	for i, key := range []render.SectionKey{render.SecDefinition, render.SecTimeline, render.SecSegment, render.SecCompanies} {
		out.Selections = append(out.Selections, finalreport.Selection{Section: key, SnapshotID: id, SentenceIDs: []int{i + 1}})
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func newWorkflowEnv(t *testing.T, level sql.IsolationLevel, mode string) *workflowEnv {
	t.Helper()
	budget := taskBudget()
	budget.MaxCalls = 6
	budget.MaxTokens = 5000
	local := mode == "local" || mode == "local_deep"
	x := newReportEnvWithBudgetLocal(t, level, time.Minute, false, budget, local)
	snapshots := []sources.Snapshot{x.snap}
	if mode == "multi_source" {
		second, err := sources.BuildSnapshot(x.source, "https://evidence.example.test/second", sources.MediaPlain, []byte(x.snap.Body), x.snap.FetchedAt, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		second, err = repository.NewAISourceRepo(x.db).SaveSnapshot(x.ctx, second)
		if err != nil {
			t.Fatal(err)
		}
		// 故意倒序，不能把规划器返回顺序当成判定输入顺序。
		snapshots = []sources.Snapshot{second, x.snap}
	}
	reportDDL(t, x.pointsTestEnv, "20261015_create_tm_ai_agent_runs.sql")
	x.mustExec(`CREATE TABLE test_only_check_audits(operation_key varchar(80),phase varchar(8),payload mediumblob,PRIMARY KEY(operation_key,phase)) ENGINE=InnoDB`)
	selectionText := workflowSelectionJSON(t, x.snap.ID)
	if mode == "local_deep" {
		var picks finalreport.SelectionResponse
		if err := json.Unmarshal([]byte(selectionText), &picks); err != nil {
			t.Fatal(err)
		}
		picks.Selections[2].SentenceIDs = []int{3, 4}
		picks.Selections[3].SentenceIDs = []int{5, 6}
		raw, err := json.Marshal(picks)
		if err != nil {
			t.Fatal(err)
		}
		selectionText = string(raw)
	}
	if mode == "multi_source" {
		var picks finalreport.SelectionResponse
		if err := json.Unmarshal([]byte(selectionText), &picks); err != nil {
			t.Fatal(err)
		}
		picks.Selections[2].SnapshotID = snapshots[0].ID
		picks.Selections[3].SnapshotID = snapshots[0].ID
		raw, err := json.Marshal(picks)
		if err != nil {
			t.Fatal(err)
		}
		selectionText = string(raw)
	}
	selection := providertest.Success(selectionText, provider.Usage{Prompt: 2, Completion: 1, Total: 3})
	steps := []providertest.Step{selection, checkModelReply("pass", "supported"), checkModelReply("pass", "factual")}
	switch mode {
	case "ent_block":
		steps = steps[:2]
		steps[1] = checkModelReply("block", "unsupported")
	case "sem_uncertain":
		steps[2] = checkModelReply("uncertain", "uncertain")
	case "malformed":
		steps = steps[:1]
		steps[0] = providertest.Success(`{"selections":[]}`, provider.Usage{Prompt: 2, Completion: 1, Total: 3})
	case "transport":
		steps = []providertest.Step{providertest.Abort()}
	case "slow":
		steps[0] = providertest.SlowHeader(1200*time.Millisecond, steps[0])
	}
	srv := providertest.NewServer(t, steps...)
	model, err := provider.NewDirect(provider.Config{Name: "test-only-l1", BaseURL: srv.URL(), Model: "测试模型", TimeoutMS: 3000}, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := finalreport.CheckConfig{Model: "测试模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: model.DirectProfileDigest(), MaxInputTokens: 32}
	if local {
		cfg.FilingNo = "仅测试:公示"
		cfg.Version = "local-eval:v1"
		cfg.ApprovalRef = "local-eval:approval"
		cfg.QualityRef = "local-eval:quality"
		cfg.HardBudgetRef = "local-eval:budget"
		cfg.SelectionMaxTokens = 2048
		cfg.CheckMaxTokens = 64
	}
	if mode == "local_deep" {
		cfg.AnalysisType = 2
	}
	ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, cfg)
	if err != nil {
		t.Fatal(err)
	}
	sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, cfg)
	if err != nil {
		t.Fatal(err)
	}
	fc := finalreport.Config{Ref: "test-only-policy", ModelName: cfg.Model, FilingNo: cfg.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: []string{x.source.Domain}}
	if local {
		fc.Ref = "local-eval:policy"
	}
	if mode == "local_deep" {
		fc.AnalysisType = 2
		fc.MinQuotes = 6
	}
	x.policy, err = finalreport.NewPolicy(fc, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	e := &workflowEnv{reportEnv: x, server: srv, model: model, payloads: repository.NewAIAgentRepo(x.db), store: &workflowStore{AITaskRepo: x.repo, x: x}}
	bearer := strings.Repeat("z", 40)
	cn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+bearer {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/api/analysis/single":
			e.submits.Add(1)
			io.WriteString(w, `{"success":true,"data":{"task_id":"l1_test_remote","status":"queued"}}`)
		case "/api/analysis/tasks/l1_test_remote/status":
			e.agentPolls.Add(1)
			if mode == "agent_running" && !e.agentComplete.Load() {
				io.WriteString(w, `{"success":true,"data":{"task_id":"l1_test_remote","status":"running"}}`)
			} else if mode == "agent_failed" {
				io.WriteString(w, `{"success":true,"data":{"task_id":"l1_test_remote","status":"failed"}}`)
			} else if mode == "agent_cancelled" {
				io.WriteString(w, `{"success":true,"data":{"task_id":"l1_test_remote","status":"cancelled"}}`)
			} else {
				io.WriteString(w, `{"success":true,"data":{"task_id":"l1_test_remote","status":"completed"}}`)
			}
		case "/api/analysis/tasks/l1_test_remote/result":
			io.WriteString(w, `{"success":true,"data":{"analysis_id":"l1_test_remote","reports":{"news":"test-only-agent-untrusted-marker"}}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(cn.Close)
	cc := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: cn.URL, ApprovedHost: "127.0.0.1", Bearer: bearer, ApprovalRef: "test-only:approval", LicenseRef: "test-only:license", QuickModel: "test-quick", DeepModel: "test-deep", Timeout: 3 * time.Second, MaxSubmissions: 10}
	client, err := tradingcn.New(cc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	e.agent, err = service.NewAIAgentService(e.payloads, client, cc.Profile())
	if err != nil {
		t.Fatal(err)
	}
	a := agenttask.Plan{UserID: x.v.UserID, TaskID: x.v.ID, TaskNo: x.v.No, SubjectType: x.v.SubjectType, SubjectID: x.v.SubjectID, ActorID: 7, Symbol: "600001", ProfileDigest: cc.Profile().Digest, Units: task.Units{Calls: 1, Tokens: 100}}
	a.OperationKey = a.RequestID()
	theme := "示例题材"
	if local {
		theme = "仅测试:合成题材"
	}
	e.planner = &workflowPlanner{x: x, p: service.AIL1ReportPlan{Agent: a, Model: cfg, Policy: fc, ThemeName: theme, Snapshots: snapshots, AnalysisType: fc.AnalysisType}}
	e.planner.enabled.Store(true)
	e.control = &checkSQLControl{x: x, cfg: cfg}
	if local {
		e.control.theme = theme
	}
	return e
}
func (e *workflowEnv) worker(t *testing.T, enabled bool) *service.AIL1ReportWorker {
	t.Helper()
	w, err := service.NewAIL1ReportWorker(e.tasks, e.store, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.v.LeaseOwner, agenttask.Options{Enabled: enabled, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func TestIntegrationAIReportWorkflowLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"pass", "ent_block", "sem_uncertain", "malformed", "slow", "multi_source"} {
				t.Run(mode, func(t *testing.T) {
					e := newWorkflowEnv(t, level, mode)
					w := e.worker(t, true)
					got, err := w.Run(e.ctx, e.v.UserID, e.v.ID)
					if err != nil {
						t.Fatal(err)
					}
					status := task.Succeeded
					calls := 3
					if mode == "ent_block" {
						status = task.Failed
						calls = 2
					}
					if mode == "sem_uncertain" {
						status = task.Failed
					}
					if mode == "malformed" {
						status = task.Failed
						calls = 1
					}
					if got.Task.TaskStatus != status || got.Task.ReportAvailable || e.submits.Load() != 1 || e.server.Count() != calls || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
						t.Fatal("主链路未闭合", got, e.submits.Load(), e.server.Count())
					}
					if status == task.Succeeded {
						if _, _, err := e.payloads.Quarantined(e.ctx, e.planner.p.Agent.ActorID+1, e.planner.p.Agent.RequestID()); !errors.Is(err, agents.ErrNotFound) {
							t.Fatal("隔离正文操作人越权", err)
						}
						if e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3") != 1 {
							t.Fatal("结算未原子闭合")
						}
						var raw []byte
						if err := e.db.QueryRowContext(e.ctx, "SELECT payload FROM tm_ai_report_commits").Scan(&raw); err != nil {
							t.Fatal(err)
						}
						if strings.Contains(string(raw), "untrusted-marker") || !strings.Contains(string(e.server.Requests()[0].Body), "test-only-agent-untrusted-marker") {
							t.Fatal("Agent线索未使用或原文泄漏")
						}
						var p finalreport.Proposal
						if json.Unmarshal(raw, &p) != nil {
							t.Fatal("报告无法读取")
						}
						for i, sec := range p.Report.Sections {
							if sec.Nodes[0].Text != e.snap.Sentences[i].Text {
								t.Fatal("未逐字回取")
							}
						}
					} else if e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE payload IS NOT NULL") != 0 {
						t.Fatal("失败未释放或留正文")
					}
					count := e.server.Count()
					again, err := w.Run(e.ctx, e.v.UserID, e.v.ID)
					if err != nil || again.Task.TaskStatus != status || e.server.Count() != count || e.submits.Load() != 1 {
						t.Fatal("终态重放重复外呼", again, err)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
func TestIntegrationAIReportWorkflowGuards(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"off", "foreign", "withdrawn", "proof", "model", "budget", "transport", "cancel"} {
				t.Run(name, func(t *testing.T) {
					mode := "pass"
					if name == "transport" {
						mode = name
					}
					e := newWorkflowEnv(t, level, mode)
					uid := e.v.UserID
					switch name {
					case "foreign":
						uid++
					case "withdrawn":
						if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
							t.Fatal(err)
						}
					case "proof":
						e.planner.enabled.Store(false)
					case "model":
						e.planner.p.Model.ProfileDigest = strings.Repeat("e", 64)
					case "budget":
						e.planner.p.Model.MaxInputTokens = 5000
						var err error
						e.planner.p.Policy.EntailmentVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckEntailment, e.planner.p.Model)
						if err != nil {
							t.Fatal(err)
						}
						e.planner.p.Policy.SemanticVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckSemantic, e.planner.p.Model)
						if err != nil {
							t.Fatal(err)
						}
					case "cancel":
						if _, err := e.tasks.Cancel(e.ctx, e.v.UserID, e.v.ID); err != nil {
							t.Fatal(err)
						}
					}
					got, err := e.worker(t, name != "off").Run(e.ctx, uid, e.v.ID)
					if name == "budget" && !errors.Is(err, task.ErrBudget) {
						t.Fatal("未实际覆盖整链容量拒绝", err)
					}
					if name == "cancel" {
						if err != nil || got.Task.TaskStatus != task.Cancelled {
							t.Fatal(got, err)
						}
					} else if name == "withdrawn" {
						if err != nil || got.Task.TaskStatus != task.Failed || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
							t.Fatal("已知撤回未失败释放", got, err)
						}
					} else if err == nil {
						t.Fatal("应拒绝", got)
					}
					if name == "transport" {
						if e.submits.Load() != 1 || e.server.Count() != 1 {
							t.Fatal("未到达未知边界")
						}
						again, err := e.worker(t, true).Run(e.ctx, e.v.UserID, e.v.ID)
						if err != nil || again.Task.Outcome != "response_unknown" || e.server.Count() != 1 {
							t.Fatal("未知请求重发", again, err)
						}
					} else if e.submits.Load() != 0 || e.server.Count() != 0 {
						t.Fatal("前置拒绝仍外呼")
					}
				})
			}
		})
	}
}
func TestIntegrationAIReportWorkflowRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, phase := range []string{"budget", "capture", "final"} {
				t.Run(phase, func(t *testing.T) {
					e := newWorkflowEnv(t, level, "pass")
					e.store.phase = phase
					w := e.worker(t, true)
					got, err := w.Run(e.ctx, e.v.UserID, e.v.ID)
					if !errors.Is(err, errCheckTest) || got.Commit.TaskID != 0 {
						t.Fatal("未知提交返回成功", got, err)
					}
					again, err := w.Run(e.ctx, e.v.UserID, e.v.ID)
					if err != nil {
						t.Fatal(again, err)
					}
					if phase == "budget" {
						if again.Task.Outcome != "response_unknown" || e.server.Count() != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
							t.Fatal("预算未知被补发/释放", again)
						}
					} else if again.Task.TaskStatus != task.Succeeded || e.server.Count() != 3 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 {
						t.Fatal("捕获或结算未恢复", again)
					}
					if e.submits.Load() != 1 {
						t.Fatal("Agent重复提交")
					}
				})
			}
		})
	}
}
func TestIntegrationAIReportWorkflowConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newWorkflowEnv(t, level, "pass")
			w := e.worker(t, true)
			var wg sync.WaitGroup
			errorsSeen := make(chan error, 12)
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); _, err := w.Run(e.ctx, e.v.UserID, e.v.ID); errorsSeen <- err }()
			}
			wg.Wait()
			close(errorsSeen)
			for err := range errorsSeen {
				// 竞争者可遇到活跃租约、已终态或尚未捕获的前缀，不放过其他数据库/审计错误。
				if err != nil && !errors.Is(err, task.ErrLease) && !errors.Is(err, task.ErrNotReady) && !errors.Is(err, points.ErrNotFound) && !errors.Is(err, points.ErrIdempotency) && !errors.Is(err, points.ErrClosed) {
					t.Fatal("非预期并发错误", err)
				}
			}
			got, err := w.Run(e.ctx, e.v.UserID, e.v.ID)
			if err != nil || got.Task.TaskStatus != task.Succeeded || e.submits.Load() != 1 || e.server.Count() != 3 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 {
				t.Fatal("并发没有收敛单次", got, err, e.server.Count())
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}
