package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"strings"
	"testing"
)

func TestPointsBalanceIndependentFlag(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, raw := range []string{"", "false", "true", "yes", "TRUE"} {
		t.Setenv("TM_AI_POINTS_BALANCE_ENABLED", raw)
		t.Setenv("TM_DEV_GRANT_PAID", "true")
		on, err := LoadPointsBalance(good)
		if on != (raw == "true") || (err != nil) != (raw == "yes" || raw == "TRUE") {
			t.Fatal(raw, on, err)
		}
	}
	t.Setenv("TM_AI_POINTS_BALANCE_ENABLED", "true")
	for _, bad := range []AITaskReadConfig{{}, {Enabled: true}} {
		if on, err := LoadPointsBalance(bad); on || err == nil {
			t.Fatal(on, err)
		}
	}
}
