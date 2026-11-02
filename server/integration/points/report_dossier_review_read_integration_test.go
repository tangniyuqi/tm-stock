//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"strconv"
	"testing"
)

type dossierReviewReadControl func(context.Context, service.AIReportPublication) error

func (c dossierReviewReadControl) Recheck(ctx context.Context, p service.AIReportPublication) error {
	return c(ctx, p)
}

func TestIntegrationAIReportDossierReviewRead(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					f := newDossierWorkflowFixture(t, level, kind, "partial")
					e := f.env
					if _, err := f.worker.Run(e.ctx, 1, f.id); err != nil {
						t.Fatal(err)
					}
					control, err := service.NewAILocalReportReadControl(f.admission)
					if err != nil {
						t.Fatal(err)
					}
					base, err := service.NewAIReportService(e.repo, e.ledger, nil, false)
					if err != nil {
						t.Fatal(err)
					}
					reader, err := service.NewAIReportReadServiceWithPolicySource(base, control, control, true)
					if err != nil {
						t.Fatal(err)
					}
					calls, txns := len(e.server.Requests()), e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
					out, err := reader.Read(e.ctx, 1, f.id)
					if err != nil || out.DossierReview == nil || out.DossierReview.Version != 1 || len(out.DossierReview.Items) != 1 || out.DossierReview.Items[0].Verdict != "partial" || out.DossierReview.Items[0].Reason != "partial_support" || out.DossierReview.Items[0].Checked.SourceURL == "" {
						t.Fatal("本人成功报告没有原字段核验", err)
					}
					for _, entry := range []struct{ name, table, where string }{{"use", "tm_ai_dossier_support_uses", ""}, {"publication", "tm_ai_dossier_publications", ""}, {"grant", "tm_ai_dossier_grants", ""}, {"source", "tm_ai_sources", " WHERE id=1"}} {
						t.Run(entry.name, func(t *testing.T) {
							e.mustExec("UPDATE " + entry.table + " SET enabled=0" + entry.where)
							denied, err := reader.Read(e.ctx, 1, f.id)
							if err == nil || denied.TaskID != 0 || denied.DossierReview != nil || len(denied.Sections) != 0 {
								t.Fatal("当前撤权泄漏部分报告")
							}
							e.mustExec("UPDATE " + entry.table + " SET enabled=1" + entry.where)
							if restored, err := reader.Read(e.ctx, 1, f.id); err != nil || restored.DossierReview == nil {
								t.Fatal("恢复后不能重读", err)
							}
						})
					}
					t.Run("cross_member", func(t *testing.T) {
						denied, err := reader.Read(e.ctx, 2, f.id)
						if err == nil || denied.TaskID != 0 || denied.DossierReview != nil {
							t.Fatal("跨会员读取")
						}
					})
					t.Run("between_reads", func(t *testing.T) {
						first := true
						wrapper := dossierReviewReadControl(func(ctx context.Context, p service.AIReportPublication) error {
							if err := control.Recheck(ctx, p); err != nil {
								return err
							}
							if first {
								first = false
								_, err := e.db.ExecContext(ctx, `UPDATE tm_ai_dossier_support_uses SET enabled=0`)
								return err
							}
							return nil
						})
						raced, err := service.NewAIReportReadServiceWithPolicySource(base, control, wrapper, true)
						if err != nil {
							t.Fatal(err)
						}
						denied, err := raced.Read(e.ctx, 1, f.id)
						if err == nil || denied.TaskID != 0 || denied.DossierReview != nil {
							t.Fatal("两次读取之间撤权仍交付")
						}
						e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
					})
					if len(e.server.Requests()) != calls || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != txns || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 {
						t.Fatal("重复读取触发模型或账务")
					}
					v, err := e.tasks.Inspect(e.ctx, 1, f.id)
					if err != nil || v.Status != task.Succeeded {
						t.Fatal("读取改变任务", err)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
