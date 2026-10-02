package provider

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxCodeLen = 64
	maxIDLen   = 128
	// maxRetryAfter 只用于防止溢出与荒谬值：再大的 Retry-After 也按一天处理。
	maxRetryAfter = 24 * time.Hour
)

// 下面三组是已知的错误码、错误类型字样，匹配时统一转小写后做子串判断。
// 它们不是穷尽的：各厂商的错误码差异很大（设计 5.4，均未实测），
// 上线前要逐家用契约快照补齐，补充时只增不减。
var (
	// billingNeedles 表示额度或欠费类问题。命中时无论 HTTP 状态是 402、429 还是 400 都归 Billing，
	// 因为这类问题靠重试解决不了，且需要告警。
	billingNeedles = []string{
		"insufficient_quota", "insufficient_balance", "insufficient_user_quota", "insufficient_funds",
		"billing", "exceeded_current_quota", "arrearage", "accountoverdue", "account_overdue",
		"out_of_credit",
	}
	// filterNeedles 表示内容安全拦截。
	filterNeedles = []string{
		"content_filter", "content_policy", "data_inspection", "responsibleaipolicy",
		"inappropriate_content", "sensitive_content",
	}
	// contextNeedles 表示输入超出上下文长度。
	contextNeedles = []string{
		"context_length", "context_window", "max_context", "string_above_max_length",
	}
	// contextMessageNeedles 是兜底：有些厂商把上下文超长写成通用的 invalid_request_error，
	// 只能从消息文本里认。仅在 400、413、422 时使用，且只用来选分类，不会进入错误文本。
	contextMessageNeedles = []string{
		"maximum context length", "context length", "context window",
	}
)

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// errorInfo 是从响应体里取出的错误信息，字段含义对应 OpenAI 风格的 error 对象。
type errorInfo struct {
	message string
	typ     string
	code    string
}

func (i errorInfo) present() bool {
	return i.message != "" || i.typ != "" || i.code != ""
}

// parseErrorBody 兼容两种结构：{"error":{"message","type","code"}} 与顶层 {"code","message"}；
// code 可以是字符串或数字，error 也可以直接是一个字符串。解析不了就返回零值。
func parseErrorBody(raw []byte) errorInfo {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return errorInfo{}
	}
	var top struct {
		Error   json.RawMessage `json:"error"`
		Code    json.RawMessage `json:"code"`
		Message json.RawMessage `json:"message"`
		Type    json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return errorInfo{}
	}
	info := errorInfo{
		code:    scalarString(top.Code),
		message: scalarString(top.Message),
		typ:     scalarString(top.Type),
	}
	inner := bytes.TrimSpace(top.Error)
	if len(inner) == 0 {
		return info
	}
	switch inner[0] {
	case '{':
		var obj struct {
			Message json.RawMessage `json:"message"`
			Type    json.RawMessage `json:"type"`
			Code    json.RawMessage `json:"code"`
		}
		if json.Unmarshal(inner, &obj) == nil {
			// 嵌套的 error 对象比顶层字段更具体，优先。
			if v := scalarString(obj.Code); v != "" {
				info.code = v
			}
			if v := scalarString(obj.Type); v != "" {
				info.typ = v
			}
			if v := scalarString(obj.Message); v != "" {
				info.message = v
			}
		}
	case '"':
		if v := scalarString(inner); v != "" {
			info.message = v
		}
	}
	return info
}

// scalarString 把 JSON 标量转成字符串：字符串取其值，数字与布尔取字面量，null、对象、数组返回空串。
func scalarString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return ""
		}
		return s
	case 'n', '{', '[':
		return ""
	default:
		return string(raw)
	}
}

// classifyHTTP 把非 2xx 的响应归类。判断顺序即优先级：
// 额度类错误码 > 402 > 401/403 > 429 > 408 > 5xx > 其余 4xx（再细分内容安全、上下文超长）。
// 返回的 code 已净化，可放进错误文本。
func classifyHTTP(status int, info errorInfo) (ErrKind, string) {
	code := sanitizeToken(firstNonEmpty(info.code, info.typ), maxCodeLen)
	tags := strings.ToLower(info.code + " " + info.typ)
	if containsAny(tags, billingNeedles) {
		return Billing, code
	}
	switch {
	case status == http.StatusPaymentRequired:
		return Billing, code
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return Auth, code
	case status == http.StatusTooManyRequests:
		return RateLimit, code
	case status == http.StatusRequestTimeout:
		return Timeout, code
	case status >= 500 && status <= 599:
		return Overloaded, code
	case status >= 400 && status <= 499:
		if containsAny(tags, filterNeedles) {
			return ContentFilter, code
		}
		if containsAny(tags, contextNeedles) {
			return ContextLen, code
		}
		switch status {
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
			if containsAny(strings.ToLower(info.message), contextMessageNeedles) {
				return ContextLen, code
			}
		}
		return BadRequest, code
	default:
		// 1xx、3xx 等不该出现在这个接口上：多半是 BaseURL 配错（例如 http 被重定向到 https）。
		return BadRequest, "unexpected_status"
	}
}

// classifyBodyError 用于“2xx 却没有 choices、但带了 error 对象”的响应：
// 只认额度、内容安全、上下文超长三类，认不出来的由调用方按 Malformed 处理。
func classifyBodyError(info errorInfo) (ErrKind, bool) {
	tags := strings.ToLower(info.code + " " + info.typ)
	switch {
	case containsAny(tags, billingNeedles):
		return Billing, true
	case containsAny(tags, filterNeedles):
		return ContentFilter, true
	case containsAny(tags, contextNeedles):
		return ContextLen, true
	}
	return Unknown, false
}

// parseRetryAfter 解析 Retry-After：十进制秒数（也容忍小数）或 HTTP-date。
// HTTP-date 相对 now 换算；解析失败、非正值一律返回 0，超过一天按一天处理。
func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if secs, err := strconv.ParseInt(value, 10, 64); err == nil {
		if secs <= 0 {
			return 0
		}
		if secs >= int64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(secs) * time.Second
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		if f <= 0 {
			return 0
		}
		if f >= maxRetryAfter.Seconds() {
			return maxRetryAfter
		}
		return time.Duration(f * float64(time.Second))
	}
	if t, err := http.ParseTime(value); err == nil {
		d := t.Sub(now)
		if d <= 0 {
			return 0
		}
		return min(d, maxRetryAfter)
	}
	return 0
}

// requestIDFromHeader 从响应头取供应商的请求编号并净化。
func requestIDFromHeader(h http.Header) string {
	for _, name := range []string{"X-Request-Id", "Request-Id"} {
		if v := sanitizeToken(h.Get(name), maxIDLen); v != "" {
			return v
		}
	}
	return ""
}

// truncateUTF8 把 s 截到不超过 n 字节，且不切断多字节字符。
func truncateUTF8(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
