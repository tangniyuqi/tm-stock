package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

// 此控制器仅为测试替身，不是生产授权/硬预算/质量验收或持久审计实现。
type reportCheckControl struct {
	checks, starts, finishes int
	denyAt                   int
	startErr, finishErr      error
	hook                     func()
}

func (c *reportCheckControl) Recheck(ctx context.Context, _ task.Task, _ *finalreport.CheckPlan) error {
	c.checks++
	if c.checks == c.denyAt {
		return errPointsInjected
	}
	return ctx.Err()
}
func (c *reportCheckControl) Start(ctx context.Context, _ task.StageRequest, _ provider.Request) error {
	c.starts++
	if c.startErr != nil {
		return c.startErr
	}
	return ctx.Err()
}
func (c *reportCheckControl) Finish(ctx context.Context, _ task.StageRequest, _ provider.Response, _ error) error {
	c.finishes++
	if c.hook != nil {
		c.hook()
	}
	if c.finishErr != nil {
		return c.finishErr
	}
	return ctx.Err()
}

func reportCheckFixture(t *testing.T, steps ...providertest.Step) (*AIReportCheckWorker, *stageScript, *recoveryControl, *reportCheckControl, *providertest.Server, task.StageRequest, *finalreport.CheckPlan) {
	t.Helper()
	_, b, _ := budgetFixture(t)
	st := &stageScript{budgetScript: b, results: map[int]task.StageResult{}, steps: map[int]task.StageCheckpoint{}, stageEvents: map[int64]task.Event{}}
	f := &recoveryControl{st: st, claimed: st.v}
	control := &reportCheckControl{}
	cfg := finalreport.CheckConfig{Model: "测试判定模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", MaxInputTokens: 32}
	srv := providertest.NewServer(t, steps...)
	client, err := provider.NewDirect(provider.Config{Name: "test-only", BaseURL: srv.URL(), Model: cfg.Model, TimeoutMS: 1000}, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ProfileDigest = client.DirectProfileDigest()
	p, err := finalreport.NewSemanticCheck(cfg, st.v.No, "示例题材仅用于测试资料。")
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewAIReportCheckWorker(f, st, client, control, st.v.LeaseOwner, AIReportCheckOptions{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Second, Heartbeat: 50 * time.Millisecond, Timeout: 2 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	r := task.StageRequest{UserID: st.v.UserID, TaskID: st.v.ID, Fence: st.v.Fence, Owner: st.v.LeaseOwner, Stage: "judge", StepNo: 1, OperationKey: p.OperationKey(st.v, 1), InputDigest: p.InputDigest()}
	return w, st, f, control, srv, r, p
}
func checkSuccess() providertest.Step {
	return providertest.Success(`{"verdict":"pass","reason":"factual"}`, provider.Usage{Prompt: 2, Completion: 1, Total: 3})
}

func TestAIReportCheckOwnedLifecycleAndReplay(t *testing.T) {
	w, st, _, ctl, srv, r, p := reportCheckFixture(t, checkSuccess())
	got, err := w.RunOwned(context.Background(), r, p)
	if err != nil || got.Task.Outcome != "check_pass" || got.Evidence == nil || got.Task.ReportAvailable {
		t.Fatal(got, err)
	}
	if srv.Count() != 1 || ctl.starts != 1 || ctl.finishes != 1 || st.v.CurrentStep != 1 || st.v.Status != task.Running || st.p.account.Frozen != 7 {
		t.Fatal("判定未隔离或提前结算")
	}
	again, err := w.RunOwned(context.Background(), r, p)
	if err != nil || again.Task != got.Task || again.Evidence.Decision() != got.Evidence.Decision() || srv.Count() != 1 || ctl.starts != 1 {
		t.Fatal("回放重复调用", again, err)
	}
	b, err := json.Marshal(got)
	if err != nil || string(b) == "" {
		t.Fatal(err)
	}
	// 已有响应但未写检查点，可由新围栏复用；不能创建第二个模型调用。
	st.v.CurrentStep = 0
	delete(st.steps, 1)
	st.v.Fence++
	r.Fence = st.v.Fence
	st.v.LeaseOwner = "worker_check_new_001"
	r.Owner = st.v.LeaseOwner
	w.owner = r.Owner
	if out, err := w.RunOwned(context.Background(), r, p); err != nil || out.Task.Outcome != "check_pass" || srv.Count() != 1 {
		t.Fatal("接管不能复用已捕获响应", out, err)
	}
}

func TestAIReportCheckOwnedUnknownAndAuditFailures(t *testing.T) {
	for _, name := range []string{"start", "finish", "transport", "capture", "missing_response", "estimated", "malformed"} {
		t.Run(name, func(t *testing.T) {
			step := checkSuccess()
			if name == "transport" {
				step = providertest.Abort()
			}
			if name == "estimated" {
				step = providertest.SuccessNoUsage(`{"verdict":"pass","reason":"factual"}`)
			}
			if name == "malformed" {
				step = providertest.Success(`{"verdict":"pass","reason":"bad"}`, provider.Usage{Prompt: 2, Completion: 1, Total: 3})
			}
			w, st, _, ctl, srv, r, p := reportCheckFixture(t, step)
			switch name {
			case "start":
				ctl.startErr = errPointsInjected
			case "finish":
				ctl.finishErr = errPointsInjected
			case "capture":
				ctl.hook = func() { st.p.failAt = st.p.calls + 1 }
			case "missing_response":
				b, _ := NewAITaskBudgetService(st)
				if _, err := b.ReserveStep(context.Background(), task.ReserveRequest{UserID: r.UserID, TaskID: r.TaskID, Owner: r.Owner, Fence: r.Fence, Key: r.OperationKey, InputDigest: r.InputDigest, Units: p.Units()}, 1); err != nil {
					t.Fatal(err)
				}
			}
			out, err := w.RunOwned(context.Background(), r, p)
			if name == "estimated" || name == "malformed" {
				if err != nil || out.Task.Outcome != "check_uncertain" || out.Evidence.Decision().Verdict != finalreport.Uncertain {
					t.Fatal("未知用量/畸形返回误放行", out, err)
				}
			} else if name == "missing_response" {
				if err != nil || out.Task.Outcome != "response_unknown" || srv.Count() != 0 {
					t.Fatal(out, err)
				}
			} else if err == nil || out.Evidence != nil {
				t.Fatal("调用或审计故障返回证据", out, err)
			}
			count := srv.Count()
			ctl.startErr = nil
			ctl.finishErr = nil
			ctl.hook = nil
			st.p.failAt = 0
			replay, err := w.RunOwned(context.Background(), r, p)
			if err != nil || srv.Count() != count {
				t.Fatal("未知调用被重发", err)
			}
			if name != "estimated" && name != "malformed" && replay.Task.Outcome != "response_unknown" {
				t.Fatal("未知现场被清除", replay)
			}
			if st.p.account.Frozen != 7 {
				t.Fatal("判定步骤擅自释放冻结")
			}
		})
	}
}

func TestAIReportCheckOwnedGatesAndStorage(t *testing.T) {
	for _, name := range []string{"off", "owner", "user", "key", "input", "fence", "advance", "cancel", "admission", "pre_call_admission", "post_call_admission", "post_load_admission", "inspect", "heartbeat"} {
		t.Run(name, func(t *testing.T) {
			w, st, f, ctl, srv, r, p := reportCheckFixture(t, checkSuccess())
			switch name {
			case "user":
				r.UserID++
			case "off":
				w.options.Enabled = false
			case "owner":
				r.Owner = "worker_wrong_owner"
			case "key":
				r.OperationKey = "test_wrong_operation"
			case "input":
				r.InputDigest = task.StageDigest([]byte("changed"))
			case "fence":
				r.Fence++
			case "advance":
				r.StepNo = 2
				r.OperationKey = p.OperationKey(st.v, 2)
			case "cancel":
				st.v.CancelRequested = true
			case "admission":
				ctl.denyAt = 1
			case "pre_call_admission":
				ctl.denyAt = 2
			case "post_call_admission":
				ctl.denyAt = 3
			case "post_load_admission":
				ctl.denyAt = 4
			case "inspect":
				f.inspectError = errPointsInjected
			case "heartbeat":
				f.beatError = task.ErrLease
				f.beatFailAt = 1
			}
			got, err := w.RunOwned(context.Background(), r, p)
			if err == nil || got.Evidence != nil || got.Task != (task.RecoveryResult{}) {
				t.Fatal("门禁未关闭", got, err)
			}
			if name != "post_call_admission" && name != "post_load_admission" && srv.Count() != 0 {
				t.Fatal("拒绝前调用了模型")
			}
			if name == "off" && (ctl.checks != 0 || st.p.calls != 0) {
				t.Fatal("关闭仍访问依赖")
			}
		})
	}
	w, st, f, ctl, _, r, p := reportCheckFixture(t, checkSuccess())
	var empty *AIReportCheckWorker
	if _, err := empty.RunOwned(context.Background(), r, p); err == nil {
		t.Fatal("空worker")
	}
	if _, err := w.RunOwned(context.Background(), r, nil); err == nil {
		t.Fatal("空计划")
	}
	for _, bad := range []struct {
		tasks recoveryTasks
		store taskAgentStore
		owner string
		o     AIReportCheckOptions
	}{{nil, st, w.owner, w.options}, {f, nil, w.owner, w.options}, {f, st, "bad", w.options}, {f, st, w.owner, AIReportCheckOptions{}}} {
		if _, err := NewAIReportCheckWorker(bad.tasks, bad.store, w.model, ctl, bad.owner, bad.o); err == nil {
			t.Fatal("错误构造通过")
		}
	}
	if _, err := NewAIReportCheckWorker(f, st, nil, ctl, w.owner, w.options); err == nil {
		t.Fatal("缺模型通过")
	}
	if _, err := NewAIReportCheckWorker(f, st, w.model, nil, w.owner, w.options); err == nil {
		t.Fatal("缺控制器通过")
	}
	ordinary, err := provider.New(provider.Config{Name: "test-only", BaseURL: "https://model.example.test", Model: p.Config().Model})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAIReportCheckWorker(f, st, ordinary, ctl, w.owner, w.options); err == nil {
		t.Fatal("普通含代理能力的适配器通过")
	}
	changed := p.Config()
	changed.ProfileDigest = task.StageDigest([]byte("test-only-other-profile"))
	other, err := finalreport.NewSemanticCheck(changed, st.v.No, "示例题材仅用于测试资料。")
	if err != nil {
		t.Fatal(err)
	}
	wrong := r
	wrong.InputDigest = other.InputDigest()
	if _, err := w.RunOwned(context.Background(), wrong, other); err == nil {
		t.Fatal("其他端点配置可复用")
	}
	changed = p.Config()
	changed.Model = "另一测试模型"
	other, err = finalreport.NewSemanticCheck(changed, st.v.No, "示例题材仅用于测试资料。")
	if err != nil {
		t.Fatal(err)
	}
	wrong = r
	wrong.InputDigest = other.InputDigest()
	if _, err := w.RunOwned(context.Background(), wrong, other); err == nil {
		t.Fatal("同端点身份覆盖模型绕过固定配置")
	}
	changed = p.Config()
	foreignNo := task.StageDigest([]byte("test-only-other-task"))[:32]
	other, err = finalreport.NewSemanticCheck(changed, foreignNo, "示例题材仅用于测试资料。")
	if err != nil {
		t.Fatal(err)
	}
	wrong = r
	wrong.InputDigest = other.InputDigest()
	if _, err := w.RunOwned(context.Background(), wrong, other); err == nil {
		t.Fatal("跨任务计划进入调用控制器")
	}
	for _, phase := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			w, st, _, _, srv, r, p := reportCheckFixture(t, checkSuccess())
			st.failCommitAt = phase
			got, err := w.RunOwned(context.Background(), r, p)
			if !errors.Is(err, errPointsInjected) || got.Evidence != nil {
				t.Fatal("提交未知泄露判定", got, err)
			}
			calls := srv.Count()
			st.failCommitAt = 0
			if _, err := w.RunOwned(context.Background(), r, p); err != nil || srv.Count() != calls {
				t.Fatal("提交未知重复调用", err)
			}
		})
	}
}

