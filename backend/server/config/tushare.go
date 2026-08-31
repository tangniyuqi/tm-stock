package config

type Tushare struct {
	Token      string `mapstructure:"token" json:"token" yaml:"token"`                  // Tushare 接口 token（必填，从 Tushare 官网获取）
	ApiUrl     string `mapstructure:"api-url" json:"apiUrl" yaml:"api-url"`             // Tushare 接口地址（默认 https://api.tushare.pro）
	Timeout    int    `mapstructure:"timeout" json:"timeout" yaml:"timeout"`            // 请求超时时间（单位：秒，默认 30）
	MaxRetries int    `mapstructure:"max-retries" json:"maxRetries" yaml:"max-retries"` // 失败重试次数（默认 3）
}
