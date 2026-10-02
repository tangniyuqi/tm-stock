package provider_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

var okUsage = provider.Usage{Prompt: 3, Completion: 4, Total: 7}

func Test集成_429两次后成功恰好3次请求且Retry_After被遵守(t *testing.T) {
	srv := providertest.NewServer(t,
		providertest.RateLimited("1"),
		providertest.RateLimited("1"),
		providertest.Success("你好，这是一条测试回复", okUsage),
	)
	clock := providertest.NewFakeClock(time.Time{})
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })
	policy := provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), MaxDelay: 8 * time.Second}

	// 日志放在重试内层：每次尝试一条日志，靠递增的 Attempt 区分。
	p := provider.Chain(newAdapter(t, srv),
		provider.RetryMiddleware(policy, clock, nil),
		provider.LoggingMiddleware(sink, clock),
	)
	resp, err := p.Generate(context.Background(), provider.Request{TaskID: "task-1", Step: 1, Prompt: "你好"})
	if err != nil || resp.Text != "你好，这是一条测试回复" {
		t.Fatalf("得到 %+v, %v", resp, err)
	}
	if srv.Count() != 3 {
		t.Fatalf("上游请求数 = %d，期望恰好 3", srv.Count())
	}
	// 退避 100ms、200ms 都小于 Retry-After 的 1s，所以两次都取 1s。
	if got := clock.Sleeps(); !sameDurations(got, []time.Duration{time.Second, time.Second}) {
		t.Fatalf("等待序列 = %v，期望 [1s 1s]", got)
	}

	if len(entries) != 3 {
		t.Fatalf("日志条数 = %d，期望每次尝试一条共 3 条", len(entries))
	}
	for i, e := range entries {
		if e.Attempt != i+1 || e.TaskID != "task-1" || e.Step != 1 {
			t.Fatalf("第 %d 条日志的追溯键 = %+v，期望 Attempt=%d", i+1, e, i+1)
		}
		if e.RequestID != providertest.RequestIDFor(i+1) {
			t.Fatalf("第 %d 条日志的 RequestID = %q", i+1, e.RequestID)
		}
	}
	for i := 0; i < 2; i++ {
		e := entries[i]
		if e.OK || e.ErrKind != provider.RateLimit || e.HTTPStatus != 429 || !strings.Contains(string(e.RawResponse), "rate_limit_exceeded") {
			t.Fatalf("第 %d 条日志应记录限流失败及其响应体: %+v", i+1, e)
		}
	}
	if last := entries[2]; !last.OK || last.Usage != okUsage {
		t.Fatalf("第 3 条日志应记录成功及用量: %+v", last)
	}
}

func Test集成_日志放在重试外层时只记一条汇总(t *testing.T) {
	srv := providertest.NewServer(t, providertest.RateLimited("1"), providertest.Success("你好，这是一条测试回复", okUsage))
	clock := providertest.NewFakeClock(time.Time{})
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })
	policy := provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100)}

	p := provider.Chain(newAdapter(t, srv),
		provider.LoggingMiddleware(sink, clock),
		provider.RetryMiddleware(policy, clock, nil),
	)
	if _, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"}); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if srv.Count() != 2 || len(entries) != 1 {
		t.Fatalf("上游请求数 = %d、日志条数 = %d，期望 2 与 1", srv.Count(), len(entries))
	}
	if e := entries[0]; !e.OK || e.Attempt != 0 || !strings.Contains(string(e.RawResponse), "你好，这是一条测试回复") {
		t.Fatalf("汇总记录应是最终结果，原始报文取最后一次尝试: %+v", e)
	}
}

func Test集成_无Retry_After时按指数退避(t *testing.T) {
	srv := providertest.NewServer(t, providertest.RateLimited(""), providertest.RateLimited(""), providertest.Success("好", okUsage))
	clock := providertest.NewFakeClock(time.Time{})
	p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100)}, clock, nil))

	if _, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"}); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if srv.Count() != 3 {
		t.Fatalf("上游请求数 = %d，期望 3", srv.Count())
	}
	if got := clock.Sleeps(); !sameDurations(got, []time.Duration{ms(100), ms(200)}) {
		t.Fatalf("等待序列 = %v，期望 [100ms 200ms]", got)
	}
}

