//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"testing"
	"time"

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

type l2Planner struct{ p service.AIL1ReportPlan }

func (p l2Planner) Plan(context.Context, task.Task) (service.AIL1ReportPlan, error)  { return p.p, nil }
func (p l2Planner) Recheck(context.Context, task.Task, service.AIL1ReportPlan) error { return nil }

type l2SelectionControl struct{}

func (l2SelectionControl) Recheck(context.Context, task.Task, *finalreport.SelectionModelPlan) error {
	return nil
}
func (l2SelectionControl) Start(context.Context, task.StageRequest, provider.Request) error {
	return nil
}
func (l2SelectionControl) Finish(context.Context, task.StageRequest, provider.Response, error) error {
	return nil
}

type l2CheckControl struct{ cfg finalreport.CheckConfig }

func (c l2CheckControl) Recheck(ctx context.Context, _ task.Task, p *finalreport.CheckPlan) error {
	if p.Config() != c.cfg {
		return errCheckTest
	}
	return ctx.Err()
}
func (l2CheckControl) Start(context.Context, task.StageRequest, provider.Request) error { return nil }
func (l2CheckControl) Finish(context.Context, task.StageRequest, provider.Response, error) error {
	return nil
}

func mustPolicy(t *testing.T, c finalreport.Config) *finalreport.Policy {
	t.Helper()
	p, err := finalreport.NewPolicy(c, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Pass, nil
	}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Pass, nil })
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type l2ReadControl struct {
	policy            *finalreport.Policy
	taskID, subjectID int64
	model, filing     string
}

func (c l2ReadControl) Recheck(_ context.Context, p service.AIReportPublication) error {
	if p.UserID != 1 || p.TaskID != c.taskID || p.SubjectID != c.subjectID || !finalreport.ValidDigest(p.ReportDigest) || p.PolicyDigest != c.policy.Digest() || p.ModelName != c.model || p.FilingNo != c.filing {
		return finalreport.ErrUnavailable
	}
	return nil
}

