package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

// fixedRand 返回固定值的随机源，让抖动可预测。
func fixedRand(v float64) func() float64 {
	return func() float64 { return v }
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func sameDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Test重试_各种错误的尝试次数(t *testing.T) {
	policy := provider.RetryPolicy{MaxAttempts: 4, BaseDelay: ms(100), MaxDelay: time.Second}
	cases := []struct {
		kind provider.ErrKind
		want int
	}{
		{provider.Unknown, 1},
		{provider.Auth, 1},
		{provider.Billing, 1},
		{provider.RateLimit, 4},
		{provider.Overloaded, 4},
		{provider.Timeout, 4},
		{provider.BadRequest, 1},
		{provider.ContextLen, 1},
		{provider.ContentFilter, 1},
		{provider.Malformed, 2}, // 第一次 + 默认的 1 次修复重试
		{provider.Canceled, 1},
		{provider.BudgetExceeded, 1},
		{provider.CircuitOpen, 1},
	}
	if len(cases) != len(allKinds) {
		t.Fatalf("用例覆盖了 %d 种错误，而全部种类有 %d 种", len(cases), len(allKinds))
	}
	for _, c := range cases {
		t.Run(c.kind.String(), func(t *testing.T) {
			inner := &scripted{results: []scriptedResult{failResult(c.kind)}, repeat: true}
			clock := providertest.NewFakeClock(time.Time{})
			p := provider.Chain(inner, provider.RetryMiddleware(policy, clock, fixedRand(0.5)))
			_, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
			if err == nil {
				t.Fatal("期望失败")
			}
			if kind, ok := provider.KindOf(err); !ok || kind != c.kind {
				t.Fatalf("返回的错误种类 = %v, %v，期望原样返回最后一次的 %v", kind, ok, c.kind)
			}
			if inner.Calls() != c.want {
				t.Fatalf("下层被调用 %d 次，期望 %d 次", inner.Calls(), c.want)
			}
		})
	}

	t.Run("未分类的裸错误不重试", func(t *testing.T) {
		inner := &scripted{results: []scriptedResult{{err: errors.New("裸错误")}}, repeat: true}
		p := provider.Chain(inner, provider.RetryMiddleware(policy, providertest.NewFakeClock(time.Time{}), nil))
		if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err == nil {
			t.Fatal("期望失败")
		}
		if inner.Calls() != 1 {
			t.Fatalf("下层被调用 %d 次，期望 1 次", inner.Calls())
		}
	})

	t.Run("底层直接返回的context错误不重试", func(t *testing.T) {
		inner := &scripted{results: []scriptedResult{{err: context.DeadlineExceeded}}, repeat: true}
		p := provider.Chain(inner, provider.RetryMiddleware(policy, providertest.NewFakeClock(time.Time{}), nil))
		if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("得到 %v，期望原样返回 DeadlineExceeded", err)
		}
		if inner.Calls() != 1 {
			t.Fatalf("下层被调用 %d 次，期望 1 次", inner.Calls())
		}
	})
}

