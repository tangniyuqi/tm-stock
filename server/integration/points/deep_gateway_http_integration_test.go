//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 复用协作者原深度计划、会话网关及SQL限流器；只有CN和模型是回环替身。
// 人工入口由browser标签单独编译，普通用例不能因继承H5环境变量进入人工等待。
func newDeepH5Gateway(t *testing.T, level sql.IsolationLevel, origin string, browserContext context.Context) *h5ReportGateway {
	t.Helper()
	var plan *service.AIL2ReportPlan
	var checkCfg finalreport.CheckConfig
	var originalModel *providertest.Server
	e := newSubmissionHTTPEnvWithPlan(t, level, "pass", func(e *submissionHTTPEnv) {
		if browserContext != nil {
			e.pointsTestEnv.ctx = browserContext
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
		originalModel = e.server
		e.server = providertest.NewServer(t,
			providertest.Success(string(selectionText), provider.Usage{Prompt: 20, Completion: 30, Total: 50}),
			checkModelReply("pass", "supported"), checkModelReply("pass", "factual"),
		)
		e.model, err = provider.NewDirect(provider.Config{Enabled: true, Name: "test-only-l2", BaseURL: e.server.URL(), Model: "测试模型", APIKey: "test-only", TimeoutMS: 5000}, true)
		if err != nil {
			t.Fatal(err)
		}
		checkCfg = finalreport.CheckConfig{Model: "测试模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: e.model.DirectProfileDigest(), MaxInputTokens: 2048, AnalysisType: task.DeepAnalysisType}
		ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, checkCfg)
		if err != nil {
			t.Fatal(err)
		}
		sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, checkCfg)
		if err != nil {
			t.Fatal(err)
		}
		policyCfg := finalreport.Config{Ref: "test-only-l2-policy", ModelName: checkCfg.Model, FilingNo: checkCfg.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: []string{e.source.Domain}, AnalysisType: task.DeepAnalysisType, MinQuotes: task.DeepMinQuotes}
		// 未接到原检查存证时不能靠默认Pass发布报告。
		e.reportEnv.deepPolicy, err = finalreport.NewPolicy(policyCfg,
			func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
				return finalreport.Uncertain, nil
			},
			func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
		if err != nil {
			t.Fatal(err)
		}
		agentPlan := agenttask.Plan{SubjectType: task.Theme, SubjectID: 1, ActorID: 7, Symbol: "600001", ProfileDigest: e.planner.p.Agent.ProfileDigest, Units: task.Units{Calls: 1, Tokens: 100}}
		e.planner.p = service.AIL1ReportPlan{Agent: agentPlan, Model: checkCfg, Policy: policyCfg, ThemeName: "示例题材", Snapshots: []sources.Snapshot{e.snap, second}, AnalysisType: task.DeepAnalysisType}
		plan, err = service.NewAIL2ReportPlan(e.planner.p, points.Price{Version: "v1", Points: task.DeepPricePoints}, task.Budget{MaxAttempts: 2, MaxCalls: 10, MaxTokens: 12000, MaxPages: 6, MaxDurationMS: 300000}, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
		if err != nil {
			t.Fatal(err)
		}
		e.ctl.p, err = plan.Admission(task.AdmissionScope{UserID: 1, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 1})
		if err != nil {
			t.Fatal(err)
		}
		e.ctl.allowDeep.Store(true)
	})
	now := time.Now().UTC().Truncate(time.Millisecond)
	grant := pointGrant(now, task.DeepPricePoints)
	grant.Key, grant.BizID = "test_deep_gateway_grant", "deep-gateway-grant"
	if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
		t.Fatal(err)
	}
	g := newH5ReportGateway(t, e, origin)
	g.intentMode = true
	limiter, err := service.NewSQLDeepLimiter(e.db)
	if err != nil {
		t.Fatal(err)
	}
	g.intentCreator.AITaskCreateService.SetDeepLimiter(limiter)
	execution, err := service.NewAIL2ExecutionPlanner(plan)
	if err != nil {
		t.Fatal(err)
	}
	g.createHook = func(t *testing.T, id int64) {
		v := e.v
		if v.ID != id || v.AnalysisType != task.DeepAnalysisType || v.PricePoints != task.DeepPricePoints || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != int(task.DeepPricePoints) {
			t.Fatal("深度原任务类型或冻结金额不符")
		}
		w, err := service.NewAIL1ReportWorker(e.tasks, e.store, e.ledger, e.agent, e.payloads, e.model, execution, l2SelectionControl{}, l2CheckControl{cfg: checkCfg}, "worker_deep_gw_001", agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		scan := &loopObservedScanner{MemberQueueScanner: e.repo, started: make(chan struct{}), ready: make(chan struct{})}
		close(scan.ready)
		queue, err := service.NewAIL2ReportQueue(w, scan, true)
		if err != nil {
			t.Fatal(err)
		}
		ctx, stop := context.WithCancel(e.ctx)
		g.workerDone = make(chan struct{})
		go func() {
			defer close(g.workerDone)
			// 夹具绑定后启动原持续队列；后续扫描独立于HTTP通知。
			loopDone := make(chan error, 1)
			go func() { loopDone <- queue.RunLoop(ctx, 10, time.Second) }()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case err := <-loopDone:
					g.workerErr = err
					if err == nil {
						g.workerErr = points.ErrInvariant
					}
					return
				case <-ticker.C:
					current, err := e.tasks.Inspect(ctx, v.UserID, v.ID)
					if err != nil {
						stop()
						<-loopDone
						g.workerErr = err
						return
					}
					if current.Status == task.Succeeded && scan.rounds.Load() >= 4 {
						stop()
						if err := <-loopDone; !errors.Is(err, context.Canceled) {
							g.workerErr = points.ErrInvariant
						}
						return
					}
				}
			}
		}()
		t.Cleanup(func() {
			stop()
			select {
			case <-g.workerDone:
			case <-time.After(10 * time.Second):
				t.Error("深度网关worker未退出")
			}
		})
	}
	t.Cleanup(func() {
		if len(originalModel.Requests()) != 0 {
			t.Error("深度执行错误回退到原轻度模型夹具")
		}
	})
	return g
}

