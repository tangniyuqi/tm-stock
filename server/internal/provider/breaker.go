package provider

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// BreakerState 是熔断器某条线路的状态。
type BreakerState uint8

const (
	// BreakerClosed 关闭：正常放行，统计滚动窗口内的失败率。
	BreakerClosed BreakerState = iota
	// BreakerOpen 打开：直接拒绝，不向上游发请求，直到 OpenFor 过去。
	BreakerOpen
	// BreakerHalfOpen 半开：只放行少量探测请求，探测成功则关闭，失败则重新打开。
	BreakerHalfOpen
)

// String 返回 closed、open、half-open。
func (s BreakerState) String() string {
	switch s {
	case BreakerClosed:
		return "closed"
	case BreakerOpen:
		return "open"
	case BreakerHalfOpen:
		return "half-open"
	default:
		return "BreakerState(" + strconv.Itoa(int(s)) + ")"
	}
}

// BreakerConfig 是熔断参数。取值不合法（不为正，或 FailureRatio 为 NaN）的字段按
// DefaultBreakerConfig 的值处理；FailureRatio 大于 1 按 1 处理。
type BreakerConfig struct {
	// Window 是滚动统计窗口。
	Window time.Duration
	// MinRequests 是窗口内至少要有多少个有效样本才允许按失败率熔断，避免个别失败就跳闸。
	MinRequests int
	// FailureRatio 是触发熔断的失败率阈值，取值 (0, 1]，达到即触发。
	FailureRatio float64
	// OpenFor 是打开后保持多久才转半开。
	OpenFor time.Duration
	// HalfOpenMax 是半开状态下允许同时在途的探测请求数，超出的请求直接被拒。
	HalfOpenMax int
	// OnStateChange 可选，状态变化时回调，用来接告警（Auth、Billing 熔断必须告警）。
	// 回调在引起变化的调用方 goroutine 上同步执行（不持有熔断器内部的锁），可能并发，须快速返回。
	OnStateChange func(StateChange)
}

// StateChange 描述熔断器一条线路的一次状态变化。
type StateChange struct {
	Vendor   string
	Model    string
	From, To BreakerState
	// Cause 是触发打开的失败种类；由定时器转半开、探测成功而关闭引起的变化为 Unknown。
	Cause ErrKind
}

// DefaultBreakerConfig 返回起点参数：60 秒窗口，至少 5 个样本，失败率达 50% 熔断，
// 打开 30 秒后转半开，半开同时只放 1 个探测。具体数值须结合各厂商的稳定性调整。
func DefaultBreakerConfig() BreakerConfig {
	return BreakerConfig{
		Window:       time.Minute,
		MinRequests:  5,
		FailureRatio: 0.5,
		OpenFor:      30 * time.Second,
		HalfOpenMax:  1,
	}
}

func (c BreakerConfig) normalized() BreakerConfig {
	d := DefaultBreakerConfig()
	if c.Window <= 0 {
		c.Window = d.Window
	}
	if c.MinRequests <= 0 {
		c.MinRequests = d.MinRequests
	}
	if !(c.FailureRatio > 0) { // 同时挡住 NaN
		c.FailureRatio = d.FailureRatio
	}
	c.FailureRatio = min(c.FailureRatio, 1)
	if c.OpenFor <= 0 {
		c.OpenFor = d.OpenFor
	}
	if c.HalfOpenMax <= 0 {
		c.HalfOpenMax = d.HalfOpenMax
	}
	return c
}

// Breaker 按（厂商名，模型）分别维护熔断状态，所有线路共用同一份配置与时钟。
// 一个 Breaker 可以同时服务多个厂商：对每条路由的 Provider 调用 Middleware(厂商名) 套上去即可。
//
// 计入失败率的只有 Overloaded、Timeout、Malformed；Auth 与 Billing 立即熔断（不看样本数）；
// 其余种类（含 RateLimit、BadRequest、ContentFilter、ContextLen、Canceled）以及调用方 ctx 已结束时
// 的失败都不计入——前者不代表线路不健康，后者是调用方自己的原因。
// 成功会计入窗口，使失败率反映真实比例。失败率只在记录失败时判断。
type Breaker struct {
	cfg   BreakerConfig
	clock Clock

	mu       sync.Mutex
	circuits map[breakerKey]*circuit
}