func Test集成_HTTPdate形式的Retry_After同样被遵守(t *testing.T) {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	clock := providertest.NewFakeClock(base)
	srv := providertest.NewServer(t,
		providertest.RateLimited(base.Add(90*time.Second).Format(http.TimeFormat)),
		providertest.Success("好", okUsage),
	)
	p := provider.Chain(newAdapter(t, srv, provider.WithClock(clock)),
		provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 2, BaseDelay: ms(100)}, clock, nil))

	if _, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"}); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if got := clock.Sleeps(); !sameDurations(got, []time.Duration{90 * time.Second}) {
		t.Fatalf("等待序列 = %v，期望 [1m30s]", got)
	}
	if srv.Count() != 2 {
		t.Fatalf("上游请求数 = %d，期望 2", srv.Count())
	}
}

func Test集成_不可重试的失败恰好1次请求(t *testing.T) {
	cases := []struct {
		name string
		step providertest.Step
		kind provider.ErrKind
	}{
		{"401鉴权失败", providertest.Status(401, "{}"), provider.Auth},
		{"402欠费", providertest.Status(402, "{}"), provider.Billing},
		{"429但错误码为额度耗尽", providertest.Status(429, `{"error":{"code":"insufficient_quota"}}`), provider.Billing},
		{"400请求错误", providertest.Status(400, "{}"), provider.BadRequest},
		{"400内容安全", providertest.Status(400, `{"error":{"code":"content_filter"}}`), provider.ContentFilter},
		{"400上下文超长", providertest.Status(400, `{"error":{"code":"context_length_exceeded"}}`), provider.ContextLen},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 脚本只有 1 步：任何重试都会让 mock 报“脚本耗尽”而使本测试失败。
			srv := providertest.NewServer(t, c.step)
			clock := providertest.NewFakeClock(time.Time{})
			p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)}, clock, nil))

			_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"})
			if kind, _ := provider.KindOf(err); kind != c.kind {
				t.Fatalf("错误种类 = %v，期望 %v（%v）", kind, c.kind, err)
			}
			if srv.Count() != 1 || len(clock.Sleeps()) != 0 {
				t.Fatalf("上游请求数 = %d、等待 %v，期望恰好 1 次且不等待", srv.Count(), clock.Sleeps())
			}
		})
	}
}

func Test集成_过载耗尽重试后返回最后的错误(t *testing.T) {
	srv := providertest.NewServer(t, providertest.Status(503, "{}"), providertest.Status(503, "{}"), providertest.Status(503, "{}"))
	clock := providertest.NewFakeClock(time.Time{})
	p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), MaxDelay: time.Second}, clock, nil))

	_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"})
	pe := asProviderError(t, err)
	if pe.Kind != provider.Overloaded || pe.HTTPStatus != 503 {
		t.Fatalf("得到 %+v，期望 Overloaded/503", pe)
	}
	if srv.Count() != 3 {
		t.Fatalf("上游请求数 = %d，期望恰好 3", srv.Count())
	}
	if got := clock.Sleeps(); !sameDurations(got, []time.Duration{ms(100), ms(200)}) {
		t.Fatalf("等待序列 = %v，期望 [100ms 200ms]", got)
	}
}

