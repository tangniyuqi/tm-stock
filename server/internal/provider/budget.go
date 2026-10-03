package provider

import (
	"context"
	"sync"
)

// Budget 是一组共享的调用预算，经 context 携带（WithBudget），由 BudgetMiddleware 强制。
// 同一个 *Budget 可以被多个调用、多个 goroutine 共用；不得按值复制。
//
// MaxCalls 与 MaxTokens 不为正表示该维度不限。
//   - 调用数在调用前预占，用满后的调用直接被拒，不会向上游发请求；预占的名额不退还，
//     因为失败的请求同样可能已经产生上游成本。
//   - token 在调用后累计，累计达到上限后，后续调用被拒。token 是事后累计，
//     并发在途的调用可能让总量略微超出上限；要硬封顶请同时设置 MaxCalls。
type Budget struct {
	MaxCalls  int
	MaxTokens int

	mu     sync.Mutex
	calls  int
	tokens int
}

type budgetKey struct{}

// WithBudget 返回携带 b 的 ctx；b 为 nil 时原样返回 ctx。
func WithBudget(ctx context.Context, b *Budget) context.Context {
	if b == nil {
		return ctx
	}
	return context.WithValue(ctx, budgetKey{}, b)
}

// BudgetFrom 取出 ctx 里携带的预算，没有则返回 nil。
func BudgetFrom(ctx context.Context) *Budget {
	b, _ := ctx.Value(budgetKey{}).(*Budget)
	return b
}

// Calls 返回已预占的调用数。
func (b *Budget) Calls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

// Tokens 返回已累计的 token 数。
func (b *Budget) Tokens() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens
}

// reserve 预占一次调用；预算已用尽时返回 BudgetExceeded。检查与计数在同一把锁里，并发下精确。
func (b *Budget) reserve() *Error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.MaxCalls > 0 && b.calls >= b.MaxCalls {
		return &Error{Kind: BudgetExceeded, Code: "calls_exhausted"}
	}
	if b.MaxTokens > 0 && b.tokens >= b.MaxTokens {
		return &Error{Kind: BudgetExceeded, Code: "tokens_exhausted"}
	}
	b.calls++
	return nil
}

// settle 把一次调用的用量计入累计。
func (b *Budget) settle(u Usage) {
	if u.Total <= 0 {
		return
	}
	b.mu.Lock()
	b.tokens += u.Total
	b.mu.Unlock()
}

// BudgetMiddleware 按 ctx 里的 Budget 做调用前预占与调用后累计。
// ctx 里没有预算时直接放行。预算用尽返回 *Error{Kind: BudgetExceeded}，且不会调用下层。
// 失败但已计费的用量（Error.Usage）同样会累计。
func BudgetMiddleware() Middleware {
	return func(next Provider) Provider {
		return ProviderFunc(func(ctx context.Context, req Request) (Response, error) {
			b := BudgetFrom(ctx)
			if b == nil {
				return next.Generate(ctx, req)
			}
			if rerr := b.reserve(); rerr != nil {
				return Response{}, rerr
			}
			resp, err := next.Generate(ctx, req)
			if err == nil {
				b.settle(resp.Usage)
			} else if pe := asError(err); pe != nil {
				b.settle(pe.Usage)
			}
			return resp, err
		})
	}
}
