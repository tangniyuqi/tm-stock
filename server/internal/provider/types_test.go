package provider_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func Test错误种类_字符串(t *testing.T) {
	cases := []struct {
		kind provider.ErrKind
		want string
	}{
		{provider.Unknown, "Unknown"},
		{provider.Auth, "Auth"},
		{provider.Billing, "Billing"},
		{provider.RateLimit, "RateLimit"},
		{provider.Overloaded, "Overloaded"},
		{provider.Timeout, "Timeout"},
		{provider.BadRequest, "BadRequest"},
		{provider.ContextLen, "ContextLen"},
		{provider.ContentFilter, "ContentFilter"},
		{provider.Malformed, "Malformed"},
		{provider.Canceled, "Canceled"},
		{provider.BudgetExceeded, "BudgetExceeded"},
		{provider.CircuitOpen, "CircuitOpen"},
		{provider.ErrKind(200), "ErrKind(200)"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.kind.String(); got != c.want {
				t.Fatalf("String() = %q，期望 %q", got, c.want)
			}
		})
	}
	if len(allKinds) != 13 {
		t.Fatalf("辅助清单 allKinds 有 %d 项，应为 13（含零值 Unknown）；新增种类时请同步更新测试", len(allKinds))
	}
}

func Test错误_文本只含种类状态与错误码(t *testing.T) {
	cases := []struct {
		name string
		err  *provider.Error
		want string
	}{
		{"仅种类", &provider.Error{Kind: provider.Auth}, "provider: Auth"},
		{"带HTTP状态", &provider.Error{Kind: provider.RateLimit, HTTPStatus: 429}, "provider: RateLimit (http=429)"},
		{"带错误码", &provider.Error{Kind: provider.Malformed, Code: "empty_content"}, "provider: Malformed (code=empty_content)"},
		{"状态与错误码", &provider.Error{Kind: provider.Billing, HTTPStatus: 402, Code: "insufficient_quota"},
			"provider: Billing (http=402 code=insufficient_quota)"},
		{"供应商原文只留在Detail与Err里", &provider.Error{
			Kind: provider.BadRequest, HTTPStatus: 400, Code: "x",
			Detail: vendorMsg, Err: errors.New(vendorMsg),
		}, "provider: BadRequest (http=400 code=x)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Fatalf("Error() = %q，期望 %q", got, c.want)
			}
		})
	}

	var nilErr *provider.Error
	if got := nilErr.Error(); got != "provider: <nil>" {
		t.Fatalf("nil 接收者的 Error() = %q", got)
	}
	if nilErr.Unwrap() != nil {
		t.Fatal("nil 接收者的 Unwrap 应返回 nil")
	}
}

func Test错误_Unwrap与KindOf(t *testing.T) {
	e := &provider.Error{Kind: provider.Canceled, Err: context.Canceled}
	if !errors.Is(e, context.Canceled) {
		t.Fatal("errors.Is 应能穿过 Unwrap 找到底层原因")
	}
	wrapped := fmt.Errorf("外层包装: %w", e)
	if kind, ok := provider.KindOf(wrapped); !ok || kind != provider.Canceled {
		t.Fatalf("KindOf(包装后) = %v, %v，期望 Canceled, true", kind, ok)
	}
	if kind, ok := provider.KindOf(nil); ok || kind != provider.Unknown {
		t.Fatalf("KindOf(nil) = %v, %v，期望 Unknown, false", kind, ok)
	}
	if _, ok := provider.KindOf(errors.New("裸错误")); ok {
		t.Fatal("非 *Error 不应被识别为有种类")
	}
	// 装着 nil 指针的 error 接口（经典陷阱）不能被当成有效的 *Error。
	if _, ok := provider.KindOf((*provider.Error)(nil)); ok {
		t.Fatal("typed nil 的 *Error 不应被识别")
	}
}

