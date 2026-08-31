package config

// AIModelConfig 单个大模型配置
type AIModelConfig struct {
	Name        string `mapstructure:"name" json:"name" yaml:"name"`                        // 模型ID（必填，接口请求中的 model 字段）
	DisplayName string `mapstructure:"display-name" json:"displayName" yaml:"display-name"` // 显示名称（前端下拉展示，留空使用 name）
	ApiKey      string `mapstructure:"api-key" json:"apiKey" yaml:"api-key"`                // 模型级密钥（可选，留空继承厂商级 api-key）
	BaseURL     string `mapstructure:"base-url" json:"baseUrl" yaml:"base-url"`             // 模型级接口地址（可选，留空继承厂商级 base-url）
	WebSearch   *bool  `mapstructure:"web-search" json:"webSearch" yaml:"web-search"`       // 联网搜索（可选，模型级覆盖厂商级；不配置则继承厂商级 web-search）
	ApiFormat   string `mapstructure:"api-format" json:"apiFormat" yaml:"api-format"`       // 接口格式（可选，模型级覆盖厂商级，不配置则继承厂商级；responses=Responses API 原生 web_search 联网搜索，chat-completions=OpenAI 兼容接口）
	// ReasoningEffort 思考强度（可选，模型级覆盖厂商级；不配置则不传思考控制参数）
	// 取值：none=关闭思考，low/high/max=思考强度（不同厂商支持的取值不同，DeepSeek V4 系列支持 none/low/high/max）
	// 深度思考模型（如 DeepSeek V4）思考内容计入 max-tokens 预算，复杂长输出任务思考过长会耗尽预算导致最终回答为空，建议配置 none 或 low
	ReasoningEffort string `mapstructure:"reasoning-effort" json:"reasoningEffort" yaml:"reasoning-effort"`
	// MaxOutputTokens 单次回答最大输出 token 数（可选，模型级覆盖厂商级，不配置则继承厂商级，默认 8192）
	// 深度思考模型思考内容计入该上限，复杂任务建议调大（如 DeepSeek V4 系列官方最大输出 384K，但输出按量计费，够用即可）
	MaxOutputTokens int `mapstructure:"max-output-tokens" json:"maxOutputTokens" yaml:"max-output-tokens"`
}

// AIProviderConfig AI 大模型单个厂商配置
type AIProviderConfig struct {
	Name      string `mapstructure:"name" json:"name" yaml:"name"`                  // 厂商显示名称（可选，留空使用 key）
	ApiKey    string `mapstructure:"api-key" json:"apiKey" yaml:"api-key"`          // 厂商级 API 密钥（模型未单独配置时使用）
	BaseURL   string `mapstructure:"base-url" json:"baseUrl" yaml:"base-url"`       // 厂商级 OpenAI 兼容接口地址（模型未单独配置时使用）
	Model     string `mapstructure:"model" json:"model" yaml:"model"`               // 默认模型（兼容旧配置；配置了 models 时优先使用 models 列表第一个）
	WebSearch *bool  `mapstructure:"web-search" json:"webSearch" yaml:"web-search"` // 联网搜索（可选，厂商级覆盖全局 ai.web-search；不配置则继承全局开关）
	ApiFormat string `mapstructure:"api-format" json:"apiFormat" yaml:"api-format"` // 接口格式（厂商级默认，默认 responses；模型未单独配置时生效）
	// ReasoningEffort 思考强度（可选，厂商级默认，模型未单独配置时生效；取值同模型级 reasoning-effort）
	ReasoningEffort string `mapstructure:"reasoning-effort" json:"reasoningEffort" yaml:"reasoning-effort"`
	// MaxOutputTokens 单次回答最大输出 token 数（可选，厂商级默认，模型未单独配置时生效，默认 8192）
	MaxOutputTokens int             `mapstructure:"max-output-tokens" json:"maxOutputTokens" yaml:"max-output-tokens"`
	Models          []AIModelConfig `mapstructure:"models" json:"models" yaml:"models"` // 模型列表（一个厂商可配置多个大模型）
}

// AIConfig AI 大模型配置
type AIConfig struct {
	// ResponsesAPI Responses API 全局开关（默认 true；nil 视为 true）
	// true=默认走 Responses API（原生 web_search 联网搜索，由模型服务端执行）
	// false=默认走 OpenAI 兼容 chat/completions 接口（联网搜索时由服务端调用所选搜索引擎预检索）
	// 各厂商/模型的 api-format 可覆盖此开关
	ResponsesAPI *bool `mapstructure:"responses-api" json:"responsesApi" yaml:"responses-api"`
	// WebSearch 联网搜索全局开关（默认 true；nil 视为 true）
	// true=开启联网搜索，AI 分析前自动检索相关最新新闻供大模型参考
	// false=关闭联网搜索
	// 各厂商/模型的 web-search 可覆盖此开关
	WebSearch *bool `mapstructure:"web-search" json:"webSearch" yaml:"web-search"`
	// Timeout 大模型请求超时时间（单位秒，默认 600；<=0 时使用默认 10 分钟）
	// 深度推理类模型生成长文本分析耗时较长（如硅基流动的 DeepSeek-V4 系列），超时过短会报 context deadline exceeded
	Timeout int `mapstructure:"timeout" json:"timeout" yaml:"timeout"`
	// Providers 各厂商配置: deepseek/doubao/qwen/kimi/zhipu/minimax/claude/gpt/gemini
	Providers map[string]AIProviderConfig `mapstructure:"providers" json:"providers" yaml:"providers"`
}
