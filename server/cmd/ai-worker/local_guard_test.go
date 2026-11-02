package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 只验证命令层转发；不模拟数据库保护或模型语义。
type localGuardProbe struct {
	t        *testing.T
	calls    []string
	ctx      context.Context
	task     task.Task
	plan     service.AIL1ReportPlan
	request  task.StageRequest
	prompt   provider.Request
	response provider.Response
	callErr  error
}

func (p *localGuardProbe) hit(ctx context.Context, name string) {
	p.t.Helper()
	if ctx != p.ctx {
		p.t.Fatal("转发丢失原上下文")
	}
	p.calls = append(p.calls, name)
}
func (p *localGuardProbe) Plan(ctx context.Context, v task.Task) (service.AIL1ReportPlan, error) {
	p.hit(ctx, "plan")
	if v != p.task {
		p.t.Fatal("计划任务变更")
	}
	return p.plan, points.ErrInvariant
}
func (p *localGuardProbe) Recheck(ctx context.Context, v task.Task, plan service.AIL1ReportPlan) error {
	p.hit(ctx, "recheck")
	if v != p.task || plan.ThemeName != p.plan.ThemeName {
		p.t.Fatal("复核参数变更")
	}
	return points.ErrInvariant
}
func (p *localGuardProbe) MemberQueueRange(ctx context.Context) (task.MemberQueueRange, error) {
	p.hit(ctx, "range")
	return task.MemberQueueRange{UpperID: 9}, points.ErrInvariant
}
func (p *localGuardProbe) MemberQueuePage(ctx context.Context, after, upper int64, limit int) ([]task.MemberQueueCandidate, error) {
	p.hit(ctx, "page")
	if after != 3 || upper != 9 || limit != 2 {
		p.t.Fatal("分页边界被改写")
	}
	return []task.MemberQueueCandidate{{ID: 7}}, points.ErrInvariant
}
func (p *localGuardProbe) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	p.hit(ctx, "start")
	if r != p.request || req.Prompt != p.prompt.Prompt {
		p.t.Fatal("调用开始参数变更")
	}
	return points.ErrInvariant
}
func (p *localGuardProbe) Finish(ctx context.Context, r task.StageRequest, out provider.Response, err error) error {
	p.hit(ctx, "finish")
	if r != p.request || out.Text != p.response.Text || err != p.callErr {
		p.t.Fatal("已返回回执或原错误丢失")
	}
	return points.ErrInvariant
}

func localGuardFiles(t *testing.T, change func(*config.AILocalPlans, *config.AILocalExecution)) (*localExecutionGuard, string, string) {
	t.Helper()
	r := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: strings.Repeat("a", 32), Database: "tm_ai_local_" + strings.Repeat("a", 32), Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
	now := time.Now().UnixMilli()
	c := config.AILocalExecution{Version: 1, InstanceID: r.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: now - 1000, ExpiresAtMS: now + 60000, Model: config.AILocalExecutionModel{Name: "local-eval", BaseURL: "http://127.0.0.1:8123", Model: "仅测试:模型", TimeoutMS: 3000}, Agent: config.AILocalExecutionAgent{BaseURL: "http://127.0.0.1:8124", ApprovalRef: "local-eval:agent", LicenseRef: "local-eval:license", QuickModel: "local-quick", DeepModel: "local-deep", TimeoutMS: 3000, MaxSubmissions: 10}, LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 30000, AgentPollMS: 50}
	model, err := provider.NewDirect(c.ModelConfig(), true)
	if err != nil {
		t.Fatal(err)
	}
	p := config.AILocalPlan{AnalysisType: 1, SubjectID: 1, SubjectName: "仅测试:题材", ActorID: 7, Symbol: "000001", AgentProfileDigest: strings.Repeat("b", 64), AgentCalls: 1, AgentTokens: 100, ModelName: c.Model.Model, ModelProfileDigest: strings.Repeat("c", 64), FilingNo: "仅测试:公示编号", ModelVersion: "local-eval:v1", MaxInputTokens: 32, SelectionMaxTokens: 2048, CheckMaxTokens: 64, PolicyRef: "local-eval:policy", AllowedDomains: []string{"evidence.example.test"}, ApprovalRef: "local-eval:approval", QualityRef: "local-eval:quality", HardBudgetRef: "local-eval:hard-budget", PriceRef: "local-eval:price", SourceRef: "local-eval:source", PriceVersion: "local-v1", PricePoints: 7, Budget: task.Budget{MaxAttempts: 1, MaxCalls: 6, MaxTokens: 5000, MaxPages: 10, MaxDurationMS: 60000}, Materials: []config.AILocalMaterial{{SnapshotID: 1, Digest: strings.Repeat("d", 64), LicenseRef: "local-eval:license"}}}
	plans := config.AILocalPlans{Version: 1, InstanceID: r.InstanceID, Revision: "v1", Scope: "synthetic-local-only", IssuedAtMS: now - 1000, ExpiresAtMS: now + 60000, Members: []int64{1, 2}, Plans: []config.AILocalPlan{p}}
	p.AgentProfileDigest, p.ModelProfileDigest = c.AgentConfig("").Profile().Digest, model.DirectProfileDigest()
	plans.Plans[0] = p
	if change != nil {
		change(&plans, &c)
	}
	dir := t.TempDir()
	planFile, executionFile := filepath.Join(dir, "plans.json"), filepath.Join(dir, "execution.json")
	for file, value := range map[string]any{planFile: plans, executionFile: c} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ps, err := config.NewAILocalPlanSource(planFile, r)
	if err != nil {
		t.Fatal(err)
	}
	es, err := config.NewAILocalExecutionSource(executionFile, r)
	if err != nil {
		t.Fatal(err)
	}
	return &localExecutionGuard{execution: es, plans: ps, modelName: p.ModelName, modelProfile: p.ModelProfileDigest, agentProfile: p.AgentProfileDigest}, planFile, executionFile
}

