package provider_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

func Test适配层_连接与时限(t *testing.T) {
	ctx := context.Background()
	req := provider.Request{Prompt: "你好"}

	t.Run("响应体读到一半连接断开归Malformed且只请求一次", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Abort())
		_, err := newAdapter(t, srv).Generate(ctx, req)
		pe := asProviderError(t, err)
		if pe.Kind != provider.Malformed || pe.Code != "truncated_body" || pe.HTTPStatus != 200 {
			t.Fatalf("得到 %+v，期望 Malformed/truncated_body/200", pe)
		}
		if srv.Count() != 1 {
			t.Fatalf("上游请求数 = %d，期望恰好 1（证明 net/http 没有隐式重试）", srv.Count())
		}
	})

	t.Run("响应前连接被断开归Overloaded且只请求一次", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Drop())
		_, err := newAdapter(t, srv).Generate(ctx, req)
		pe := asProviderError(t, err)
		if pe.Kind != provider.Overloaded || pe.Code != "transport_error" || pe.HTTPStatus != 0 {
			t.Fatalf("得到 %+v，期望 Overloaded/transport_error/无状态", pe)
		}
		if srv.Count() != 1 {
			t.Fatalf("上游请求数 = %d，期望恰好 1", srv.Count())
		}
	})

	t.Run("单次尝试时限到了归Timeout", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Hang())
		start := time.Now()
		_, err := newAdapter(t, srv).Generate(ctx, provider.Request{Prompt: "你好", AttemptTimeout: 300 * time.Millisecond})
		pe := asProviderError(t, err)
		if pe.Kind != provider.Timeout || pe.Code != "attempt_timeout" {
			t.Fatalf("得到 %+v，期望 Timeout/attempt_timeout", pe)
		}
		if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			t.Fatalf("底层原因应是 DeadlineExceeded 而不是 Canceled: %v", pe.Err)
		}
		if srv.Count() != 1 {
			t.Fatalf("上游请求数 = %d，期望 1", srv.Count())
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("单次时限没有生效，调用被挂住了")
		}
	})

	t.Run("响应头迟到超过单次时限归Timeout", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SlowHeader(2*time.Second))
		_, err := newAdapter(t, srv).Generate(ctx, provider.Request{Prompt: "你好", AttemptTimeout: 300 * time.Millisecond})
		if pe := asProviderError(t, err); pe.Kind != provider.Timeout {
			t.Fatalf("分类 = %v，期望 Timeout", pe.Kind)
		}
		if srv.Count() != 1 {
			t.Fatalf("上游请求数 = %d，期望 1", srv.Count())
		}
	})

	t.Run("调用方取消归Canceled而不是Timeout", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Hang())
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			// 等请求确实到了服务端再取消，避免靠睡眠猜时机。
			srv.WaitCount(1, 5*time.Second)
			cancel()
		}()
		_, err := newAdapter(t, srv).Generate(cctx, req)
		pe := asProviderError(t, err)
		if pe.Kind != provider.Canceled || pe.Code != "canceled" {
			t.Fatalf("得到 %+v，期望 Canceled/canceled", pe)
		}
		if !errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("底层原因应是 Canceled: %v", pe.Err)
		}
		if srv.Count() != 1 {
			t.Fatalf("上游请求数 = %d，期望 1", srv.Count())
		}
	})

	t.Run("调用方deadline到期归Timeout", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Hang())
		dctx, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
		defer cancel()
		_, err := newAdapter(t, srv).Generate(dctx, provider.Request{Prompt: "你好", AttemptTimeout: 10 * time.Second})
		pe := asProviderError(t, err)
		if pe.Kind != provider.Timeout || pe.Code != "deadline_exceeded" {
			t.Fatalf("得到 %+v，期望 Timeout/deadline_exceeded（调用方自己的时限，与单次尝试时限区分）", pe)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("底层原因应是 DeadlineExceeded: %v", pe.Err)
		}
	})

	t.Run("ctx已取消时不向上游发请求", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := newAdapter(t, srv).Generate(cctx, req)
		if pe := asProviderError(t, err); pe.Kind != provider.Canceled {
			t.Fatalf("分类 = %v，期望 Canceled", pe.Kind)
		}
		if srv.Count() != 0 {
			t.Fatalf("上游请求数 = %d，期望 0", srv.Count())
		}
	})

	t.Run("ctx已过期时不向上游发请求", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
		dctx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
		defer cancel()
		_, err := newAdapter(t, srv).Generate(dctx, req)
		pe := asProviderError(t, err)
		if pe.Kind != provider.Timeout || pe.Code != "deadline_exceeded" {
			t.Fatalf("得到 %+v，期望 Timeout/deadline_exceeded", pe)
		}
		if srv.Count() != 0 {
			t.Fatalf("上游请求数 = %d，期望 0", srv.Count())
		}
	})

	t.Run("服务已关闭时连接被拒归Overloaded", func(t *testing.T) {
		srv := providertest.NewServer(t)
		p := newAdapter(t, srv)
		srv.Close()
		_, err := p.Generate(ctx, req)
		pe := asProviderError(t, err)
		if pe.Kind != provider.Overloaded || pe.Code != "transport_error" {
			t.Fatalf("得到 %+v，期望 Overloaded/transport_error", pe)
		}
	})

	t.Run("Config.TimeoutMS是默认的单次时限", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Hang())
		_, err := newAdapterTimeout(t, srv, 50).Generate(ctx, req)
		if pe := asProviderError(t, err); pe.Kind != provider.Timeout || pe.Code != "attempt_timeout" {
			t.Fatalf("得到 %+v，期望 Timeout/attempt_timeout", pe)
		}
	})

	t.Run("AttemptTimeout可以把默认时限放长", func(t *testing.T) {
		// 默认时限 50ms 本来撑不住 200ms 的慢响应，AttemptTimeout 放长后应当成功。
		srv := providertest.NewServer(t, providertest.SlowHeader(200*time.Millisecond))
		resp, err := newAdapterTimeout(t, srv, 50).Generate(ctx, provider.Request{Prompt: "你好", AttemptTimeout: 5 * time.Second})
		if err != nil || resp.Text != "慢响应" {
			t.Fatalf("放长单次时限后应成功，得到 %q, %v", resp.Text, err)
		}
	})
}