func Test集成_Malformed至多修复重试一次(t *testing.T) {
	policy := provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)}

	t.Run("修复重试后成功", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Malformed(providertest.EmptyContent), providertest.Success("好", okUsage))
		clock := providertest.NewFakeClock(time.Time{})
		p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(policy, clock, nil))
		if resp, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"}); err != nil || resp.Text != "好" {
			t.Fatalf("得到 %+v, %v", resp, err)
		}
		if srv.Count() != 2 || len(clock.Sleeps()) != 0 {
			t.Fatalf("上游请求数 = %d、等待 %v，期望 2 次且 Malformed 的修复重试不退避", srv.Count(), clock.Sleeps())
		}
	})

	t.Run("连续畸形则只重试一次就放弃", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Malformed(providertest.EmptyChoices), providertest.Malformed(providertest.EmptyChoices))
		p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(policy, providertest.NewFakeClock(time.Time{}), nil))
		_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"})
		if pe := asProviderError(t, err); pe.Kind != provider.Malformed || pe.Code != "empty_choices" {
			t.Fatalf("得到 %+v，期望 Malformed/empty_choices", pe)
		}
		if srv.Count() != 2 {
			t.Fatalf("上游请求数 = %d，期望恰好 2（第一次 + 1 次修复重试）", srv.Count())
		}
	})

	t.Run("连接中途断开按Malformed处理并重试一次", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.Abort(), providertest.Success("好", okUsage))
		p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(policy, providertest.NewFakeClock(time.Time{}), nil))
		if resp, err := p.Generate(context.Background(), provider.Request{Prompt: "你好"}); err != nil || resp.Text != "好" {
			t.Fatalf("得到 %+v, %v", resp, err)
		}
		if srv.Count() != 2 {
			t.Fatalf("上游请求数 = %d，期望 2", srv.Count())
		}
	})
}

func Test集成_单次尝试超时后重试(t *testing.T) {
	srv := providertest.NewServer(t, providertest.Hang(), providertest.Hang(), providertest.Hang())
	clock := providertest.NewFakeClock(time.Time{})
	p := provider.Chain(newAdapter(t, srv), provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100)}, clock, nil))

	_, err := p.Generate(context.Background(), provider.Request{Prompt: "你好", AttemptTimeout: 300 * time.Millisecond})
	if pe := asProviderError(t, err); pe.Kind != provider.Timeout || pe.Code != "attempt_timeout" {
		t.Fatalf("得到 %+v，期望 Timeout/attempt_timeout", pe)
	}
	if srv.Count() != 3 {
		t.Fatalf("上游请求数 = %d，期望恰好 3", srv.Count())
	}
	if got := clock.Sleeps(); !sameDurations(got, []time.Duration{ms(100), ms(200)}) {
		t.Fatalf("等待序列 = %v，期望 [100ms 200ms]", got)
	}
}

func Test集成_预算放在重试外层按逻辑调用计数(t *testing.T) {
	srv := providertest.NewServer(t, providertest.Status(503, "{}"), providertest.Status(503, "{}"), providertest.Success("好", okUsage))
	budget := &provider.Budget{MaxCalls: 1}
	p := provider.Chain(newAdapter(t, srv),
		provider.BudgetMiddleware(),
		provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 3}, providertest.NewFakeClock(time.Time{}), nil),
	)
	ctx := provider.WithBudget(context.Background(), budget)
	if _, err := p.Generate(ctx, provider.Request{Prompt: "你好"}); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if srv.Count() != 3 || budget.Calls() != 1 {
		t.Fatalf("上游请求数 = %d、预占调用数 = %d，期望 3 与 1（外层预算只数逻辑调用）", srv.Count(), budget.Calls())
	}
}

func Test集成_预算放在重试内层按上游请求计数(t *testing.T) {
	// 脚本只有 2 步：预算用尽后，第 3 次尝试被预算拒绝，不会到达上游。
	srv := providertest.NewServer(t, providertest.Status(503, "{}"), providertest.Status(503, "{}"))
	budget := &provider.Budget{MaxCalls: 2}
	p := provider.Chain(newAdapter(t, srv),
		provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5}, providertest.NewFakeClock(time.Time{}), nil),
		provider.BudgetMiddleware(),
	)
	ctx := provider.WithBudget(context.Background(), budget)
	_, err := p.Generate(ctx, provider.Request{Prompt: "你好"})
	if pe := asProviderError(t, err); pe.Kind != provider.BudgetExceeded {
		t.Fatalf("得到 %+v，期望 BudgetExceeded（预算不可重试，重试层应立即停止）", pe)
	}
	if srv.Count() != 2 || budget.Calls() != 2 {
		t.Fatalf("上游请求数 = %d、预占调用数 = %d，期望都是 2", srv.Count(), budget.Calls())
	}
}