func TestLocalWorkerGuardForwarding(t *testing.T) {
	g, _, execution := localGuardFiles(t, nil)
	ctx := context.Background()
	p := &localGuardProbe{t: t, ctx: ctx, task: task.Task{ID: 7, UserID: 1}, plan: service.AIL1ReportPlan{ThemeName: "仅测试:题材"}, request: task.StageRequest{TaskID: 7}, prompt: provider.Request{Prompt: "仅测试:请求"}, response: provider.Response{Text: "仅测试:已返回回执"}, callErr: errors.New("仅测试:传输未知")}
	g.planner, g.repo, g.audit = p, p, p
	if plan, err := g.Plan(ctx, p.task); !errors.Is(err, points.ErrInvariant) || plan.ThemeName != p.plan.ThemeName {
		t.Fatal("计划错误被吞或结果变更")
	}
	if err := g.Recheck(ctx, p.task, p.plan); !errors.Is(err, points.ErrInvariant) {
		t.Fatal("复核错误被吞")
	}
	if v, err := g.MemberQueueRange(ctx); !errors.Is(err, points.ErrInvariant) || v.UpperID != 9 {
		t.Fatal("范围错误或结果变更")
	}
	if v, err := g.MemberQueuePage(ctx, 3, 9, 2); !errors.Is(err, points.ErrInvariant) || len(v) != 1 || v[0].ID != 7 {
		t.Fatal("分页错误或结果变更")
	}
	if err := g.Start(ctx, p.request, p.prompt); !errors.Is(err, points.ErrInvariant) {
		t.Fatal("开始审计错误被吞")
	}
	if strings.Join(p.calls, ",") != "plan,recheck,range,page,start" {
		t.Fatal("转发次数错误")
	}
	if err := os.WriteFile(execution, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.Finish(ctx, p.request, p.response, p.callErr); !errors.Is(err, points.ErrInvariant) || len(p.calls) != 6 || p.calls[5] != "finish" {
		t.Fatal("失效后丢弃已返回调用的存证")
	}
}

func TestLocalWorkerGuardRejectsBeforeForwarding(t *testing.T) {
	for _, mode := range []string{"execution_removed", "plans_removed", "execution_changed", "plans_changed", "execution_lifetime", "model_name", "model_profile", "agent_profile", "worker_timeout", "lease_timeout", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			g, planFile, executionFile := localGuardFiles(t, func(p *config.AILocalPlans, c *config.AILocalExecution) {
				if mode == "execution_lifetime" {
					p.ExpiresAtMS -= 1000
				}
				if mode == "worker_timeout" {
					c.WorkerTimeoutMS = 70000
				}
				if mode == "lease_timeout" {
					c.LeaseMS = 70000
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p := &localGuardProbe{t: t, ctx: ctx}
			g.planner, g.repo, g.audit = p, p, p
			switch mode {
			case "execution_removed":
				if err := os.Remove(executionFile); err != nil {
					t.Fatal(err)
				}
			case "plans_removed":
				if err := os.Remove(planFile); err != nil {
					t.Fatal(err)
				}
			case "execution_changed":
				raw, err := os.ReadFile(executionFile)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(executionFile, []byte(strings.Replace(string(raw), `"revision":"v1"`, `"revision":"v2"`, 1)), 0600); err != nil {
					t.Fatal(err)
				}
			case "plans_changed":
				raw, err := os.ReadFile(planFile)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(planFile, []byte(strings.Replace(string(raw), `"revision":"v1"`, `"revision":"v2"`, 1)), 0600); err != nil {
					t.Fatal(err)
				}
			case "model_name":
				g.modelName = "另一个测试模型"
			case "model_profile":
				g.modelProfile = strings.Repeat("e", 64)
			case "agent_profile":
				g.agentProfile = strings.Repeat("e", 64)
			case "cancelled":
				cancel()
			}
			if _, err := g.Plan(ctx, task.Task{}); err == nil {
				t.Fatal("失效仍转发计划")
			}
			if err := g.Recheck(ctx, task.Task{}, service.AIL1ReportPlan{}); err == nil {
				t.Fatal("失效仍转发复核")
			}
			if _, err := g.MemberQueueRange(ctx); err == nil {
				t.Fatal("失效仍扫描范围")
			}
			if _, err := g.MemberQueuePage(ctx, 3, 9, 2); err == nil {
				t.Fatal("失效仍扫描分页")
			}
			if err := g.Start(ctx, task.StageRequest{}, provider.Request{}); err == nil {
				t.Fatal("失效仍开始调用")
			}
			if len(p.calls) != 0 {
				t.Fatal("拒绝后仍触达下游")
			}
		})
	}
}
