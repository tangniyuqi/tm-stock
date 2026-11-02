//go:build integration

package points_test

import (
	"database/sql"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func TestIntegrationAIReportDossierComparisonWorkflow(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"supported", "partial_consistent", "uncertain", "mismatch", "entailment", "semantic", "comparison_transport", "comparison_malformed"} {
				t.Run(mode, func(t *testing.T) {
					f := newDossierWorkflowFixture(t, level, 2, "comparison_"+mode)
					e := f.env
					got, err := f.worker.Run(e.ctx, 1, f.id)
					if mode == "comparison_transport" || mode == "comparison_malformed" {
						v, readErr := e.tasks.Inspect(e.ctx, 1, f.id)
						if readErr != nil || v.Status != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != int(f.spec.PricePoints) {
							t.Fatal("未知比较调用释放或改变终态", readErr)
						}
						calls := len(e.server.Requests())
						again, retryErr := f.worker.Run(e.ctx, 1, f.id)
						if retryErr != nil || again.Task.Outcome != "response_unknown" || len(e.server.Requests()) != calls {
							t.Fatal("未知比较恢复重发或丢失原键", retryErr)
						}
						if calls != 4 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 8 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 {
							t.Fatal("比较未知调用审计或副作用不符", err)
						}
						return
					}
					if err != nil {
						state, _ := e.tasks.Inspect(e.ctx, 1, f.id)
						t.Fatal("七步比较执行失败", err, "当前步", state.CurrentStep, "模型请求数", len(e.server.Requests()), "审计末步", e.queryInt(`SELECT COALESCE(MAX(step_no),0) FROM tm_ai_report_call_audits`))
					}
					v, err := e.tasks.Inspect(e.ctx, 1, f.id)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "supported" || mode == "partial_consistent" {
						if got.Commit.Status != task.Succeeded || v.Status != task.Succeeded || v.CurrentStep != 7 || len(e.server.Requests()) != 5 || e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 10 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3`) != 1 {
							t.Fatal("没有完整七步、原审计或单报告", "收据状态", got.Commit.Status, "任务状态", v.Status, "步号", v.CurrentStep, "模型请求", len(e.server.Requests()), "Agent请求", e.submits.Load(), "审计数", e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`), "成功报告数", e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3`))
						}
					} else if v.Status != task.Failed || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3`) != 0 {
						t.Fatal("不足或矛盾回执被当成功报告")
					}
					if e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
						t.Fatal("终检未结算或释放冻结")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
					calls := len(e.server.Requests())
					txns := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
					if _, err = f.worker.Run(e.ctx, 1, f.id); err != nil || len(e.server.Requests()) != calls || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != txns {
						t.Fatal("终态重放重复调用或账务", err)
					}
					if v.Status == task.Succeeded {
						ctl, err := service.NewAILocalReportReadControl(f.admission)
						if err != nil {
							t.Fatal(err)
						}
						resolved, err := ctl.Resolve(e.ctx, 1, f.id)
						if err != nil || resolved.Policy == nil {
							t.Fatal("七步成功报告策略重建失败", err)
						}
						comparison, review := resolved.Policy.ComparisonProof()
						if !comparison.Valid() || review == nil {
							t.Fatal("成功读取丢失比较捕获")
						}
						base, err := service.NewAIReportService(e.repo, e.ledger, nil, false)
						if err != nil {
							t.Fatal(err)
						}
						reads, err := service.NewAIReportReadServiceWithPolicySource(base, ctl, ctl, true)
						if err != nil {
							t.Fatal(err)
						}
						if report, err := reads.Read(e.ctx, 1, f.id); err != nil || report.TaskID != f.id || report.DossierReview == nil || report.DossierReview.Comparison == nil || len(report.DossierReview.Comparison.Axes) != 7 || report.DossierReview.Comparison.Left.Key != "test_card" || report.DossierReview.Comparison.Right.Key != "test_card_b" {
							t.Fatal("七步成功报告无法读取", err)
						}
						if report, err := reads.Read(e.ctx, 2, f.id); err == nil || report.TaskID != 0 || report.DossierReview != nil || len(report.Sections) != 0 {
							t.Fatal("他人读到比较报告")
						}
						for _, component := range []string{f.spec.DossierSupportDigest, f.spec.DossierComparisonDigest} {
							if _, err = e.db.ExecContext(e.ctx, `UPDATE tm_ai_dossier_support_uses SET enabled=0 WHERE component_digest=?`, component); err != nil {
								t.Fatal(err)
							}
							if report, err := reads.Read(e.ctx, 1, f.id); err == nil || report.TaskID != 0 || report.DossierReview != nil || len(report.Sections) != 0 {
								t.Fatal("任一用途撤回后仍交付报告")
							}
							if _, err = e.db.ExecContext(e.ctx, `UPDATE tm_ai_dossier_support_uses SET enabled=1 WHERE component_digest=?`, component); err != nil {
								t.Fatal(err)
							}
							if report, err := reads.Read(e.ctx, 1, f.id); err != nil || report.TaskID != f.id || report.DossierReview == nil || report.DossierReview.Comparison == nil {
								t.Fatal("恢复用途不能复读原报告", err)
							}
						}
						for _, revoke := range []struct{ off, on string }{
							{`UPDATE tm_ai_dossier_grants SET enabled=0 WHERE user_id=1`, `UPDATE tm_ai_dossier_grants SET enabled=1 WHERE user_id=1`},
							{`UPDATE tm_ai_dossier_publications SET enabled=0`, `UPDATE tm_ai_dossier_publications SET enabled=1`},
							{`UPDATE tm_ai_sources SET enabled=0 WHERE id=1`, `UPDATE tm_ai_sources SET enabled=1 WHERE id=1`},
						} {
							e.mustExec(revoke.off)
							if report, err := reads.Read(e.ctx, 1, f.id); err == nil || report.TaskID != 0 || report.DossierReview != nil || len(report.Sections) != 0 {
								t.Fatal("会员、发布或来源撤回后仍泄漏比较内容")
							}
							e.mustExec(revoke.on)
							if report, err := reads.Read(e.ctx, 1, f.id); err != nil || report.DossierReview == nil || report.DossierReview.Comparison == nil {
								t.Fatal("恢复授权后不能复读原比较", err)
							}
						}
						if len(e.server.Requests()) != calls || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != txns {
							t.Fatal("读取新增调用或账务")
						}
					}
				})
			}
		})
	}
}
