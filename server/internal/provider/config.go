package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadConfig 从环境变量加载 Provider 配置。
// 密钥只读取 TM_AI_API_KEY，错误和摘要都不会包含其内容。
func LoadConfig() (*Config, error) {
	c := &Config{
		Enabled:   envBool("TM_AI_ENABLED", false),
		Name:      envString("TM_AI_PROVIDER", "openai-compatible"),
		BaseURL:   strings.TrimRight(os.Getenv("TM_AI_BASE_URL"), "/"),
		Model:     os.Getenv("TM_AI_MODEL"),
		APIKey:    os.Getenv("TM_AI_API_KEY"),
		TimeoutMS: envInt("TM_AI_TIMEOUT_MS", 30000),
	}
	if !c.Enabled {
		return c, nil
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("启用模型 Provider 时缺少环境变量 TM_AI_BASE_URL")
	}
	if c.Model == "" {
		return nil, fmt.Errorf("启用模型 Provider 时缺少环境变量 TM_AI_MODEL")
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("启用模型 Provider 时缺少环境变量 TM_AI_API_KEY")
	}
	if c.TimeoutMS <= 0 {
		return nil, fmt.Errorf("TM_AI_TIMEOUT_MS 必须为正数")
	}
	return c, nil
}

// Redacted 返回可写入日志的配置摘要。
func (c Config) Redacted() string {
	return fmt.Sprintf("enabled=%v provider=%s model=%s baseURL=%s timeout=%dms apiKey=<已隐藏>",
		c.Enabled, c.Name, c.Model, c.BaseURL, c.TimeoutMS)
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
