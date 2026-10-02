package provider

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

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

// Format 是期望的模型输出格式。零值 FormatText 表示普通文本。
type Format uint8

const (
	// FormatText 普通文本，请求体不带 response_format。
	FormatText Format = iota
	// FormatJSONObject 要求返回 JSON 对象，对应 response_format.type = json_object。
	FormatJSONObject
	// FormatJSONSchema 要求按 JSON Schema 返回，需配套 Request.Schema，
	// 对应 response_format.type = json_schema。
	FormatJSONSchema
)

// String 返回格式名，便于日志与测试输出。
func (f Format) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatJSONObject:
		return "json_object"
	case FormatJSONSchema:
		return "json_schema"
	default:
		return "format(" + strconv.Itoa(int(f)) + ")"
	}
}

// Request 是内部模型任务的请求。System、Prompt、MaxTokens 之外的字段全部可选，零值即“不指定”。
type Request struct {
	System    string
	Prompt    string
	MaxTokens int

	// TaskID、Step、Attempt 是追溯键，只用于日志与审计，不会发给供应商。
	// Retry 层每次尝试前会把 Attempt 加一（在请求副本上，不改调用方的值）。
	TaskID  string
	Step    int
	Attempt int

	// Model 为空时使用 Config.Model。经 Router 回退到不同厂商时，各厂商的模型名通常不同，
	// 此时应留空，或在 Route.Model 上为每条路由单独指定。
	Model string
	// Temperature 与 Seed 为 nil 表示不发送；指向 0 的指针会被如实发送。
	Temperature *float64
	Seed        *int64

	// Format 为 FormatJSONSchema 时必须提供合法的 Schema；Strict 对应 json_schema.strict。
	Format Format
	Schema json.RawMessage
	Strict bool

	// AttemptTimeout 是单次尝试的时限；为 0 时取 Config.TimeoutMS。
	// 总时限由 ctx（即任务 deadline）控制，不在这里。
	AttemptTimeout time.Duration
}

// Usage 是一次调用的 token 用量。
type Usage struct {
	Prompt     int
	Completion int
	Total      int
	// Cached 是命中缓存的提示词 token 数，Reasoning 是推理 token 数；供应商没给就是 0。
	Cached    int
	Reasoning int
}

// Response 是 Provider 返回的客观文本结果。
type Response struct {
	Text  string
	Model string

	FinishReason string
	Usage        Usage
	// UsageEstimated 为 true 表示供应商没有返回 usage，Usage 是按字符数估出来的粗值，
	// 只能用于预算封顶，不得用于计费结算。
	UsageEstimated bool
	// RequestID 是供应商给出的请求编号，便于对账与工单。
	RequestID string
	// Raw 是原始响应体，留作审计；其中不会出现配置的 API 密钥。
	Raw []byte
}

// Provider 是模型服务的最小适配接口。
type Provider interface {
	Generate(ctx context.Context, request Request) (Response, error)
}
