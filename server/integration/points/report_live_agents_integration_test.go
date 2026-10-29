//go:build integration && liveagents

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 双构建标签+显式隔离实例+一次性回环凭据：普通测试/CI绝不隐式产生模型费用。
func TestIntegrationAIReportLiveAgents(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LIVE") != "local-fixture-only" || os.Getenv("TM_TEST_DSN") == "" || flag.Lookup("test.count").Value.String() != "1" || flag.Lookup("test.run").Value.String() != "^TestIntegrationAIReportLiveAgents$" {
		t.Fatal("真实联调必须由隔离MySQL脚本显式启动")
	}
	conversation := os.Getenv("TM_TEST_AI_LIVE_CONVERSATION") == "local-fixture-only"
	depth := os.Getenv("TM_TEST_AI_LIVE_DEPTH")
	if !liveDepthConfigurationValid(depth, conversation) {
		t.Fatal("真实深度套餐只允许显式会话模式")
	}
	deep := depth == "deep"
	if !liveConversationConfigurationValid(os.Getenv("TM_TEST_AI_LIVE_CONVERSATION"), os.Getenv("TM_TEST_AI_H5_INTENTS"), os.Getenv("TM_TEST_AI_H5_LOGIN"), os.Getenv("TM_TEST_AI_H5_LOGIN_RUNNER"), os.Getenv("TM_TEST_AI_H5_ORIGIN")) {
		t.Fatal("真实会话须显式启用原意图与原会员登录，不允许隐式降级")
	}
	for _, key := range []string{"TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_TIMEOUT_RECOVERY", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_REPORT_QA", "TM_TEST_AI_H5_QA_HISTORY", "TM_TEST_AI_H5_ASSISTANT", "TM_TEST_AI_H5_DEEP"} {
		if os.Getenv(key) != "" {
			t.Fatal("真实链路不能混用其它浏览器专项")
		}
	}
	cnURL, modelURL := os.Getenv("TM_TEST_AI_AGENT_URL"), os.Getenv("TM_TEST_AI_MODEL_URL")
	if _, err := h5TestOrigin(cnURL); err != nil {
		t.Fatal("异步桥必须为字面回环根地址")
	}
	if !strings.HasSuffix(modelURL, "/v1") {
		t.Fatal("模型桥路径无效")
	}
	if _, err := h5TestOrigin(strings.TrimSuffix(modelURL, "/v1")); err != nil {
		t.Fatal("模型桥必须为字面回环地址")
	}
	origin := os.Getenv("TM_TEST_AI_H5_ORIGIN")
	if _, err := h5TestOrigin(origin); err != nil {
		t.Fatal("必须提供实际本地H5源")
	}
	ctx, stop := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(stop)
	reportModel := os.Getenv("TM_TEST_AI_REPORT_MODEL")
	if reportModel == "" {
		reportModel = "deepseek-flash"
	}
	if reportModel != "deepseek-flash" && reportModel != "qwen3.7-plus" {
		t.Fatal("隔离报告模型不在白名单中")
	}
	var deepPlan *service.AIL2ReportPlan
	e := newSubmissionHTTPEnvWithPlan(t, sql.LevelReadCommitted, "success", func(e *submissionHTTPEnv) {
		e.pointsTestEnv.ctx = ctx
		model, err := provider.NewDirect(provider.Config{Name: "local-evaluation-json-bridge-v1", BaseURL: modelURL, Model: reportModel, APIKey: os.Getenv("TM_TEST_AI_MODEL_TOKEN"), TimeoutMS: 90000}, true)
		if err != nil {
			t.Fatal(err)
		}
		e.model = model
		cfg := e.planner.p.Model
		cfg.Model, cfg.ProfileDigest, cfg.MaxInputTokens = reportModel, model.DirectProfileDigest(), 32768
		cfg.Version = "local-fixture-live-v1"
		policy := e.planner.p.Policy
		policy.ModelName = cfg.Model
		if deep {
			cfg.AnalysisType = task.DeepAnalysisType
			policy.AnalysisType, policy.MinQuotes = task.DeepAnalysisType, task.DeepMinQuotes
			e.planner.p.AnalysisType = task.DeepAnalysisType
		}
		policy.EntailmentVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckEntailment, cfg)
		if err != nil {
			t.Fatal(err)
		}
		policy.SemanticVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckSemantic, cfg)
		if err != nil {
			t.Fatal(err)
		}
		// 终检使用真实判定步骤的持久凭据；意外调用同步默认判断时一律uncertain。
		e.policy, err = finalreport.NewPolicy(policy, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
			return finalreport.Uncertain, nil
		}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
		if err != nil {
			t.Fatal(err)
		}
		if deep {
			e.reportEnv.deepPolicy = e.policy
		}
		material := "示例题材是用于本地接线验收的测试资料主题。2026年10月21日，示例题材测试资料登记了设备验收事件。示例题材的测试流程包含样例数据采集和离线处理两个环节。测试公司甲在示例题材测试资料中登记为设备资料提供方。"
		if deep {
			material += "示例题材仅指本次接线测试资料集合，不代表真实市场题材。2026年10月22日，示例题材测试资料登记了离线处理验收事件。"
		}
		snap, err := sources.BuildSnapshot(e.source, "https://evidence.example.test/live-fixture", sources.MediaPlain, []byte(material), e.snap.FetchedAt, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		e.snap, err = repository.NewAISourceRepo(e.db).SaveSnapshot(ctx, snap)
		if err != nil {
			t.Fatal(err)
		}
		// Agent桥协议别名独立于报告模型，实际供应商与模型保存在父桥回执。
		cc := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: cnURL, ApprovedHost: "127.0.0.1", Bearer: os.Getenv("TM_TEST_AI_AGENT_TOKEN"), ApprovalRef: "local-test:user-approved", LicenseRef: "community:51060a7:local-evaluation", QuickModel: "deepseek-flash", DeepModel: "deepseek-flash", Timeout: 3 * time.Second, MaxSubmissions: 1}
		client, err := tradingcn.New(cc)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(client.Close)
		e.agent, err = service.NewAIAgentService(e.payloads, client, cc.Profile())
		if err != nil {
			t.Fatal(err)
		}
		e.planner.p.Agent.ProfileDigest = cc.Profile().Digest
		e.planner.p.Model, e.planner.p.Policy, e.planner.p.Snapshots = cfg, policy, []sources.Snapshot{e.snap}
		e.control.cfg = cfg
		e.ctl.p.Budget.MaxTokens, e.ctl.p.Budget.MaxDurationMS = 131072, 900000
		e.ctl.p.ModelName, e.ctl.p.ProfileDigest = cfg.Model, cfg.ProfileDigest
		e.ctl.p.Materials[0] = task.AdmissionMaterial{SnapshotID: e.snap.ID, Digest: e.snap.Hash, Domain: e.source.Domain, Category: e.snap.SourceCategory, URL: e.snap.URL, CollectedAt: e.snap.FetchedAt}
		e.ctl.p.ExecutionDigest, err = e.planner.p.AdmissionDigest()
		if err != nil {
			t.Fatal(err)
		}
		if deep {
			deepPlan, err = service.NewAIL2ReportPlan(e.planner.p, points.Price{Version: "v1", Points: task.DeepPricePoints}, e.ctl.p.Budget, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
			if err != nil {
				t.Fatal(err)
			}
			e.ctl.p, err = deepPlan.Admission(task.AdmissionScope{UserID: 1, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 1})
			if err != nil {
				t.Fatal(err)
			}
			e.ctl.allowDeep.Store(true)
		}
	})
	g := newH5ReportGateway(t, e, origin, true)
	var execution service.AIL1ReportPlanner = e.planner
	wantPrice := 7
	if deep {
		wantPrice = int(task.DeepPricePoints)
		grant := pointGrant(time.Now().UTC().Truncate(time.Millisecond), task.DeepPricePoints)
		grant.Key, grant.BizID = "test_live_deep_grant", "live-deep-grant"
		if _, err := e.ledger.Grant(ctx, grant); err != nil {
			t.Fatal(err)
		}
		var err error
		execution, err = service.NewAIL2ExecutionPlanner(deepPlan)
		if err != nil {
			t.Fatal(err)
		}
		limiter, err := service.NewSQLDeepLimiter(e.db)
		if err != nil {
			t.Fatal(err)
		}
		g.intentCreator.AITaskCreateService.SetDeepLimiter(limiter)
	}
	t.Log("真实多Agent本地H5入口（仅测试资料与积分）：", g.server.URL+"/__test__/")
	var worker *service.AIL1ReportWorker
	var done <-chan struct{}
	for {
		select {
		case job := <-g.actions:
			switch job.kind {
			case "create":
				if worker != nil {
					t.Fatal("不得创建第二个付费测试任务")
				}
				e.bindNewTask(t, job.id)
				if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != wantPrice || e.v.PricePoints != int64(wantPrice) {
					t.Fatal("未冻结原测试锁价")
				}
				var err error
				worker, err = service.NewAIL1ReportWorker(e.tasks, e.store.AITaskRepo, e.ledger, e.agent, e.payloads, e.model, execution, workflowSelectionControl{e.control, e.planner}, e.control, e.owner,
					agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: time.Second, Timeout: 5 * time.Minute}, PollInterval: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				ch := make(chan struct{})
				done = ch
				go func() { defer close(ch); _, g.workerErr = worker.Run(ctx, 1, e.v.ID) }()
				t.Cleanup(func() {
					stop()
					select {
					case <-ch:
					case <-time.After(10 * time.Second):
						t.Error("真实worker未退出")
					}
				})
			case "finish":
				if job.done != nil {
					defer close(job.done)
				}
				if done == nil {
					t.Fatal("尚未由页面创建任务")
				}
				// 先取原worker终态，失败不能被后续“未读到报告”断言遮蔽。
				select {
				case <-done:
				case <-ctx.Done():
					t.Fatal("真实链路超时")
				}
				if g.workerErr != nil {
					t.Fatal("真实编排失败", g.workerErr)
				}
				v, err := e.tasks.Inspect(ctx, 1, e.v.ID)
				if err != nil {
					t.Fatal("真实任务终态读取失败", err)
				}
				if v.Status != task.Succeeded {
					t.Fatalf("真实链路未成功：status=%v step=%d code=%s；测试账本avail=%d frozen=%d settle=%d release=%d", v.Status, v.CurrentStep, v.ErrorCode,
						e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1"), e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1"),
						e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'"), e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'"))
				}
				if conversation {
					g.serial.Lock()
					valid := g.intentMode && g.memberLogin != nil && g.creationPosts == 0 && g.conversationPosts == 1 && g.intentPrepares == 1 && g.intentConfirms == 1 && g.intentCreator.calls.Load() == 1 && g.reportReads >= 2
					g.serial.Unlock()
					if !valid {
						t.Fatal("真实会话未取得原登录、单次会话创建/确认及页面报告重读证据")
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_tasks WHERE user_id=1 AND state='linked'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=1 AND kind='task'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=2") != 0 {
						t.Fatal("真实会话未唯一关联原任务或会员隔离失败")
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_tasks i JOIN tm_ai_conversation_messages m ON m.user_id=i.user_id AND m.conversation_id=i.conversation_id AND m.task_id=i.task_id WHERE i.user_id=1 AND i.state='linked' AND m.kind='task' AND i.task_id=?", e.v.ID) != 1 {
						t.Fatal("真实会话引用不是本次实际执行的原任务")
					}
				}
				if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3") != 1 {
					t.Fatal("测试结算/报告未原子闭合一次")
				}
				if e.server.Count() != 0 || e.submits.Load() != 0 {
					t.Fatal("误用了模型或Agent替身")
				}
				if e.queryInt("SELECT COUNT(*) FROM test_only_check_audits WHERE phase='finish'") != 3 {
					t.Fatal("真实选句与两个独立判定未完整留痕")
				}
				before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
				if _, err := worker.Run(ctx, 1, e.v.ID); err != nil {
					t.Fatal(err)
				}
				if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
					t.Fatal("重放重复结算")
				}
				token, foreign := e.web.token(t, 1, "client"), e.web.token(t, 2, "client")
				path := "/api/v1/ai/tasks/" + strconv.FormatInt(e.v.ID, 10) + "/report"
				body := h5GatewayRequest(t, g, "GET", path, token, "", "", 200)
				if deep {
					var out struct {
						Code int          `json:"code"`
						Data dto.AIReport `json:"data"`
					}
					if json.Unmarshal(body, &out) != nil || out.Code != 0 || out.Data.TaskID != e.v.ID || len(out.Data.Sections) != 4 || !out.Data.Disclosure.AIGenerated || out.Data.GeneratedAt <= 0 || e.v.AnalysisType != task.DeepAnalysisType {
						t.Fatal("深度真实报告公开契约不符")
					}
					quotes := 0
					for _, section := range out.Data.Sections {
						if !section.AIGenerated || len(section.Quotes) == 0 {
							t.Fatal("深度报告章节缺失引文")
						}
						for _, quote := range section.Quotes {
							matched := false
							for _, sentence := range e.snap.Sentences {
								if sentence.Text == quote.Text && quote.CollectedAt == e.snap.FetchedAt.UnixMilli() {
									matched = true
								}
							}
							if !matched {
								t.Fatal("深度真实报告不是原快照逐字引文")
							}
							quotes++
						}
					}
					if quotes < task.DeepMinQuotes || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 {
						t.Fatal("深度引文数量或测试结算金额不符")
					}
				} else {
					assertPublicReport(t, body, e.reportEnv)
				}
				for _, private := range []string{e.v.No, e.snap.LicenseRef, "snapshot_id", "input_digest", "terminal_txn_id", "fence", "verdict", "test-only-agent-untrusted-marker"} {
					if strings.Contains(string(body), private) {
						t.Fatal("真实报告泄漏内部字段")
					}
				}
				h5GatewayRequest(t, g, "GET", path, foreign, "", "", 404)
				var public map[string]any
				if json.Unmarshal(body, &public) != nil || strings.Contains(string(body), "TMEVAL") {
					t.Fatal("原始Agent线索泄漏")
				}
				requirePointAudit(t, e.pointsTestEnv, e.ledger)
				if conversation {
					g.memberLogin.finish(t, ctx)
					t.Log("TM_STOCK_LIVE_CONVERSATION_VERIFIED")
					if deep {
						t.Log("TM_STOCK_LIVE_DEEP_VERIFIED")
					}
				}
				t.Log("已验：页面创建→真实异步提交/轮询→V2核心→真实选句/两判定→报告终检→测试账本一次结算；替身零调用、越权404、终态重放无重复结算")
				return
			default:
				if job.done != nil {
					close(job.done)
				}
			}
		case <-ctx.Done():
			t.Fatal("本地真实多Agent会话未在期限内完成")
		}
	}
}
