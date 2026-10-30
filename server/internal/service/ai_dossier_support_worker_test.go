package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 该夹具只模拟已提交状态/确认丢失；真实回滚和隔离另由MySQL测试证明。
type supportWorkerStore struct {
	*stageScript
	record                                           task.AdmissionRecord
	found                                            bool
	audits                                           map[string]finalreport.CallAudit
	unknownBudget, unknownCapture, unknownCheckpoint bool
}

func (s *supportWorkerStore) WithinAdmissionTx(ctx context.Context, fn func(task.AdmissionReadTx) error) error {
	return s.scriptedTaskStore.WithinTaskTx(ctx, func(task.Tx) error { return fn(s) })
}
func (s *supportWorkerStore) ReadAdmission(ctx context.Context, _ task.Task) (task.AdmissionRecord, bool, error) {
	return s.record, s.found, ctx.Err()
}
func (s *supportWorkerStore) WithinCallAuditTx(ctx context.Context, fn func(finalreport.CallAuditTx) error) error {
	return s.scriptedTaskStore.WithinTaskTx(ctx, func(task.Tx) error { return fn(s) })
}
func (s *supportWorkerStore) ReadCallAudit(ctx context.Context, _, _ int64, step int, phase string) (finalreport.CallAudit, error) {
	a, ok := s.audits[fmt.Sprint(step)+":"+phase]
	if !ok {
		return a, points.ErrNotFound
	}
	return a, ctx.Err()
}
func (s *supportWorkerStore) InsertCallAudit(ctx context.Context, a finalreport.CallAudit) error {
	if e := a.Validate(); e != nil {
		return e
	}
	s.audits[fmt.Sprint(a.Request.StepNo)+":"+a.Phase] = a
	return ctx.Err()
}
func (s *supportWorkerStore) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	e := s.stageScript.WithinBudgetTx(ctx, fn)
	if e == nil && s.unknownBudget {
		s.unknownBudget = false
		return errPointsInjected
	}
	return e
}
func (s *supportWorkerStore) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	before, checkpoints := len(s.results), len(s.steps)
	e := s.stageScript.WithinStageTx(ctx, fn)
	if e == nil && s.unknownCapture && len(s.results) > before {
		s.unknownCapture = false
		return errPointsInjected
	}
	if e == nil && s.unknownCheckpoint && len(s.steps) > checkpoints {
		s.unknownCheckpoint = false
		return errPointsInjected
	}
	return e
}

type supportWorkerControl struct {
	audit               *AIReportCallAuditService
	store               *supportWorkerStore
	input, execution    string
	allowed             bool
	checks, denyAt      int
	startErr, finishErr error
	hook                func()
}

func (c *supportWorkerControl) Recheck(ctx context.Context, v task.Task, p *dossier.SupportPlan) error {
	c.checks++
	if !c.allowed || c.checks == c.denyAt || !p.MatchesTask(v) || p.InputDigest() != c.input || c.store.record.Bind(v) != nil || c.store.record.Plan.ExecutionDigest != c.execution {
		return errPointsInjected
	}
	return ctx.Err()
}
func (c *supportWorkerControl) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	if c.startErr != nil {
		return c.startErr
	}
	return c.audit.Start(ctx, r, req)
}
func (c *supportWorkerControl) Finish(ctx context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	if c.finishErr != nil {
		return c.finishErr
	}
	if e := c.audit.Finish(ctx, r, out, callErr); e != nil {
		return e
	}
	if c.hook != nil {
		c.hook()
	}
	return nil
}

