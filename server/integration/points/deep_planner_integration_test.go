//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
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

func TestIntegrationDeepAdmissionExecutionCoherence(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newWorkflowEnv(t, level, "pass")
			profileDigest, symbol := e.planner.p.Agent.ProfileDigest, "600001"
			maxDuration := int64(300000)
			leaseTimeout := 3 * time.Minute
			if os.Getenv("TM_TA_PRECOMPLETED") == "1" {
				taCfg := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: "http://127.0.0.1:8000", ApprovedHost: "127.0.0.1", Bearer: "local-precompleted-test-only", ApprovalRef: "local:test-only", LicenseRef: "local:self-use", QuickModel: "qwen3.7-plus", DeepModel: "qwen3.7-plus", Timeout: 30 * time.Second, MaxSubmissions: 2}
				payload, perr := os.ReadFile(os.Getenv("TM_TA_PAYLOAD"))
				if perr != nil {
					t.Fatal("读取预完成报告失败", perr)
				}
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if json.Unmarshal(payload, &envelope) == nil && len(envelope.Data) > 0 {
					payload = envelope.Data
				}
				e.agent, perr = service.NewAIAgentService(e.payloads, precompletedExecutor{remote: "30215e82-dcac-4776-b087-0136953468e9", payload: payload}, taCfg.Profile())
				if perr != nil {
					t.Fatal(perr)
				}
				profileDigest, symbol = taCfg.Profile().Digest, "000001"
			} else if taToken := os.Getenv("TM_TA_TOKEN"); taToken != "" {
				taCfg := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: "http://127.0.0.1:8000", ApprovedHost: "127.0.0.1", Bearer: taToken,
					ApprovalRef: "local:test-only", LicenseRef: "local:self-use", QuickModel: "qwen3.7-plus", DeepModel: "qwen3.7-plus", Timeout: 30 * time.Second, MaxSubmissions: 2, SelectedAnalysts: []string{"news"}, AnalysisDate: "2024-06-28"}
				client, terr := tradingcn.New(taCfg)
				if terr != nil {
					t.Fatal(terr)
				}
				t.Cleanup(client.Close)
				realAgent, terr := service.NewAIAgentService(e.payloads, client, taCfg.Profile())
				if terr != nil {
					t.Fatal(terr)
				}
				e.agent = realAgent
				profileDigest, symbol = taCfg.Profile().Digest, "000001"
				if err := task.SetMaxRecoveryTimeout(30 * time.Minute); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = task.SetMaxRecoveryTimeout(5 * time.Minute) })
				maxDuration, leaseTimeout = 1200000, 20*time.Minute
			}
			if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, e.v.LeaseOwner, e.v.Fence, "internal_failure", 0); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Millisecond)
			grant := pointGrant(now, 30)
			grant.Key, grant.BizID = "test_grant_0000003", "grant-3"
			if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
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
			modelName := "测试模型"
			var model *provider.OpenAICompat
			selectionTokens, checkTokens := 0, 0
			maxTokens := int64(12000)
			if realKey := os.Getenv("TM_REAL_MODEL_KEY"); realKey != "" {
				selectionTokens, checkTokens = 8192, 1024
				maxTokens = 200000
				modelName = os.Getenv("TM_REAL_MODEL_NAME")
				if modelName == "" {
					modelName = "deepseek-flash"
				}
				baseURL := os.Getenv("TM_REAL_BASE_URL")
				if baseURL == "" {
					baseURL = "https://api.deepseek.com"
				}
				nativeSchema := os.Getenv("TM_REAL_NATIVE_SCHEMA") == "1"
				model, err = provider.NewDirect(provider.Config{Enabled: true, Name: "real-model", BaseURL: baseURL, Model: modelName, APIKey: realKey, TimeoutMS: 240000, JSONSchemaFallback: !nativeSchema, ReasoningEffort: os.Getenv("TM_REAL_REASONING_EFFORT")}, true)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				usage := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
				upstream := providertest.NewServer(t,
					providertest.Success(string(selectionText), usage),
					checkModelReply("pass", "supported"),
					checkModelReply("pass", "factual"),
				)
				model, err = provider.NewDirect(provider.Config{Enabled: true, Name: "test-only-l2", BaseURL: upstream.URL(), Model: "测试模型", APIKey: "test-only", TimeoutMS: 5000}, true)
				if err != nil {
					t.Fatal(err)
				}
			}
			checkCfg := finalreport.CheckConfig{Model: modelName, FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: model.DirectProfileDigest(), MaxInputTokens: 24000, SelectionMaxTokens: selectionTokens, CheckMaxTokens: checkTokens, AnalysisType: task.DeepAnalysisType}
			ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, checkCfg)
			if err != nil {
				t.Fatal(err)
			}
			sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, checkCfg)
			if err != nil {
				t.Fatal(err)
			}
			policyCfg := finalreport.Config{Ref: "test-only-l2-policy", ModelName: checkCfg.Model, FilingNo: checkCfg.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: []string{e.source.Domain}, AnalysisType: task.DeepAnalysisType, MinQuotes: task.DeepMinQuotes}
			agentPlan := agenttask.Plan{UserID: 1, TaskID: 0, TaskNo: "", SubjectType: task.Theme, SubjectID: 1, ActorID: 7, Symbol: symbol, ProfileDigest: profileDigest, Units: task.Units{Calls: 1, Tokens: 100}}
			template := service.AIL1ReportPlan{Agent: agentPlan, Model: checkCfg, Policy: policyCfg, ThemeName: "示例题材", Snapshots: []sources.Snapshot{e.snap, second}, AnalysisType: task.DeepAnalysisType}
			plan, err := service.NewAIL2ReportPlan(template, points.Price{Version: "v1", Points: task.DeepPricePoints}, task.Budget{MaxAttempts: 2, MaxCalls: 10, MaxTokens: maxTokens, MaxPages: 6, MaxDurationMS: maxDuration}, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
			if err != nil {
				t.Fatal(err)
			}
			admission, err := service.NewAIL2AdmissionControl(plan)
			if err != nil {
				t.Fatal(err)
			}
			core, err := service.NewAITaskService(e.store, e.ledger)
			if err != nil {
				t.Fatal(err)
			}
			createSvc, err := service.NewAITaskCreateService(core, admission, true)
			if err != nil {
				t.Fatal(err)
			}
			limiter, err := service.NewSQLDeepLimiter(e.db)
			if err != nil {
				t.Fatal(err)
			}
			createSvc.SetDeepLimiter(limiter)
			created, err := createSvc.CreateQuoted(e.ctx, 1, "test_l2_planner_001", dto.AITaskCreateInput{AnalysisType: "deep", SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, task.DeepPricePoints)
			if err != nil {
				t.Fatal("深度创建准入失败", err)
			}
			v, err := e.tasks.Claim(e.ctx, 1, created.TaskID, "worker_l2_planner_001", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			execution, err := service.NewAIL2ExecutionPlanner(plan)
			if err != nil {
				t.Fatal(err)
			}
			w, err := service.NewAIL1ReportWorker(e.tasks, e.store, e.ledger, e.agent, e.payloads, model, execution, l2SelectionControl{}, l2CheckControl{cfg: checkCfg}, v.LeaseOwner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: leaseTimeout}, PollInterval: 50 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			runCtx := e.ctx
			if os.Getenv("TM_TA_TOKEN") != "" {
				var runCancel context.CancelFunc
				runCtx, runCancel = context.WithTimeout(context.Background(), 15*time.Minute)
				defer runCancel()
			}
			if ep, eerr := plan.Execution(v); eerr == nil {
				t.Logf("AGENTPLAN user=%d task=%d no=%q subject=%s/%d actor=%d profile=%s op=%q req=%q scope=%v", ep.Agent.UserID, ep.Agent.TaskID, ep.Agent.TaskNo, ep.Agent.SubjectType, ep.Agent.SubjectID, ep.Agent.ActorID, ep.Agent.ProfileDigest, ep.Agent.OperationKey, ep.Agent.RequestID(), ep.Agent.ValidateScope(v))
			}
			got, err := w.Run(runCtx, v.UserID, v.ID)
			if err != nil || got.Task.TaskStatus != task.Succeeded {
				rows, qerr := e.db.QueryContext(e.ctx, "SELECT step_no,stage,input_digest FROM tm_ai_task_steps WHERE task_id=? ORDER BY step_no", v.ID)
				if qerr == nil {
					defer rows.Close()
					for rows.Next() {
						var step int
						var stage, input string
						if rows.Scan(&step, &stage, &input) == nil {
							t.Logf("stage step=%d stage=%s input=%s", step, stage, input)
						}
					}
				} else {
					t.Logf("stage query failed: %v", qerr)
				}
				rrows, rerr := e.db.QueryContext(e.ctx, "SELECT seq,reservation_key,input_digest FROM tm_ai_task_reservations WHERE task_id=? ORDER BY seq", v.ID)
				if rerr == nil {
					defer rrows.Close()
					for rrows.Next() {
						var seq int
						var k, in string
						if rrows.Scan(&seq, &k, &in) == nil {
							t.Logf("reservation seq=%d key=%s input=%s", seq, k, in)
						}
					}
				} else {
					t.Logf("reservation query failed: %v", rerr)
				}
				var payload []byte
				_ = e.db.QueryRowContext(e.ctx, "SELECT payload FROM tm_ai_task_step_results WHERE task_id=? AND step_no=2", v.ID).Scan(&payload)
				if len(payload) > 1200 {
					payload = payload[:1200]
				}
				t.Logf("step2 payload=%s", string(payload))
				var judge []byte
				_ = e.db.QueryRowContext(e.ctx, "SELECT payload FROM tm_ai_task_step_results WHERE task_id=? AND step_no=4", v.ID).Scan(&judge)
				if len(judge) > 1500 {
					judge = judge[:1500]
				}
				t.Logf("step4 payload=%s", string(judge))
				var no string
				_ = e.db.QueryRowContext(e.ctx, "SELECT no FROM tm_ai_tasks WHERE id=?", v.ID).Scan(&no)
				if kb, kerr := json.Marshal([]any{"l1-final-operation-v1", int64(1), v.ID, no}); kerr == nil {
					t.Logf("want final key=%s", task.StageDigest(kb))
				}
				var cur int
				var status, code string
				_ = e.db.QueryRowContext(e.ctx, "SELECT current_step,status,error_code FROM tm_ai_tasks WHERE id=?", v.ID).Scan(&cur, &status, &code)
				t.Logf("task current_step=%d status=%s code=%s", cur, status, code)
				t.Fatal("准入与执行摘要不一致或执行失败", err, got.Task)
			}
			if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3") != 1 {
				t.Fatal("深度准入执行未结算/未恰一次报告")
			}
			l2Policy := mustPolicy(t, policyCfg)
			l1base, err := service.NewAIReportService(e.repo, e.ledger, e.reportEnv.policy, false)
			if err != nil {
				t.Fatal(err)
			}
			l2base, err := service.NewAIReportService(e.repo, e.ledger, l2Policy, false)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := service.NewAIReportReadServiceWithDeep(l1base, l2base, l2ReadControl{policy: l2Policy, taskID: v.ID, subjectID: v.SubjectID, model: modelName, filing: "test-only-filing"}, true)
			if err != nil {
				t.Fatal(err)
			}
			report, err := reader.Read(e.ctx, 1, v.ID)
			if err != nil {
				t.Fatal("深度报告不可读", err)
			}
			quotes := 0
			for _, section := range report.Sections {
				quotes += len(section.Quotes)
			}
			if len(report.Sections) != 4 || quotes < task.DeepMinQuotes {
				t.Fatal("深度报告未达四节六引文", len(report.Sections), quotes)
			}
		})
	}
}
