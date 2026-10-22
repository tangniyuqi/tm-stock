package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"strings"
	"testing"
)

func TestAIFeedbackIndependentFlags(t *testing.T) {
	read := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, value := range []string{"", "false", "true", "TRUE", "1", "bad"} {
		t.Run("flag_"+value, func(t *testing.T) {
			t.Setenv("TM_AI_FEEDBACK_ENABLED", value)
			t.Setenv("TM_AI_FEEDBACK_ADMIN_ENABLED", value)
			on, err := LoadAIFeedback(read)
			admin, e := LoadAIFeedbackAdmin()
			valid := value == "" || value == "false" || value == "true"
			if (err == nil) != valid || (e == nil) != valid || on != (value == "true") || admin != on {
				t.Fatal("反馈开关口径不一致")
			}
		})
	}
	t.Setenv("TM_AI_FEEDBACK_ENABLED", "true")
	for _, bad := range []AITaskReadConfig{{}, {Enabled: true}} {
		if _, err := LoadAIFeedback(bad); err == nil {
			t.Fatal("未配置会员身份即开放")
		}
	}
}