func Test集成_重试耗尽后回退且熔断打开后上游0次请求(t *testing.T) {
	// A 一直过载；B 一直成功。A 的脚本恰好 4 步：第 4 个失败让熔断打开，此后 A 不得再收到请求。
	srvA := providertest.NewServer(t,
		providertest.Status(503, "{}"), providertest.Status(503, "{}"),
		providertest.Status(503, "{}"), providertest.Status(503, "{}"),
	)
	srvB := providertest.NewServer(t,
		providertest.Success("来自B", okUsage), providertest.Success("来自B", okUsage), providertest.Success("来自B", okUsage),
	)
	clock := providertest.NewFakeClock(time.Time{})
	breakers := provider.NewBreaker(breakerConfig(nil), clock)
	policy := provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), MaxDelay: time.Second}

	caps := provider.CapJSONObject | provider.CapJSONSchema
	routeA := provider.Route{
		Name:      "a",
		Compliant: true,
		Caps:      caps,
		Provider:  provider.Chain(newAdapter(t, srvA), provider.RetryMiddleware(policy, clock, nil), breakers.Middleware("a")),
	}
	routeB := provider.Route{
		Name:      "b",
		Compliant: true,
		Caps:      caps,
		Provider:  provider.Chain(newAdapter(t, srvB), breakers.Middleware("b")),
	}
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })
	p := provider.Chain(provider.NewRouter(routeA, routeB),
		provider.LoggingMiddleware(sink, clock),
		provider.BudgetMiddleware(),
	)
	budget := &provider.Budget{MaxCalls: 10, MaxTokens: 1000}
	ctx := provider.WithBudget(context.Background(), budget)
	req := provider.Request{TaskID: "task-1", Prompt: "你好", Format: provider.FormatJSONObject}

	call := func(n int) {
		t.Helper()
		resp, err := p.Generate(ctx, req)
		if err != nil || resp.Text != "来自B" {
			t.Fatalf("第 %d 次调用得到 %+v, %v，期望回退到 B 并成功", n, resp, err)
		}
	}

	// 第 1 次：A 重试到上限（3 次请求）才回退，B 收到 1 次。
	call(1)
	if srvA.Count() != 3 || srvB.Count() != 1 {
		t.Fatalf("第 1 次后 A=%d B=%d，期望 3 与 1", srvA.Count(), srvB.Count())
	}
	if breakers.State("a", "") != provider.BreakerClosed {
		t.Fatalf("A 只失败了 3 次，不足 4 个样本，熔断器应仍为 closed")
	}

	// 第 2 次：A 再收到 1 次请求，这是第 4 个失败，熔断打开；重试的第 2 次尝试被熔断拒绝，回退到 B。
	call(2)
	if srvA.Count() != 4 || srvB.Count() != 2 {
		t.Fatalf("第 2 次后 A=%d B=%d，期望 4 与 2", srvA.Count(), srvB.Count())
	}
	if breakers.State("a", "") != provider.BreakerOpen {
		t.Fatal("A 的熔断器应已打开")
	}

	// 第 3 次：A 已熔断，直接回退，A 收到 0 个请求。
	call(3)
	if srvA.Count() != 4 || srvB.Count() != 3 {
		t.Fatalf("第 3 次后 A=%d B=%d，期望 4 与 3（熔断打开时上游 0 次）", srvA.Count(), srvB.Count())
	}

	// 等待序列：第 1 次 [100ms 200ms]；第 2 次只有第 1 次尝试失败后的 100ms，第 2 次尝试已被熔断拒绝。
	if got := clock.Sleeps(); !sameDurations(got, []time.Duration{ms(100), ms(200), ms(100)}) {
		t.Fatalf("等待序列 = %v，期望 [100ms 200ms 100ms]", got)
	}
	if budget.Calls() != 3 || budget.Tokens() != 3*okUsage.Total {
		t.Fatalf("预算 calls=%d tokens=%d，期望 3 与 %d", budget.Calls(), budget.Tokens(), 3*okUsage.Total)
	}
	if len(entries) != 3 {
		t.Fatalf("外层日志条数 = %d，期望每次调用一条汇总共 3 条", len(entries))
	}
	for i, e := range entries {
		if !e.OK || e.TaskID != "task-1" || e.Usage != okUsage {
			t.Fatalf("第 %d 条汇总日志不对: %+v", i+1, e)
		}
	}
}
