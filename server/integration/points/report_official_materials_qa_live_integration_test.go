//go:build integration && liveagents

package points_test

import (
	"net/url"
	"os"
	"strconv"
	"testing"
)

func TestIntegrationAIReportOfficialMaterialQALive(t *testing.T) {
	endpoint := os.Getenv("TM_TEST_AI_OFFICIAL_QA_ENDPOINT")
	u, err := url.Parse(endpoint)
	port := 0
	if err == nil {
		port, err = strconv.Atoi(u.Port())
	}
	if os.Getenv("TM_TEST_DSN") == "" || os.Getenv("TM_TEST_AI_LIVE") != "local-fixture-only" || err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || port < 1024 || port > 65535 || u.Host != "127.0.0.1:"+strconv.Itoa(port) || u.User != nil || u.Path != "/v1" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		t.Fatal("真实正式资料问答仅允许显式隔离库和本次回环网关")
	}
	verifyOfficialMaterialQA(t, endpoint)
}
