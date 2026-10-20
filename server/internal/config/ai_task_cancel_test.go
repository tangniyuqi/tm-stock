package config

import (
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

func TestLoadAITaskCancelIsIndependentAndClosed(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, tc := range []struct {
		raw           string
		read          AITaskReadConfig
		want, invalid bool
	}{{"", good, false, false}, {"false", good, false, false}, {"true", good, true, false}, {"yes", good, false, true}, {"true", AITaskReadConfig{}, false, true}, {"true", AITaskReadConfig{Enabled: true}, false, true}, {"false", AITaskReadConfig{}, false, false}} {
		t.Setenv("TM_AI_TASK_CANCEL_ENABLED", tc.raw)
		t.Setenv("TM_DEV_GRANT_PAID", "true")
		got, err := LoadAITaskCancel(tc.read)
		if got != tc.want || (err != nil) != tc.invalid {
			t.Fatal(tc.raw, got, err)
		}
	}
}
