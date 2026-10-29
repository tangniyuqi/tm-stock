//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 真MySQL/原完整worker/原发布读取器；Agent和模型仅为明确回环协议替身。
// 不声称真实多Agent/模型批准，不能把本测试的网络控制器装入正式服务。
func TestIntegrationAIReportLocalPublication(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		for _, kind := range []int{1, 2} {
			t.Run(level.String()+"/"+strconv.Itoa(kind), func(t *testing.T) {
				mode := "local"
				if kind == 2 {
					mode = "local_deep"
				}
				e := newWorkflowEnv(t, level, mode)
				owner := e.v.LeaseOwner
				if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, owner, e.v.Fence, "internal_failure", 0); err != nil {
					t.Fatal(err)
				}
				grant := pointGrant(time.Now().UTC().Truncate(time.Millisecond), 40)
				grant.Key = "local_extra_grant_key"
				grant.BizID = "local-extra-grant"
				if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
					t.Fatal(err)
				}
				p := e.planner.p
				m := p.Model
				price := int64(7)
				if kind == 2 {
					price = 30
				}
				spec := config.AILocalPlan{AnalysisType: kind, SubjectID: 1, SubjectName: p.ThemeName, ActorID: p.Agent.ActorID, Symbol: p.Agent.Symbol, AgentProfileDigest: p.Agent.ProfileDigest, AgentCalls: p.Agent.Units.Calls, AgentTokens: p.Agent.Units.Tokens, AgentPages: p.Agent.Units.Pages, ModelName: m.Model, ModelProfileDigest: m.ProfileDigest, FilingNo: m.FilingNo, ModelVersion: m.Version, MaxInputTokens: m.MaxInputTokens, SelectionMaxTokens: m.SelectionTokens(), CheckMaxTokens: m.CheckTokens(), PolicyRef: p.Policy.Ref, MinQuotes: p.Policy.MinQuotes, AllowedDomains: p.Policy.AllowedDomains, ApprovalRef: m.ApprovalRef, QualityRef: m.QualityRef, HardBudgetRef: m.HardBudgetRef, PriceRef: "local-eval:price", SourceRef: "local-eval:source", PriceVersion: "local-v1", PricePoints: price, Budget: e.v.Budget}
				for _, s := range p.Snapshots {
					spec.Materials = append(spec.Materials, config.AILocalMaterial{SnapshotID: s.ID, Digest: s.Hash, LicenseRef: s.LicenseRef})
				}
				id := strings.TrimPrefix(e.database, "tm_ai_local_")
				runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: e.database, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
				at := time.Now().UnixMilli()
				file := config.AILocalPlans{Version: 1, InstanceID: id, Revision: "v1", Scope: "synthetic-local-only", IssuedAtMS: at - 1000, ExpiresAtMS: at + 600000, Members: []int64{1, 2}, Plans: []config.AILocalPlan{spec}}
				path := filepath.Join(t.TempDir(), "plans.json")
				raw, err := json.Marshal(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				source, err := config.NewAILocalPlanSource(path, runtime)
				if err != nil {
					t.Fatal(err)
				}
				admission, err := service.NewAILocalAdmissionControl(source, repository.NewAILocalInstanceVerifier(e.db), e.repo)
				if err != nil {
					t.Fatal(err)
				}
				if err := admission.Ready(e.ctx); err != nil {
					t.Fatal(err)
				}
				creates, err := service.NewAITaskCreateService(e.tasks, admission, true)
				if err != nil {
					t.Fatal(err)
				}
				limiter, err := service.NewSQLDeepLimiter(e.db)
				if err != nil {
					t.Fatal(err)
				}
				creates.SetDeepLimiter(limiter)
				analysis := "light"
				if kind == 2 {
					analysis = "deep"
				}
				created, err := creates.Create(e.ctx, 1, "local_publication_task", dto.AITaskCreateInput{AnalysisType: analysis, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "local-v1"})
				if err != nil {
					t.Fatal(err)
				}
				e.v, err = e.tasks.Inspect(e.ctx, 1, created.TaskID)
				if err != nil {
					t.Fatal(err)
				}
				planner, err := service.NewAILocalReportPlanner(admission)
				if err != nil {
					t.Fatal(err)
				}
				e.planner.p, err = planner.Plan(e.ctx, e.v)
				if err != nil {
					t.Fatal(err)
				}
				worker, err := service.NewAIL1ReportWorker(e.tasks, e.repo, e.ledger, e.agent, e.payloads, e.model, planner, workflowSelectionControl{e.control, e.planner}, e.control, owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				result, err := worker.Run(e.ctx, 1, created.TaskID)
				if err != nil || result.Commit.Status != task.Succeeded {
					t.Fatal("本地完整编排未交付", err, result)
				}
				beforeTxns := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
				beforeModels := len(e.server.Requests())
				beforeAgent := e.submits.Load()
				if beforeModels != 3 || beforeAgent != 1 || result.Task.CurrentStep != 5 {
					t.Fatal("未实际经过完整回环Agent/选句/两判定/终检五阶段")
				}
				base, err := service.NewAIReportService(e.repo, e.ledger, nil, false)
				if err != nil {
					t.Fatal(err)
				}
				control, err := service.NewAILocalReportReadControl(admission)
				if err != nil {
					t.Fatal(err)
				}
				reader, err := service.NewAIReportReadServiceWithPolicySource(base, control, control, true)
				if err != nil {
					t.Fatal(err)
				}
				e.mustExec(`CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL)`)
				web := &reportHTTPEnv{x: e.reportEnv, reader: reader, key: strings.Repeat("r", 40)}
				verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: web.key, Issuer: "GVA", Audience: "GVA"})
				if err != nil {
					t.Fatal(err)
				}
				sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(e.db))
				if err != nil {
					t.Fatal(err)
				}
				auth, err := middleware.NewMemberAuth(sessions)
				if err != nil {
					t.Fatal(err)
				}
				mux := http.NewServeMux()
				handler.NewAIReportReadHandler(reader).Register(mux, auth)
				srv := httptest.NewServer(mux)
				t.Cleanup(srv.Close)
				web.server = srv
				url := "/api/v1/ai/tasks/" + strconv.FormatInt(created.TaskID, 10) + "/report"
				token := web.token(t, 1, "client")
				web.mustGet(t, url, web.token(t, 2, "client"), 404)
				body := web.mustGet(t, url, token, 200)
				var report struct {
					Data dto.AIReport `json:"data"`
				}
				if json.Unmarshal(body, &report) != nil || len(report.Data.Sections) != 4 || report.Data.SubjectName != spec.SubjectName || report.Data.TaskID != created.TaskID {
					t.Fatal("报告投影不符")
				}
				for _, private := range []string{spec.ApprovalRef, e.snap.LicenseRef, "execution_digest", "policy_digest", "snapshot_id", "budget"} {
					if strings.Contains(string(body), private) {
						t.Fatal("报告泄漏内部元数据")
					}
				}
				// 只读重放不能调模型或重复结算。
				web.mustGet(t, url, token, 200)
				e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
				web.mustGet(t, url, token, 422)
				e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
				web.mustGet(t, url, token, 200)
				file.Revision = "withdrawn"
				changed, err := json.Marshal(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, changed, 0600); err != nil {
					t.Fatal(err)
				}
				web.mustGet(t, url, token, 422)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				web.mustGet(t, url, token, 200)
				if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != beforeTxns || len(e.server.Requests()) != beforeModels || e.submits.Load() != beforeAgent || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 {
					t.Fatal("读取/撤回触发调用或重复结算")
				}
				requirePointAudit(t, e.pointsTestEnv, e.ledger)
			})
		}
	}
}