func finishDeepH5Gateway(t *testing.T, g *h5ReportGateway, job h5ReportAction) {
	t.Helper()
	// 控制页等待必须在失败时也释放；原process另行持有网关互斥锁。
	if job.done != nil {
		defer close(job.done)
		job.done = nil
	}
	if g.workerDone == nil {
		t.Fatal("深度会话未启动原worker")
	}
	select {
	case <-g.workerDone:
		if g.workerErr != nil {
			t.Fatal("深度原worker执行失败", g.workerErr)
		}
	case <-g.e.ctx.Done():
		t.Fatal("深度原worker未在原上下文内完成")
	}
	e := g.e
	if e.v.AnalysisType != task.DeepAnalysisType || e.v.PricePoints != task.DeepPricePoints {
		t.Fatal("结束时不是原深度任务")
	}
	before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
	for i := 0; i < 2; i++ {
		report, err := e.web.reader.Read(e.ctx, 1, e.v.ID)
		if err != nil {
			t.Fatal("原注册读取器拒绝深度报告", err)
		}
		quotes := 0
		for _, section := range report.Sections {
			quotes += len(section.Quotes)
		}
		if report.TaskID != e.v.ID || len(report.Sections) != 4 || quotes != task.DeepMinQuotes {
			t.Fatal("深度报告未满足四节六条原引文")
		}
	}
	if before != e.queryInt("SELECT COUNT(*) FROM tm_points_txns") || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 {
		t.Fatal("原报告重读改变账本或深度金额未恰好结算")
	}
	g.process(t, job, "success")
	t.Log("深度原会话意图→持续数据库队列→异步worker→原读取器四节六引文；至少四轮扫描后取消退出，30测试积分恰好一次结算、冻结0、重读零改账")
}

func TestIntegrationDeepH5GatewayHTTP(t *testing.T) {
	// 普通自动化不接外部H5，也不继承其它浏览器专项的登录/故障/问答开关。
	for _, key := range []string{"TM_TEST_AI_H5_ORIGIN", "TM_TEST_AI_H5_LOGIN", "TM_TEST_AI_H5_LOGIN_RUNNER", "TM_TEST_AI_H5_INTENTS", "TM_TEST_AI_H5_REPORT_QA", "TM_TEST_AI_H5_QA_HISTORY", "TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_ASSISTANT"} {
		t.Setenv(key, "")
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "仅自动化深度网关，非实际H5") }))
			t.Cleanup(front.Close)
			g := newDeepH5Gateway(t, level, front.URL, nil)
			e := g.e
			token, foreign := e.web.token(t, 1, "client"), e.web.token(t, 2, "client")
			var created struct {
				Data dto.AIConversationReceipt `json:"data"`
			}
			if json.Unmarshal(h5GatewayRequest(t, g, "POST", "/api/v1/ai/conversations", token, "deep_gw_conversation_key", `{"title":"仅测试深度会话"}`, 200), &created) != nil || created.Data.ID <= 0 {
				t.Fatal("深度网关会话创建失败")
			}
			base := fmt.Sprintf("/api/v1/ai/conversations/%d", created.Data.ID)
			var prepared struct {
				Data dto.AIConversationTask `json:"data"`
			}
			if json.Unmarshal(h5GatewayRequest(t, g, "POST", base+"/task-intents", token, "deep_gw_intent_key", `{"analysis_type":"deep","subject_type":"theme","subject_id":1,"price_version":"v1"}`, 200), &prepared) != nil || prepared.Data.State != "prepared" || prepared.Data.AnalysisType != "deep" || prepared.Data.PricePoints != task.DeepPricePoints || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("深度网关意图准备或零冻结不符")
			}
			var linked struct {
				Data dto.AIConversationTask `json:"data"`
			}
			confirmPath := fmt.Sprintf("%s/task-intents/%d/confirm", base, prepared.Data.ID)
			if json.Unmarshal(h5GatewayRequest(t, g, "POST", confirmPath, token, "", `{"confirmed":true}`, 200), &linked) != nil || linked.Data.State != "linked" || linked.Data.TaskID <= 0 {
				t.Fatal("深度网关确认失败")
			}
			select {
			case action := <-g.actions:
				g.process(t, action, "deep")
			case <-e.ctx.Done():
				t.Fatal("深度网关未产生创建动作")
			}
			finishDeepH5Gateway(t, g, h5ReportAction{kind: "finish"})
			path := fmt.Sprintf("/api/v1/ai/tasks/%d", linked.Data.TaskID)
			h5GatewayRequest(t, g, "GET", path+"/report", foreign, "", "", 404)
			h5GatewayRequest(t, g, "GET", base+"/messages", foreign, "", "", 404)
			var report struct {
				Data dto.AIReport `json:"data"`
			}
			if json.Unmarshal(h5GatewayRequest(t, g, "GET", path+"/report", token, "", "", 200), &report) != nil || len(report.Data.Sections) != 4 {
				t.Fatal("原公开接口未交付深度报告")
			}
		})
	}
}
