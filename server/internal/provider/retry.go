package provider

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// RetryPolicy 是 Retry 中间件的策略。
type RetryPolicy struct {
	// MaxAttempts 是总尝试次数（含第一次），小于 1 按 1 处理，即不重试。
	MaxAttempts int
	// BaseDelay 是第一次退避的基准时长，之后每次加倍；不为正表示不退避（仍遵守 Retry-After）。
	BaseDelay time.Duration
	// MaxDelay 是退避时长（含抖动后）的上限；不为正表示不设上限。它不约束 Retry-After。
	MaxDelay time.Duration
	// Jitter 是抖动幅度，取值 0 到 1：实际退避在 [1-Jitter, 1+Jitter] 倍之间浮动。
	Jitter float64
	// MalformedRetries 是 Malformed 的最多重试次数（不含第一次）。0 取默认值 1，负数表示不重试。
	// 它还受 MaxAttempts 约束。Malformed 的重试不退避：上游并不过载，只是这次的输出不可用。
	MalformedRetries int
}

// DefaultRetryPolicy 返回生产起点：最多 3 次尝试，退避 0.5 秒起步、封顶 8 秒，抖动 20%，
// Malformed 至多重试 1 次。具体数值须结合各厂商的限流策略调整。
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:      3,
		BaseDelay:        500 * time.Millisecond,
		MaxDelay:         8 * time.Second,
		Jitter:           0.2,
		MalformedRetries: 1,
	}
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}
	p.BaseDelay = max(p.BaseDelay, 0)
	p.MaxDelay = max(p.MaxDelay, 0)
	if !(p.Jitter > 0) { // 同时挡住 NaN
		p.Jitter = 0
	}
	p.Jitter = min(p.Jitter, 1)
	switch {
	case p.MalformedRetries == 0:
		p.MalformedRetries = 1
	case p.MalformedRetries < 0:
		p.MalformedRetries = 0
	}
	return p
}

// retryableKind 说明哪些失败值得再试一次。Auth、Billing、BadRequest、ContentFilter、ContextLen、
// Canceled、BudgetExceeded、CircuitOpen 与未分类错误都不重试：重试只会重复同一个结果，或者徒增成本。
func retryableKind(k ErrKind) bool {
	switch k {
	case RateLimit, Overloaded, Timeout, Malformed:
		return true
	}
	return false
}

// RetryMiddleware 返回重试中间件。clock 与 rnd 可注入：clock 为 nil 用 RealClock，
// rnd（返回 [0,1) 的随机数）为 nil 用 math/rand/v2。
//
// 规则：
//   - RateLimit、Overloaded、Timeout：指数退避加抖动，等待时长取 max(退避, Retry-After)；
//     若 ctx 的剩余时间不够等这么久，立即返回当前错误（保留其 RetryAfter，便于上层回排队），不空等；
//   - Malformed：至多重试 MalformedRetries 次，不退避；
//   - 其余种类与未分类错误：不重试；
//   - 每次尝试前把 Request.Attempt 加一（作用在请求副本上），保证下层日志能区分尝试；
//   - 重试耗尽后返回最后一次的错误。
func RetryMiddleware(policy RetryPolicy, clock Clock, rnd func() float64) Middleware {
	policy = policy.normalized()
	clk := clockOrReal(clock)
	if rnd == nil {
		rnd = rand.Float64
	}
	return func(next Provider) Provider {
		return &retrier{next: next, policy: policy, clock: clk, rnd: rnd}
	}
}

type retrier struct {
	next   Provider
	policy RetryPolicy
	clock  Clock
	rnd    func() float64
}

func (r *retrier) Generate(ctx context.Context, req Request) (Response, error) {
	var backoffs, malformed int // 已发生的退避型失败数、Malformed 失败数
	for attempt := 1; ; attempt++ {
		req.Attempt++ // req 是副本，不会改动调用方的值
		resp, err := r.next.Generate(ctx, req)
		if err == nil {
			return resp, nil
		}
		pe := asError(err)
		if pe == nil || !retryableKind(pe.Kind) {
			return Response{}, err
		}
		if attempt >= r.policy.MaxAttempts || ctx.Err() != nil {
			return Response{}, err
		}

		var wait time.Duration
		if pe.Kind == Malformed {
			malformed++
			if malformed > r.policy.MalformedRetries {
				return Response{}, err
			}
		} else {
			backoffs++
			wait = max(r.backoff(backoffs), pe.RetryAfter)
		}
		if wait <= 0 {
			continue
		}
		if deadline, ok := ctx.Deadline(); ok && r.clock.Now().Add(wait).After(deadline) {
			return Response{}, err // 时限不够等，立即返回当前错误
		}
		if serr := r.clock.Sleep(ctx, wait); serr != nil {
			return Response{}, ctxToError(serr)
		}
	}
}

// backoff 计算第 k 次（从 1 起）退避型失败后的等待时长：BaseDelay 逐次加倍（指数增长），封顶 MaxDelay，
// 再按 Jitter 在 [1-Jitter, 1+Jitter] 倍之间浮动，浮动后仍不超过 MaxDelay。
func (r *retrier) backoff(k int) time.Duration {
	p := r.policy
	if p.BaseDelay <= 0 {
		return 0
	}
	d := p.BaseDelay
	for i := 1; i < k; i++ {
		if p.MaxDelay > 0 && d >= p.MaxDelay {
			break
		}
		if d > math.MaxInt64/2 {
			d = math.MaxInt64
			break
		}
		d *= 2
	}
	if p.MaxDelay > 0 && d > p.MaxDelay {
		d = p.MaxDelay
	}
	if p.Jitter > 0 {
		x := min(max(r.rnd(), 0), 1)
		d = time.Duration(float64(d) * (1 + p.Jitter*(2*x-1)))
		if p.MaxDelay > 0 && d > p.MaxDelay {
			d = p.MaxDelay
		}
	}
	return max(d, 0)
}