func Test重试_退避时长序列(t *testing.T) {
	cases := []struct {
		name     string
		policy   provider.RetryPolicy
		rnd      func() float64
		results  []scriptedResult
		repeat   bool
		calls    int
		sleeps   []time.Duration
		wantOK   bool
		wantKind provider.ErrKind
	}{
		{
			name:     "指数退避并封顶MaxDelay",
			policy:   provider.RetryPolicy{MaxAttempts: 7, BaseDelay: ms(100), MaxDelay: time.Second},
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    7,
			sleeps:   []time.Duration{ms(100), ms(200), ms(400), ms(800), ms(1000), ms(1000)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "未设MaxDelay时不封顶",
			policy:   provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)},
			results:  []scriptedResult{failResult(provider.Timeout)},
			repeat:   true,
			calls:    5,
			sleeps:   []time.Duration{ms(100), ms(200), ms(400), ms(800)},
			wantKind: provider.Timeout,
		},
		{
			// 第 1 次退避 100ms 小于 Retry-After 的 2s，取 2s；
			// 第 2 次退避 200ms 大于 Retry-After 的 50ms，取 200ms；
			// 第 3 次没有 Retry-After，取退避 400ms。
			name:   "Retry-After取与退避中较大的一个",
			policy: provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100), MaxDelay: time.Second},
			results: []scriptedResult{
				failRetryAfter(provider.RateLimit, 2*time.Second),
				failRetryAfter(provider.RateLimit, ms(50)),
				failResult(provider.RateLimit),
				okResult("好"),
			},
			calls:  4,
			sleeps: []time.Duration{2 * time.Second, ms(200), ms(400)},
			wantOK: true,
		},
		{
			name:     "Retry-After不受MaxDelay约束",
			policy:   provider.RetryPolicy{MaxAttempts: 2, BaseDelay: ms(100), MaxDelay: ms(500)},
			results:  []scriptedResult{failRetryAfter(provider.Overloaded, 30*time.Second), okResult("好")},
			calls:    2,
			sleeps:   []time.Duration{30 * time.Second},
			wantOK:   true,
			wantKind: provider.Unknown,
		},
		{
			name:    "Malformed的修复重试不退避也不推进指数",
			policy:  provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100), MaxDelay: time.Second},
			results: []scriptedResult{failResult(provider.Overloaded), failResult(provider.Malformed), failResult(provider.Overloaded), okResult("好")},
			calls:   4,
			sleeps:  []time.Duration{ms(100), ms(200)},
			wantOK:  true,
		},
		{
			name:     "抖动下限：随机源为0时取(1-Jitter)倍",
			policy:   provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), Jitter: 0.5},
			rnd:      fixedRand(0),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    3,
			sleeps:   []time.Duration{ms(50), ms(100)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "抖动中点：随机源为0.5时恰为基准",
			policy:   provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), Jitter: 0.5},
			rnd:      fixedRand(0.5),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    3,
			sleeps:   []time.Duration{ms(100), ms(200)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "抖动上限：随机源为1时取(1+Jitter)倍",
			policy:   provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), Jitter: 0.5},
			rnd:      fixedRand(1),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    3,
			sleeps:   []time.Duration{ms(150), ms(300)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "随机源越界时被夹到[0,1]",
			policy:   provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), Jitter: 0.5},
			rnd:      fixedRand(7),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    3,
			sleeps:   []time.Duration{ms(150), ms(300)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "抖动之后仍不超过MaxDelay",
			policy:   provider.RetryPolicy{MaxAttempts: 2, BaseDelay: ms(100), MaxDelay: ms(120), Jitter: 1},
			rnd:      fixedRand(1),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    2,
			sleeps:   []time.Duration{ms(120)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "Jitter大于1按1处理",
			policy:   provider.RetryPolicy{MaxAttempts: 2, BaseDelay: ms(100), Jitter: 5},
			rnd:      fixedRand(1),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    2,
			sleeps:   []time.Duration{ms(200)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "Jitter为负或NaN按0处理",
			policy:   provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(100), Jitter: -1},
			rnd:      fixedRand(0),
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    3,
			sleeps:   []time.Duration{ms(100), ms(200)},
			wantKind: provider.Overloaded,
		},
		{
			name:     "BaseDelay为0时不退避",
			policy:   provider.RetryPolicy{MaxAttempts: 3},
			results:  []scriptedResult{failResult(provider.Overloaded)},
			repeat:   true,
			calls:    3,
			sleeps:   nil,
			wantKind: provider.Overloaded,
		},
		{
			name:     "BaseDelay为负按0处理但仍遵守Retry-After",
			policy:   provider.RetryPolicy{MaxAttempts: 2, BaseDelay: -ms(100)},
			results:  []scriptedResult{failRetryAfter(provider.RateLimit, 3*time.Second), okResult("好")},
			calls:    2,
			sleeps:   []time.Duration{3 * time.Second},
			wantOK:   true,
			wantKind: provider.Unknown,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{results: c.results, repeat: c.repeat}
			clock := providertest.NewFakeClock(time.Time{})
			p := provider.Chain(inner, provider.RetryMiddleware(c.policy, clock, c.rnd))
			resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
			if c.wantOK {
				if err != nil || resp.Text != "好" {
					t.Fatalf("期望最终成功并返回成功那次的响应，得到 %+v, %v", resp, err)
				}
			} else {
				kind, ok := provider.KindOf(err)
				if !ok || kind != c.wantKind {
					t.Fatalf("错误种类 = %v, %v，期望 %v", kind, ok, c.wantKind)
				}
			}
			if inner.Calls() != c.calls {
				t.Fatalf("下层被调用 %d 次，期望 %d 次", inner.Calls(), c.calls)
			}
			if got := clock.Sleeps(); !sameDurations(got, c.sleeps) {
				t.Fatalf("退避时长序列 = %v，期望 %v", got, c.sleeps)
			}
		})
	}
}

