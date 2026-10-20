package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type l1TestPlanner struct {
	p   AIL1ReportPlan
	err error
}

func (p l1TestPlanner) Plan(context.Context, task.Task) (AIL1ReportPlan, error)  { return p.p, p.err }
func (p l1TestPlanner) Recheck(context.Context, task.Task, AIL1ReportPlan) error { return p.err }

type l1TestPayloadReader struct{}

func (l1TestPayloadReader) Quarantined(context.Context, int64, string) (agents.Record, agents.Result, error) {
	return agents.Record{}, agents.Result{}, agents.ErrNotFound
}

type l1TestSelectionControl struct{ *reportCheckControl }

func (c l1TestSelectionControl) Recheck(ctx context.Context, _ task.Task, _ *finalreport.SelectionModelPlan) error {
	return ctx.Err()
}

func l1PlanFixture(t *testing.T) (AIL1ReportPlan, task.Task, *provider.OpenAICompat) {
	t.Helper()
	s, st, _ := reportFixture(t)
	check, _, _, _, _, _, _ := reportCheckFixture(t)
	v := st.v
	v.Budget.MaxCalls = 6
	v.Budget.MaxTokens = 5000
	cfg := finalreport.CheckConfig{Model: "测试判定模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: check.model.DirectProfileDigest(), MaxInputTokens: 32}
	ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, cfg)
	if err != nil {
		t.Fatal(err)
	}
	sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a := agenttask.Plan{UserID: v.UserID, TaskID: v.ID, TaskNo: v.No, SubjectType: v.SubjectType, SubjectID: v.SubjectID, ActorID: 7, Symbol: "600001", ProfileDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}}
	a.OperationKey = a.RequestID()
	p := AIL1ReportPlan{Agent: a, Model: cfg, Policy: finalreport.Config{Ref: "test-only-policy", ModelName: cfg.Model, FilingNo: cfg.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: []string{"evidence.example.test"}}, ThemeName: st.theme, Snapshots: append([]sources.Snapshot(nil), st.snaps...)}
	if policy, err := p.validate(v, check.model); err != nil || !policy.Valid() {
		t.Fatal(s, err)
	}
	return p, v, check.model
}
func TestAIL1ReportPlanFailClosed(t *testing.T) {
	for _, name := range []string{"agent", "type", "model", "disclosure", "filing", "meta", "versions", "budget", "policy", "source"} {
		t.Run(name, func(t *testing.T) {
			p, v, model := l1PlanFixture(t)
			switch name {
			case "agent":
				p.Agent.ActorID = 0
			case "type":
				v.AnalysisType = 2
			case "model":
				model = nil
			case "disclosure":
				p.Policy.ModelName = "其他模型"
			case "filing":
				p.Policy.FilingNo = "different"
			case "meta":
				p.Model.QualityRef = ""
			case "versions":
				p.Policy.SemanticVersion = "other"
			case "budget":
				v.Budget.MaxCalls = 4
			case "policy":
				p.Policy.Ref = ""
			case "source":
				p.Snapshots = nil
			}
			if _, err := p.validate(v, model); err == nil {
				t.Fatal("计划未关闭")
			}
		})
	}
	p, v, _ := l1PlanFixture(t)
	adapter := l1AgentPlanner{l1TestPlanner{p: p}, p}
	if a, err := adapter.Plan(context.Background(), v); err != nil || a != p.Agent {
		t.Fatal(a, err)
	}
	if adapter.Recheck(context.Background(), v, p.Agent) != nil {
		t.Fatal("合法身份未通过")
	}
	a := p.Agent
	a.ActorID++
	if adapter.Recheck(context.Background(), v, a) == nil {
		t.Fatal("换计划")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", p, p), p.Snapshots[0].Body) {
		t.Fatal("计划正文泄漏")
	}
}
func TestAIL1ReportWorkerClosedConstructionAndEarlyGates(t *testing.T) {
	s, st, _ := reportFixture(t)
	f := &recoveryControl{st: st.stageScript}
	o := agenttask.Options{Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: time.Second}, PollInterval: 50 * time.Millisecond}
	w, err := NewAIL1ReportWorker(f, st, s.core.ledger, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Run(context.Background(), st.v.UserID, st.v.ID); !errors.Is(err, finalreport.ErrDisabled) {
		t.Fatal(err)
	}
	var missing *AIL1ReportWorker
	if _, err := missing.Run(context.Background(), 1, 1); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := w.Run(context.Background(), 0, 1); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	bad := o
	bad.Enabled = true
	if _, err := NewAIL1ReportWorker(f, st, s.core.ledger, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, bad); err == nil {
		t.Fatal("缺依赖启用")
	}
	bad = o
	bad.Lease.Lease = time.Second
	if _, err := NewAIL1ReportWorker(f, st, s.core.ledger, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, bad); err == nil {
		t.Fatal("终检租约不足")
	}
	if _, err := NewAIL1ReportWorker(nil, st, s.core.ledger, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, o); err == nil {
		t.Fatal("无任务服务")
	}
	if _, err := NewAIL1ReportWorker(f, nil, s.core.ledger, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, o); err == nil {
		t.Fatal("无仓储")
	}
	if _, err := NewAIL1ReportWorker(f, st, nil, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, o); err == nil {
		t.Fatal("无账本")
	}
	// 已终态不再向规划/外部依赖取数；这是内部状态回放，不宣称报告已可供会员读取。
	w.opts.Enabled = true
	st.v.Status = task.Succeeded
	got, err := w.Run(context.Background(), st.v.UserID, st.v.ID)
	if err != nil || got.Task.TaskStatus != task.Succeeded || got.Task.ReportAvailable {
		t.Fatal(got, err)
	}
	if _, err := NewAIReportSelectionWorker(f, st, nil, nil, st.v.LeaseOwner, AIReportCheckOptions{Enabled: true, Lease: o.Lease}); err == nil {
		t.Fatal("选择控制器缺失")
	}
	sw, err := NewAIReportSelectionWorker(f, st, nil, nil, st.v.LeaseOwner, AIReportCheckOptions{Lease: o.Lease})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sw.RunOwned(context.Background(), task.StageRequest{}, nil); !errors.Is(err, finalreport.ErrDisabled) {
		t.Fatal(err)
	}
	var noSelector *AIReportSelectionWorker
	if _, err := noSelector.RunOwned(context.Background(), task.StageRequest{}, nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.ConvertModel(context.Background(), task.StageRequest{}, nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("空模型转换", err)
	}
}
