//go:build integration

package points_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestIntegrationAIReportFollowupFeedbackProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	admin := buildLocalFeedbackAdmin(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			verifyLocalAPIWorker(t, level, 2, api, worker, login, localProcessCase{dossier: true, comparison: true, qa: true, inputs: true, feedbackAdmin: admin, configure: func(t *testing.T, f *localPublicationFixture, c *config.AILocalExecution) {
				f.spec.CheckProtocol = finalreport.PairedCheckProtocol
				configureDossierLocalFixture(t, f, "comparison_supported")
				c.Model.BaseURL = f.env.server.URL()
			}})
		})
	}
}
