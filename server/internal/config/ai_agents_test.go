package config

import (
	"strings"
	"testing"
)

func TestAIAgentsConfigRequiresExplicitApproval(t *testing.T) {
	values := map[string]string{"TM_AI_AGENTS_ENABLED": "true", "TM_AI_AGENTS_URL": "https://agents.example.test", "TM_AI_AGENTS_APPROVED_HOST": "agents.example.test", "TM_AI_AGENTS_TOKEN": strings.Repeat("x", 40), "TM_AI_AGENTS_APPROVAL_REF": "test-only:approval", "TM_AI_AGENTS_LICENSE_REF": "test-only:license", "TM_AI_AGENTS_QUICK_MODEL": "test-quick", "TM_AI_AGENTS_DEEP_MODEL": "test-deep", "TM_AI_AGENTS_TIMEOUT_MS": "1000", "TM_AI_AGENTS_MAX_SUBMISSIONS": "2"}
	for k, v := range values {
		t.Setenv(k, v)
	}
	if c, err := LoadAIAgents(); err != nil || !c.Enabled {
		t.Fatal(err)
	}
	for k := range values {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, "")
			_, err := LoadAIAgents()
			if k != "TM_AI_AGENTS_ENABLED" && err == nil {
				t.Fatal("缺失关键配置未拒绝")
			}
		})
	}
	for _, v := range []string{"1", "yes", "True", " false"} {
		t.Setenv("TM_AI_AGENTS_ENABLED", v)
		if _, err := LoadAIAgents(); err == nil {
			t.Fatal("开关含糊值未拒绝")
		}
	}
	for _, v := range []string{"", "false"} {
		t.Setenv("TM_AI_AGENTS_ENABLED", v)
		t.Setenv("TM_AI_AGENTS_URL", "bad")
		c, err := LoadAIAgents()
		if err != nil || c.Enabled {
			t.Fatal("默认未关闭")
		}
	}
}
