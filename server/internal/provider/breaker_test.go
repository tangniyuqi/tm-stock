package provider_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

// changeLog 收集熔断器的状态变化回调，并发安全。
type changeLog struct {
	mu    sync.Mutex
	items []provider.StateChange
}

func (l *changeLog) add(c provider.StateChange) {
	l.mu.Lock()
	l.items = append(l.items, c)
	l.mu.Unlock()
}

func (l *changeLog) all() []provider.StateChange {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]provider.StateChange, len(l.items))
	copy(out, l.items)
	return out
}

// breakerConfig 是熔断测试的统一参数：10 秒窗口，至少 4 个样本，失败率 50%，打开 30 秒，半开同时 1 个探测。
func breakerConfig(onChange func(provider.StateChange)) provider.BreakerConfig {
	return provider.BreakerConfig{
		Window:        10 * time.Second,
		MinRequests:   4,
		FailureRatio:  0.5,
		OpenFor:       30 * time.Second,
		HalfOpenMax:   1,
		OnStateChange: onChange,
	}
}

// breakerCase 是一套熔断器测试夹具：假时钟、状态变化记录、被包装的 Provider。
type breakerCase struct {
	clock *providertest.FakeClock
	log   *changeLog
	b     *provider.Breaker
	inner *scripted
	p     provider.Provider
}

func newBreakerCase(cfg func(onChange func(provider.StateChange)) provider.BreakerConfig, inner *scripted) *breakerCase {
	c := &breakerCase{
		clock: providertest.NewFakeClock(time.Time{}),
		log:   &changeLog{},
		inner: inner,
	}
	c.b = provider.NewBreaker(cfg(c.log.add), c.clock)
	c.p = provider.Chain(inner, c.b.Middleware("vendor-a"))
	return c
}

func (c *breakerCase) call(model string) error {
	_, err := c.p.Generate(context.Background(), provider.Request{Model: model, Prompt: "x"})
	return err
}

// failures 返回 n 个同种类的失败结果。
func failures(kind provider.ErrKind, n int) []scriptedResult {
	out := make([]scriptedResult, n)
	for i := range out {
		out[i] = failResult(kind)
	}
	return out
}

func concat(parts ...[]scriptedResult) []scriptedResult {
	var out []scriptedResult
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func wantState(t *testing.T, b *provider.Breaker, vendor, model string, want provider.BreakerState) {
	t.Helper()
	if got := b.State(vendor, model); got != want {
		t.Fatalf("State(%q, %q) = %v，期望 %v", vendor, model, got, want)
	}
}

func wantCircuitOpen(t *testing.T, err error, code string) *provider.Error {
	t.Helper()
	pe := asProviderError(t, err)
	if pe.Kind != provider.CircuitOpen || pe.Code != code {
		t.Fatalf("得到 %+v，期望 CircuitOpen/%s", pe, code)
	}
	return pe
}

func Test熔断_失败率与样本数都达标才打开(t *testing.T) {
	c := newBreakerCase(breakerConfig, &scripted{results: failures(provider.Overloaded, 4)})

	// 前 3 次失败：失败率 100% 但样本数不足 MinRequests，不熔断。
	for i := 0; i < 3; i++ {
		if err := c.call(""); err == nil {
			t.Fatal("应返回上游的失败")
		}
		wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
	}
	// 第 4 次失败：样本数达标且失败率 100%，打开。
	if err := c.call(""); err == nil {
		t.Fatal("应返回上游的失败")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)

	// 打开后的请求直接被拒，且不到达上游。
	pe := wantCircuitOpen(t, c.call(""), "circuit_open")
	if pe.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v，期望距离转半开还剩 30s", pe.RetryAfter)
	}
	if c.inner.Calls() != 4 {
		t.Fatalf("上游被调用 %d 次，期望 4 次（熔断后不再到达上游）", c.inner.Calls())
	}

	changes := c.log.all()
	if len(changes) != 1 {
		t.Fatalf("状态变化 = %+v，期望只有一次 closed→open", changes)
	}
	want := provider.StateChange{Vendor: "vendor-a", Model: "", From: provider.BreakerClosed, To: provider.BreakerOpen, Cause: provider.Overloaded}
	if changes[0] != want {
		t.Fatalf("状态变化 = %+v，期望 %+v", changes[0], want)
	}
}

