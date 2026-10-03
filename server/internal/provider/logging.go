package provider

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"
	"time"
)

// redactRule 是一条脱敏规则。repl 里的 ${1} 引用第 1 个捕获组，用来保留字段名与前缀。
type redactRule struct {
	re   *regexp.Regexp
	repl []byte
}

func newRedactRule(pattern string) redactRule {
	return redactRule{re: regexp.MustCompile(pattern), repl: []byte("${1}" + redactedMark)}
}

// redactRules 的顺序有意义：先处理 JSON 字段形态，再处理文本形态，最后兜底处理 Bearer 与 sk- 前缀。
// 匹配 JSON 引号时允许前置反斜杠，这样被转义后嵌进字符串里的 JSON 片段（\"api_key\":\"…\"）也能命中。
var redactRules = []redactRule{
	// JSON 里的 Authorization 字段，值可能是字符串，也可能是 http.Header 序列化出的单元素数组。
	newRedactRule(`(?i)(\\*"(?:proxy-)?authorization\\*"\s*:\s*\[?\s*\\*")[^"\\]*`),
	// JSON 里的 api_key、access_token 等字段。注意不含单独的 token，否则会误伤 max_tokens 这类字段。
	newRedactRule(`(?i)(\\*"(?:(?:x-)?api[_-]?key|access[_-]?token|client[_-]?secret|secret[_-]?key|secret|password)\\*"\s*:\s*\[?\s*\\*")[^"\\]*`),
	// 文本形态：Authorization: Bearer xxx、Authorization=xxx。
	newRedactRule(`(?i)((?:proxy-)?authorization[ \t]*[:=][ \t]*(?:(?:bearer|basic|token)[ \t]+)?)[^\s"',;&\\]+`),
	// 文本形态：api_key=xxx、access_token: xxx 等。
	newRedactRule(`(?i)((?:(?:x-)?api[_-]?key|access[_-]?token|client[_-]?secret|secret[_-]?key|password)[ \t]*[:=][ \t]*["']?)[^\s"',;&\\]+`),
	// 任何位置的 Bearer 令牌。
	newRedactRule(`(?i)\b(bearer[ \t]+)[A-Za-z0-9._~+/=-]{8,}`),
	// 常见的 sk- 前缀密钥（供应商错误体偶尔会回显密钥片段）。
	{re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}`), repl: []byte("sk-" + redactedMark)},
}

// Redact 对 b 做形态脱敏并返回一份新切片，不修改入参。
// 覆盖 Authorization（含 Bearer、Basic）、api_key、access_token、password 等字段的 JSON 与文本形态，
// 以及孤立的 Bearer 令牌和 sk- 前缀密钥。它是兜底手段，只认形态；
// 适配层另外会按字面量把自己配置的密钥抹掉，两者叠加才是“密钥不落地”的保证。
func Redact(b []byte) []byte {
	out := b
	for _, rule := range redactRules {
		out = rule.re.ReplaceAll(out, rule.repl)
	}
	return out
}

// rawCapture 收集适配层实际发出与收到的原始报文，供日志中间件审计使用。
// 适配层把已抹掉自身密钥的副本交给它；每开始一次新的尝试就清掉上一次的响应。
type rawCapture struct {
	mu   sync.Mutex
	req  []byte
	resp []byte
}

type captureKey struct{}

func withRawCapture(ctx context.Context) (context.Context, *rawCapture) {
	c := &rawCapture{}
	return context.WithValue(ctx, captureKey{}, c), c
}

func rawCaptureFrom(ctx context.Context) *rawCapture {
	c, _ := ctx.Value(captureKey{}).(*rawCapture)
	return c
}

// begin 记录一次尝试的请求报文，并清空上一次尝试遗留的响应。c 为 nil 时什么也不做。
func (c *rawCapture) begin(req []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.req, c.resp = req, nil
	c.mu.Unlock()
}

// finish 记录响应报文。c 为 nil 时什么也不做。
func (c *rawCapture) finish(resp []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.resp = resp
	c.mu.Unlock()
}

func (c *rawCapture) snapshot() (req, resp []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.req, c.resp
}

// LogEntry 是一次调用的日志记录。写入 LogSink 之前，原始报文与 Detail 都已做过脱敏。
type LogEntry struct {
	TaskID  string
	Step    int
	Attempt int
	// Model 优先取响应里的实际模型名，失败时取请求里指定的模型（可能为空，表示用默认模型）。
	Model   string
	Latency time.Duration
	// Usage 成功时是响应用量，失败时是失败但已计费的用量（通常为零值）。
	Usage Usage

	// OK 为 true 表示调用成功；此时 ErrKind、HTTPStatus、ErrCode 都无意义。
	OK         bool
	ErrKind    ErrKind
	HTTPStatus int
	ErrCode    string
	RequestID  string

	// RawRequest 与 RawResponse 是适配层实际收发的报文（已脱敏）。日志放在 Retry 外层时
	// 只保留最后一次尝试的报文；底层不是 OpenAICompat 时退化为请求的 JSON 摘要与 Response.Raw。
	RawRequest  []byte
	RawResponse []byte
	// Detail 是供应商原文消息（已脱敏、已截断），仅供内部排障，不得进入用户可见文本。
	Detail string
}

// LogSink 接收日志记录。实现必须并发安全。
type LogSink interface {
	Log(entry LogEntry)
}

// LogSinkFunc 让一个函数满足 LogSink。
type LogSinkFunc func(entry LogEntry)

// Log 调用函数本身。
func (f LogSinkFunc) Log(entry LogEntry) { f(entry) }

// LoggingMiddleware 为每次 Generate 调用向 sink 写一条脱敏后的记录。
// sink 为 nil 时不做任何事；clock 为 nil 时用 RealClock，用它测耗时。
//
// 想要“每次尝试一条日志”，把它放在 Retry 的内层；放在最外层则一次 Generate 只有一条汇总。
func LoggingMiddleware(sink LogSink, clock Clock) Middleware {
	return func(next Provider) Provider {
		if sink == nil {
			return next
		}
		clk := clockOrReal(clock)
		return ProviderFunc(func(ctx context.Context, req Request) (Response, error) {
			ctx, capture := withRawCapture(ctx)
			start := clk.Now()
			resp, err := next.Generate(ctx, req)

			entry := LogEntry{
				TaskID:  req.TaskID,
				Step:    req.Step,
				Attempt: req.Attempt,
				Model:   req.Model,
				Latency: clk.Now().Sub(start),
			}
			rawReq, rawResp := capture.snapshot()
			if err == nil {
				entry.OK = true
				entry.Usage = resp.Usage
				entry.RequestID = resp.RequestID
				entry.Model = firstNonEmpty(resp.Model, req.Model)
				if rawResp == nil {
					rawResp = resp.Raw
				}
			} else if pe := asError(err); pe != nil {
				entry.ErrKind = pe.Kind
				entry.HTTPStatus = pe.HTTPStatus
				entry.ErrCode = pe.Code
				entry.RequestID = pe.RequestID
				entry.Usage = pe.Usage
				entry.Detail = string(Redact([]byte(pe.Detail)))
			} else {
				entry.ErrKind = Unknown
				entry.ErrCode = "unclassified"
			}
			if rawReq == nil {
				rawReq = marshalRequestForLog(req)
			}
			entry.RawRequest = Redact(rawReq)
			entry.RawResponse = Redact(rawResp)
			sink.Log(entry)
			return resp, err
		})
	}
}

// marshalRequestForLog 在底层 Provider 没有留下原始报文时，用请求本身生成一份 JSON 摘要。
// Schema 不是合法 JSON 等无法序列化的情况返回 nil。
func marshalRequestForLog(req Request) []byte {
	v := struct {
		Model       string          `json:"model,omitempty"`
		System      string          `json:"system,omitempty"`
		Prompt      string          `json:"prompt"`
		MaxTokens   int             `json:"max_tokens,omitempty"`
		Temperature *float64        `json:"temperature,omitempty"`
		Seed        *int64          `json:"seed,omitempty"`
		Format      string          `json:"format,omitempty"`
		Schema      json.RawMessage `json:"schema,omitempty"`
		Strict      bool            `json:"strict,omitempty"`
	}{
		Model:       req.Model,
		System:      req.System,
		Prompt:      req.Prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Seed:        req.Seed,
		Schema:      req.Schema,
		Strict:      req.Strict,
	}
	if req.Format != FormatText {
		v.Format = req.Format.String()
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