func supportWorkerReply(verdict, reason string) providertest.Step {
	return providertest.Success(fmt.Sprintf(`{"results":[{"id":1,"verdict":%q,"reason":%q}]}`, verdict, reason), provider.Usage{Prompt: 2, Completion: 2, Total: 4})
}
func newSupportWorkerFixture(t *testing.T, step int, reply providertest.Step) (*AIDossierSupportWorker, *supportWorkerStore, *supportWorkerControl, *providertest.Server, task.StageRequest, *dossier.SupportPlan) {
	t.Helper()
	_, base, _ := budgetFixture(t)
	base.v.Budget = task.Budget{MaxAttempts: 3, MaxCalls: 6, MaxTokens: 10000, MaxPages: 2, MaxDurationMS: 300000}
	base.v.Deadline = base.p.now.Add(5 * time.Minute)
	st := &supportWorkerStore{stageScript: &stageScript{budgetScript: base, results: map[int]task.StageResult{}, steps: map[int]task.StageCheckpoint{}, stageEvents: map[int64]task.Event{}}, found: true, audits: map[string]finalreport.CallAudit{}}
	srv := providertest.NewServer(t, reply)
	model, e := provider.NewDirect(provider.Config{Name: "test-only", BaseURL: srv.URL(), Model: "test-only-field-model", TimeoutMS: 1000}, true)
	if e != nil {
		t.Fatal(e)
	}
	cfg := dossier.SupportConfig{Model: "test-only-field-model", Version: "test-only-v1", FilingNo: "test-only-filing", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: model.DirectProfileDigest(), MaxInputTokens: 32, MaxOutputTokens: 128, MaxPromptBytes: 16000, ReviewStep: step}
	snap, e := sources.BuildSnapshot(sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "test-only", LicenseRef: "test-only"}, "https://example.test/support-only", sources.MediaPlain, []byte("仅测试：样本描述只用于单步核验执行。"), st.p.now, 4096)
	if e != nil {
		t.Fatal(e)
	}
	snap.ID = 1
	sentence := snap.Sentences[0]
	u := dossier.Field{State: "unknown"}
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: st.v.SubjectID, Version: 1, CreatedAt: st.p.now.UnixMilli(), Cards: []dossier.FactCard{{Key: "test_card", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: 1, Hash: snap.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
	b, e := dossier.BindDraft(d, []sources.Snapshot{snap})
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := dossier.PrepareSupport(context.Background(), cfg, "仅测试题材", b, []sources.Snapshot{snap}, []string{"test_card"})
	if e != nil {
		t.Fatal(e)
	}
	p, e := prepared.BindTask(context.Background(), st.v)
	if e != nil {
		t.Fatal(e)
	}
	plan := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: st.v.UserID, AnalysisType: st.v.AnalysisType, SubjectType: st.v.SubjectType, SubjectID: st.v.SubjectID}, Price: points.Price{Version: st.v.PriceVersion, Points: st.v.PricePoints}, Budget: st.v.Budget, SubjectName: "仅测试题材", ModelName: cfg.Model, FilingNo: cfg.FilingNo, ProfileDigest: cfg.ProfileDigest, ExecutionDigest: task.StageDigest([]byte("test-only-overall-profile")), DossierSupportDigest: p.ComponentDigest(), ApprovalRef: "test-only-admission", QualityRef: cfg.QualityRef, HardBudgetRef: cfg.HardBudgetRef, PriceRef: "test-only-price", SourceRef: "test-only-source", Materials: prepared.Materials()}
	_, digest, e := plan.Encode()
	if e != nil {
		t.Fatal(e)
	}
	st.record = task.AdmissionRecord{TaskID: st.v.ID, UserID: st.v.UserID, TaskNo: st.v.No, RequestHash: st.v.Hash, Digest: digest, Plan: plan, CreatedAt: st.p.now}
	if e = st.record.Bind(st.v); e != nil {
		t.Fatal(e)
	}
	// 前置阶段是显著合成夹具，使用原预算/捕获/检查点服务，不冒充真实Agent执行。
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	budget, _ := NewAITaskBudgetService(st)
	policy := task.StagePolicy{Digest: task.StageDigest([]byte("test-only-predecessor-policy")), Check: func(ctx context.Context, _ task.StageValidationInput) error { return ctx.Err() }}
	stages, _ := NewAITaskStageService(st, map[string]task.StagePolicy{"extract": policy})
	for n := 1; n < step; n++ {
		key := task.StageDigest([]byte(fmt.Sprint("test-only-predecessor-", n)))
		input := task.StageDigest([]byte(key))
		r := task.StageRequest{TaskID: st.v.ID, UserID: st.v.UserID, Owner: st.v.LeaseOwner, Fence: st.v.Fence, StepNo: n, Stage: "extract", OperationKey: key, InputDigest: input}
		if _, e = budget.ReserveStep(ctx, task.ReserveRequest{TaskID: r.TaskID, UserID: r.UserID, Owner: r.Owner, Fence: r.Fence, Key: key, InputDigest: input, Units: task.Units{Calls: 1, Tokens: 1}}, n); e != nil {
			t.Fatal(e)
		}
		if _, e = stages.Capture(ctx, r, []byte(`{"testOnly":true}`)); e != nil {
			t.Fatal(e)
		}
		if _, e = stages.Complete(ctx, r); e != nil {
			t.Fatal(e)
		}
	}
	audit, e := NewAIReportCallAuditService(st)
	if e != nil {
		t.Fatal(e)
	}
	ctl := &supportWorkerControl{audit: audit, store: st, input: p.InputDigest(), execution: plan.ExecutionDigest, allowed: true}
	tasks := &recoveryControl{st: st.stageScript, claimed: st.v}
	w, e := NewAIDossierSupportWorker(tasks, st, model, ctl, st.v.LeaseOwner, AIReportCheckOptions{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Second, Heartbeat: 50 * time.Millisecond, Timeout: 2 * time.Second}})
	if e != nil {
		t.Fatal(e)
	}
	r := task.StageRequest{TaskID: st.v.ID, UserID: st.v.UserID, Owner: st.v.LeaseOwner, Fence: st.v.Fence, StepNo: step, Stage: "judge", OperationKey: p.OperationKey(st.v, step), InputDigest: p.InputDigest()}
	return w, st, ctl, srv, r, p
}

