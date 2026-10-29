//go:build integration

package points_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func reportInsertBody(t *testing.T, e *pointsTestEnv) string {
	t.Helper()
	var body string
	if err := e.conn.QueryRowContext(e.ctx, "SELECT action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name='tm_ai_report_insert'").Scan(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestIntegrationAIReportDeepUpgradeExecution(t *testing.T) {
	runDeepAdmissionExecution(t, []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead}, func(t *testing.T, e *workflowEnv, _ *deepWorkflowConfig) {
		// 明确重现仅执行过旧迁移的库，随后只执行新增量，不靠重放旧建表迁移升级。
		reportDDL(t, e.pointsTestEnv, "20261016_create_tm_ai_report_commits.sql")
		if !strings.Contains(reportInsertBody(t, e.pointsTestEnv), "t.analysis_type=1") {
			t.Fatal("未复现旧 L1 触发器")
		}
		verifier := repository.NewAIReportSchemaVerifier(e.db)
		if err := verifier.Verify(e.ctx); err != nil {
			t.Fatal("旧库轻度保护失效", err)
		}
		if err := verifier.VerifyWithDeep(e.ctx); err == nil {
			t.Fatal("旧库错误通过深度启动探针")
		}
		reportDDL(t, e.pointsTestEnv, "20261023_allow_deep_ai_report_commits.sql")
		if err := verifier.VerifyWithDeep(e.ctx); err != nil {
			t.Fatal("升级后深度启动探针失败", err)
		}
		// 其后执行原深度准入→实际worker→报告→结算断言，两种隔离级别均走真实SQL。
	})
}

func TestIntegrationAIReportDeepUpgradePreservesHistory(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newWorkflowEnv(t, level, "pass")
			if _, err := e.worker(t, true).Run(e.ctx, e.v.UserID, e.v.ID); err != nil {
				t.Fatal(err)
			}
			var before, after string
			query := "SELECT CONCAT(content_digest,':',terminal_txn_id,':',HEX(payload)) FROM tm_ai_report_commits WHERE task_id=?"
			if err := e.db.QueryRowContext(e.ctx, query, e.v.ID).Scan(&before); err != nil {
				t.Fatal(err)
			}
			txns := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			reportDDL(t, e.pointsTestEnv, "20261016_create_tm_ai_report_commits.sql")
			reportDDL(t, e.pointsTestEnv, "20261023_allow_deep_ai_report_commits.sql")
			if err := e.db.QueryRowContext(e.ctx, query, e.v.ID).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after || txns != e.queryInt("SELECT COUNT(*) FROM tm_points_txns") {
				t.Fatal("升级修改历史报告或积分账本")
			}
			if _, err := e.db.ExecContext(e.ctx, "UPDATE tm_ai_report_commits SET payload=payload WHERE task_id=?", e.v.ID); err == nil {
				t.Fatal("升级丢失不可变保护")
			}
			if _, err := e.db.ExecContext(e.ctx, "DELETE FROM tm_ai_report_commits WHERE task_id=?", e.v.ID); err == nil {
				t.Fatal("升级允许删除历史")
			}
		})
	}
}
