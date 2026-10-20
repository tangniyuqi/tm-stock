package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"strings"
	"testing"
)

func TestLoadAIReportReadClosedAndIndependent(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, tc := range []struct {
		raw       string
		read      AITaskReadConfig
		want, bad bool
	}{{"", good, false, false}, {"false", good, false, false}, {"true", good, true, false}, {"yes", good, false, true}, {"true", AITaskReadConfig{}, false, true}, {"true", AITaskReadConfig{Enabled: true}, false, true}, {"false", AITaskReadConfig{}, false, false}} {
		t.Setenv("TM_AI_REPORT_READ_ENABLED", tc.raw)
		t.Setenv("TM_DEV_GRANT_PAID", "true")
		t.Setenv("TM_AI_AGENTS_ENABLED", "true")
		got, err := LoadAIReportRead(tc.read)
		if got != tc.want || (err != nil) != tc.bad {
			t.Fatal(tc.raw, got, err)
		}
	}
}
