package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultAttemptTimeout = 30 * time.Second
	defaultMaxResponse    = 4 << 20 // 响应体读取上限：4 MiB
	// maxAttemptTimeout 是单次尝试时限的上限，同时也是 Transport.ResponseHeaderTimeout 的兜底值。
	maxAttemptTimeout = 30 * time.Minute
	maxDetailBytes    = 512
	redactedMark      = "<已隐藏>"
)

// OpenAICompat 是 OpenAI 兼容端点（/chat/completions）的适配层，只依赖标准库。
//
// 它从不重试：单次 Generate 至多向上游发一次请求。为此做了四件事：
// http.Client.Timeout 为 0（时限全部交给 context）；请求体不设 GetBody（net/http 只会在
// 请求可重放时才悄悄重发）；禁止跟随重定向（POST 被 301/302 改成 GET 同样是隐式重试）；
// 重试只允许发生在 Retry 层，那里每次尝试都有日志。
type OpenAICompat struct {
	name     string
	endpoint string
	model    string
	bearer   string
	timeout  time.Duration
	maxBody  int64
	clock    Clock
	client   *http.Client
}

var _ Provider = (*OpenAICompat)(nil)

type options struct {
	transport http.RoundTripper
	clock     Clock
	maxBody   int64
}

// Option 调整适配层的构造参数。
type Option func(*options)

// WithTransport 指定底层 RoundTripper，用于自定义代理，或在测试里观察实际发出的请求。
// 不指定时使用克隆自 http.DefaultTransport 的 Transport，不会改动全局默认值。
// 传入者需自行保证它不会重发请求。
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// WithClock 注入时钟，目前只用于把 HTTP-date 形式的 Retry-After 换算成时长。
func WithClock(c Clock) Option {
	return func(o *options) { o.clock = c }
}

// WithMaxResponseBytes 调整响应体读取上限，默认 4 MiB；超过上限按 Malformed 处理。
func WithMaxResponseBytes(n int64) Option {
	return func(o *options) {
		if n > 0 {
			o.maxBody = n
		}
	}
}

