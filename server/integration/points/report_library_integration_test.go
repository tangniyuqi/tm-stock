//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅示例来源及回环模拟Agent/模型；从HTTP首次准入绑定公司，而不是改写已创建任务。
func newLibraryReportEnv(t *testing.T, level sql.IsolationLevel, mode string) *submissionHTTPEnv {
	t.Helper()
	return newCompanyReportEnvWithPlan(t, level, 0, func(e *submissionHTTPEnv) {
		prepareLibraryReportPlan(t, e, mode)
	})
}

func prepareLibraryReportPlan(t *testing.T, e *submissionHTTPEnv, mode string) {
	t.Helper()
	selection := finalreport.SelectionResponse{Selections: []finalreport.Selection{{Section: "definition", SnapshotID: e.snap.ID, SentenceIDs: []int{1}}, {Section: "timeline", SnapshotID: e.snap.ID, SentenceIDs: []int{2}}, {Section: "segment", SnapshotID: e.snap.ID, SentenceIDs: []int{3}}}}
	raw, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	steps := []providertest.Step{providertest.Success(string(raw), provider.Usage{Prompt: 2, Completion: 1, Total: 3}), checkModelReply("pass", "supported"), checkModelReply("pass", "factual")}
	if mode == "reject" {
		steps = steps[:2]
		steps[1] = checkModelReply("block", "unsupported")
	}
	e.server = providertest.NewServer(t, steps...)
	e.model, err = provider.NewDirect(provider.Config{Name: "test-only-library", BaseURL: e.server.URL(), Model: "测试模型", TimeoutMS: 3000}, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := e.planner.p.Model
	cfg.ProfileDigest = e.model.DirectProfileDigest()
	fc := e.planner.p.Policy
	fc.EntailmentVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckEntailment, cfg)
	if err != nil {
		t.Fatal(err)
	}
	fc.SemanticVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckSemantic, cfg)
	if err != nil {
		t.Fatal(err)
	}
	e.planner.p.Model, e.planner.p.Policy, e.control.cfg = cfg, fc, cfg
	e.ctl.p.ProfileDigest = cfg.ProfileDigest
	e.ctl.p.ExecutionDigest, err = e.planner.p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	uncertainE := func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}
	uncertainS := func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil }
	base, err := finalreport.NewPolicy(fc, uncertainE, uncertainS)
	if err != nil {
		t.Fatal(err)
	}
	var facts *finalreport.CompanyDraft
	err = e.repo.WithinReportTx(e.ctx, func(tx finalreport.Tx) error {
		snaps, _, err := tx.ReportEvidence(e.ctx, []int64{e.snap.ID}, 1)
		if err != nil {
			return err
		}
		r, err := tx.(finalreport.CompanyReader).ReportCompanies(e.ctx, 1)
		if err != nil {
			return err
		}
		now, err := tx.Points().Now(e.ctx)
		if err != nil {
			return err
		}
		facts, err = finalreport.NewCompanyDraft(base, e.ctl.p, snaps, r, now)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	fc.CompanyDigest = facts.ContentDigest()
	e.planner.p.Policy, e.planner.p.Companies = fc, facts
	e.ctl.p.CompanyDigest = fc.CompanyDigest
	e.ctl.p.ExecutionDigest, err = e.planner.p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	e.policy, err = finalreport.NewCompanyPolicy(fc, facts, uncertainE, uncertainS)
	if err != nil {
		t.Fatal(err)
	}
	e.svc, err = service.NewAIReportService(e.repo, e.ledger, e.policy, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationAIReportLibraryLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"pass", "reject"} {
				t.Run(mode, func(t *testing.T) {
					e := newLibraryReportEnv(t, level, mode)
					out, err := admissionWorker(t, e).Run(e.ctx, 1, e.v.ID)
					if err != nil {
						t.Fatal("公司报告主链失败", err)
					}
					if mode == "reject" {
						if out.Commit.Status != task.Failed || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
							t.Fatal("公司报告拒绝未释放", out)
						}
						return
					}
					if out.Commit.Status != task.Succeeded || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3`) != 1 {
						t.Fatal("公司报告未原子交付", out)
					}
					web := reportHTTPFixtureWithMembers(t, e.reportEnv, "", false)
					body := web.mustGet(t, web.path(), web.token(t, 1, "client"), 200)
					var got struct {
						Data dto.AIReport `json:"data"`
					}
					if json.Unmarshal(body, &got) != nil || len(got.Data.Sections) != 4 || len(got.Data.Sections[3].Companies) != 2 || got.Data.Sections[3].AIGenerated || len(got.Data.Sections[3].Quotes) != 0 {
						t.Fatal("会员报告缺公司或冒充AI")
					}
					for _, marker := range []string{"示例公司", "600001", "trimmed"} {
						if !strings.Contains(string(body), marker) {
							t.Fatal("公司客观字段丢失", marker)
						}
					}
					for _, hidden := range []string{"test-only-agent-untrusted-marker", "ReviewerID", "LicenseRef", "CompanyDigest", "change"} {
						if strings.Contains(string(body), hidden) {
							t.Fatal("报告泄露内部或行情字段", hidden)
						}
					}
					web.mustGet(t, web.path(), web.token(t, 2, "client"), 404)
					requests := e.server.Requests()
					if len(requests) != 3 {
						t.Fatal("模型未走选择及双判定", len(requests))
					}
					if strings.Contains(string(requests[0].Body), `\"companies\"`) {
						t.Fatal("选择Schema仍允许模型选择公司")
					}
					if !strings.Contains(string(requests[1].Body), "示例公司") || !strings.Contains(string(requests[2].Body), "600001") {
						t.Fatal("判定遗漏公司事实")
					}
					calls := len(requests)
					if _, err := admissionWorker(t, e).Run(e.ctx, 1, e.v.ID); err != nil || len(e.server.Requests()) != calls || e.submits.Load() != 1 {
						t.Fatal("重放重复调用", err)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportLibraryBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"deleted", "new_mapping", "source", "review", "missing_binding", "during_check", "read_between"} {
				t.Run(mode, func(t *testing.T) {
					e := newLibraryReportEnv(t, level, "pass")
					if mode == "read_between" {
						if _, err := admissionWorker(t, e).Run(e.ctx, 1, e.v.ID); err != nil {
							t.Fatal(err)
						}
						web := reportHTTPFixtureWithMembers(t, e.reportEnv, "", false)
						web.control.hook = func(ctx context.Context) error {
							_, err := e.db.ExecContext(ctx, `UPDATE addon_quant_theme_stock SET status=0 WHERE id=1`)
							return err
						}
						body := web.mustGet(t, web.path(), web.token(t, 1, "client"), 422)
						if strings.Contains(string(body), "示例公司") {
							t.Fatal("迟到公司正文泄露")
						}
						if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 {
							t.Fatal("拒读撤销原结算")
						}
						return
					}
					switch mode {
					case "deleted":
						e.mustExec(`UPDATE addon_quant_theme_stock SET deleted_at=UTC_TIMESTAMP(3) WHERE id=1`)
					case "new_mapping":
						e.mustExec(`INSERT INTO addon_quant_theme_stock(theme_id,stock_id,ts_code,source_type,source_excerpt,excerpt_trimmed,source_url,collected_at,audit_status,audit_by,audit_at,status) SELECT 2,stock_id,ts_code,source_type,source_excerpt,excerpt_trimmed,source_url,collected_at,audit_status,audit_by,audit_at,status FROM addon_quant_theme_stock WHERE id=1`)
					case "source":
						e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
					case "review":
						e.mustExec(`UPDATE addon_quant_theme_stock SET audit_by=audit_by+1 WHERE id=1`)
					case "missing_binding":
						e.planner.p.Companies = nil
					case "during_check":
						e.control.hook = func() error {
							if e.server.Count() != 3 {
								return nil
							}
							_, err := e.db.ExecContext(e.ctx, `UPDATE addon_quant_theme_stock SET status=0 WHERE id=1`)
							return err
						}
					}
					out, err := admissionWorker(t, e).Run(e.ctx, 1, e.v.ID)
					if err == nil && out.Commit.Status == task.Succeeded {
						t.Fatal("失效公司范围被交付", mode)
					}
					if mode == "deleted" || mode == "new_mapping" || mode == "source" || mode == "review" {
						if err != nil || out.Task.TaskStatus != task.Failed || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release' AND biz_id=?`, "task:"+e.v.No) != 1 {
							t.Fatal("公司范围已知失效没有失败释放", mode, out, err)
						}
					}
					if mode == "during_check" && (err != nil || out.Task.Outcome != "response_unknown" || out.Task.TaskStatus != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7) {
						t.Fatal("判定响应尚未持久确认却被释放或伪装终态", out, err)
					}
					if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 {
						t.Fatal("异常公司结算")
					}
					if mode != "during_check" && (e.submits.Load() != 0 || len(e.server.Requests()) != 0) {
						t.Fatal("准入失效仍发起调用")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportLibraryAtomicity(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"report", "event", "points", "unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := newLibraryReportEnv(t, level, "pass")
					switch mode {
					case "report":
						e.mustExec(`CREATE TRIGGER test_library_report_abort BEFORE INSERT ON tm_ai_report_commits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only library report failure'`)
					case "event":
						e.mustExec(`CREATE TRIGGER test_library_event_abort BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='succeeded' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only library event failure'; END IF; END`)
					case "points":
						e.mustExec(`CREATE TRIGGER test_library_points_abort BEFORE INSERT ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only library points failure'`)
					case "unknown":
						e.workflowEnv.store.phase = "final"
					}
					w, err := service.NewAIL1ReportWorker(e.tasks, e.workflowEnv.store, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
					if err != nil {
						t.Fatal(err)
					}
					out, err := w.Run(e.ctx, 1, e.v.ID)
					if err == nil || out.Commit.TaskID != 0 {
						t.Fatal("事务故障返回候选成功", mode, err)
					}
					if mode != "unknown" {
						if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
							t.Fatal("公司报告留下半提交")
						}
						switch mode {
						case "report":
							e.mustExec(`DROP TRIGGER test_library_report_abort`)
						case "event":
							e.mustExec(`DROP TRIGGER test_library_event_abort`)
						case "points":
							e.mustExec(`DROP TRIGGER test_library_points_abort`)
						}
					}
					again, err := w.Run(e.ctx, 1, e.v.ID)
					if err != nil || again.Task.TaskStatus != task.Succeeded || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.submits.Load() != 1 || e.server.Count() != 3 {
						t.Fatal("恢复重跑模型或重复结算", mode, again, err)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
