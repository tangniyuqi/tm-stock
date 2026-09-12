package provider

import "testing"

func TestLoadConfigDisabledByDefault(t *testing.T) {
	for _, key := range []string{"TM_AI_ENABLED", "TM_AI_PROVIDER", "TM_AI_BASE_URL", "TM_AI_MODEL", "TM_AI_API_KEY", "TM_AI_TIMEOUT_MS"} {
		t.Setenv(key, "")
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("默认配置不应失败: %v", err)
	}
	if c.Enabled || c.Name != "openai-compatible" || c.TimeoutMS != 30000 {
		t.Fatalf("默认配置异常: %+v", c)
	}
}

func TestLoadConfigEnabledValidation(t *testing.T) {
	t.Setenv("TM_AI_ENABLED", "true")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("启用后缺少必填配置时应失败")
	}
	t.Setenv("TM_AI_BASE_URL", "https://example.invalid/v1/")
	t.Setenv("TM_AI_MODEL", "model-a")
	t.Setenv("TM_AI_API_KEY", "test-key")
	t.Setenv("TM_AI_TIMEOUT_MS", "1200")
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("完整配置不应失败: %v", err)
	}
	if c.BaseURL != "https://example.invalid/v1" || c.TimeoutMS != 1200 {
		t.Fatalf("配置未按预期解析: %+v", c)
	}
	if got := c.Redacted(); got == "" || containsSecret(got, "test-key") {
		t.Fatalf("摘要泄露密钥或为空: %s", got)
	}
}

func TestLoadConfigRejectsNonPositiveTimeout(t *testing.T) {
	t.Setenv("TM_AI_ENABLED", "1")
	t.Setenv("TM_AI_BASE_URL", "https://example.invalid")
	t.Setenv("TM_AI_MODEL", "model-a")
	t.Setenv("TM_AI_API_KEY", "test-key")
	t.Setenv("TM_AI_TIMEOUT_MS", "0")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("非正超时应失败")
	}
}

func containsSecret(value, secret string) bool {
	return len(secret) > 0 && len(value) >= len(secret) && stringContains(value, secret)
}

func stringContains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
