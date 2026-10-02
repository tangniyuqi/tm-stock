package provider_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

// errBody 构造 OpenAI 风格的错误体，message 固定为 vendorMsg，用来检查原文不外泄。
func errBody(typ, code string) string {
	return `{"error":{"message":"` + vendorMsg + `","type":"` + typ + `","code":"` + code + `"}}`
}

func Test适配层_错误分类且上游恰好一次请求(t *testing.T) {
	cases := []struct {
		name       string
		step       providertest.Step
		kind       provider.ErrKind
		status     int
		code       string // 为空表示不检查
		retryAfter time.Duration
		wantDetail bool // 期望 Detail 里保留了供应商原文
	}{
		// 鉴权与欠费
		{"401鉴权失败", providertest.Status(401, errBody("invalid_request_error", "invalid_api_key")), provider.Auth, 401, "invalid_api_key", 0, true},
		{"403拒绝访问", providertest.Status(403, "{}"), provider.Auth, 403, "", 0, false},
		{"402欠费", providertest.Status(402, errBody("", "")), provider.Billing, 402, "", 0, true},
		{"429但错误码为额度耗尽", providertest.Status(429, errBody("insufficient_quota", "insufficient_quota")), provider.Billing, 429, "insufficient_quota", 0, true},
		{"429错误码为余额不足", providertest.Status(429, errBody("", "insufficient_balance")), provider.Billing, 429, "insufficient_balance", 0, true},
		{"429错误码含billing", providertest.Status(429, errBody("", "billing_hard_limit_reached")), provider.Billing, 429, "billing_hard_limit_reached", 0, true},
		{"400错误码为欠费", providertest.Status(400, errBody("", "Arrearage")), provider.Billing, 400, "Arrearage", 0, true},
		{"403错误码为欠费优先于鉴权", providertest.Status(403, errBody("", "AccountOverdueError")), provider.Billing, 403, "AccountOverdueError", 0, true},

		// 限流与 Retry-After
		{"429限流带秒数", providertest.RateLimited("7"), provider.RateLimit, 429, "rate_limit_exceeded", 7 * time.Second, false},
		{"429限流无Retry-After", providertest.RateLimited(""), provider.RateLimit, 429, "rate_limit_exceeded", 0, false},
		{"429限流带小数秒", providertest.RateLimited("1.5"), provider.RateLimit, 429, "", 1500 * time.Millisecond, false},
		{"429限流垃圾值按0处理", providertest.RateLimited("soon"), provider.RateLimit, 429, "", 0, false},
		{"429限流负数按0处理", providertest.RateLimited("-5"), provider.RateLimit, 429, "", 0, false},
		{"429限流零按0处理", providertest.RateLimited("0"), provider.RateLimit, 429, "", 0, false},
		{"429限流超大值截为一天", providertest.RateLimited("99999999999"), provider.RateLimit, 429, "", 24 * time.Hour, false},
		{"429限流超大小数截为一天", providertest.RateLimited("1e12"), provider.RateLimit, 429, "", 24 * time.Hour, false},
		{"429限流无穷大按0处理", providertest.RateLimited("inf"), provider.RateLimit, 429, "", 0, false},

		// 过载与超时
		{"500", providertest.Status(500, errBody("server_error", "internal_error")), provider.Overloaded, 500, "internal_error", 0, true},
		{"502", providertest.Status(502, errBody("", "")), provider.Overloaded, 502, "", 0, true},
		{"503", providertest.Status(503, errBody("", "")), provider.Overloaded, 503, "", 0, true},
		{"504", providertest.Status(504, errBody("", "")), provider.Overloaded, 504, "", 0, true},
		{"529", providertest.Status(529, errBody("", "overloaded_error")), provider.Overloaded, 529, "overloaded_error", 0, true},
		{"599其余5xx", providertest.Status(599, ""), provider.Overloaded, 599, "", 0, false},
		{"502非JSON网关页", providertest.Status(502, "<html>bad gateway</html>"), provider.Overloaded, 502, "", 0, false},
		{"503带Retry-After", providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(errBody("server_error", "overloaded")))
		}), provider.Overloaded, 503, "overloaded", 3 * time.Second, true},
		{"408请求超时", providertest.Status(408, ""), provider.Timeout, 408, "", 0, false},

		// 上下文超长与内容安全
		{"400上下文超长(错误码)", providertest.Status(400, errBody("invalid_request_error", "context_length_exceeded")), provider.ContextLen, 400, "context_length_exceeded", 0, true},
		{"400上下文超长(仅消息)", providertest.Status(400, `{"error":{"message":"`+vendorMsg+` maximum context length is 65536 tokens","type":"invalid_request_error"}}`), provider.ContextLen, 400, "invalid_request_error", 0, true},
		{"413上下文超长(仅消息)", providertest.Status(413, `{"error":{"message":"`+vendorMsg+` context window exceeded"}}`), provider.ContextLen, 413, "", 0, true},
		{"400内容安全(content_filter)", providertest.Status(400, errBody("invalid_request_error", "content_filter")), provider.ContentFilter, 400, "content_filter", 0, true},
		{"400内容安全(类型content_policy_violation)", providertest.Status(400, errBody("content_policy_violation", "")), provider.ContentFilter, 400, "content_policy_violation", 0, true},
		{"400内容安全(data_inspection_failed)", providertest.Status(400, errBody("", "data_inspection_failed")), provider.ContentFilter, 400, "data_inspection_failed", 0, true},

		// 其余 4xx
		{"400普通请求错误", providertest.Status(400, errBody("invalid_request_error", "")), provider.BadRequest, 400, "invalid_request_error", 0, true},
		{"404", providertest.Status(404, errBody("", "model_not_found")), provider.BadRequest, 404, "model_not_found", 0, true},
		{"422", providertest.Status(422, errBody("", "")), provider.BadRequest, 422, "", 0, true},
		{"错误体顶层数字code", providertest.Status(400, `{"code":40001,"message":"`+vendorMsg+`"}`), provider.BadRequest, 400, "40001", 0, true},
		{"错误体顶层字符串code", providertest.Status(400, `{"code":"E_PARAM","message":"`+vendorMsg+`"}`), provider.BadRequest, 400, "E_PARAM", 0, true},
		{"错误体嵌套数字code", providertest.Status(400, `{"error":{"code":1301,"message":"`+vendorMsg+`"}}`), provider.BadRequest, 400, "1301", 0, true},
		{"错误体error为字符串", providertest.Status(400, `{"error":"`+vendorMsg+`"}`), provider.BadRequest, 400, "", 0, true},
		{"错误体不是JSON", providertest.Status(400, "plain text "+vendorMsg), provider.BadRequest, 400, "", 0, false},
		{"错误体为空", providertest.Status(400, ""), provider.BadRequest, 400, "", 0, false},
		{"错误码含非法字符被净化", providertest.Status(400, errBody("", "bad code!<script>")), provider.BadRequest, 400, "bad_code__script_", 0, true},
		{"错误码过长被截断", providertest.Status(400, errBody("", strings.Repeat("a", 100))), provider.BadRequest, 400, strings.Repeat("a", 64), 0, true},
		{"错误码为非ASCII按字符净化", providertest.Status(400, errBody("", "错误码")), provider.BadRequest, 400, "___", 0, true},

		// 不该出现的状态码
		{"300按未预期状态处理", providertest.Status(300, ""), provider.BadRequest, 300, "unexpected_status", 0, false},
		{"302重定向不跟随", providertest.Redirect(302, "http://127.0.0.1:1/elsewhere"), provider.BadRequest, 302, "unexpected_status", 0, false},

		// 2xx 但不可用
		{"2xx截断JSON", providertest.Malformed(providertest.TruncatedJSON), provider.Malformed, 200, "invalid_json", 0, false},
		{"2xx空choices", providertest.Malformed(providertest.EmptyChoices), provider.Malformed, 200, "empty_choices", 0, false},
		{"2xx空content", providertest.Malformed(providertest.EmptyContent), provider.Malformed, 200, "empty_content", 0, false},
		{"2xx内容为null", providertest.Malformed(providertest.NullContent), provider.Malformed, 200, "empty_content", 0, false},
		{"2xx非JSON", providertest.Malformed(providertest.NotJSON), provider.Malformed, 200, "invalid_json", 0, false},
		{"2xx内容仅空白", providertest.Status(200, `{"choices":[{"message":{"content":"  \n "},"finish_reason":"stop"}]}`), provider.Malformed, 200, "empty_content", 0, false},
		{"2xx内容为数组", providertest.Status(200, `{"choices":[{"message":{"content":[{"type":"text","text":"x"}]},"finish_reason":"stop"}]}`), provider.Malformed, 200, "empty_content", 0, false},
		{"2xx缺少message", providertest.Status(200, `{"choices":[{"finish_reason":"stop"}]}`), provider.Malformed, 200, "empty_content", 0, false},
		{"201空响应体", providertest.Status(201, ""), provider.Malformed, 201, "invalid_json", 0, false},
		{"2xx里的JSON数组", providertest.Status(200, `[1,2,3]`), provider.Malformed, 200, "invalid_json", 0, false},
		{"finish_reason为content_filter", providertest.Status(200, `{"id":"x","choices":[{"message":{"content":"部分文本"},"finish_reason":"content_filter"}]}`), provider.ContentFilter, 200, "content_filter", 0, false},
		{"200里的错误体为欠费", providertest.Status(200, errBody("", "insufficient_quota")), provider.Billing, 200, "insufficient_quota", 0, true},
		{"200里的错误体为内容安全", providertest.Status(200, errBody("", "content_filter")), provider.ContentFilter, 200, "content_filter", 0, true},
		{"200里的错误体为上下文超长", providertest.Status(200, errBody("", "context_length_exceeded")), provider.ContextLen, 200, "context_length_exceeded", 0, true},
		{"200里的错误体认不出来按畸形", providertest.Status(200, errBody("", "server_error")), provider.Malformed, 200, "empty_choices", 0, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := providertest.NewServer(t, c.step)
			p := newAdapter(t, srv)
			_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"})
			if err == nil {
				t.Fatal("期望失败，实际成功")
			}
			pe := asProviderError(t, err)
			if pe.Kind != c.kind {
				t.Fatalf("分类 = %v，期望 %v（%v）", pe.Kind, c.kind, err)
			}
			if pe.HTTPStatus != c.status {
				t.Fatalf("HTTPStatus = %d，期望 %d", pe.HTTPStatus, c.status)
			}
			if c.code != "" && pe.Code != c.code {
				t.Fatalf("Code = %q，期望 %q", pe.Code, c.code)
			}
			if pe.RetryAfter != c.retryAfter {
				t.Fatalf("RetryAfter = %v，期望 %v", pe.RetryAfter, c.retryAfter)
			}
			if got := srv.Count(); got != 1 {
				t.Fatalf("上游请求数 = %d，期望恰好 1（不得隐式重试）", got)
			}
			if strings.Contains(err.Error(), vendorMsg) {
				t.Fatalf("Error() 泄露了供应商原文: %s", err.Error())
			}
			if c.wantDetail && !strings.Contains(pe.Detail, vendorMsg) {
				t.Fatalf("Detail = %q，应保留供应商原文供脱敏日志使用", pe.Detail)
			}
		})
	}
}

