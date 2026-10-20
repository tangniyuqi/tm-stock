package repository

import (
	"strings"
	"testing"
)

func TestAISourceMigrationsAreOrderedAndFailClosed(t *testing.T) {
	sourceName := "20261010_create_ai_source_snapshots.sql"
	candidateName := "20261010_create_tm_ai_extraction_candidates.sql"
	if sourceName >= candidateName {
		t.Fatalf("候选迁移必须在来源/快照迁移之后按文件名执行：%s, %s", sourceName, candidateName)
	}
	sourceSQL := readMigration(t, sourceName)
	candidateSQL := readMigration(t, candidateName)
	runName := "20261010_create_tm_ai_l0_extraction_runs.sql"
	if candidateName >= runName {
		t.Fatal("调用审计迁移必须排在候选迁移之后")
	}
	runSQL := readMigration(t, runName)
	inspectionName := "20261010_create_tm_ai_review_inspections.sql"
	if runName >= inspectionName {
		t.Fatal("抽检迁移应排在审核与运行迁移之后")
	}
	inspectionSQL := readMigration(t, inspectionName)
	auditName := "20261010_create_tm_ai_source_audits.sql"
	if inspectionName >= auditName {
		t.Fatal("来源操作审计迁移须排在已依赖迁移之后")
	}
	auditSQL := readMigration(t, auditName)
	for _, required := range []string{"CREATE TABLE IF NOT EXISTS `tm_ai_source_audits`", "tm_ai_source_audits_no_update", "tm_ai_source_audits_no_delete", "idx_tm_ai_source_audit_history", "REFERENCES `tm_ai_sources`", "REFERENCES `tm_ai_source_snapshots`", "`before_json` json", "`after_json` json", "`snapshot_id` IS NOT NULL"} {
		if !strings.Contains(auditSQL, required) {
			t.Errorf("来源审计迁移缺少 %s", required)
		}
	}
	for _, table := range []string{"tm_ai_inspection_batches", "tm_ai_inspection_items", "tm_ai_inspection_events"} {
		for _, required := range []string{"CREATE TABLE IF NOT EXISTS `" + table + "`", table + "_no_update", table + "_no_delete"} {
			if !strings.Contains(inspectionSQL, required) {
				t.Errorf("抽检迁移缺少 %s", required)
			}
		}
	}
	for _, required := range []string{"uk_tm_ai_inspection_batch_request", "uk_tm_ai_inspection_event_request", "uk_tm_ai_inspection_event_item", "population_review_ids", "idx_tm_ai_inspection_issue"} {
		if !strings.Contains(inspectionSQL, required) {
			t.Errorf("抽检迁移缺少 %s", required)
		}
	}
	for _, required := range []string{"CREATE TABLE IF NOT EXISTS `tm_ai_l0_extraction_runs`", "CREATE TABLE IF NOT EXISTS `tm_ai_l0_call_events`", "tm_ai_l0_call_events_no_update", "tm_ai_l0_call_events_no_delete", "tm_ai_l0_runs_no_delete", "UNIQUE KEY `uk_tm_ai_l0_run_request`"} {
		if !strings.Contains(runSQL, required) {
			t.Errorf("抽取请求迁移缺少安全契约 %q", required)
		}
	}
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS `tm_ai_sources`",
		"`enabled` tinyint NOT NULL DEFAULT 0",
		"CREATE TABLE IF NOT EXISTS `tm_ai_source_bodies`",
		"CREATE TABLE IF NOT EXISTS `tm_ai_source_snapshots`",
		"`body_ref` char(64)",
		"tm_ai_source_snapshots_no_update",
		"tm_ai_source_snapshots_no_delete",
		"tm_ai_source_bodies_no_update",
		"tm_ai_source_bodies_no_delete",
	} {
		if !strings.Contains(sourceSQL, required) {
			t.Errorf("来源迁移缺少安全契约 %q", required)
		}
	}
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS `tm_ai_extraction_candidates`",
		"tm_ai_candidates_no_update",
		"tm_ai_candidates_no_delete",
		"CREATE TABLE IF NOT EXISTS `tm_ai_review_audits`",
		"UNIQUE KEY `uk_tm_ai_review_candidate` (`candidate_id`)",
		"UNIQUE KEY `uk_tm_ai_review_request` (`request_id`)",
		"tm_ai_review_audits_no_update",
		"tm_ai_review_audits_no_delete",
		"REFERENCES `tm_ai_source_snapshots` (`id`)",
		"`source_excerpt` varchar(1000)",
		"`sentence_ids` json NOT NULL",
		"`quote_segments` json NOT NULL",
	} {
		if !strings.Contains(candidateSQL, required) {
			t.Errorf("候选迁移缺少安全契约 %q", required)
		}
	}
	if strings.Contains(candidateSQL, "INSERT INTO `addon_quant_theme_stock`") || strings.Contains(candidateSQL, "INSERT INTO addon_quant_theme_stock") {
		t.Fatal("待审候选迁移不得直接写入对外题材股票表")
	}
}