func Test熔断_成功样本稀释失败率(t *testing.T) {
	results := concat([]scriptedResult{okResult("好"), okResult("好"), okResult("好")}, failures(provider.Timeout, 3))
	c := newBreakerCase(breakerConfig, &scripted{results: results})

	// 3 成功 + 1 失败：25%；再 1 失败：2/5 = 40%；都低于 50%。
	for i := 0; i < 5; i++ {
		_ = c.call("")
		wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
	}
	// 第 6 个样本是失败：3/6 = 50%，达到阈值，打开。
	_ = c.call("")
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
}

func Test熔断_窗口滑出后旧失败不再计入(t *testing.T) {
	c := newBreakerCase(breakerConfig, &scripted{results: failures(provider.Overloaded, 8), repeat: true})
	for i := 0; i < 3; i++ {
		_ = c.call("")
	}
	// 推进到超出 10 秒窗口之后，此时窗口里只剩下新来的这 1 个样本，不足 4 个。
	c.clock.Advance(11 * time.Second)
	_ = c.call("")
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)

	// 之后再连续失败，样本数重新累积到 4 个才会打开。
	for i := 0; i < 2; i++ {
		_ = c.call("")
		wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
	}
	_ = c.call("")
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
}

func Test熔断_完整状态流转(t *testing.T) {
	results := concat(failures(provider.Overloaded, 4), []scriptedResult{okResult("好")}, failures(provider.Overloaded, 3))
	c := newBreakerCase(breakerConfig, &scripted{results: results})

	for i := 0; i < 4; i++ {
		_ = c.call("")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)

	c.clock.Advance(29 * time.Second)
	pe := wantCircuitOpen(t, c.call(""), "circuit_open")
	if pe.RetryAfter != time.Second {
		t.Fatalf("RetryAfter = %v，期望还剩 1s", pe.RetryAfter)
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)

	c.clock.Advance(time.Second) // 满 30 秒，转半开
	wantState(t, c.b, "vendor-a", "", provider.BreakerHalfOpen)

	if err := c.call(""); err != nil { // 探测成功
		t.Fatalf("半开探测应放行并成功: %v", err)
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)

	// 关闭后窗口已清空：再失败 3 次不足 4 个样本，不会立刻重新打开。
	for i := 0; i < 3; i++ {
		_ = c.call("")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)

	want := []provider.StateChange{
		{Vendor: "vendor-a", From: provider.BreakerClosed, To: provider.BreakerOpen, Cause: provider.Overloaded},
		{Vendor: "vendor-a", From: provider.BreakerOpen, To: provider.BreakerHalfOpen},
		{Vendor: "vendor-a", From: provider.BreakerHalfOpen, To: provider.BreakerClosed},
	}
	got := c.log.all()
	if len(got) != len(want) {
		t.Fatalf("状态变化 = %+v，期望 %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 次状态变化 = %+v，期望 %+v", i+1, got[i], want[i])
		}
	}
	// 4 次失败 + 1 次探测 + 3 次关闭后的失败；打开期间被拒的那一次不算。
	if c.inner.Calls() != 8 {
		t.Fatalf("上游被调用 %d 次，期望 8 次", c.inner.Calls())
	}
}

