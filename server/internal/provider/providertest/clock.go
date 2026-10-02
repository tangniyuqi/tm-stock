package providertest

import (
	"context"
	"sync"
	"time"
)

// FakeClock 是可手动推进的假时钟，并发安全，满足 provider.Clock。
//
// Sleep 不会真的等待：它记录被请求的时长、把虚拟时间向前推进同样的时长，然后立即返回。
// 这样重试退避的整个时长序列都能被确定性地断言，测试也不会真睡。
// 熔断窗口等需要“时间流逝”的场景用 Advance 手动推进。
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

// NewFakeClock 创建假时钟。start 为零值时取 2026-01-01 00:00:00 UTC。
// 需要与带 deadline 的 context 搭配时，传入 time.Now()，两者才处在同一时间基准上。
func NewFakeClock(start time.Time) *FakeClock {
	if start.IsZero() {
		start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return &FakeClock{now: start}
}

// Now 返回当前虚拟时间。
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance 把虚拟时间向前推进 d；d 不为正时不动。
func (c *FakeClock) Advance(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// Sleep 记录 d 并推进虚拟时间后立即返回。ctx 已结束时返回 ctx.Err()，不记录也不推进。
func (c *FakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	if d > 0 {
		c.now = c.now.Add(d)
	}
	c.mu.Unlock()
	return nil
}

// Sleeps 返回迄今为止所有 Sleep 请求的时长（副本，按调用顺序）。
func (c *FakeClock) Sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]time.Duration, len(c.sleeps))
	copy(out, c.sleeps)
	return out
}
