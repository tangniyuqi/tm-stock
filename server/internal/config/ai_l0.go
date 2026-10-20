package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// AIL0Config 是内部抽取专用批准配置；不代表允许向公众提供 AI。
type AIL0Config struct {
	Enabled        bool
	Provider       provider.Config
	ApprovalRef    string
	RegistryPath   string
	ModelID        string
	MaxCalls       int
	MaxPromptBytes int
}

func LoadAIL0() (*AIL0Config, error) {
	cfg := &AIL0Config{}
	raw := strings.TrimSpace(os.Getenv("TM_AI_L0_ENABLED"))
	if raw == "" || raw == "false" {
		return cfg, nil
	}
	if raw != "true" {
		return nil, fmt.Errorf("TM_AI_L0_ENABLED 只接受 true 或 false")
	}
	cfg.Enabled = true
	p, err := provider.LoadConfig()
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, fmt.Errorf("启用内部抽取还须明确启用 TM_AI_ENABLED")
	}
	u, err := url.Parse(p.BaseURL)
	domain := strings.ToLower(strings.TrimSpace(os.Getenv("TM_AI_L0_PROVIDER_DOMAIN")))
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" || net.ParseIP(u.Hostname()) != nil || strings.ToLower(u.Hostname()) != domain || (u.Port() != "" && u.Port() != "443") {
		return nil, fmt.Errorf("内部模型地址须为已批准精确域名的 HTTPS 地址")
	}
	if len(p.Name) > 64 || strings.TrimSpace(p.Name) == "" || len(p.Model) > 128 || strings.TrimSpace(p.Model) != p.Model || p.TimeoutMS < 1 || p.TimeoutMS > 30000 {
		return nil, fmt.Errorf("内部 Provider 标识或单次时限无效")
	}
	cfg.Provider = *p
	cfg.ApprovalRef = strings.TrimSpace(os.Getenv("TM_AI_L0_APPROVAL_REF"))
	cfg.RegistryPath = strings.TrimSpace(os.Getenv("TM_AI_MODEL_REGISTRY"))
	cfg.ModelID = strings.TrimSpace(os.Getenv("TM_AI_L0_MODEL_ID"))
	if cfg.ApprovalRef == "" || len(cfg.ApprovalRef) > 512 || cfg.RegistryPath == "" || cfg.ModelID == "" {
		return nil, fmt.Errorf("内部抽取缺少书面批准引用、模型注册表路径或模型编号")
	}
	cfg.MaxCalls, err = strconv.Atoi(os.Getenv("TM_AI_L0_MAX_CALLS"))
	if err != nil || cfg.MaxCalls < 1 || cfg.MaxCalls > 1000 {
		return nil, fmt.Errorf("须明确设置 TM_AI_L0_MAX_CALLS 为 1 至 1000 次进程调用上限")
	}
	cfg.MaxPromptBytes, err = strconv.Atoi(os.Getenv("TM_AI_L0_MAX_PROMPT_BYTES"))
	if err != nil || cfg.MaxPromptBytes < 4096 || cfg.MaxPromptBytes > 256<<10 {
		return nil, fmt.Errorf("须明确设置 TM_AI_L0_MAX_PROMPT_BYTES 为 4096 至 262144 字节")
	}
	return cfg, nil
}

// ValidateModel 要求批准模型已登记备案号与封闭 JSON Schema 能力；不据此推定法务批准的真实性。
func (c AIL0Config) ValidateModel(registry *provider.Registry) error {
	m, ok := registry.Lookup(c.ModelID)
	filing := strings.TrimFunc(m.FilingNo, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) })
	if !ok || m.Vendor != c.Provider.Name || m.Name != c.Provider.Model || filing == "" || m.Caps()&provider.CapJSONSchema == 0 {
		return fmt.Errorf("内部模型与批准注册项不一致或缺少备案/结构能力")
	}
	return nil
}