func Test熔断_半开探测失败则重新打开(t *testing.T) {
	results := concat(failures(provider.Overloaded, 4), failures(provider.Timeout, 1), []scriptedResult{okResult("好")})
	c := newBreakerCase(breakerConfig, &scripted{results: results})
	for i := 0; i < 4; i++ {
		_ = c.call("")
	}
	c.clock.Advance(30 * time.Second)
	wantState(t, c.b, "vendor-a", "", provider.BreakerHalfOpen)

	if err := c.call(""); err == nil { // 探测失败
		t.Fatal("探测应返回上游的失败")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)

	// 熔断计时从探测失败那一刻重新开始。
	c.clock.Advance(29 * time.Second)
	wantCircuitOpen(t, c.call(""), "circuit_open")
	if c.inner.Calls() != 5 {
		t.Fatalf("上游被调用 %d 次，期望 5 次（重新打开后不再到达上游）", c.inner.Calls())
	}
	c.clock.Advance(time.Second)
	wantState(t, c.b, "vendor-a", "", provider.BreakerHalfOpen)
	if err := c.call(""); err != nil {
		t.Fatalf("再次探测应成功: %v", err)
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)

	changes := c.log.all()
	reopened := changes[len(changes)-3]
	if reopened.From != provider.BreakerHalfOpen || reopened.To != provider.BreakerOpen || reopened.Cause != provider.Timeout {
		t.Fatalf("探测失败的状态变化 = %+v，期望 half-open→open 且原因为 Timeout", reopened)
	}
}

func Test熔断_半开只放行限额内的探测(t *testing.T) {
	for _, probes := range []int{1, 2} {
		probes := probes
		t.Run(map[int]string{1: "限额1", 2: "限额2"}[probes], func(t *testing.T) {
			// 前 4 次失败打开；其后 probes 个探测在 hook 里被卡住，再放行成功。
			results := concat(failures(provider.Overloaded, 4), []scriptedResult{okResult("好"), okResult("好"), okResult("好")})
			started := make(chan struct{}, 4)
			release := make(chan struct{})
			inner := &scripted{
				results: results,
				hook: func(ctx context.Context, n int) {
					if n > 4 { // 探测请求
						started <- struct{}{}
						<-release
					}
				},
			}
			cfg := func(onChange func(provider.StateChange)) provider.BreakerConfig {
				c := breakerConfig(onChange)
				c.HalfOpenMax = probes
				return c
			}
			c := newBreakerCase(cfg, inner)
			for i := 0; i < 4; i++ {
				_ = c.call("")
			}
			c.clock.Advance(30 * time.Second)

			var wg sync.WaitGroup
			errs := make(chan error, probes)
			for i := 0; i < probes; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					errs <- c.call("")
				}()
			}
			for i := 0; i < probes; i++ {
				<-started // 探测名额用满，且都已到达上游
			}

			// 名额已满：再来的请求被拒，且不到达上游。
			wantCircuitOpen(t, c.call(""), "circuit_probing")
			if inner.Calls() != 4+probes {
				t.Fatalf("上游被调用 %d 次，期望 %d 次", inner.Calls(), 4+probes)
			}

			close(release)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatalf("探测应成功: %v", err)
				}
			}
			wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
		})
	}
}

func Test熔断_Auth与Billing立即熔断(t *testing.T) {
	for _, kind := range []provider.ErrKind{provider.Auth, provider.Billing} {
		t.Run(kind.String(), func(t *testing.T) {
			c := newBreakerCase(breakerConfig, &scripted{results: failures(kind, 1), repeat: true})
			if err := c.call(""); err == nil {
				t.Fatal("应返回上游的失败")
			}
			wantState(t, c.b, "vendor-a", "", provider.BreakerOpen) // 只有 1 个样本，也立即打开
			wantCircuitOpen(t, c.call(""), "circuit_open")
			if c.inner.Calls() != 1 {
				t.Fatalf("上游被调用 %d 次，期望 1 次", c.inner.Calls())
			}
			changes := c.log.all()
			if len(changes) != 1 || changes[0].Cause != kind || changes[0].To != provider.BreakerOpen {
				t.Fatalf("状态变化 = %+v，期望一次由 %v 引起的打开（供告警使用）", changes, kind)
			}

			// 半开探测再遇到同样的失败，重新打开。
			c.clock.Advance(30 * time.Second)
			if err := c.call(""); err == nil {
				t.Fatal("探测应返回上游的失败")
			}
			wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
		})
	}
}

