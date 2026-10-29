//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 先攻击实际控制器的调用前入口，再交付原请求；不替换其准入或审计判断。
type localSelectionStartGuard struct {
	*service.AILocalReportSelectionControl
	e *workflowEnv
}
type localCheckStartGuard struct {
	*service.AILocalReportCheckControl
	e *workflowEnv
}

// 直接调用实际审计服务测试重放；来源撤回不应阻断已返回调用的留证。
type localAuditReplayGuard struct {
	e     *workflowEnv
	audit *service.AIReportCallAuditService
}

func (g localAuditReplayGuard) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	if err := g.audit.Start(ctx, r, req); err != nil {
		return err
	}
	if err := g.audit.Start(ctx, r, req); !errors.Is(err, task.ErrUnconfirmed) {
		g.e.t.Fatal("重复调用前审计未阻止重发", err)
	}
	return nil
}
func (g localAuditReplayGuard) Finish(ctx context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	g.e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
	defer g.e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
	// 两个实际事务竞争同一个finish，必须都确认同一条结果。
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { done <- g.audit.Finish(ctx, r, out, callErr) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			return err
		}
	}
	if err := g.audit.Finish(ctx, r, out, callErr); err != nil {
		g.e.t.Fatal("同结果审计重放失败", err)
	}
	out.Text += "仅测试:变更结果"
	if err := g.audit.Finish(ctx, r, out, callErr); !errors.Is(err, points.ErrIdempotency) {
		g.e.t.Fatal("不同结果覆盖原审计", err)
	}
	return nil
}

func verifyLocalModelStartGuard(e *workflowEnv, r task.StageRequest, req provider.Request, start func(task.StageRequest, provider.Request) error) error {
	e.t.Helper()
	before := e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`)
	for _, mode := range []string{"提示", "模型", "追溯", "输入摘要"} {
		changed, stage := req, r
		switch mode {
		case "提示":
			changed.Prompt += "仅测试:额外输入"
		case "模型":
			changed.Model = "仅测试:其它模型"
		case "追溯":
			changed.TaskID = "other"
		case "输入摘要":
			stage.InputDigest = task.StageDigest([]byte("other"))
		}
		if err := start(stage, changed); err == nil {
			e.t.Fatal("实际本地控制器放行被篡改请求", mode)
		}
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != before {
			e.t.Fatal("拒绝请求仍进入调用审计", mode)
		}
	}
	return start(r, req)
}
func (g localSelectionStartGuard) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	return verifyLocalModelStartGuard(g.e, r, req, func(r task.StageRequest, req provider.Request) error {
		return g.AILocalReportSelectionControl.Start(ctx, r, req)
	})
}
func (g localCheckStartGuard) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	return verifyLocalModelStartGuard(g.e, r, req, func(r task.StageRequest, req provider.Request) error {
		return g.AILocalReportCheckControl.Start(ctx, r, req)
	})
}

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
				f := newLocalPublicationFixture(t, level, kind, mode)
				e, owner, spec, file, path, raw, admission, creates := f.env, f.owner, f.spec, f.file, f.path, f.raw, f.admission, f.creates
				m := e.planner.p.Model
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
				audit, err := service.NewAIReportCallAuditService(e.repo)
				if err != nil {
					t.Fatal(err)
				}
				selection, checks, model, err := service.NewAILocalReportModelControls(admission, e.ledger, e.payloads, workflowModelConfig(e.server.URL()), localAuditReplayGuard{e, audit})
				if err != nil {
					t.Fatal(err)
				}
				if !model.MatchesDirect(m.ProfileDigest, m.Model) {
					t.Fatal("实际回环适配器身份未绑定本地计划")
				}
				worker, err := service.NewAIL1ReportWorker(e.tasks, e.repo, e.ledger, e.agent, e.payloads, model, planner, localSelectionStartGuard{selection, e}, localCheckStartGuard{checks, e}, owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
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
				beforeAudits := e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`)
				if beforeModels != 3 || beforeAgent != 1 || result.Task.CurrentStep != 5 || beforeAudits != 6 {
					t.Fatal("未实际经过完整回环Agent/选句/两判定/终检五阶段")
				}
				for _, statement := range []string{`UPDATE tm_ai_report_call_audits SET request_digest=REPEAT('0',64)`, `DELETE FROM tm_ai_report_call_audits`} {
					_, err := e.db.ExecContext(e.ctx, statement)
					var denied *mysqldrv.MySQLError
					if !errors.As(err, &denied) || denied.Number != 1644 {
						t.Fatal("调用审计不可变保护未明确拒绝", err)
					}
				}
				for _, mode := range []string{"终态补造调用", "结果错围栏"} {
					err := e.repo.WithinCallAuditTx(e.ctx, func(tx finalreport.CallAuditTx) error {
						a, err := tx.ReadCallAudit(e.ctx, 1, created.TaskID, 2, "start")
						if err != nil {
							return err
						}
						if mode == "结果错围栏" {
							a.Phase = "finish"
							a.Outcome = "returned"
							a.ResultDigest = task.StageDigest([]byte("other"))
							a.Request.Fence++
						}
						return tx.InsertCallAudit(e.ctx, a)
					})
					var denied *mysqldrv.MySQLError
					if !errors.As(err, &denied) || denied.Number != 1644 {
						t.Fatal("审计原始绑定触发器未明确拒绝", mode, err)
					}
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
				if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != beforeTxns || len(e.server.Requests()) != beforeModels || e.submits.Load() != beforeAgent || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != beforeAudits {
					t.Fatal("读取/撤回触发调用或重复结算")
				}
				requirePointAudit(t, e.pointsTestEnv, e.ledger)
			})
		}
	}
}
