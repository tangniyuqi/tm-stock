package provider_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

// testKey 是测试用的假密钥。带 fake- 前缀：它不是任何真实凭据，也不会被密钥扫描器误报。
const testKey = "sk-fake-0123456789abcdef"

// vendorMsg 是塞进供应商错误体的原文标记，用来证明它不会出现在 Error() 里。
const vendorMsg = "供应商原文-不得外泄"

// newAdapter 用 mock 服务构造适配层，默认单次时限 5 秒。
func newAdapter(t *testing.T, srv *providertest.Server, opts ...provider.Option) *provider.OpenAICompat {
	t.Helper()
	return newAdapterTimeout(t, srv, 5000, opts...)
}

// newAdapterTimeout 同 newAdapter，但可指定 Config.TimeoutMS。
func newAdapterTimeout(t *testing.T, srv *providertest.Server, timeoutMS int, opts ...provider.Option) *provider.OpenAICompat {
	t.Helper()
	p, err := provider.New(provider.Config{
		Enabled:   true,
		Name:      "mock",
		BaseURL:   srv.URL(),
		Model:     "model-a",
		APIKey:    testKey,
		TimeoutMS: timeoutMS,
	}, opts...)
	if err != nil {
		t.Fatalf("构造适配层失败: %v", err)
	}
	return p
}

// asProviderError 断言 err 链里有 *provider.Error 并返回它。
func asProviderError(t *testing.T, err error) *provider.Error {
	t.Helper()
	var pe *provider.Error
	if !errors.As(err, &pe) {
		t.Fatalf("期望 *provider.Error，实际 %T: %v", err, err)
	}
	return pe
}

// scriptedResult 是 scripted 的一次应答。
type scriptedResult struct {
	resp provider.Response
	err  error
}

// scripted 是按调用次序出队的 Provider 替身：第 N 次调用返回第 N 个结果；
// repeat 为 true 时，结果用完后一直重复最后一个；否则超出脚本的调用返回一个未分类错误。
// hook 在每次调用返回结果之前执行（参数 n 从 1 起），用来在调用过程中取消 ctx、推进时钟等。
type scripted struct {
	mu       sync.Mutex
	results  []scriptedResult
	repeat   bool
	calls    int
	requests []provider.Request
	hook     func(ctx context.Context, n int)
}

func (s *scripted) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.requests = append(s.requests, req)
	var r scriptedResult
	switch {
	case n <= len(s.results):
		r = s.results[n-1]
	case s.repeat && len(s.results) > 0:
		r = s.results[len(s.results)-1]
	default:
		r = scriptedResult{err: errors.New("scripted: 脚本已耗尽")}
	}
	hook := s.hook
	s.mu.Unlock()
	if hook != nil {
		hook(ctx, n)
	}
	return r.resp, r.err
}

// Calls 返回被调用的次数。
func (s *scripted) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// Requests 返回收到的请求副本，按调用顺序。
func (s *scripted) Requests() []provider.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func okResult(text string) scriptedResult {
	return scriptedResult{resp: provider.Response{
		Text:  text,
		Usage: provider.Usage{Prompt: 3, Completion: 2, Total: 5},
	}}
}

func failResult(kind provider.ErrKind) scriptedResult {
	return scriptedResult{err: &provider.Error{Kind: kind, Code: "scripted"}}
}

func failRetryAfter(kind provider.ErrKind, d time.Duration) scriptedResult {
	return scriptedResult{err: &provider.Error{Kind: kind, Code: "scripted", RetryAfter: d}}
}

// roundTripFunc 让一个函数满足 http.RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// allKinds 是全部错误种类（含零值 Unknown），按声明顺序。
var allKinds = []provider.ErrKind{
	provider.Unknown, provider.Auth, provider.Billing, provider.RateLimit, provider.Overloaded,
	provider.Timeout, provider.BadRequest, provider.ContextLen, provider.ContentFilter,
	provider.Malformed, provider.Canceled, provider.BudgetExceeded, provider.CircuitOpen,
}