func Test格式与能力_字符串与位运算(t *testing.T) {
	formats := map[provider.Format]string{
		provider.FormatText:       "text",
		provider.FormatJSONObject: "json_object",
		provider.FormatJSONSchema: "json_schema",
		provider.Format(9):        "format(9)",
	}
	for f, want := range formats {
		if got := f.String(); got != want {
			t.Fatalf("Format(%d).String() = %q，期望 %q", uint8(f), got, want)
		}
	}

	capCases := []struct {
		caps provider.Caps
		want string
	}{
		{0, "none"},
		{provider.CapJSONObject, "json_object"},
		{provider.CapJSONObject | provider.CapJSONSchema, "json_object|json_schema"},
		{provider.CapJSONObject | provider.CapJSONSchema | provider.CapTools | provider.CapStream,
			"json_object|json_schema|tools|stream"},
	}
	for _, c := range capCases {
		if got := c.caps.String(); got != c.want {
			t.Fatalf("Caps(%d).String() = %q，期望 %q", uint8(c.caps), got, c.want)
		}
	}

	both := provider.CapJSONObject | provider.CapJSONSchema
	if !both.Has(provider.CapJSONSchema) || !both.Has(both) {
		t.Fatal("Has 应对已含的能力返回 true")
	}
	if provider.CapJSONObject.Has(both) {
		t.Fatal("缺任何一个被要求的能力，Has 都应返回 false")
	}
	if !provider.Caps(0).Has(0) {
		t.Fatal("不要求任何能力时恒为 true")
	}
}

func Test装配_Chain顺序与nil跳过(t *testing.T) {
	var trace []string
	mark := func(name string) provider.Middleware {
		return func(next provider.Provider) provider.Provider {
			return provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
				trace = append(trace, name+"进")
				resp, err := next.Generate(ctx, req)
				trace = append(trace, name+"出")
				return resp, err
			})
		}
	}
	base := provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		trace = append(trace, "底层")
		return provider.Response{Text: "好"}, nil
	})

	p := provider.Chain(base, mark("A"), nil, mark("B"), mark("C"))
	resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
	if err != nil || resp.Text != "好" {
		t.Fatalf("Chain 结果异常: %+v, %v", resp, err)
	}
	want := []string{"A进", "B进", "C进", "底层", "C出", "B出", "A出"}
	if fmt.Sprint(trace) != fmt.Sprint(want) {
		t.Fatalf("调用顺序 = %v，期望最先写的在最外层 %v", trace, want)
	}

	// 没有中间件时直接用底层。
	trace = nil
	if _, err := provider.Chain(base).Generate(context.Background(), provider.Request{}); err != nil {
		t.Fatalf("无中间件的 Chain 调用失败: %v", err)
	}
	if fmt.Sprint(trace) != fmt.Sprint([]string{"底层"}) {
		t.Fatalf("无中间件时调用轨迹 = %v", trace)
	}
}

func Test时钟_RealClock(t *testing.T) {
	var c provider.Clock = provider.RealClock{}

	before := time.Now()
	if now := c.Now(); now.Before(before) || time.Since(now) > time.Minute {
		t.Fatalf("RealClock.Now() = %v，与系统时间不符", now)
	}

	if err := c.Sleep(context.Background(), 0); err != nil {
		t.Fatalf("Sleep(0) 在 ctx 正常时应返回 nil: %v", err)
	}
	if err := c.Sleep(context.Background(), -time.Second); err != nil {
		t.Fatalf("Sleep(负数) 在 ctx 正常时应返回 nil: %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Sleep(canceled, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sleep(0) 在 ctx 已取消时应返回 ctx.Err()，实际 %v", err)
	}
	start := time.Now()
	if err := c.Sleep(canceled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sleep 在 ctx 已取消时应立即返回 ctx.Err()，实际 %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("ctx 已取消时 Sleep 不应真的等待")
	}

	start = time.Now()
	if err := c.Sleep(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatalf("Sleep(20ms) 返回 %v", err)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("Sleep(20ms) 只等了 %v", elapsed)
	}
}