// New 构造适配层。是否启用（Config.Enabled）由调用方在装配处判断，这里不检查。
// BaseURL 必须是带主机名的 http(s) 地址，不得内嵌账号密码或带查询串；
// TimeoutMS 不为正时取 30 秒；APIKey 为空时不发送 Authorization 头（供内网网关使用）。
// 报错信息不会回显 BaseURL 或密钥。
func New(cfg Config, opts ...Option) (*OpenAICompat, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, errors.New("模型适配层缺少 BaseURL")
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("模型适配层的 BaseURL 必须是带主机名的 http 或 https 地址")
	}
	if u.User != nil {
		return nil, errors.New("模型适配层的 BaseURL 不得内嵌账号密码")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("模型适配层的 BaseURL 不得带查询串或片段")
	}

	o := options{maxBody: defaultMaxResponse}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	timeout := defaultAttemptTimeout
	if cfg.TimeoutMS > 0 {
		timeout = maxAttemptTimeout
		if cfg.TimeoutMS < int(maxAttemptTimeout/time.Millisecond) {
			timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
		}
	}
	rt := o.transport
	if rt == nil {
		rt = newTransport()
	}
	return &OpenAICompat{
		name:     cfg.Name,
		endpoint: base + "/chat/completions",
		model:    cfg.Model,
		bearer:   cfg.APIKey,
		timeout:  timeout,
		maxBody:  o.maxBody,
		clock:    clockOrReal(o.clock),
		client: &http.Client{
			Transport: rt,
			Timeout:   0, // 时限只由 context 控制，见 Generate
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// newTransport 克隆默认 Transport 并设置 ResponseHeaderTimeout。
//
// 单次尝试的真正时限由 Generate 里的 context.WithTimeout 施加，它覆盖拨号、写请求、等响应头、
// 读响应体全过程，且能被 Request.AttemptTimeout 逐次覆盖。Transport 上的值不能随请求变化，
// 而非流式补全的响应头要等生成结束才会返回，设得比某次请求的时限短就会误杀，所以这里取
// 时限上限，只当兜底。
func newTransport() *http.Transport {
	var t *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t = dt.Clone()
	} else {
		t = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	t.ResponseHeaderTimeout = maxAttemptTimeout
	return t
}

// Name 返回配置里的厂商名。
func (c *OpenAICompat) Name() string { return c.name }

// chatRequest 是发给 /chat/completions 的请求体，字段名遵循 OpenAI 兼容协议。
type chatRequest struct {
	Model          string          `json:"model,omitempty"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	Seed           *int64          `json:"seed,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *jsonSchemaSpec `json:"json_schema,omitempty"`
}

type jsonSchemaSpec struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

// chatResponse 只取我们用得到的字段；usage 单独宽松解析，坏了不能拖垮整个响应。
type chatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage json.RawMessage `json:"usage"`
}

// buildBody 构造请求体。本地校验失败返回 BadRequest，此时不会向上游发任何请求。
func buildBody(req Request, model string) ([]byte, *Error) {
	body := chatRequest{
		Model:       model,
		MaxTokens:   max(req.MaxTokens, 0),
		Temperature: req.Temperature,
		Seed:        req.Seed,
	}
	if req.System != "" {
		body.Messages = append(body.Messages, chatMessage{Role: "system", Content: req.System})
	}
	body.Messages = append(body.Messages, chatMessage{Role: "user", Content: req.Prompt})

	switch req.Format {
	case FormatText:
	case FormatJSONObject:
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	case FormatJSONSchema:
		if len(bytes.TrimSpace(req.Schema)) == 0 || !json.Valid(req.Schema) {
			return nil, &Error{Kind: BadRequest, Code: "invalid_schema"}
		}
		body.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: &jsonSchemaSpec{Name: "result", Schema: req.Schema, Strict: req.Strict},
		}
	default:
		return nil, &Error{Kind: BadRequest, Code: "invalid_format"}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // 提示词里的 < > & 原样发出，审计看到的就是发出的
	if err := enc.Encode(body); err != nil {
		return nil, &Error{Kind: BadRequest, Code: "invalid_request", Err: err}
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Generate 向上游发一次 /chat/completions 请求并解析结果。
//
// 调用方 ctx 在发请求前就已结束，或请求体本地校验失败时，不会产生任何上游请求。
func (c *OpenAICompat) Generate(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	body, berr := buildBody(req, model)
	if berr != nil {
		return Response{}, berr
	}
	if cerr := ctx.Err(); cerr != nil {
		return Response{}, ctxToError(cerr)
	}

	timeout := c.timeout
	if req.AttemptTimeout > 0 {
		timeout = req.AttemptTimeout
	}
	timeout = min(timeout, maxAttemptTimeout)
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 请求体用 NopCloser 包一层：NewRequest 只对 bytes.Reader 等已知类型自动设置 GetBody，
	// 包过之后 GetBody 为 nil，net/http 在连接失效时不会悄悄重发这个 POST。
	httpReq, nerr := http.NewRequestWithContext(attemptCtx, http.MethodPost, c.endpoint,
		io.NopCloser(bytes.NewReader(body)))
	if nerr != nil {
		return Response{}, &Error{Kind: BadRequest, Code: "invalid_request", Err: nerr}
	}
	httpReq.ContentLength = int64(len(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.bearer)
	}

	capture := rawCaptureFrom(ctx)
	capture.begin(c.scrub(body))

	httpResp, derr := c.client.Do(httpReq)
	if derr != nil {
		return Response{}, c.transportError(ctx, attemptCtx, derr, false)
	}
	defer httpResp.Body.Close()

	status := httpResp.StatusCode
	raw, tooLarge, rerr := readAtMost(httpResp.Body, c.maxBody)
	scrubbed := c.scrub(raw)
	capture.finish(scrubbed)

	if status < 200 || status > 299 {
		// 非 2xx 以状态码与错误体为准；读体失败也不改变分类。
		return Response{}, c.statusError(status, httpResp.Header, raw)
	}
	if rerr != nil {
		e := c.transportError(ctx, attemptCtx, rerr, true)
		e.HTTPStatus = status
		e.RequestID = requestIDFromHeader(httpResp.Header)
		return Response{}, e
	}
	if tooLarge {
		return Response{}, &Error{
			Kind: Malformed, HTTPStatus: status, Code: "response_too_large",
			RequestID: requestIDFromHeader(httpResp.Header),
		}
	}
	return c.parseSuccess(req, model, status, httpResp.Header, raw, scrubbed)
}

// readAtMost 最多读 limit 字节；超出时返回前 limit 字节并置 tooLarge。
func readAtMost(r io.Reader, limit int64) (data []byte, tooLarge bool, err error) {
	data, err = io.ReadAll(io.LimitReader(r, limit+1))
	if int64(len(data)) > limit {
		return data[:limit], true, err
	}
	return data, false, err
}

// transportError 归类传输层失败。判断顺序：
//  1. 调用方 ctx 已结束：取消 → Canceled，到期 → Timeout，绝不把取消算成超时；
//  2. 单次尝试的时限到了，或底层报告超时（含 ResponseHeaderTimeout）→ Timeout；
//  3. 已收到 2xx 响应头后读体失败（连接中途断开、被截断）→ Malformed。理由：此时上游多半已处理并
//     计费，Malformed 只允许有限次修复重试（默认 1 次），比 5xx 的整套退避更克制；
//  4. 其余（连接被拒、被重置、DNS 失败等，尚无任何响应）→ Overloaded，按上游不可用处理。
func (c *OpenAICompat) transportError(parent, attempt context.Context, err error, inBody bool) *Error {
	if perr := parent.Err(); perr != nil {
		e := ctxToError(perr)
		// 两者都挂上：无论传输层报的是什么错，errors.Is(e, context.Canceled) 之类的判断都成立。
		e.Err = errors.Join(perr, err)
		return e
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(attempt.Err(), context.DeadlineExceeded) || isNetTimeout(err) {
		return &Error{Kind: Timeout, Code: "attempt_timeout", Err: err}
	}
	if inBody {
		return &Error{Kind: Malformed, Code: "truncated_body", Err: err}
	}
	return &Error{Kind: Overloaded, Code: "transport_error", Err: err}
}

func isNetTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// statusError 把非 2xx 响应转成 *Error。
func (c *OpenAICompat) statusError(status int, h http.Header, raw []byte) *Error {
	info := parseErrorBody(raw)
	kind, code := classifyHTTP(status, info)
	return &Error{
		Kind:       kind,
		HTTPStatus: status,
		Code:       code,
		RetryAfter: parseRetryAfter(h.Get("Retry-After"), c.clock.Now()),
		Detail:     c.detail(info.message),
		RequestID:  requestIDFromHeader(h),
	}
}

// parseSuccess 解析 2xx 响应。能用的结果才返回 Response，其余都是 Malformed 或 ContentFilter。
func (c *OpenAICompat) parseSuccess(req Request, model string, status int, h http.Header, raw, scrubbed []byte) (Response, error) {
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Response{}, &Error{
			Kind: Malformed, HTTPStatus: status, Code: "invalid_json", Err: err,
			RequestID: requestIDFromHeader(h),
		}
	}
	requestID := sanitizeToken(cr.ID, maxIDLen)
	if requestID == "" {
		requestID = requestIDFromHeader(h)
	}
	usage, hasUsage := parseUsage(cr.Usage)
	fail := func(kind ErrKind, code string) *Error {
		e := &Error{Kind: kind, HTTPStatus: status, Code: code, RequestID: requestID}
		if hasUsage {
			e.Usage = usage // 失败但可能已计费的用量，交给预算与日志
		}
		return e
	}

	if len(cr.Choices) == 0 {
		// 有些网关用 200 承载错误体，此时按错误码里的额度、安全、长度线索分类，认不出的才算畸形。
		info := parseErrorBody(raw)
		if kind, ok := classifyBodyError(info); ok {
			e := fail(kind, sanitizeToken(firstNonEmpty(info.code, info.typ), maxCodeLen))
			e.Detail = c.detail(info.message)
			return Response{}, e
		}
		e := fail(Malformed, "empty_choices")
		e.Detail = c.detail(info.message)
		return Response{}, e
	}

	choice := cr.Choices[0]
	if choice.FinishReason == "content_filter" {
		return Response{}, fail(ContentFilter, "content_filter")
	}
	// 文本里若出现配置的密钥（供应商回显了请求头之类的异常），同样抹掉：这串字符不可能合法出现。
	text := c.scrubString(contentString(choice.Message.Content))
	if strings.TrimSpace(text) == "" {
		return Response{}, fail(Malformed, "empty_content")
	}

	resp := Response{
		Text:         text,
		Model:        firstNonEmpty(cr.Model, model),
		FinishReason: choice.FinishReason,
		RequestID:    requestID,
		Raw:          scrubbed,
	}
	if hasUsage {
		resp.Usage = usage
	} else {
		resp.Usage = estimateUsage(req, text)
		resp.UsageEstimated = true
	}
	return resp, nil
}

// contentString 取 message.content 的字符串值；null、缺失或非字符串都视为空。
func contentString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

// parseUsage 宽松解析 usage：计数字段按浮点读（个别厂商会给 15.0），读不出来或全为 0 都算“缺失”。
// 命中缓存的 token 数优先取 prompt_tokens_details.cached_tokens，其次取部分厂商的
// prompt_cache_hit_tokens。
func parseUsage(raw json.RawMessage) (Usage, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return Usage{}, false
	}
	var u struct {
		PromptTokens         float64 `json:"prompt_tokens"`
		CompletionTokens     float64 `json:"completion_tokens"`
		TotalTokens          float64 `json:"total_tokens"`
		PromptCacheHitTokens float64 `json:"prompt_cache_hit_tokens"`
		PromptDetails        struct {
			CachedTokens float64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionDetails struct {
			ReasoningTokens float64 `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return Usage{}, false
	}
	out := Usage{
		Prompt:     toCount(u.PromptTokens),
		Completion: toCount(u.CompletionTokens),
		Total:      toCount(u.TotalTokens),
		Cached:     toCount(u.PromptDetails.CachedTokens),
		Reasoning:  toCount(u.CompletionDetails.ReasoningTokens),
	}
	if out.Cached == 0 {
		out.Cached = toCount(u.PromptCacheHitTokens)
	}
	if out.Total == 0 {
		out.Total = out.Prompt + out.Completion
	}
	if out.Prompt == 0 && out.Completion == 0 && out.Total == 0 {
		return Usage{}, false
	}
	return out, true
}

func toCount(f float64) int {
	switch {
	case f <= 0 || math.IsNaN(f):
		return 0
	case f > 1e12:
		return 1e12
	default:
		return int(math.Round(f))
	}
}

// estimateUsage 在供应商没有给 usage 时做粗估：提示词与系统词的字符（rune）数记作输入用量，
// 回复文本的字符数记作输出用量。
//
// 这只是粗估，不是 token 数：中文约 1 字 0.6 到 1 个 token，英文约 4 个字符 1 个 token，
// 所以通常高估或持平，用来给预算封顶偏安全；生僻字与 emoji 可能被低估。
// 它绝不能用于计费结算（设计 I6：积分锁价，不按 token 补扣），调用方必须看 UsageEstimated。
func estimateUsage(req Request, text string) Usage {
	prompt := utf8.RuneCountInString(req.System) + utf8.RuneCountInString(req.Prompt)
	completion := utf8.RuneCountInString(text)
	return Usage{Prompt: prompt, Completion: completion, Total: prompt + completion}
}

// scrub 把配置的 API 密钥从 b 里抹掉。没有命中时原样返回 b（不复制），调用方不得修改返回值。
func (c *OpenAICompat) scrub(b []byte) []byte {
	if c.bearer == "" || len(b) == 0 {
		return b
	}
	key := []byte(c.bearer)
	if !bytes.Contains(b, key) {
		return b
	}
	return bytes.ReplaceAll(b, key, []byte(redactedMark))
}

// scrubString 是 scrub 的字符串版本。
func (c *OpenAICompat) scrubString(s string) string {
	if c.bearer == "" || !strings.Contains(s, c.bearer) {
		return s
	}
	return strings.ReplaceAll(s, c.bearer, redactedMark)
}

// detail 生成供脱敏日志使用的供应商原文：抹掉密钥、按形态脱敏、截断。
func (c *OpenAICompat) detail(msg string) string {
	if msg == "" {
		return ""
	}
	b := Redact(c.scrub([]byte(msg)))
	return truncateUTF8(string(b), maxDetailBytes)
}