func Test熔断_不计入失败率的种类(t *testing.T) {
	kinds := []provider.ErrKind{
		provider.Unknown, provider.RateLimit, provider.BadRequest, provider.ContextLen,
		provider.ContentFilter, provider.Canceled, provider.BudgetExceeded, provider.CircuitOpen,
	}
	for _, kind := range kinds {
		t.Run(kind.String(), func(t *testing.T) {
			c := newBreakerCase(breakerConfig, &scripted{results: failures(kind, 1), repeat: true})
			for i := 0; i < 12; i++ {
				_ = c.call("")
			}
			wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
			if c.inner.Calls() != 12 {
				t.Fatalf("上游被调用 %d 次，期望 12 次（线路应一直放行）", c.inner.Calls())
			}
		})
	}

	t.Run("非Error的裸错误", func(t *testing.T) {
		c := newBreakerCase(breakerConfig, &scripted{results: []scriptedResult{{err: errors.New("裸错误")}}, repeat: true})
		for i := 0; i < 12; i++ {
			_ = c.call("")
		}
		wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
	})

	t.Run("调用方ctx已结束时的失败不计入", func(t *testing.T) {
		c := newBreakerCase(breakerConfig, &scripted{results: failures(provider.Timeout, 1), repeat: true})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for i := 0; i < 12; i++ {
			_, _ = c.p.Generate(ctx, provider.Request{Prompt: "x"})
		}
		wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
	})
}

func Test熔断_线路按厂商与模型相互独立(t *testing.T) {
	c := newBreakerCase(breakerConfig, &scripted{results: failures(provider.Overloaded, 1), repeat: true})
	other := provider.Chain(&scripted{results: []scriptedResult{okResult("好")}, repeat: true}, c.b.Middleware("vendor-b"))

	for i := 0; i < 4; i++ {
		_ = c.call("m1")
	}
	wantState(t, c.b, "vendor-a", "m1", provider.BreakerOpen)
	wantCircuitOpen(t, c.call("m1"), "circuit_open")

	// 同一厂商的另一个模型、另一个厂商的同名模型都不受影响。
	callsBefore := c.inner.Calls()
	if err := c.call("m2"); err == nil || c.inner.Calls() != callsBefore+1 {
		t.Fatalf("同厂商的 m2 应被放行到上游（得到错误 %v，上游调用 %d→%d）", err, callsBefore, c.inner.Calls())
	}
	if _, err := other.Generate(context.Background(), provider.Request{Model: "m1", Prompt: "x"}); err != nil {
		t.Fatalf("另一个厂商的同名模型应放行: %v", err)
	}
	wantState(t, c.b, "vendor-a", "m2", provider.BreakerClosed)
	wantState(t, c.b, "vendor-b", "m1", provider.BreakerClosed)
	wantState(t, c.b, "vendor-c", "never-seen", provider.BreakerClosed)
}