func TestIntegrationAIL2ReportWorkflow(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		for _, mode := range []string{"pass", "block"} {
			t.Run(level.String()+"/"+mode, func(t *testing.T) {
				e := newWorkflowEnv(t, level, "pass")
				// 释放夹具自带的 L1 任务，另建类型 2 任务，避免改动 L1 夹具语义。
				if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, e.v.LeaseOwner, e.v.Fence, "internal_failure", 0); err != nil {
					t.Fatal(err)
				}
				if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
					t.Fatal("L1夹具冻结未释放")
				}
				now := time.Now().UTC().Truncate(time.Millisecond)
				grant := pointGrant(now, 30)
				grant.Key, grant.BizID = "test_grant_"+strconv.FormatInt(now.UnixNano(), 10), "grant-"+strconv.FormatInt(now.UnixNano(), 10)
				if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
					t.Fatal(err)
				}
				budget := taskBudget()
				budget.MaxCalls, budget.MaxTokens, budget.MaxDurationMS = 10, 12000, 300000
				created, err := e.tasks.Create(e.ctx, task.CreateRequest{UserID: 1, Key: "test_l2_request_" + strconv.FormatInt(time.Now().UnixNano(), 10), AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, points.Price{Version: "v1", Points: task.DeepPricePoints}, budget)
				if err != nil {
					t.Fatal("深度任务创建失败", err)
				}
				v, err := e.tasks.Claim(e.ctx, 1, created.TaskID, "worker_l2_test_001", time.Minute)
				if err != nil {
					t.Fatal(err)
				}

				second, err := sources.BuildSnapshot(e.source, "https://evidence.example.test/second", sources.MediaPlain, []byte("示例题材的第二段资料。示例题材的第二项事实。"), e.snap.FetchedAt, 1<<20)
				if err != nil {
					t.Fatal(err)
				}
				second, err = repository.NewAISourceRepo(e.db).SaveSnapshot(e.ctx, second)
				if err != nil {
					t.Fatal(err)
				}
				selections := finalreport.SelectionResponse{Selections: []finalreport.Selection{
					{Section: render.SecDefinition, SnapshotID: e.snap.ID, SentenceIDs: []int{1, 2}},
					{Section: render.SecTimeline, SnapshotID: e.snap.ID, SentenceIDs: []int{3, 4}},
					{Section: render.SecSegment, SnapshotID: second.ID, SentenceIDs: []int{1}},
					{Section: render.SecCompanies, SnapshotID: second.ID, SentenceIDs: []int{2}},
				}}
				selectionText, err := json.Marshal(selections)
				if err != nil {
					t.Fatal(err)
				}
				usage := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
				steps := []providertest.Step{providertest.Success(string(selectionText), usage), checkModelReply("pass", "supported")}
				if mode == "block" {
					steps[1] = checkModelReply("block", "unsupported")
				} else {
					steps = append(steps, checkModelReply("pass", "factual"))
				}
				upstream := providertest.NewServer(t, steps...)
				model, err := provider.NewDirect(provider.Config{Enabled: true, Name: "test-only-l2", BaseURL: upstream.URL(), Model: "测试模型", APIKey: "test-only", TimeoutMS: 5000}, true)
				if err != nil {
					t.Fatal(err)
				}
				checkCfg := finalreport.CheckConfig{Model: "测试模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: model.DirectProfileDigest(), MaxInputTokens: 32, AnalysisType: task.DeepAnalysisType}
				ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, checkCfg)
				if err != nil {
					t.Fatal(err)
				}
				sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, checkCfg)
				if err != nil {
					t.Fatal(err)
				}
				policyCfg := finalreport.Config{Ref: "test-only-l2-policy", ModelName: checkCfg.Model, FilingNo: checkCfg.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: []string{e.source.Domain}, AnalysisType: task.DeepAnalysisType, MinQuotes: task.DeepMinQuotes}
				agentPlan := agenttask.Plan{UserID: v.UserID, TaskID: v.ID, TaskNo: v.No, SubjectType: v.SubjectType, SubjectID: v.SubjectID, ActorID: 7, Symbol: "600001", ProfileDigest: e.planner.p.Agent.ProfileDigest, Units: task.Units{Calls: 1, Tokens: 100}}
				agentPlan.OperationKey = agentPlan.RequestID()
				plan := service.AIL1ReportPlan{Agent: agentPlan, Model: checkCfg, Policy: policyCfg, ThemeName: "示例题材", Snapshots: []sources.Snapshot{e.snap, second}, AnalysisType: task.DeepAnalysisType}
				w, err := service.NewAIL1ReportWorker(e.tasks, e.store, e.ledger, e.agent, e.payloads, model, l2Planner{plan}, l2SelectionControl{}, l2CheckControl{cfg: checkCfg}, v.LeaseOwner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				txnsBefore := e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE user_id=1")
				got, err := w.Run(e.ctx, v.UserID, v.ID)
				if err != nil {
					t.Fatal("L2执行失败", err)
				}
				if mode == "block" {
					if got.Task.TaskStatus != task.Failed {
						t.Fatal("L2拒绝未失败", got.Task)
					}
					if upstream.Count() != 2 {
						t.Fatal("L2拒绝模型调用次数不符", upstream.Count())
					}
					if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
						t.Fatal("L2失败后仍有冻结")
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=4") != 1 {
						t.Fatal("L2失败未留不可变拒绝收据")
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE user_id=1") != txnsBefore+1 {
						t.Fatal("L2失败释放流水不是恰好一次")
					}
					return
				}
				if got.Task.TaskStatus != task.Succeeded {
					t.Fatal("L2任务未成功", got.Task)
				}
				if upstream.Count() != 3 {
					t.Fatal("L2模型调用次数不符", upstream.Count())
				}
				if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
					t.Fatal("L2结算后仍有冻结")
				}
				// 读取服务必须使用与深度报告一致的策略摘要与任务绑定，不能用 L1 策略/旧任务冒充。
				l2Policy := mustPolicy(t, policyCfg)
				base, err := service.NewAIReportService(e.repo, e.ledger, l2Policy, false)
				if err != nil {
					t.Fatal(err)
				}
				reader, err := service.NewAIReportReadService(base, l2ReadControl{policy: l2Policy, taskID: v.ID, subjectID: v.SubjectID, model: "测试模型", filing: "test-only-filing"}, true)
				if err != nil {
					t.Fatal(err)
				}
				report, err := reader.Read(e.ctx, 1, v.ID)
				if err != nil {
					t.Fatal("L2报告不可读", err)
				}
				quotes := 0
				for _, section := range report.Sections {
					quotes += len(section.Quotes)
				}
				if len(report.Sections) != 4 || quotes < task.DeepMinQuotes {
					t.Fatal("L2报告未达四节六引文", len(report.Sections), quotes)
				}
				if e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE user_id=1") != txnsBefore+1 {
					t.Fatal("L2结算流水不是恰好一次")
				}
			})
		}
	}
}