type breakerKey struct {
	vendor string
	model  string
}

// circuit 是一条线路的状态。gen 在每次状态变化时加一，用来丢弃“迟到的结果”：
// 请求放行时记下当时的 gen，完成时若已变化说明线路在此期间变过状态，该结果不再有参考价值。
type circuit struct {
	state    BreakerState
	gen      uint64
	openedAt time.Time
	probes   int // 半开状态下在途的探测数
	events   []breakerEvent
}

type breakerEvent struct {
	at     time.Time
	failed bool
}

// NewBreaker 创建熔断器。clock 为 nil 时用 RealClock。
func NewBreaker(cfg BreakerConfig, clock Clock) *Breaker {
	return &Breaker{
		cfg:      cfg.normalized(),
		clock:    clockOrReal(clock),
		circuits: make(map[breakerKey]*circuit),
	}
}

// State 返回某条线路此刻的状态。熔断时间已满的线路会先转为半开再返回；从未见过的线路是 closed。
//
// Request.Model 为空时，线路键里的模型是空串，对应下层 Provider 的默认模型。
func (b *Breaker) State(vendor, model string) BreakerState {
	key := breakerKey{vendor: vendor, model: model}
	var changes []StateChange
	b.mu.Lock()
	c := b.circuits[key]
	if c == nil {
		b.mu.Unlock()
		return BreakerClosed
	}
	b.refreshLocked(key, c, b.clock.Now(), &changes)
	state := c.state
	b.mu.Unlock()
	b.notify(changes)
	return state
}

// Middleware 返回套在某个厂商的 Provider 上的熔断中间件。线路键是（vendor, Request.Model）。
// 线路打开时直接返回 *Error{Kind: CircuitOpen}，其 RetryAfter 是距离转半开还剩的时间，且不调用下层。
func (b *Breaker) Middleware(vendor string) Middleware {
	return func(next Provider) Provider {
		return ProviderFunc(func(ctx context.Context, req Request) (Response, error) {
			key := breakerKey{vendor: vendor, model: req.Model}
			ticket, rejection := b.admit(key)
			if rejection != nil {
				return Response{}, rejection
			}
			// 下层若 panic，必须释放探测名额，否则线路会永远卡在半开。
			done := false
			defer func() {
				if !done {
					b.complete(key, ticket, outcomeNeutral, Unknown)
				}
			}()
			resp, err := next.Generate(ctx, req)
			done = true
			o, kind := classifyOutcome(ctx, err)
			b.complete(key, ticket, o, kind)
			return resp, err
		})
	}
}

type outcome uint8

const (
	outcomeNeutral outcome = iota // 不计入，也不改变状态
	outcomeSuccess
	outcomeFailure // 计入失败率
	outcomeFatal   // Auth、Billing：立即熔断
)

func classifyOutcome(ctx context.Context, err error) (outcome, ErrKind) {
	if err == nil {
		return outcomeSuccess, Unknown
	}
	if ctx.Err() != nil {
		return outcomeNeutral, Unknown // 调用方自己结束了，不怪线路
	}
	pe := asError(err)
	if pe == nil {
		return outcomeNeutral, Unknown
	}
	switch pe.Kind {
	case Overloaded, Timeout, Malformed:
		return outcomeFailure, pe.Kind
	case Auth, Billing:
		return outcomeFatal, pe.Kind
	}
	return outcomeNeutral, pe.Kind
}

// permit 是放行凭证，记着放行时线路所处的 gen。
type permit struct {
	gen uint64
}

// admit 决定是否放行一个请求。拒绝时返回 CircuitOpen，且此时不会向上游发请求。
func (b *Breaker) admit(key breakerKey) (permit, *Error) {
	var changes []StateChange
	b.mu.Lock()
	c := b.circuitLocked(key)
	now := b.clock.Now()
	b.refreshLocked(key, c, now, &changes)

	var (
		p         permit
		rejection *Error
	)
	switch c.state {
	case BreakerOpen:
		rejection = &Error{
			Kind:       CircuitOpen,
			Code:       "circuit_open",
			RetryAfter: max(c.openedAt.Add(b.cfg.OpenFor).Sub(now), 0),
		}
	case BreakerHalfOpen:
		if c.probes >= b.cfg.HalfOpenMax {
			rejection = &Error{Kind: CircuitOpen, Code: "circuit_probing"}
		} else {
			c.probes++
			p = permit{gen: c.gen}
		}
	default:
		p = permit{gen: c.gen}
	}
	b.mu.Unlock()
	b.notify(changes)
	return p, rejection
}