func TestDossierSupportOwnedLifecycle(t *testing.T) {
	for _, step := range []int{3, 4} {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			for _, state := range []struct{ verdict, reason string }{{"supported", "direct_support"}, {"partial", "partial_support"}, {"unsupported", "not_supported"}, {"uncertain", "insufficient_context"}} {
				t.Run(state.verdict, func(t *testing.T) {
					w, st, _, srv, r, p := newSupportWorkerFixture(t, step, supportWorkerReply(state.verdict, state.reason))
					got, e := w.RunOwned(context.Background(), r, p)
					if e != nil || got.Review == nil || got.Task.Outcome != "dossier_support_recorded" || got.Task.ReportAvailable || got.Review.Results()[0].Verdict != state.verdict {
						t.Fatal("未保留独立核验结果", e)
					}
					if st.v.Status != task.Running || st.v.CurrentStep != step || st.p.account.Frozen != 7 || srv.Count() != 1 || len(st.audits) != 2 {
						t.Fatal("提前终态、结算或重复调用")
					}
					again, e := w.RunOwned(context.Background(), r, p)
					if e != nil || again.Task != got.Task || again.Review == nil || srv.Count() != 1 {
						t.Fatal("回放重复执行", e)
					}
					raw, e := json.Marshal(got)
					if e != nil || strings.Contains(string(raw), "verdict") || strings.Contains(string(raw), "test_card") {
						t.Fatal("内部核验进入公开回执")
					}
				})
			}
		})
	}
}

func TestDossierSupportOwnedUnknownAndRefusal(t *testing.T) {
	for _, mode := range []string{"budget_unknown", "capture_unknown", "checkpoint_unknown", "start", "finish", "transport", "estimated", "malformed", "revoked_after_call"} {
		t.Run(mode, func(t *testing.T) {
			reply := supportWorkerReply("partial", "partial_support")
			if mode == "transport" {
				reply = providertest.Abort()
			}
			if mode == "estimated" {
				reply = providertest.SuccessNoUsage(`{"results":[{"id":1,"verdict":"supported","reason":"direct_support"}]}`)
			}
			if mode == "malformed" {
				reply = providertest.Success(`{"results":[]}`, provider.Usage{Prompt: 2, Completion: 2, Total: 4})
			}
			w, st, ctl, srv, r, p := newSupportWorkerFixture(t, 3, reply)
			switch mode {
			case "budget_unknown":
				st.unknownBudget = true
			case "capture_unknown":
				st.unknownCapture = true
			case "checkpoint_unknown":
				st.unknownCheckpoint = true
			case "start":
				ctl.startErr = errPointsInjected
			case "finish":
				ctl.finishErr = errPointsInjected
			case "revoked_after_call":
				ctl.hook = func() { ctl.allowed = false }
			}
			got, e := w.RunOwned(context.Background(), r, p)
			if e == nil || got.Review != nil {
				t.Fatal("异常路径交付核验")
			}
			count := srv.Count()
			ctl.allowed = true
			ctl.hook = nil
			ctl.startErr = nil
			ctl.finishErr = nil
			again, e := w.RunOwned(context.Background(), r, p)
			if srv.Count() != count || st.p.account.Frozen != 7 {
				t.Fatal("未知重发或擅自释放")
			}
			switch mode {
			case "capture_unknown", "checkpoint_unknown":
				if e != nil || again.Review == nil || again.Task.Outcome != "dossier_support_recorded" {
					t.Fatal("原捕获未恢复", e)
				}
			case "estimated", "malformed":
				if !errors.Is(e, task.ErrStageRejected) || again.Review != nil {
					t.Fatal("非法回执被当作不确定语义", e)
				}
			default:
				if e != nil || again.Review != nil || again.Task.Outcome != "response_unknown" {
					t.Fatal("未知现场未保留", e)
				}
			}
		})
	}
}

