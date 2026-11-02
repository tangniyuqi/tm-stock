//go:build integration

package points_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 原API/worker独立进程和密码登录；Agent/模型仍为合成回环协议。
func TestIntegrationAIReportDossierSevenStepProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			verifyLocalAPIWorker(t, level, 2, api, worker, login, localProcessCase{dossier: true, comparison: true, configure: func(t *testing.T, f *localPublicationFixture, execution *config.AILocalExecution) {
				configureDossierLocalFixture(t, f, "comparison_supported")
				execution.Model.BaseURL = f.env.server.URL()
			}})
		})
	}
}
