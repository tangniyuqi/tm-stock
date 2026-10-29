//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
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

type deepWorkflowConfig struct {
	profileDigest, symbol, modelName    string
	maxDuration, maxTokens, inputTokens int64
	leaseTimeout                        time.Duration
	selectionTokens, checkTokens        int
	model                               *provider.OpenAICompat
}

func TestIntegrationDeepAdmissionExecutionCoherence(t *testing.T) {
	// 即使父会话曾配置真实联调，普通回归也不能按环境凭据切换执行器。
	t.Setenv("TM_TA_TOKEN", "test-only-must-not-be-used")
	t.Setenv("TM_REAL_MODEL_KEY", "test-only-must-not-be-used")
	t.Setenv("TM_REAL_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("TM_TA_PRECOMPLETED", "1")
	t.Setenv("TM_TA_PAYLOAD", "test-only-must-not-be-read.json")
	runDeepAdmissionExecution(t, []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead}, nil)
}

// 普通入口只构造回环替身；真实装配仅在 integration && liveagents 文件中实现并显式调用。
func runDeepAdmissionExecution(t *testing.T, levels []sql.IsolationLevel, setup func(*testing.T, *workflowEnv, *deepWorkflowConfig)) {
	runDeepAdmissionExecutionMode(t, levels, setup, false)
}

func TestIntegrationAIReportDeepQueue(t *testing.T) {
	runDeepAdmissionExecutionMode(t, []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead}, nil, true)
}