func Test熔断_线路换过状态后迟到的结果作废(t *testing.T) {
	// 第 1 个请求在关闭状态放行后被卡住；其间线路打开又关闭；它迟到的失败不应再计入新窗口。
	results := concat(failures(provider.Overloaded, 5), []scriptedResult{okResult("好")}, failures(provider.Overloaded, 3))
	started := make(chan struct{})
	release := make(chan struct{})
	inner := &scripted{
		results: results,
		hook: func(ctx context.Context, n int) {
			if n == 1 {
				close(started)
				<-release
			}
		},
	}
	c := newBreakerCase(breakerConfig, inner)

	lateDone := make(chan error, 1)
	go func() { lateDone <- c.call("") }()
	<-started // 第 1 个请求已被放行并卡在上游

	for i := 0; i < 4; i++ { // 第 2 到第 5 个请求失败，线路打开
		_ = c.call("")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
	c.clock.Advance(30 * time.Second)
	if err := c.call(""); err != nil { // 探测成功，线路关闭
		t.Fatalf("探测应成功: %v", err)
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)

	close(release)
	if err := <-lateDone; err == nil {
		t.Fatal("迟到的请求应返回它自己的失败")
	}

	// 若迟到的失败被错误地计入了新窗口，再失败 3 次就会凑够 4 个样本而打开。
	for i := 0; i < 3; i++ {
		_ = c.call("")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
}

func Test熔断_半开时没有结论的探测归还名额(t *testing.T) {
	results := concat(failures(provider.Overloaded, 4), failures(provider.BadRequest, 1), []scriptedResult{okResult("好")})
	c := newBreakerCase(breakerConfig, &scripted{results: results})
	for i := 0; i < 4; i++ {
		_ = c.call("")
	}
	c.clock.Advance(30 * time.Second)

	if err := c.call(""); err == nil { // 探测得到 BadRequest：线路是通的，但不能据此判断恢复
		t.Fatal("探测应返回上游的失败")
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerHalfOpen)
	if err := c.call(""); err != nil { // 名额已归还，下一个请求可以再探
		t.Fatalf("名额归还后应能再次探测: %v", err)
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
}

func Test熔断_下层panic时释放探测名额(t *testing.T) {
	results := concat(failures(provider.Overloaded, 4), []scriptedResult{okResult("好"), okResult("好")})
	inner := &scripted{
		results: results,
		hook: func(ctx context.Context, n int) {
			if n == 5 {
				panic("下层 panic")
			}
		},
	}
	c := newBreakerCase(breakerConfig, inner)
	for i := 0; i < 4; i++ {
		_ = c.call("")
	}
	c.clock.Advance(30 * time.Second)

	func() {
		defer func() { _ = recover() }()
		_ = c.call("")
	}()
	wantState(t, c.b, "vendor-a", "", provider.BreakerHalfOpen)
	if err := c.call(""); err != nil {
		t.Fatalf("panic 后探测名额应已释放，下一个请求应能探测: %v", err)
	}
	wantState(t, c.b, "vendor-a", "", provider.BreakerClosed)
}

func Test熔断_参数规整(t *testing.T) {
	t.Run("零值配置取默认值", func(t *testing.T) {
		zero := func(func(provider.StateChange)) provider.BreakerConfig { return provider.BreakerConfig{} }
		c := newBreakerCase(zero, &scripted{results: failures(provider.Overloaded, 1), repeat: true})
		for i := 0; i < 4; i++ {
			_ = c.call("")
		}
		wantState(t, c.b, "vendor-a", "", provider.BreakerClosed) // 默认至少 5 个样本
		_ = c.call("")
		wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
		pe := wantCircuitOpen(t, c.call(""), "circuit_open")
		if pe.RetryAfter != 30*time.Second {
			t.Fatalf("默认打开时长应为 30s，实际 RetryAfter = %v", pe.RetryAfter)
		}
	})

	t.Run("失败率大于1按1处理", func(t *testing.T) {
		cfg := func(onChange func(provider.StateChange)) provider.BreakerConfig {
			c := breakerConfig(onChange)
			c.MinRequests, c.FailureRatio = 2, 7
			return c
		}
		// 全是失败时失败率恰为 100%，7 被当作 1，达到阈值而打开；若不规整，100% 永远追不上 700%。
		all := newBreakerCase(cfg, &scripted{results: failures(provider.Overloaded, 2)})
		_ = all.call("")
		_ = all.call("")
		wantState(t, all.b, "vendor-a", "", provider.BreakerOpen)

		// 窗口里混有成功样本时，失败率到不了 100%，不会打开。
		results := concat([]scriptedResult{okResult("好")}, failures(provider.Overloaded, 2))
		mixed := newBreakerCase(cfg, &scripted{results: results})
		for i := 0; i < 3; i++ {
			_ = mixed.call("")
		}
		wantState(t, mixed.b, "vendor-a", "", provider.BreakerClosed)
	})

	t.Run("失败率为NaN或非正按默认的50%处理", func(t *testing.T) {
		for _, ratio := range []float64{math.NaN(), 0, -1} {
			cfg := func(onChange func(provider.StateChange)) provider.BreakerConfig {
				c := breakerConfig(onChange)
				c.MinRequests, c.FailureRatio = 2, ratio
				return c
			}
			c := newBreakerCase(cfg, &scripted{results: failures(provider.Overloaded, 2)})
			_ = c.call("")
			_ = c.call("")
			wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
		}
	})

	t.Run("没有回调函数也能正常工作", func(t *testing.T) {
		noHook := func(func(provider.StateChange)) provider.BreakerConfig {
			c := breakerConfig(nil)
			return c
		}
		c := newBreakerCase(noHook, &scripted{results: failures(provider.Auth, 1)})
		_ = c.call("")
		wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
	})

	t.Run("状态名", func(t *testing.T) {
		names := map[provider.BreakerState]string{
			provider.BreakerClosed:   "closed",
			provider.BreakerOpen:     "open",
			provider.BreakerHalfOpen: "half-open",
			provider.BreakerState(9): "BreakerState(9)",
		}
		for s, want := range names {
			if got := s.String(); got != want {
				t.Fatalf("BreakerState(%d).String() = %q，期望 %q", uint8(s), got, want)
			}
		}
	})

	t.Run("默认配置的参数", func(t *testing.T) {
		d := provider.DefaultBreakerConfig()
		if d.Window != time.Minute || d.MinRequests != 5 || d.FailureRatio != 0.5 || d.OpenFor != 30*time.Second || d.HalfOpenMax != 1 {
			t.Fatalf("DefaultBreakerConfig() = %+v", d)
		}
	})
}

func Test熔断_并发调用不出数据竞争(t *testing.T) {
	c := newBreakerCase(breakerConfig, &scripted{results: failures(provider.Overloaded, 1), repeat: true})
	const workers = 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.call("")
		}()
	}
	wg.Wait()
	wantState(t, c.b, "vendor-a", "", provider.BreakerOpen)
	if calls := c.inner.Calls(); calls < 4 || calls > workers {
		t.Fatalf("上游被调用 %d 次，应在 [4, %d] 之间", calls, workers)
	}
	opened := 0
	for _, ch := range c.log.all() {
		if ch.To == provider.BreakerOpen {
			opened++
		}
	}
	if opened != 1 {
		t.Fatalf("打开了 %d 次，期望并发下也只打开 1 次", opened)
	}
}

func Test熔断_打开时上游0次请求(t *testing.T) {
	// 脚本恰好 4 步：线路打开后再有请求到达上游，mock 会报“脚本耗尽”而使本测试失败。
	srv := providertest.NewServer(t,
		providertest.Status(503, "{}"), providertest.Status(503, "{}"),
		providertest.Status(503, "{}"), providertest.Status(503, "{}"),
	)
	clock := providertest.NewFakeClock(time.Time{})
	b := provider.NewBreaker(breakerConfig(nil), clock)
	p := provider.Chain(newAdapter(t, srv), b.Middleware("mock"))

	for i := 0; i < 4; i++ {
		_, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
		if kind, _ := provider.KindOf(err); kind != provider.Overloaded {
			t.Fatalf("第 %d 次应得到上游的 Overloaded，实际 %v", i+1, err)
		}
	}
	for i := 0; i < 3; i++ {
		_, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
		wantCircuitOpen(t, err, "circuit_open")
	}
	if srv.Count() != 4 {
		t.Fatalf("上游请求数 = %d，期望恰好 4（熔断打开后不得再发请求）", srv.Count())
	}
}
