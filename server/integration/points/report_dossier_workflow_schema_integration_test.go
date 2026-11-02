//go:build integration

package points_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func TestIntegrationAIReportDossierWorkflowSchema(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"current", "legacy_step", "missing_profile_guard"} {
				t.Run(mode, func(t *testing.T) {
					f := newDossierWorkflowFixture(t, level, 1, "partial")
					e := f.env
					if mode == "legacy_step" {
						e.mustExec(`ALTER TABLE tm_ai_report_call_audits DROP CHECK chk_tm_ai_report_call_step; ALTER TABLE tm_ai_report_call_audits ADD CONSTRAINT chk_tm_ai_report_call_step CHECK((step_no=2 AND stage='extract') OR (step_no IN (3,4) AND stage='judge'))`)
					} else if mode == "missing_profile_guard" {
						var body string
						if err := e.db.QueryRowContext(e.ctx, `SELECT action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name='tm_ai_report_call_insert'`).Scan(&body); err != nil {
							t.Fatal(err)
						}
						clause := "AND (NEW.step_no<>5 OR JSON_UNQUOTE(JSON_EXTRACT(CONVERT(a.payload USING utf8mb4),'$.Plan.ReportProfile'))='dossier-report-v1')"
						if strings.Count(body, clause) != 1 {
							t.Fatal("未找到精确保护条件，故障未注入")
						}
						e.mustExec(`DROP TRIGGER tm_ai_report_call_insert`)
						e.mustExec("CREATE TRIGGER tm_ai_report_call_insert BEFORE INSERT ON tm_ai_report_call_audits FOR EACH ROW " + strings.Replace(body, clause, "", 1))
					}
					before := map[string]int{}
					for _, table := range []string{"tm_points_accounts", "tm_points_txns", "tm_points_holds", "tm_ai_tasks", "tm_ai_task_admissions", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_events", "tm_ai_report_call_audits", "tm_ai_report_commits"} {
						before[table] = e.queryInt("SELECT COUNT(*) FROM " + table)
					}
					// 原入口仍可验旧2—4步；开启六步必须额外验证第5步正反例。
					if err := repository.NewAIReportCallAuditSchemaVerifier(e.db).Verify(e.ctx); err != nil {
						t.Fatal("原自检误拒兼容结构", err)
					}
					err := f.admission.Ready(e.ctx)
					if (mode == "current") != (err == nil) {
						t.Fatal("启动未区分新版结构及编排门禁", err)
					}
					for table, count := range before {
						if e.queryInt("SELECT COUNT(*) FROM "+table) != count {
							t.Fatal("回滚探针留下数据", table)
						}
					}
					if len(e.server.Requests()) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
						t.Fatal("启动核验请求模型或改变冻结")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