func TestAITaskBudgetStepIdentity(t *testing.T) {
	b, st, r := budgetFixture(t)
	if _, err := b.ReserveStep(context.Background(), r, 0); err == nil {
		t.Fatal("无效步骤")
	}
	if _, err := b.ReserveStep(context.Background(), r, 2); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("超前预留", err)
	}
	if _, err := b.ReserveStep(context.Background(), r, 1); err != nil {
		t.Fatal(err)
	}
	wrong := r
	wrong.Key = "step_different_operation"
	if _, err := b.ReserveStep(context.Background(), wrong, 1); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("同一步换键", err)
	}
	if _, err := b.ReserveStep(context.Background(), r, 2); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("原键换步骤", err)
	}
	if _, err := b.ReserveStep(context.Background(), wrong, 2); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("未完成前序就预留", err)
	}
	st.v.CurrentStep = 1
	if _, err := b.ReserveStep(context.Background(), wrong, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReserveStep(context.Background(), r, 1); err != nil {
		t.Fatal("历史原键不能核查", err)
	}
}

func TestAIReportCheckHeartbeatAndParentCancellation(t *testing.T) {
	w, st, f, ctl, srv, r, p := reportCheckFixture(t, providertest.Hang())
	f.beatError = task.ErrLease
	f.beatFailAt = 3
	got, err := w.RunOwned(context.Background(), r, p)
	if !errors.Is(err, task.ErrLease) || got.Evidence != nil || srv.Count() != 1 || ctl.finishes != 1 || len(st.results) != 0 || st.p.account.Frozen != 7 {
		t.Fatal("心跳失败未中断网络/留审计", got, err)
	}
	f.beatError = nil
	if out, err := w.RunOwned(context.Background(), r, p); err != nil || out.Task.Outcome != "response_unknown" || srv.Count() != 1 {
		t.Fatal("失去租约后重复调用", out, err)
	}
	w, _, _, ctl, srv, r, p = reportCheckFixture(t, checkSuccess())
	ctx, cancel := context.WithCancel(context.Background())
	ctl.hook = cancel
	if out, err := w.RunOwned(ctx, r, p); !errors.Is(err, context.Canceled) || out.Evidence != nil || ctl.finishes != 1 || srv.Count() != 1 {
		t.Fatal("父取消后未保留审计/泄露判定", out, err)
	}
	ctl.hook = nil
	if out, err := w.RunOwned(context.Background(), r, p); err != nil || out.Task.Outcome != "response_unknown" || srv.Count() != 1 {
		t.Fatal("取消后补发模型", out, err)
	}
}
