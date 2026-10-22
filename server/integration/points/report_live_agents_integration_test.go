//go:build integration && liveagents

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 双构建标签+显式隔离实例+一次性回环凭据：普通测试/CI绝不隐式产生模型费用。
func TestIntegrationAIReportLiveAgents(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LIVE") != "local-fixture-only" || os.Getenv("TM_TEST_DSN") == "" {
		t.Fatal("真实联调必须由隔离MySQL脚本显式启动")
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
	e := newSubmissionHTTPEnvWithPlan(t, sql.LevelReadCommitted, "success", func(e *submissionHTTPEnv) {
		e.pointsTestEnv.ctx = ctx
		model, err := provider.NewDirect(provider.Config{Name: "local-evaluation-json-bridge-v1", BaseURL: modelURL, Model: "deepseek-flash", APIKey: os.Getenv("TM_TEST_AI_MODEL_TOKEN"), TimeoutMS: 90000}, true)
		if err != nil {
			t.Fatal(err)
		}
		e.model = model
		cfg := e.planner.p.Model
		cfg.Model, cfg.ProfileDigest, cfg.MaxInputTokens = "deepseek-flash", model.DirectProfileDigest(), 32768
		cfg.Version = "local-fixture-live-v1"
		policy := e.planner.p.Policy
		policy.ModelName = cfg.Model
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
		snap, err := sources.BuildSnapshot(e.source, "https://evidence.example.test/live-fixture", sources.MediaPlain, []byte("示例题材是用于本地接线验收的测试资料主题。2026年10月21日，示例题材测试资料登记了设备验收事件。示例题材的测试流程包含样例数据采集和离线处理两个环节。测试公司甲在示例题材测试资料中登记为设备资料提供方。"), e.snap.FetchedAt, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		e.snap, err = repository.NewAISourceRepo(e.db).SaveSnapshot(ctx, snap)
		if err != nil {
			t.Fatal(err)
		}
		cc := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: cnURL, ApprovedHost: "127.0.0.1", Bearer: os.Getenv("TM_TEST_AI_AGENT_TOKEN"), ApprovalRef: "local-test:user-approved", LicenseRef: "community:51060a7:local-evaluation", QuickModel: cfg.Model, DeepModel: cfg.Model, Timeout: 3 * time.Second, MaxSubmissions: 1}
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
	})
	g := newH5ReportGateway(t, e, origin, true)
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
				if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
					t.Fatal("未冻结原测试锁价")
				}
				var err error
				worker, err = service.NewAIL1ReportWorker(e.tasks, e.store.AITaskRepo, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner,
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
				select {
				case <-done:
				case <-ctx.Done():
					t.Fatal("真实链路超时")
				}
				if g.workerErr != nil {
					t.Fatal("真实编排失败", g.workerErr)
				}
				v, err := e.tasks.Inspect(ctx, 1, e.v.ID)
				if err != nil || v.Status != task.Succeeded {
					t.Fatal("真实链路未成功", v.Status, err)
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
				assertPublicReport(t, body, e.reportEnv)
				h5GatewayRequest(t, g, "GET", path, foreign, "", "", 404)
				var public map[string]any
				if json.Unmarshal(body, &public) != nil || strings.Contains(string(body), "TMEVAL") {
					t.Fatal("原始Agent线索泄漏")
				}
				requirePointAudit(t, e.pointsTestEnv, e.ledger)
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
