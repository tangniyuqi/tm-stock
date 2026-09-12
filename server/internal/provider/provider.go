// Package provider 定义内部运营任务可使用的模型 Provider 抽象。
//
// 该包不参与 C 端 HTTP 路由，调用方必须在内部任务边界内使用。
package provider

import "context"

// Config 描述一个兼容 Chat Completions 协议的模型服务配置。
// APIKey 只在进程内保存，禁止写入日志、响应或仓库文件。
type Config struct {
	Enabled   bool
	Name      string
	BaseURL   string
	Model     string
	APIKey    string
	TimeoutMS int
}

// Request 是内部模型任务的最小请求。
type Request struct {
	System    string
	Prompt    string
	MaxTokens int
}

// Response 是 Provider 返回的客观文本结果。
type Response struct {
	Text  string
	Model string
}

// Provider 是模型服务的最小适配接口。
type Provider interface {
	Generate(ctx context.Context, request Request) (Response, error)
}
