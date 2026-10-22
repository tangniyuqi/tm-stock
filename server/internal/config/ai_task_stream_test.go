package config

import (
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

func TestLoadAITaskStreamClosed(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, tc := range []struct {
		raw       string
		read      AITaskReadConfig
		want, bad bool
	}{{"", good, false, false}, {"false", AITaskReadConfig{}, false, false}, {"true", good, true, false}, {"yes", good, false, true}, {"true", AITaskReadConfig{}, false, true}, {"true", AITaskReadConfig{Enabled: true}, false, true}} {
		t.Setenv("TM_AI_TASK_STREAM_ENABLED", tc.raw)
		t.Setenv("TM_DEV_GRANT_PAID", "true")
		got, err := LoadAITaskStream(tc.read)
		if got != tc.want || (err != nil) != tc.bad {
			t.Fatal(tc.raw, got, err)
		}
	}
}
