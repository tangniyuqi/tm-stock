package provider

import (
	"context"
	"time"
)

// Middleware 包装一个 Provider，返回带额外行为的 Provider。
type Middleware func(Provider) Provider

// Chain 把中间件按顺序套在 p 上，最先写的在最外层：
// Chain(p, A, B, C) 等价于 A(B(C(p)))，请求先经过 A，最后才到 p。nil 中间件会被跳过。
func Chain(p Provider, mws ...Middleware) Provider {
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] != nil {
			p = mws[i](p)
		}
	}
	return p
}

// ProviderFunc 让一个函数满足 Provider 接口，便于写中间件与测试替身。
type ProviderFunc func(ctx context.Context, request Request) (Response, error)

// Generate 调用函数本身。
func (f ProviderFunc) Generate(ctx context.Context, request Request) (Response, error) {
	return f(ctx, request)
}

// Clock 是可注入的时钟。重试退避、熔断窗口、日志耗时都通过它取时间与等待，
// 测试里换成 providertest.FakeClock 即可做到确定、不真睡。
type Clock interface {
	// Now 返回当前时间。
	Now() time.Time
	// Sleep 等待 d；ctx 提前结束时立即返回 ctx.Err()。
	Sleep(ctx context.Context, d time.Duration) error
}

// RealClock 是基于系统时间的 Clock，零值可直接使用。
type RealClock struct{}

// Now 返回系统当前时间。
func (RealClock) Now() time.Time { return time.Now() }

// Sleep 等待 d 或直到 ctx 结束；d 不为正时只检查 ctx。
func (RealClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// clockOrReal 在 c 为 nil 时返回 RealClock。
func clockOrReal(c Clock) Clock {
	if c == nil {
		return RealClock{}
	}
	return c
}