func Test重试_MaxAttempts与Malformed次数的组合(t *testing.T) {
	cases := []struct {
		name      string
		policy    provider.RetryPolicy
		kind      provider.ErrKind
		wantCalls int
	}{
		{"MaxAttempts为0按1次处理", provider.RetryPolicy{MaxAttempts: 0, BaseDelay: ms(1)}, provider.Overloaded, 1},
		{"MaxAttempts为负按1次处理", provider.RetryPolicy{MaxAttempts: -3, BaseDelay: ms(1)}, provider.Overloaded, 1},
		{"MaxAttempts为1即不重试", provider.RetryPolicy{MaxAttempts: 1, BaseDelay: ms(1)}, provider.RateLimit, 1},
		{"Malformed默认重试1次", provider.RetryPolicy{MaxAttempts: 5}, provider.Malformed, 2},
		{"Malformed重试2次", provider.RetryPolicy{MaxAttempts: 5, MalformedRetries: 2}, provider.Malformed, 3},
		{"Malformed为负表示不重试", provider.RetryPolicy{MaxAttempts: 5, MalformedRetries: -1}, provider.Malformed, 1},
		{"Malformed次数受MaxAttempts约束", provider.RetryPolicy{MaxAttempts: 3, MalformedRetries: 9}, provider.Malformed, 3},
		{"MaxAttempts为1时Malformed也不重试", provider.RetryPolicy{MaxAttempts: 1, MalformedRetries: 3}, provider.Malformed, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{results: []scriptedResult{failResult(c.kind)}, repeat: true}
			p := provider.Chain(inner, provider.RetryMiddleware(c.policy, providertest.NewFakeClock(time.Time{}), nil))
			if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err == nil {
				t.Fatal("期望失败")
			}
			if inner.Calls() != c.wantCalls {
				t.Fatalf("下层被调用 %d 次，期望 %d 次", inner.Calls(), c.wantCalls)
			}
		})
	}
}

func Test重试_每次尝试递增Attempt且不改调用方的请求(t *testing.T) {
	cases := []struct {
		name  string
		start int
		want  []int
	}{
		{"从0开始", 0, []int{1, 2, 3}},
		{"沿用调用方已有的次数", 5, []int{6, 7, 8}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{results: []scriptedResult{failResult(provider.Overloaded), failResult(provider.Overloaded), okResult("好")}}
			p := provider.Chain(inner, provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 3}, providertest.NewFakeClock(time.Time{}), nil))
			req := provider.Request{TaskID: "task-1", Step: 4, Attempt: c.start, Prompt: "x"}
			if _, err := p.Generate(context.Background(), req); err != nil {
				t.Fatalf("Generate 失败: %v", err)
			}
			var got []int
			for _, r := range inner.Requests() {
				got = append(got, r.Attempt)
				if r.TaskID != "task-1" || r.Step != 4 {
					t.Fatalf("追溯键被改动: %+v", r)
				}
			}
			if len(got) != len(c.want) {
				t.Fatalf("下层看到的 Attempt = %v，期望 %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("下层看到的 Attempt = %v，期望 %v", got, c.want)
				}
			}
			if req.Attempt != c.start {
				t.Fatalf("调用方的 Request.Attempt 被改成了 %d", req.Attempt)
			}
		})
	}
}

