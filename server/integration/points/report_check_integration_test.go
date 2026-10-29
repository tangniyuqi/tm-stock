//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var errCheckTest = errors.New("test-only:判定边界注入")

// 只为示例资料/模拟模型服务的控制器。审计使用真实 SQL，但不是生产许可/质量证明实现。
type checkSQLControl struct {
	x     *reportEnv
	cfg   finalreport.CheckConfig
	fail  string
	hook  func() error
	theme string
}

func (c *checkSQLControl) Recheck(ctx context.Context, v task.Task, p *finalreport.CheckPlan) error {
	if v.ID != c.x.v.ID || v.UserID != c.x.v.UserID || v.No != c.x.v.No || p.Config() != c.cfg {
		return errCheckTest
	}
	return c.x.repo.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		snaps, theme, err := tx.ReportEvidence(ctx, []int64{c.x.snap.ID}, c.x.v.SubjectID)
		if err != nil {
			return err
		}
		expected := c.theme
		if expected == "" {
			expected = "示例题材"
		}
		if theme != expected || len(snaps) != 1 || snaps[0].Hash != c.x.snap.Hash {
			return errCheckTest
		}
		return nil
	})
}
func (c *checkSQLControl) Start(ctx context.Context, r task.StageRequest, request provider.Request) error {
	if c.fail == "start" {
		return errCheckTest
	}
	b, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = c.x.db.ExecContext(ctx, `INSERT INTO test_only_check_audits(operation_key,phase,payload) VALUES(?,'start',?)`, r.OperationKey, b)
	return err
}
func (c *checkSQLControl) Finish(ctx context.Context, r task.StageRequest, response provider.Response, callErr error) error {
	if c.fail == "finish" {
		return errCheckTest
	}
	b, err := json.Marshal(struct {
		Response provider.Response
		OK       bool
	}{response, callErr == nil})
	if err != nil {
		return err
	}
	if _, err = c.x.db.ExecContext(ctx, `INSERT INTO test_only_check_audits(operation_key,phase,payload) VALUES(?,'finish',?)`, r.OperationKey, b); err != nil {
		return err
	}
	if c.hook != nil {
		return c.hook()
	}
	return nil
}

type checkAmbiguousStore struct {
	*repository.AITaskRepo
	phase string
	once  atomic.Bool
}

func (s *checkAmbiguousStore) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	err := s.AITaskRepo.WithinBudgetTx(ctx, fn)
	if err == nil && s.phase == "budget" && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return err
}
func (s *checkAmbiguousStore) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	err := s.AITaskRepo.WithinStageTx(ctx, fn)
	if err == nil && s.phase == "capture" && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return err
}

type checkEnv struct {
	*reportEnv
	control  *checkSQLControl
	model    *provider.OpenAICompat
	server   *providertest.Server
	worker   *service.AIReportCheckWorker
	store    *checkAmbiguousStore
	cfg      finalreport.Config
	draft    service.AIReportDraft
	ent, sem *finalreport.CheckPlan
	r        task.StageRequest
	lease    time.Duration
}

