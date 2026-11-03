//go:build integration

package points_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 新协议沿原密码登录、独立API/worker和七步报告；模型仍是明确回环替身。
func TestIntegrationAIReportPairedCheckProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			verifyLocalAPIWorker(t, level, 2, api, worker, login, localProcessCase{dossier: true, comparison: true, inputs: true, configure: func(t *testing.T, f *localPublicationFixture, execution *config.AILocalExecution) {
				f.spec.CheckProtocol = finalreport.PairedCheckProtocol
				configureDossierLocalFixture(t, f, "comparison_supported")
				execution.Model.BaseURL = f.env.server.URL()
			}})
		})
	}
}
