package config

// WebSearchModelConfig 单个搜索引擎模型配置
type WebSearchModelConfig struct {
	Name        string `mapstructure:"name" json:"name" yaml:"name"`                        // 模型ID
	DisplayName string `mapstructure:"display-name" json:"displayName" yaml:"display-name"` // 显示名称
}

// WebSearchProviderConfig 第三方联网搜索引擎配置（chat/completions 接口模式下 AI 联网检索使用）
type WebSearchProviderConfig struct {
	Name      string                 `mapstructure:"name" json:"name" yaml:"name"`                  // 搜索引擎显示名称
	ApiKey    string                 `mapstructure:"api-key" json:"apiKey" yaml:"api-key"`          // 引擎 API 密钥
	BaseURL   string                 `mapstructure:"base-url" json:"baseUrl" yaml:"base-url"`       // 引擎接口地址
	ApiFormat string                 `mapstructure:"api-format" json:"apiFormat" yaml:"api-format"` // 接口格式（custom=自定义格式）
	WebSearch bool                   `mapstructure:"web-search" json:"webSearch" yaml:"web-search"` // 是否支持联网搜索
	Models    []WebSearchModelConfig `mapstructure:"models" json:"models" yaml:"models"`            // 模型列表
}

// WebSearchConfig 联网搜索引擎配置
// 直接以引擎名（key）作为子键，如 web-search.baidu / web-search.bocha / web-search.azure / web-search.tavily / web-search.serper；
// key 与前端 web/src/data/webSearchOptions.js 的 value 保持一致；baidu 为默认引擎
type WebSearchConfig map[string]WebSearchProviderConfig
