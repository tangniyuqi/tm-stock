//go:build integration && browser

package points_test

import (
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"os"
	"testing"
)

func TestIntegrationAIReportLocalAPIWorkerH5(t *testing.T) {
	mode, depth, origin := os.Getenv("TM_TEST_AI_H5_LOCAL_API_WORKER"), os.Getenv("TM_TEST_AI_H5_LOCAL_DEPTH"), os.Getenv("TM_TEST_AI_H5_ORIGIN")
	if !localAPIWorkerH5Configuration(mode, depth, origin) {
		t.Fatal("只允许显式本地原登录、轻深选择及完整H5回环源")
	}
	for _, key := range []string{"TM_TEST_AI_LIVE", "TM_TEST_AI_H5_DEEP", "TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_TIMEOUT_RECOVERY", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_REPORT_QA", "TM_TEST_AI_H5_ASSISTANT"} {
		if os.Getenv(key) != "" {
			t.Fatal("正式进程H5不能混用真实调用或其它浏览器专项")
		}
	}
	marker := os.Getenv("TM_TEST_AI_H5_DOSSIER_REVIEW")
	if marker != "" {
		if marker != "local-fixture-only" {
			t.Fatal("专题报告H5仅允许显式合成隔离模式")
		}
		runLocalProcessH5(t, depth, origin, &localProcessCase{dossier: true, configure: func(t *testing.T, f *localPublicationFixture, c *config.AILocalExecution) {
			configureDossierLocalFixture(t, f, "partial")
			c.Model.BaseURL = f.env.server.URL()
		}})
		return
	}
	runLocalProcessH5(t, depth, origin, nil)
}