func newCheckEnv(t *testing.T, level sql.IsolationLevel, lease time.Duration, steps ...providertest.Step) *checkEnv {
	t.Helper()
	modelCfg := finalreport.CheckConfig{Model: "测试判定模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", MaxInputTokens: 100}
	srv := providertest.NewServer(t, steps...)
	client, err := provider.NewDirect(provider.Config{Name: "test-only", BaseURL: srv.URL(), Model: modelCfg.Model, TimeoutMS: 5000}, true)
	if err != nil {
		t.Fatal(err)
	}
	modelCfg.ProfileDigest = client.DirectProfileDigest()
	ev, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, modelCfg)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, modelCfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg := finalreport.Config{Ref: "test-only-policy", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: ev, SemanticVersion: sv, AllowedDomains: []string{"evidence.example.test"}}
	x, selection, sel := newConversionEnvWithPolicy(t, level, lease, &cfg)
	draft, err := x.svc.Convert(x.ctx, selection, sel)
	if err != nil {
		t.Fatal(err)
	}
	in, text, err := x.policy.Inputs(string(draft.Payload), []sources.Snapshot{x.snap})
	if err != nil {
		t.Fatal(err)
	}
	ent, err := finalreport.NewEntailmentCheck(modelCfg, in)
	if err != nil {
		t.Fatal(err)
	}
	sem, err := finalreport.NewSemanticCheck(modelCfg, x.v.No, text)
	if err != nil {
		t.Fatal(err)
	}
	x.mustExec(`CREATE TABLE test_only_check_audits(operation_key varchar(80),phase varchar(8),payload mediumblob,PRIMARY KEY(operation_key,phase)) ENGINE=InnoDB`)
	e := &checkEnv{reportEnv: x, control: &checkSQLControl{x: x, cfg: modelCfg}, model: client, server: srv, cfg: cfg, draft: draft, ent: ent, sem: sem, store: &checkAmbiguousStore{AITaskRepo: x.repo}, lease: lease}
	e.worker = e.newWorker(t, x.v.LeaseOwner)
	e.r = task.StageRequest{UserID: x.v.UserID, TaskID: x.v.ID, Owner: x.v.LeaseOwner, Fence: x.v.Fence, StepNo: 2, Stage: "judge", OperationKey: ent.OperationKey(x.v, 2), InputDigest: ent.InputDigest()}
	return e
}
func (e *checkEnv) newWorker(t *testing.T, owner string) *service.AIReportCheckWorker {
	t.Helper()
	w, err := service.NewAIReportCheckWorker(e.tasks, e.store, e.model, e.control, owner, service.AIReportCheckOptions{Enabled: true, Lease: task.RecoveryOptions{Lease: e.lease, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func checkModelReply(verdict, reason string) providertest.Step {
	return providertest.Success(fmt.Sprintf(`{"verdict":%q,"reason":%q}`, verdict, reason), provider.Usage{Prompt: 2, Completion: 1, Total: 3})
}

// 只记录隔离测试库的状态/剩余租约与模拟请求数，不续租、不重试、不输出正文或凭据。
func logCheckTestLease(t *testing.T, e *checkEnv, phase string) {
	t.Helper()
	ctx, stop := context.WithTimeout(context.WithoutCancel(e.ctx), 3*time.Second)
	defer stop()
	var status, step int
	var fence, remaining int64
	err := e.db.QueryRowContext(ctx, `SELECT status,current_step,fence,COALESCE(TIMESTAMPDIFF(MICROSECOND,UTC_TIMESTAMP(3),lease_until),0) FROM tm_ai_tasks WHERE id=?`, e.v.ID).Scan(&status, &step, &fence, &remaining)
	t.Logf("慢响应租约诊断 phase=%s status=%d step=%d fence=%d remaining_us=%d model_requests=%d err=%v", phase, status, step, fence, remaining, e.server.Count(), err)
}

func TestIntegrationAIReportChecksLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"pass", "entailment_block", "semantic_uncertain", "slow_heartbeat"} {
				t.Run(name, func(t *testing.T) {
					steps := []providertest.Step{checkModelReply("pass", "supported"), checkModelReply("pass", "factual")}
					if name == "entailment_block" {
						steps = []providertest.Step{checkModelReply("block", "unsupported")}
					}
					if name == "semantic_uncertain" {
						steps[1] = checkModelReply("uncertain", "uncertain")
					}
					lease := time.Minute
					if name == "slow_heartbeat" {
						lease = time.Second
						steps[0] = providertest.SlowHeader(1300*time.Millisecond, steps[0])
					}
					e := newCheckEnv(t, level, lease, steps...)
					if name == "slow_heartbeat" {
						logCheckTestLease(t, e, "before_entailment")
					}
					ent, err := e.worker.RunOwned(e.ctx, e.r, e.ent)
					if err != nil || ent.Evidence == nil {
						logCheckTestLease(t, e, "entailment_failed")
						t.Fatal(ent, err)
					}
					last := e.r
					var semantic service.AIReportCheckReceipt
					if name != "entailment_block" {
						last.StepNo = 3
						last.OperationKey = e.sem.OperationKey(e.v, 3)
						last.InputDigest = e.sem.InputDigest()
						semantic, err = e.worker.RunOwned(e.ctx, last, e.sem)
						if err != nil || semantic.Evidence == nil {
							logCheckTestLease(t, e, "semantic_failed")
							t.Fatal(semantic, err)
						}
					}
					policy, err := finalreport.NewCapturedPolicy(e.cfg, ent.Evidence, semantic.Evidence)
					if err != nil {
						t.Fatal(err)
					}
					e.policy = policy
					e.svc, err = service.NewAIReportService(e.repo, e.ledger, policy, true)
					if err != nil {
						t.Fatal(err)
					}
					binding, err := json.Marshal([]string{e.draft.InputDigest, ent.Checkpoint.OutputDigest, ent.Checkpoint.PolicyDigest, semantic.Checkpoint.OutputDigest, semantic.Checkpoint.PolicyDigest})
					if err != nil {
						t.Fatal(err)
					}
					draft := e.draft
					draft.InputDigest = task.StageDigest(binding)
					judge := conversionJudge(t, e.reportEnv, last, draft)
					got, err := e.svc.Finalize(e.ctx, judge)
					if err != nil {
						t.Fatal(err)
					}
					want := task.Succeeded
					expectedCalls := 2
					if name == "entailment_block" || name == "semantic_uncertain" {
						want = task.Failed
					}
					if name == "entailment_block" {
						expectedCalls = 1
					}
					if got.Status != want || e.server.Count() != expectedCalls || e.queryInt("SELECT COUNT(*) FROM test_only_check_audits") != expectedCalls*2 {
						t.Fatal("判定/审计/终态不一致", got)
					}
					if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
						t.Fatal("未结束冻结")
					}
					if want == task.Failed && (got.ReportDigest != "" || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE payload IS NOT NULL") != 0) {
						t.Fatal("拒绝仍发布正文")
					}
					if want == task.Succeeded {
						var payload []byte
						if err := e.db.QueryRowContext(e.ctx, `SELECT payload FROM tm_ai_report_commits WHERE task_id=?`, e.v.ID).Scan(&payload); err != nil || string(payload) != string(e.draft.Payload) {
							t.Fatal("模型判定原文混入报告", err)
						}
					}
					if replay, err := e.svc.Finalize(e.ctx, judge); err != nil || replay != got || e.server.Count() != expectedCalls {
						t.Fatal("终检重放再次调用模型", err)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportChecksFaults(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"start", "finish", "transport", "withdrawn", "unknown_capture", "unknown_budget", "user", "foreign_plan", "fixed_model"} {
				t.Run(name, func(t *testing.T) {
					step := checkModelReply("pass", "supported")
					if name == "transport" {
						step = providertest.Abort()
					}
					e := newCheckEnv(t, level, time.Minute, step)
					plan := e.ent
					switch name {
					case "user":
						e.r.UserID++
					case "foreign_plan":
						var err error
						plan, err = finalreport.NewSemanticCheck(e.ent.Config(), task.StageDigest([]byte("test-only-foreign-task"))[:32], "示例题材仅用于测试资料。")
						if err != nil {
							t.Fatal(err)
						}
						e.r.InputDigest = plan.InputDigest()
					case "fixed_model":
						modelCfg := e.ent.Config()
						modelCfg.Model = "另一测试判定模型"
						in, _, err := e.policy.Inputs(string(e.draft.Payload), []sources.Snapshot{e.snap})
						if err != nil {
							t.Fatal(err)
						}
						plan, err = finalreport.NewEntailmentCheck(modelCfg, in)
						if err != nil {
							t.Fatal(err)
						}
						e.r.InputDigest = plan.InputDigest()
					case "start", "finish":
						e.control.fail = name
					case "withdrawn":
						e.control.hook = func() error { return repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false) }
					case "unknown_capture":
						e.store.phase = "capture"
					case "unknown_budget":
						e.store.phase = "budget"
					}
					got, err := e.worker.RunOwned(e.ctx, e.r, plan)
					if err == nil || got.Evidence != nil {
						t.Fatal("故障返回通过证据", got, err)
					}
					calls := e.server.Count()
					e.control.fail = ""
					e.control.hook = nil
					out, err := e.worker.RunOwned(e.ctx, e.r, plan)
					if name == "withdrawn" || name == "user" || name == "foreign_plan" || name == "fixed_model" {
						if err == nil || out.Evidence != nil {
							t.Fatal("撤回来源仍使用")
						}
					} else if name == "unknown_capture" {
						if err != nil || out.Evidence == nil || out.Task.Outcome != "check_pass" {
							t.Fatal("已提交捕获无法恢复", out, err)
						}
					} else if err != nil || out.Task.Outcome != "response_unknown" || out.Evidence != nil {
						t.Fatal("未知现场未保留", out, err)
					}
					if e.server.Count() != calls || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
						t.Fatal("故障重复调用或扣费")
					}
					if (name == "start" || name == "unknown_budget" || name == "user" || name == "foreign_plan" || name == "fixed_model") && calls != 0 {
						t.Fatal("调用前失败仍请求模型")
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportChecksConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCheckEnv(t, level, time.Minute, providertest.SlowHeader(100*time.Millisecond, checkModelReply("pass", "supported")))
			var wg sync.WaitGroup
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					out, err := e.worker.RunOwned(e.ctx, e.r, e.ent)
					if err != nil || (out.Task.Outcome != "check_pass" && out.Task.Outcome != "response_unknown") {
						t.Error(out, err)
					}
				}()
			}
			wg.Wait()
			out, err := e.worker.RunOwned(e.ctx, e.r, e.ent)
			if err != nil || out.Evidence == nil || e.server.Count() != 1 || e.queryInt("SELECT COUNT(*) FROM test_only_check_audits") != 2 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 2 {
				t.Fatal("并发重复调用或结果", out, err)
			}
		})
	}
}

func TestIntegrationAIReportChecksRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCheckEnv(t, level, time.Second, checkModelReply("pass", "supported"))
			first, err := e.worker.RunOwned(e.ctx, e.r, e.ent)
			if err != nil {
				t.Fatal(err)
			}
			current, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, e.pointsTestEnv, *current.LeaseUntil)
			next, err := e.tasks.Claim(e.ctx, 1, e.v.ID, "test_check_recovery_owner", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if out, err := e.worker.RunOwned(e.ctx, e.r, e.ent); err == nil || out.Evidence != nil {
				t.Fatal("旧围栏可复用")
			}
			r := e.r
			r.Owner = next.LeaseOwner
			r.Fence = next.Fence
			out, err := e.newWorker(t, r.Owner).RunOwned(e.ctx, r, e.ent)
			if err != nil || out.Evidence.Decision() != first.Evidence.Decision() || e.server.Count() != 1 {
				t.Fatal("接管重新调用", out, err)
			}
		})
	}
}
