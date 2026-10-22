//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestIntegrationDeepH5GatewayHTTP(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "pass")
			front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "深度网关测试占位") }))
			t.Cleanup(front.Close)
			origin := front.URL
			if interactive := os.Getenv("TM_TEST_AI_H5_ORIGIN"); interactive != "" {
				origin = interactive
			}
			g := newH5ReportGateway(t, e, origin)
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
			upstream := providertest.NewServer(t,
				providertest.Success(string(selectionText), usage),
				checkModelReply("pass", "supported"),
				checkModelReply("pass", "factual"),
			)
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
			agentPlan := agenttask.Plan{SubjectType: task.Theme, SubjectID: 1, ActorID: 7, Symbol: "600001", ProfileDigest: e.planner.p.Agent.ProfileDigest, Units: task.Units{Calls: 1, Tokens: 100}}
			template := service.AIL1ReportPlan{Agent: agentPlan, Model: checkCfg, Policy: policyCfg, ThemeName: "示例题材", Snapshots: []sources.Snapshot{e.snap, second}, AnalysisType: task.DeepAnalysisType}
			plan, err := service.NewAIL2ReportPlan(template, points.Price{Version: "v1", Points: task.DeepPricePoints}, task.Budget{MaxAttempts: 2, MaxCalls: 10, MaxTokens: 12000, MaxPages: 6, MaxDurationMS: 300000}, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
			if err != nil {
				t.Fatal(err)
			}
			admission, err := plan.Admission(task.AdmissionScope{UserID: 1, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 1})
			if err != nil {
				t.Fatal(err)
			}
			execution, err := service.NewAIL2ExecutionPlanner(plan)
			if err != nil {
				t.Fatal(err)
			}
			e.ctl.p = admission
			e.ctl.allowDeep.Store(true)
			e.ctl.allowed.Store(true)
			limiter, err := service.NewSQLDeepLimiter(e.db)
			if err != nil {
				t.Fatal(err)
			}
			g.intentCreator.AITaskCreateService.SetDeepLimiter(limiter)
			l2Policy := mustPolicy(t, policyCfg)
			l1base, err := service.NewAIReportService(e.store.AITaskRepo, e.ledger, e.reportEnv.policy, false)
			if err != nil {
				t.Fatal(err)
			}
			l2base, err := service.NewAIReportService(e.store.AITaskRepo, e.ledger, l2Policy, false)
			if err != nil {
				t.Fatal(err)
			}
			e.reportEnv.deepPolicy = l2Policy
			if err := e.web.reader.SetDeep(l2base); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Millisecond)
			suffix := fmt.Sprintf("%d", now.UnixNano())
			if _, err := e.ledger.Grant(e.ctx, points.GrantRequest{UserID: 1, Key: "test_deep_gw_" + suffix, BizID: "deep-gw-" + suffix, Amount: 30, Source: points.Gift, EffectiveAt: now.Add(-time.Minute)}); err != nil {
				t.Fatal(err)
			}
			g.createHook = func(t *testing.T, id int64) {
				v, err := e.tasks.Inspect(e.ctx, 1, id)
				if err != nil {
					t.Fatal(err)
				}
				v, err = e.tasks.Claim(e.ctx, 1, id, "worker_deep_gw_001", time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				w, err := service.NewAIL1ReportWorker(e.tasks, e.store, e.ledger, e.agent, e.payloads, model, execution, l2SelectionControl{}, l2CheckControl{cfg: checkCfg}, v.LeaseOwner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				got, err := w.Run(e.ctx, v.UserID, v.ID)
				if err != nil || got.Task.TaskStatus != task.Succeeded {
					t.Fatal("网关深度执行失败", err, got.Task)
				}
			}
			if os.Getenv("TM_TEST_AI_H5_ORIGIN") != "" {
				var baseID int64
				if err := e.db.QueryRowContext(e.ctx, "SELECT COALESCE(MAX(id),0) FROM tm_ai_tasks").Scan(&baseID); err != nil {
					t.Fatal(err)
				}
				t.Log("实际H5深度测试入口: " + g.server.URL + "/__test__/")
				deadline := time.Now().Add(6 * time.Minute)
				for time.Now().Before(deadline) && e.queryInt(fmt.Sprintf("SELECT COUNT(*) FROM tm_ai_tasks WHERE id>%d AND analysis_type=2 AND status=3", baseID)) == 0 {
					var pending int64
					_ = e.db.QueryRowContext(e.ctx, fmt.Sprintf("SELECT id FROM tm_ai_tasks WHERE id>%d AND analysis_type=2 AND status=1 ORDER BY id LIMIT 1", baseID)).Scan(&pending)
					if pending > 0 {
						g.createHook(t, pending)
					}
					time.Sleep(200 * time.Millisecond)
				}
				if e.queryInt(fmt.Sprintf("SELECT COUNT(*) FROM tm_ai_tasks WHERE id>%d AND analysis_type=2 AND status=3", baseID)) != 1 {
					t.Fatal("交互深度未在时限内完成")
				}
				var id int64
				if err := e.db.QueryRowContext(e.ctx, fmt.Sprintf("SELECT id FROM tm_ai_tasks WHERE id>%d AND analysis_type=2 AND status=3 LIMIT 1", baseID)).Scan(&id); err != nil {
					t.Fatal(err)
				}
				v, err := e.tasks.Inspect(e.ctx, 1, id)
				if err != nil {
					t.Fatal(err)
				}
				reader, err := service.NewAIReportReadServiceWithDeep(l1base, l2base, l2ReadControl{policy: l2Policy, taskID: v.ID, subjectID: v.SubjectID, model: "测试模型", filing: "test-only-filing"}, true)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := reader.Read(e.ctx, 1, v.ID); err != nil {
					t.Fatal("交互深度报告不可读", err)
				}
				return
			}
			token := e.web.token(t, 1, "client")
			var created struct {
				Data dto.AIConversationReceipt `json:"data"`
			}
			if json.Unmarshal(h5GatewayRequest(t, g, "POST", "/api/v1/ai/conversations", token, "deep_gw_conversation_key", `{"title":"深度网关会话"}`, 200), &created) != nil || created.Data.ID <= 0 {
				t.Fatal("深度网关会话创建失败")
			}
			base := fmt.Sprintf("/api/v1/ai/conversations/%d", created.Data.ID)
			var prepared struct {
				Data dto.AIConversationTask `json:"data"`
			}
			if json.Unmarshal(h5GatewayRequest(t, g, "POST", base+"/task-intents", token, "deep_gw_intent_key", `{"analysis_type":"deep","subject_type":"theme","subject_id":1,"price_version":"v1"}`, 200), &prepared) != nil || prepared.Data.State != "prepared" || prepared.Data.PricePoints != task.DeepPricePoints {
				t.Fatal("深度网关意图准备失败", prepared)
			}
			var linked struct {
				Data dto.AIConversationTask `json:"data"`
			}
			confirmPath := fmt.Sprintf("%s/task-intents/%d/confirm", base, prepared.Data.ID)
			if json.Unmarshal(h5GatewayRequest(t, g, "POST", confirmPath, token, "", `{"confirmed":true}`, 200), &linked) != nil || linked.Data.State != "linked" || linked.Data.TaskID <= 0 {
				t.Fatal("深度网关确认失败", linked)
			}
			select {
			case action := <-g.actions:
				g.process(t, action, "deep")
			case <-e.ctx.Done():
				t.Fatal("深度网关未产生创建动作")
			}
			v, err := e.tasks.Inspect(e.ctx, 1, linked.Data.TaskID)
			if err != nil || v.Status != task.Succeeded || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3") != 1 {
				t.Fatal("深度网关任务/结算不符", v, err)
			}
			reader, err := service.NewAIReportReadServiceWithDeep(l1base, l2base, l2ReadControl{policy: l2Policy, taskID: v.ID, subjectID: v.SubjectID, model: "测试模型", filing: "test-only-filing"}, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reader.Read(e.ctx, 1, v.ID); err != nil {
				t.Fatal("深度网关报告不可读", err)
			}
		})
	}
}
