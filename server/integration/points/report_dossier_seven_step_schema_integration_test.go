//go:build integration

package points_test

import (
	"database/sql"
	"testing"
)

func TestIntegrationAIReportDossierSevenStepSchema(t *testing.T) {
	f := newDossierWorkflowFixture(t, sql.LevelReadCommitted, 2, "comparison_supported")
	e := f.env
	// 表名均为固定内部清单，不接用户或模型输入。
	tables := []string{"tm_points_accounts", "tm_points_txns", "tm_points_holds", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_admissions", "tm_ai_report_call_audits", "tm_ai_report_commits"}
	before := map[string]int{}
	for _, table := range tables {
		before[table] = e.queryInt("SELECT COUNT(*) FROM " + table)
	}
	check := func(t *testing.T, want bool) {
		t.Helper()
		err := e.repo.VerifyDossierComparisonReportSchema(e.ctx)
		if (err == nil) != want {
			t.Fatal("七步真实正反例自检结果不符", err)
		}
		for _, table := range tables {
			if e.queryInt("SELECT COUNT(*) FROM "+table) != before[table] {
				t.Fatal("启动自检留下写入", table)
			}
		}
	}
	t.Run("实际探针全部回滚", func(t *testing.T) { check(t, true) })
	t.Run("缺第六步CHECK", func(t *testing.T) {
		e.mustExec(`ALTER TABLE tm_ai_report_call_audits DROP CHECK chk_tm_ai_report_call_step, ADD CONSTRAINT chk_tm_ai_report_call_step CHECK((step_no=2 AND stage='extract') OR (step_no IN(3,4,5) AND stage='judge'))`)
		defer reportDDL(t, e.pointsTestEnv, "20261102_enable_dossier_comparison_report_audit.sql")
		check(t, false)
	})
	t.Run("旧触发器放开第六步", func(t *testing.T) {
		reportDDL(t, e.pointsTestEnv, "20261102_allow_dossier_report_model_step.sql")
		e.mustExec(`ALTER TABLE tm_ai_report_call_audits DROP CHECK chk_tm_ai_report_call_step, ADD CONSTRAINT chk_tm_ai_report_call_step CHECK((step_no=2 AND stage='extract') OR (step_no IN(3,4,5,6) AND stage='judge'))`)
		defer reportDDL(t, e.pointsTestEnv, "20261102_enable_dossier_comparison_report_audit.sql")
		check(t, false)
	})
	t.Run("恢复后重新核验", func(t *testing.T) { check(t, true) })
	if e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
		t.Fatal("自检调用Agent或模型")
	}
}
