package config

import (
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func validL0Environment(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{"TM_AI_L0_ENABLED": "true", "TM_AI_ENABLED": "true", "TM_AI_PROVIDER": "test-vendor", "TM_AI_BASE_URL": "https://api.example.test/v1", "TM_AI_MODEL": "test-model", "TM_AI_API_KEY": "test_placeholder_key", "TM_AI_TIMEOUT_MS": "30000", "TM_AI_L0_PROVIDER_DOMAIN": "api.example.test", "TM_AI_L0_APPROVAL_REF": "test-approval:written-record", "TM_AI_MODEL_REGISTRY": "test-registry.json", "TM_AI_L0_MODEL_ID": "test-model-id", "TM_AI_L0_MAX_CALLS": "10", "TM_AI_L0_MAX_PROMPT_BYTES": "65536"} {
		t.Setenv(key, value)
	}
}

func TestL0ConfigRequiresExplicitApprovalAndFiniteLimits(t *testing.T) {
	validL0Environment(t)
	cfg, err := LoadAIL0()
	if err != nil || !cfg.Enabled || cfg.MaxCalls != 10 {
		t.Fatalf("合法测试配置失败: %v", err)
	}
	for _, tc := range []struct{ key, value string }{
		{"TM_AI_L0_ENABLED", "1"}, {"TM_AI_ENABLED", "false"}, {"TM_AI_L0_APPROVAL_REF", ""}, {"TM_AI_MODEL_REGISTRY", ""}, {"TM_AI_L0_MODEL_ID", ""},
		{"TM_AI_L0_MAX_CALLS", "0"}, {"TM_AI_L0_MAX_CALLS", "1001"}, {"TM_AI_L0_MAX_CALLS", "bad"}, {"TM_AI_L0_MAX_PROMPT_BYTES", "4095"}, {"TM_AI_L0_MAX_PROMPT_BYTES", "262145"},
		{"TM_AI_BASE_URL", "http://api.example.test/v1"}, {"TM_AI_BASE_URL", "https://other.example.test/v1"}, {"TM_AI_BASE_URL", "https://127.0.0.1/v1"},
		{"TM_AI_BASE_URL", "https://api.example.test:8443/v1"}, {"TM_AI_BASE_URL", "https://api.example.test/v1?token=test_value"}, {"TM_AI_BASE_URL", "https://test_user:test_password@api.example.test/v1"},
		{"TM_AI_TIMEOUT_MS", "30001"}, {"TM_AI_MODEL", strings.Repeat("x", 129)},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			validL0Environment(t)
			t.Setenv(tc.key, tc.value)
			if _, err := LoadAIL0(); err == nil {
				t.Fatal("无效配置被接受")
			}
		})
	}
	t.Setenv("TM_AI_L0_ENABLED", "")
	t.Setenv("TM_AI_API_KEY", "")
	cfg, err = LoadAIL0()
	if err != nil || cfg.Enabled {
		t.Fatal("默认未关闭")
	}
}

func TestL0ModelMustMatchRegisteredSchemaModel(t *testing.T) {
	validL0Environment(t)
	cfg, err := LoadAIL0()
	if err != nil {
		t.Fatal(err)
	}
	valid := `[{"id":"test-model-id","vendor":"test-vendor","name":"test-model","filing_no":"test-filing-number","enabled_for_c":false,"caps":["json_schema"]}]`
	for _, raw := range []string{valid, strings.Replace(valid, "test-vendor", "wrong-vendor", 1), strings.Replace(valid, "test-model\"", "other-model\"", 1), strings.Replace(valid, "test-filing-number", "", 1), strings.Replace(valid, "json_schema", "json_object", 1)} {
		registry, err := provider.LoadRegistry(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		err = cfg.ValidateModel(registry)
		if (raw == valid) != (err == nil) {
			t.Fatalf("注册约束结果不符: %v", err)
		}
	}
	if err := cfg.ValidateModel(nil); err == nil {
		t.Fatal("缺注册表未拒绝")
	}
}
