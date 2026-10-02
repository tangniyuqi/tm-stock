package provider

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrKind 是调用失败的分类。重试、熔断、回退都只看分类，不看错误文本。
//
// 零值 Unknown 表示“未分类”：本包的适配层从不产生它，中间件对它一律按保守处理——
// 不重试、不回退、不计入熔断。
type ErrKind uint8

const (
	// Unknown 未分类（零值）。
	Unknown ErrKind = iota
	// Auth 鉴权失败（401、403）。不重试；熔断器立即熔断。
	Auth
	// Billing 欠费或额度耗尽（402，或错误码表示额度类问题，即便 HTTP 状态是 429）。
	// 不重试；熔断器立即熔断。
	Billing
	// RateLimit 被限流（429）。退避重试并遵守 Retry-After。
	RateLimit
	// Overloaded 上游过载或不可用（5xx、529、连接建立失败）。退避重试。
	Overloaded
	// Timeout 超时（408、单次尝试超时、调用方 deadline 到期）。退避重试。
	Timeout
	// BadRequest 请求本身有问题（其余 4xx、本地校验失败、未预期的状态码）。不重试。
	BadRequest
	// ContextLen 输入超出模型上下文长度。不重试。
	ContextLen
	// ContentFilter 内容安全拦截（含 finish_reason=content_filter）。不重试。
	ContentFilter
	// Malformed 2xx 但响应不可用：JSON 畸形、choices 为空、content 为空、响应体被截断。
	// 至多重试 RetryPolicy.MalformedRetries 次。
	Malformed
	// Canceled 调用方取消了 ctx。不重试。
	Canceled
	// BudgetExceeded 预算（调用数或 token）已用尽，未向上游发请求。不重试。
	BudgetExceeded
	// CircuitOpen 熔断器处于打开（或半开且探测名额已满），未向上游发请求。不重试。
	CircuitOpen
)

var kindNames = [...]string{
	Unknown:        "Unknown",
	Auth:           "Auth",
	Billing:        "Billing",
	RateLimit:      "RateLimit",
	Overloaded:     "Overloaded",
	Timeout:        "Timeout",
	BadRequest:     "BadRequest",
	ContextLen:     "ContextLen",
	ContentFilter:  "ContentFilter",
	Malformed:      "Malformed",
	Canceled:       "Canceled",
	BudgetExceeded: "BudgetExceeded",
	CircuitOpen:    "CircuitOpen",
}

// String 返回与常量同名的稳定标识，可直接用作日志字段或指标标签。
func (k ErrKind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "ErrKind(" + strconv.Itoa(int(k)) + ")"
}

// Error 是本包所有失败的统一类型。
//
// Error() 只包含种类、HTTP 状态与错误码，绝不包含供应商返回的原文消息：
// 它可能被透传到上层日志或接口，而供应商原文可能回显提示词、账号或密钥片段。
type Error struct {
	Kind       ErrKind
	HTTPStatus int
	// Code 是经过净化的错误码：优先取供应商的 error.code（其次 error.type），
	// 否则是本包自己的标识（如 empty_content、attempt_timeout）。
	Code string
	// RetryAfter 是供应商要求的最短等待时长，没有则为 0。
	RetryAfter time.Duration
	// Err 是底层原因（如 context.Canceled、传输层错误），仅供 errors.Is/As 使用。
	Err error

	// Detail 是供应商返回的原文消息，已抹掉配置的密钥并截断，
	// 只能写入脱敏后的内部日志，禁止进入任何用户可见的文本。
	Detail string
	// RequestID 是供应商的请求编号（来自响应头），没有则为空。
	RequestID string
	// Usage 是失败但已计费的用量（例如 2xx 却内容为空），没有则为零值。
	Usage Usage
}

// Error 实现 error 接口，不含供应商原文。
func (e *Error) Error() string {
	if e == nil {
		return "provider: <nil>"
	}
	var b strings.Builder
	b.WriteString("provider: ")
	b.WriteString(e.Kind.String())
	var parts []string
	if e.HTTPStatus != 0 {
		parts = append(parts, "http="+strconv.Itoa(e.HTTPStatus))
	}
	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}
	if len(parts) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(parts, " "))
		b.WriteString(")")
	}
	return b.String()
}

// Unwrap 返回底层原因。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// KindOf 取出 err 链中第一个 *Error 的种类；err 链里没有 *Error 时 ok 为 false。
func KindOf(err error) (kind ErrKind, ok bool) {
	if pe := asError(err); pe != nil {
		return pe.Kind, true
	}
	return Unknown, false
}

// asError 取出 err 链中的 *Error，没有则返回 nil。
func asError(err error) *Error {
	var pe *Error
	if errors.As(err, &pe) && pe != nil {
		return pe
	}
	return nil
}

// ctxToError 把 context 的终止原因映射成 *Error：调用方取消 → Canceled，到期 → Timeout。
func ctxToError(cause error) *Error {
	if errors.Is(cause, context.DeadlineExceeded) {
		return &Error{Kind: Timeout, Code: "deadline_exceeded", Err: cause}
	}
	return &Error{Kind: Canceled, Code: "canceled", Err: cause}
}

// sanitizeToken 把供应商给的标识净化成可安全写进错误文本的短串：
// 只保留字母、数字与 _ . : -，其余字符换成下划线，并限制长度。
func sanitizeToken(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '.', r == ':', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		n++
	}
	return b.String()
}