func Test适配层_从不隐式重试(t *testing.T) {
	ctx := context.Background()

	t.Run("请求不设GetBody且长度已知", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
		var seen *http.Request
		rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = r
			return http.DefaultTransport.RoundTrip(r)
		})
		if _, err := newAdapter(t, srv, provider.WithTransport(rt)).Generate(ctx, provider.Request{Prompt: "你好"}); err != nil {
			t.Fatalf("Generate 失败: %v", err)
		}
		if seen == nil {
			t.Fatal("自定义 Transport 没有被使用")
		}
		if seen.GetBody != nil {
			t.Fatal("请求设置了 GetBody：net/http 可能在连接失效时悄悄重发这个 POST")
		}
		if seen.Method != http.MethodPost || seen.ContentLength <= 0 {
			t.Fatalf("方法=%s 长度=%d，期望 POST 且长度已知", seen.Method, seen.ContentLength)
		}
		if seen.Header.Get("Authorization") != "Bearer "+testKey {
			t.Fatal("Authorization 头不对")
		}
	})

	t.Run("复用的连接被服务端断开时也只发一次", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"), providertest.Drop())
		p := newAdapter(t, srv)
		if _, err := p.Generate(ctx, provider.Request{Prompt: "第一次"}); err != nil {
			t.Fatalf("第一次应成功: %v", err)
		}
		_, err := p.Generate(ctx, provider.Request{Prompt: "第二次"})
		if pe := asProviderError(t, err); pe.Kind != provider.Overloaded {
			t.Fatalf("第二次分类 = %v，期望 Overloaded", pe.Kind)
		}
		if srv.Count() != 2 {
			t.Fatalf("上游请求数 = %d，期望恰好 2（第二次失败后不得被悄悄重发）", srv.Count())
		}
	})

	t.Run("各种重定向都不跟随", func(t *testing.T) {
		for _, code := range []int{301, 302, 303, 307, 308} {
			// other 没有脚本：它若收到任何请求，会让本测试失败。
			other := providertest.NewServer(t)
			srv := providertest.NewServer(t, providertest.Redirect(code, other.URL()+"/elsewhere"))
			_, err := newAdapter(t, srv).Generate(ctx, provider.Request{Prompt: "你好"})
			pe := asProviderError(t, err)
			if pe.Kind != provider.BadRequest || pe.Code != "unexpected_status" || pe.HTTPStatus != code {
				t.Fatalf("状态 %d: 得到 %+v，期望 BadRequest/unexpected_status", code, pe)
			}
			if srv.Count() != 1 || other.Count() != 0 {
				t.Fatalf("状态 %d: 原地址收到 %d 个请求、重定向目标收到 %d 个，期望 1 与 0", code, srv.Count(), other.Count())
			}
		}
	})
}

func Test适配层_响应体上限(t *testing.T) {
	ctx := context.Background()
	body := `{"id":"c1","choices":[{"message":{"content":"好"},"finish_reason":"stop"}]}`
	n := int64(len(body))

	t.Run("恰好等于上限仍成功", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Status(200, body))
		if _, err := newAdapter(t, srv, provider.WithMaxResponseBytes(n)).Generate(ctx, provider.Request{Prompt: "x"}); err != nil {
			t.Fatalf("响应体恰好等于上限时应成功: %v", err)
		}
	})

	t.Run("超过上限一个字节归Malformed", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Status(200, body))
		_, err := newAdapter(t, srv, provider.WithMaxResponseBytes(n-1)).Generate(ctx, provider.Request{Prompt: "x"})
		pe := asProviderError(t, err)
		if pe.Kind != provider.Malformed || pe.Code != "response_too_large" || pe.HTTPStatus != 200 {
			t.Fatalf("得到 %+v，期望 Malformed/response_too_large/200", pe)
		}
	})

	t.Run("非2xx的超大响应体仍按状态码分类", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Status(429, strings.Repeat("x", 1000)))
		_, err := newAdapter(t, srv, provider.WithMaxResponseBytes(200)).Generate(ctx, provider.Request{Prompt: "x"})
		if pe := asProviderError(t, err); pe.Kind != provider.RateLimit {
			t.Fatalf("分类 = %v，期望 RateLimit", pe.Kind)
		}
	})

	t.Run("非正的上限设置被忽略", func(t *testing.T) {
		for _, limit := range []int64{0, -1} {
			srv := providertest.NewServer(t, providertest.Status(200, body))
			if _, err := newAdapter(t, srv, provider.WithMaxResponseBytes(limit)).Generate(ctx, provider.Request{Prompt: "x"}); err != nil {
				t.Fatalf("上限=%d 应被忽略并沿用默认值: %v", limit, err)
			}
		}
	})
}

func Test适配层_并发调用安全(t *testing.T) {
	const workers = 16
	steps := make([]providertest.Step, workers)
	for i := range steps {
		steps[i] = providertest.SuccessNoUsage("好")
	}
	srv := providertest.NewServer(t, steps...)
	p := newAdapter(t, srv)

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("并发调用失败: %v", err)
		}
	}
	if srv.Count() != workers {
		t.Fatalf("上游请求数 = %d，期望 %d", srv.Count(), workers)
	}
}