func Test重试_遵守ctx时限(t *testing.T) {
	t.Run("剩余时间不够等Retry-After时立即返回当前错误", func(t *testing.T) {
		clock := providertest.NewFakeClock(time.Now())
		ctx, cancel := context.WithDeadline(context.Background(), clock.Now().Add(5*time.Second))
		defer cancel()
		inner := &scripted{results: []scriptedResult{failRetryAfter(provider.RateLimit, 10*time.Second)}, repeat: true}
		p := provider.Chain(inner, provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)}, clock, nil))

		_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
		pe := asProviderError(t, err)
		if pe.Kind != provider.RateLimit || pe.RetryAfter != 10*time.Second {
			t.Fatalf("应原样返回当前错误（保留 RetryAfter 便于上层回排队），得到 %+v", pe)
		}
		if inner.Calls() != 1 {
			t.Fatalf("下层被调用 %d 次，期望 1 次（不空等）", inner.Calls())
		}
		if got := clock.Sleeps(); len(got) != 0 {
			t.Fatalf("不应有任何等待，实际 %v", got)
		}
	})

	t.Run("剩余时间够用时照常等待并重试", func(t *testing.T) {
		clock := providertest.NewFakeClock(time.Now())
		ctx, cancel := context.WithDeadline(context.Background(), clock.Now().Add(5*time.Second))
		defer cancel()
		inner := &scripted{results: []scriptedResult{failRetryAfter(provider.RateLimit, 2*time.Second), okResult("好")}}
		p := provider.Chain(inner, provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)}, clock, nil))

		if resp, err := p.Generate(ctx, provider.Request{Prompt: "x"}); err != nil || resp.Text != "好" {
			t.Fatalf("得到 %+v, %v", resp, err)
		}
		if got := clock.Sleeps(); !sameDurations(got, []time.Duration{2 * time.Second}) {
			t.Fatalf("等待序列 = %v，期望 [2s]", got)
		}
	})

	t.Run("退避本身超出剩余时间也立即返回", func(t *testing.T) {
		clock := providertest.NewFakeClock(time.Now())
		ctx, cancel := context.WithDeadline(context.Background(), clock.Now().Add(50*time.Millisecond))
		defer cancel()
		inner := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}, repeat: true}
		p := provider.Chain(inner, provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5, BaseDelay: time.Second}, clock, nil))

		_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
		if kind, _ := provider.KindOf(err); kind != provider.Overloaded {
			t.Fatalf("错误种类 = %v，期望 Overloaded", kind)
		}
		if inner.Calls() != 1 || len(clock.Sleeps()) != 0 {
			t.Fatalf("调用 %d 次、等待 %v，期望 1 次且不等待", inner.Calls(), clock.Sleeps())
		}
	})

	t.Run("两次尝试之间ctx已被取消则不再重试", func(t *testing.T) {
		clock := providertest.NewFakeClock(time.Time{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		inner := &scripted{
			results: []scriptedResult{failResult(provider.Overloaded)},
			repeat:  true,
			hook:    func(ctx context.Context, n int) { cancel() },
		}
		p := provider.Chain(inner, provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)}, clock, nil))

		_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
		if kind, _ := provider.KindOf(err); kind != provider.Overloaded {
			t.Fatalf("错误种类 = %v，期望返回取消前的上游错误 Overloaded", kind)
		}
		if inner.Calls() != 1 || len(clock.Sleeps()) != 0 {
			t.Fatalf("调用 %d 次、等待 %v，期望 1 次且不等待", inner.Calls(), clock.Sleeps())
		}
	})
}

// cancelOnSleepClock 在 Sleep 里取消 ctx 并返回取消原因，模拟“退避等待期间调用方取消”。
type cancelOnSleepClock struct {
	cancel context.CancelFunc
}

func (c cancelOnSleepClock) Now() time.Time { return time.Now() }

func (c cancelOnSleepClock) Sleep(ctx context.Context, d time.Duration) error {
	c.cancel()
	return ctx.Err()
}

func Test重试_退避等待期间被取消(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inner := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}, repeat: true}
	p := provider.Chain(inner, provider.RetryMiddleware(provider.RetryPolicy{MaxAttempts: 5, BaseDelay: ms(100)}, cancelOnSleepClock{cancel: cancel}, nil))

	_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
	pe := asProviderError(t, err)
	if pe.Kind != provider.Canceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("得到 %+v，期望 Canceled 且可 errors.Is(context.Canceled)", pe)
	}
	if inner.Calls() != 1 {
		t.Fatalf("下层被调用 %d 次，期望 1 次", inner.Calls())
	}
}

func Test重试_默认时钟与随机源可用(t *testing.T) {
	inner := &scripted{results: []scriptedResult{failResult(provider.Overloaded), okResult("好")}}
	policy := provider.RetryPolicy{MaxAttempts: 2, BaseDelay: ms(2), Jitter: 0.5}
	p := provider.Chain(inner, provider.RetryMiddleware(policy, nil, nil))
	if resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err != nil || resp.Text != "好" {
		t.Fatalf("得到 %+v, %v", resp, err)
	}
	if inner.Calls() != 2 {
		t.Fatalf("下层被调用 %d 次，期望 2 次", inner.Calls())
	}
}

func Test重试_默认策略的参数(t *testing.T) {
	got := provider.DefaultRetryPolicy()
	want := provider.RetryPolicy{MaxAttempts: 3, BaseDelay: ms(500), MaxDelay: 8 * time.Second, Jitter: 0.2, MalformedRetries: 1}
	if got != want {
		t.Fatalf("DefaultRetryPolicy() = %+v，期望 %+v", got, want)
	}
}