func Test适配层_HTTPdate形式的RetryAfter(t *testing.T) {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"未来90秒", base.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{"已过去的时间按0处理", base.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"恰好此刻按0处理", base.Format(http.TimeFormat), 0},
		{"超过一天截为一天", base.Add(72 * time.Hour).Format(http.TimeFormat), 24 * time.Hour},
		{"RFC850格式", base.Add(30 * time.Second).Format(time.RFC850), 30 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := providertest.NewServer(t, providertest.RateLimited(c.header))
			clock := providertest.NewFakeClock(base)
			p := newAdapter(t, srv, provider.WithClock(clock))
			_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"})
			pe := asProviderError(t, err)
			if pe.Kind != provider.RateLimit {
				t.Fatalf("分类 = %v，期望 RateLimit", pe.Kind)
			}
			if pe.RetryAfter != c.want {
				t.Fatalf("RetryAfter = %v，期望 %v", pe.RetryAfter, c.want)
			}
			if srv.Count() != 1 {
				t.Fatalf("上游请求数 = %d，期望 1", srv.Count())
			}
		})
	}
}

func Test适配层_错误上的请求编号与原文截断(t *testing.T) {
	t.Run("请求编号取自响应头并净化", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-Id", "req 1/2")
			w.WriteHeader(http.StatusBadRequest)
		}))
		_, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "你好"})
		if got := asProviderError(t, err).RequestID; got != "req_1_2" {
			t.Fatalf("RequestID = %q，期望 req_1_2", got)
		}
	})

	t.Run("备用请求头Request-Id", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Request-Id", "alt-77")
			w.WriteHeader(http.StatusBadRequest)
		}))
		_, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "你好"})
		if got := asProviderError(t, err).RequestID; got != "alt-77" {
			t.Fatalf("RequestID = %q，期望 alt-77", got)
		}
	})

	t.Run("Status步骤默认带请求编号", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Status(400, "{}"))
		_, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "你好"})
		if got := asProviderError(t, err).RequestID; got != providertest.RequestIDFor(1) {
			t.Fatalf("RequestID = %q，期望 %q", got, providertest.RequestIDFor(1))
		}
	})

	t.Run("供应商原文超长时按字符边界截断", func(t *testing.T) {
		long := strings.Repeat("长", 2000)
		srv := providertest.NewServer(t, providertest.Status(400, `{"error":{"message":"`+long+`"}}`))
		_, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "你好"})
		detail := asProviderError(t, err).Detail
		if len(detail) == 0 || len(detail) > 512 {
			t.Fatalf("Detail 长度 = %d 字节，应在 (0, 512]", len(detail))
		}
		if !utf8.ValidString(detail) {
			t.Fatal("Detail 被截断后不是合法 UTF-8，说明切断了多字节字符")
		}
	})
}