func TestDossierSupportOwnedGates(t *testing.T) {
	for _, mode := range []string{"off", "nil_plan", "nil_context", "cancelled", "user", "owner", "fence", "key", "input", "stage", "step", "missing_admission", "wrong_component", "wrong_execution", "bad_record", "precheck", "before_call", "after_load"} {
		t.Run(mode, func(t *testing.T) {
			w, st, ctl, srv, r, p := newSupportWorkerFixture(t, 3, supportWorkerReply("supported", "direct_support"))
			ctx := context.Background()
			switch mode {
			case "off":
				w.core.options.Enabled = false
			case "nil_plan":
				p = nil
			case "nil_context":
				ctx = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "user":
				r.UserID++
			case "owner":
				r.Owner = "test-only-other-owner"
			case "fence":
				r.Fence++
			case "key":
				r.OperationKey = task.StageDigest([]byte("other"))
			case "input":
				r.InputDigest = task.StageDigest([]byte("other"))
			case "stage":
				r.Stage = "extract"
			case "step":
				r.StepNo = 4
			case "missing_admission":
				st.found = false
			case "wrong_component":
				st.record.Plan.DossierSupportDigest = strings.Repeat("e", 64)
				_, st.record.Digest, _ = st.record.Plan.Encode()
			case "wrong_execution":
				st.record.Plan.ExecutionDigest = strings.Repeat("e", 64)
				_, st.record.Digest, _ = st.record.Plan.Encode()
			case "bad_record":
				st.record.Digest = strings.Repeat("e", 64)
			case "precheck":
				ctl.denyAt = 1
			case "before_call":
				ctl.denyAt = 2
			case "after_load":
				ctl.denyAt = 4
			}
			got, e := w.RunOwned(ctx, r, p)
			if e == nil || got.Review != nil || st.p.account.Frozen != 7 {
				t.Fatal("准入或身份失效仍交付")
			}
			want := int64(0)
			if mode == "after_load" {
				want = 1
			}
			if int64(srv.Count()) != want {
				t.Fatal("拒绝边界仍执行模型", srv.Count(), want)
			}
		})
	}
}

func TestDossierSupportOwnedConstruction(t *testing.T) {
	w, _, ctl, _, r, p := newSupportWorkerFixture(t, 3, supportWorkerReply("supported", "direct_support"))
	for _, mode := range []string{"tasks", "store", "model", "control", "owner", "lease"} {
		t.Run(mode, func(t *testing.T) {
			tasks, store, model, control, owner, options := w.core.tasks, w.core.store, w.core.model, AIDossierSupportControl(ctl), w.core.owner, w.core.options
			switch mode {
			case "tasks":
				tasks = nil
			case "store":
				store = nil
			case "model":
				model = nil
			case "control":
				control = nil
			case "owner":
				owner = "short"
			case "lease":
				options.Lease.Timeout = 0
			}
			if got, err := NewAIDossierSupportWorker(tasks, store, model, control, owner, options); err == nil || got != nil {
				t.Fatal("无执行依赖仍创建启用worker")
			}
		})
	}
	off := w.core.options
	off.Enabled = false
	disabled, err := NewAIDossierSupportWorker(w.core.tasks, w.core.store, nil, nil, w.core.owner, off)
	if err != nil {
		t.Fatal("默认关闭装配需要模型", err)
	}
	if got, err := disabled.RunOwned(context.Background(), r, p); !errors.Is(err, finalreport.ErrDisabled) || got.Review != nil {
		t.Fatal("关闭时仍执行")
	}
	var empty *AIDossierSupportWorker
	if got, err := empty.RunOwned(context.Background(), r, p); err == nil || got.Review != nil {
		t.Fatal("空worker可执行")
	}
}
