package config

type Meilisearch struct {
	Enable bool   `mapstructure:"enable" json:"enable" yaml:"enable"`                      // 是否启用 Meilisearch
	Host   string `mapstructure:"host" json:"host" yaml:"host"`                            // Meilisearch 服务地址
	Port   int    `mapstructure:"port" json:"port" yaml:"port"`                            // Meilisearch 端口号
	APIKey string `mapstructure:"api-key" json:"apiKey" yaml:"api-key"`                    // Meilisearch API 密钥（可选）
}