// complete 把一次放行请求的结果记入线路。
func (b *Breaker) complete(key breakerKey, p permit, o outcome, kind ErrKind) {
	var changes []StateChange
	b.mu.Lock()
	c := b.circuitLocked(key)
	if c.gen == p.gen { // gen 变了说明期间线路换过状态，这个迟到的结果作废
		now := b.clock.Now()
		switch c.state {
		case BreakerClosed:
			switch o {
			case outcomeSuccess:
				c.record(now, false, b.cfg.Window)
			case outcomeFailure:
				c.record(now, true, b.cfg.Window)
				if c.overThreshold(b.cfg) {
					b.tripLocked(key, c, now, kind, &changes)
				}
			case outcomeFatal:
				b.tripLocked(key, c, now, kind, &changes)
			}
		case BreakerHalfOpen:
			switch o {
			case outcomeSuccess:
				b.closeLocked(key, c, &changes)
			case outcomeFailure, outcomeFatal:
				b.tripLocked(key, c, now, kind, &changes)
			case outcomeNeutral:
				// 没有得出结论的探测：归还名额，让下一个请求再来探。
				if c.probes > 0 {
					c.probes--
				}
			}
		}
	}
	b.mu.Unlock()
	b.notify(changes)
}

func (b *Breaker) circuitLocked(key breakerKey) *circuit {
	c := b.circuits[key]
	if c == nil {
		c = &circuit{}
		b.circuits[key] = c
	}
	return c
}

// refreshLocked 在熔断时间已满时把 open 转成 half-open。
func (b *Breaker) refreshLocked(key breakerKey, c *circuit, now time.Time, changes *[]StateChange) {
	if c.state == BreakerOpen && !now.Before(c.openedAt.Add(b.cfg.OpenFor)) {
		c.state = BreakerHalfOpen
		c.gen++
		c.probes = 0
		*changes = append(*changes, StateChange{
			Vendor: key.vendor, Model: key.model, From: BreakerOpen, To: BreakerHalfOpen,
		})
	}
}

func (b *Breaker) tripLocked(key breakerKey, c *circuit, now time.Time, cause ErrKind, changes *[]StateChange) {
	from := c.state
	c.state = BreakerOpen
	c.gen++
	c.openedAt = now
	c.probes = 0
	c.events = nil
	*changes = append(*changes, StateChange{
		Vendor: key.vendor, Model: key.model, From: from, To: BreakerOpen, Cause: cause,
	})
}

func (b *Breaker) closeLocked(key breakerKey, c *circuit, changes *[]StateChange) {
	from := c.state
	c.state = BreakerClosed
	c.gen++
	c.probes = 0
	c.events = nil
	*changes = append(*changes, StateChange{
		Vendor: key.vendor, Model: key.model, From: from, To: BreakerClosed,
	})
}

func (b *Breaker) notify(changes []StateChange) {
	if b.cfg.OnStateChange == nil {
		return
	}
	for _, change := range changes {
		b.cfg.OnStateChange(change)
	}
}

// record 记入一个样本，并丢弃已滑出窗口的旧样本。
func (c *circuit) record(now time.Time, failed bool, window time.Duration) {
	cutoff := now.Add(-window)
	drop := 0
	for drop < len(c.events) && c.events[drop].at.Before(cutoff) {
		drop++
	}
	c.events = append(c.events[drop:], breakerEvent{at: now, failed: failed})
}

// overThreshold 判断窗口内的样本数与失败率是否都达到熔断条件。
func (c *circuit) overThreshold(cfg BreakerConfig) bool {
	total := len(c.events)
	if total < cfg.MinRequests {
		return false
	}
	failures := 0
	for _, e := range c.events {
		if e.failed {
			failures++
		}
	}
	return float64(failures)/float64(total) >= cfg.FailureRatio
}
