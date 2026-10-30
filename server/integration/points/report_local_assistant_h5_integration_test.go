//go:build integration && browser

package points_test

import (
	"flag"
	"os"
	"testing"
)

func TestIntegrationAIReportLocalAssistantH5(t *testing.T) {
	depth, origin := os.Getenv("TM_TEST_AI_H5_LOCAL_DEPTH"), os.Getenv("TM_TEST_AI_H5_ORIGIN")
	if os.Getenv("TM_TEST_AI_H5_LOCAL_ASSISTANT") != "local-fixture-only" || !localAPIWorkerH5Configuration("local-fixture-only", depth, origin) || flag.Lookup("test.count").Value.String() != "1" || flag.Lookup("test.run").Value.String() != "^TestIntegrationAIReportLocalAssistantH5$" {
		t.Fatal("只允许独立单次完整H5组合验收")
	}
	for _, key := range []string{"TM_TEST_AI_LIVE", "TM_TEST_AI_H5_LOCAL_API_WORKER", "TM_TEST_AI_H5_REPORT_QA", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_ASSISTANT", "TM_TEST_AI_H5_TIMEOUT_RECOVERY", "TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_DEEP"} {
		if os.Getenv(key) != "" {
			t.Fatal("不混用其它真实或浏览器专项")
		}
	}
	runLocalProcessH5(t, depth, origin, &localProcessCase{qa: true})
}