func runDeepAdmissionExecutionMode(t *testing.T, levels []sql.IsolationLevel, setup func(*testing.T, *workflowEnv, *deepWorkflowConfig), queued bool) {
	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			e := newWorkflowEnv(t, level, "pass")
			cfg := deepWorkflowConfig{profileDigest: e.planner.p.Agent.ProfileDigest, symbol: "600001", modelName: "测试模型", maxDuration: 300000, leaseTimeout: 3 * time.Minute, maxTokens: 12000, inputTokens: 2048}
			if setup != nil {
				setup(t, e, &cfg)
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
			if cfg.model == nil {
				usage := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
				upstream := providertest.NewServer(t,
					providertest.Success(string(selectionText), usage),
					checkModelReply("pass", "supported"),
					checkModelReply("pass", "factual"),
				)
				cfg.model, err = provider.NewDirect(provider.Config{Enabled: true, Name: "test-only-l2", BaseURL: upstream.URL(), Model: "测试模型", APIKey: "test-only", TimeoutMS: 5000}, true)
				if err != nil {
					t.Fatal(err)
				}
			}
			model := cfg.model
			checkCfg := finalreport.CheckConfig{Model: cfg.modelName, FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: model.DirectProfileDigest(), MaxInputTokens: cfg.inputTokens, SelectionMaxTokens: cfg.selectionTokens, CheckMaxTokens: cfg.checkTokens, AnalysisType: task.DeepAnalysisType}
			ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, checkCfg)
			if err != nil {
				t.Fatal(err)
			}
			sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, checkCfg)
			if err != nil {
				t.Fatal(err)
			}
			policyCfg := finalreport.Config{Ref: "test-only-l2-policy", ModelName: checkCfg.Model, FilingNo: checkCfg.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: []string{e.source.Domain}, AnalysisType: task.DeepAnalysisType, MinQuotes: task.DeepMinQuotes}
			agentPlan := agenttask.Plan{UserID: 1, TaskID: 0, TaskNo: "", SubjectType: task.Theme, SubjectID: 1, ActorID: 7, Symbol: cfg.symbol, ProfileDigest: cfg.profileDigest, Units: task.Units{Calls: 1, Tokens: 100}}
			template := service.AIL1ReportPlan{Agent: agentPlan, Model: checkCfg, Policy: policyCfg, ThemeName: "示例题材", Snapshots: []sources.Snapshot{e.snap, second}, AnalysisType: task.DeepAnalysisType}
			plan, err := service.NewAIL2ReportPlan(template, points.Price{Version: "v1", Points: task.DeepPricePoints}, task.Budget{MaxAttempts: 2, MaxCalls: 10, MaxTokens: cfg.maxTokens, MaxPages: 6, MaxDurationMS: cfg.maxDuration}, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
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
			v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
			if !queued && err == nil {
				v, err = e.tasks.Claim(e.ctx, 1, created.TaskID, "worker_l2_planner_001", time.Minute)
			}
			if err != nil {
				t.Fatal(err)
			}
			execution, err := service.NewAIL2ExecutionPlanner(plan)
			if err != nil {
				t.Fatal(err)
			}
			w, err := service.NewAIL1ReportWorker(e.tasks, e.store, e.ledger, e.agent, e.payloads, model, execution, l2SelectionControl{}, l2CheckControl{cfg: checkCfg}, "worker_l2_planner_001", agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: cfg.leaseTimeout}, PollInterval: 50 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			if ep, eerr := plan.Execution(v); eerr == nil {
				t.Logf("AGENTPLAN user=%d task=%d no=%q subject=%s/%d actor=%d profile=%s op=%q req=%q scope=%v", ep.Agent.UserID, ep.Agent.TaskID, ep.Agent.TaskNo, ep.Agent.SubjectType, ep.Agent.SubjectID, ep.Agent.ActorID, ep.Agent.ProfileDigest, ep.Agent.OperationKey, ep.Agent.RequestID(), ep.Agent.ValidateScope(v))
			}
			if queued {
				q, qerr := service.NewAIL2ReportQueue(w, e.repo, true)
				if qerr != nil {
					t.Fatal(qerr)
				}
				out, qerr := q.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 10})
				if qerr != nil || !out.Complete || out.TerminalObserved != 1 || len(out.Items) != 1 || out.Items[0].TaskID != v.ID || out.Items[0].Status != task.Succeeded {
					t.Fatal("深度队列未交付原任务", out, qerr)
				}
				before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
				calls, models := e.submits.Load(), e.server.Count()
				if calls != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 {
					t.Fatal("深度队列未恰好一次提交与结算")
				}
				replayed, qerr := q.RunBatch(e.ctx, task.MemberQueueOptions{MaxTasks: 10})
				if qerr != nil || !replayed.Complete || len(replayed.Items) != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before || e.submits.Load() != calls || e.server.Count() != models {
					t.Fatal("深度队列重扫产生额外执行或账务", replayed, qerr)
				}
				requirePointAudit(t, e.pointsTestEnv, e.ledger)
			} else {
				_, err = w.Run(e.ctx, v.UserID, v.ID)
			}
			current, inspectErr := e.tasks.Inspect(e.ctx, v.UserID, v.ID)
			if err != nil || inspectErr != nil || current.Status != task.Succeeded {
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
				// 真实装配也复用此断言；不得把候选正文或模型回包写入测试日志。
				var no string
				_ = e.db.QueryRowContext(e.ctx, "SELECT no FROM tm_ai_tasks WHERE id=?", v.ID).Scan(&no)
				if kb, kerr := json.Marshal([]any{"l1-final-operation-v1", int64(1), v.ID, no}); kerr == nil {
					t.Logf("want final key=%s", task.StageDigest(kb))
				}
				var cur int
				var status, code string
				_ = e.db.QueryRowContext(e.ctx, "SELECT current_step,status,error_code FROM tm_ai_tasks WHERE id=?", v.ID).Scan(&cur, &status, &code)
				t.Logf("task current_step=%d status=%s code=%s", cur, status, code)
				t.Fatal("准入与执行摘要不一致或执行失败", err, inspectErr, current.Status)
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
			reader, err := service.NewAIReportReadServiceWithDeep(l1base, l2base, l2ReadControl{policy: l2Policy, taskID: v.ID, subjectID: v.SubjectID, model: cfg.modelName, filing: "test-only-filing"}, true)
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
