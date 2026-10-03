package provider_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

func Test预算_调用数上限(t *testing.T) {
	cases := []struct {
		name       string
		maxCalls   int
		attempts   int
		wantPassed int
	}{
		{"上限3发5次只放行3次", 3, 5, 3},
		{"上限1发3次只放行1次", 1, 3, 1},
		{"上限为0表示不限", 0, 5, 5},
		{"上限为负数表示不限", -1, 5, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{results: []scriptedResult{okResult("好")}, repeat: true}
			p := provider.Chain(inner, provider.BudgetMiddleware())
			budget := &provider.Budget{MaxCalls: c.maxCalls}
			ctx := provider.WithBudget(context.Background(), budget)

			passed := 0
			for i := 0; i < c.attempts; i++ {
				_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
				if err == nil {
					passed++
					continue
				}
				pe := asProviderError(t, err)
				if pe.Kind != provider.BudgetExceeded || pe.Code != "calls_exhausted" {
					t.Fatalf("第 %d 次被拒时得到 %+v，期望 BudgetExceeded/calls_exhausted", i+1, pe)
				}
			}
			if passed != c.wantPassed {
				t.Fatalf("放行 %d 次，期望 %d 次", passed, c.wantPassed)
			}
			if inner.Calls() != c.wantPassed {
				t.Fatalf("上游被调用 %d 次，期望 %d 次（被拒的调用不得到达下层）", inner.Calls(), c.wantPassed)
			}
			if budget.Calls() != c.wantPassed {
				t.Fatalf("Budget.Calls() = %d，期望 %d", budget.Calls(), c.wantPassed)
			}
		})
	}
}

func Test预算_token上限与用量累计(t *testing.T) {
	usageOf := func(total int) scriptedResult {
		return scriptedResult{resp: provider.Response{Text: "好", Usage: provider.Usage{Total: total}}}
	}
	failedButBilled := scriptedResult{err: &provider.Error{Kind: provider.Malformed, Code: "empty_content", Usage: provider.Usage{Total: 70}}}

	cases := []struct {
		name       string
		maxTokens  int
		results    []scriptedResult
		wantPassed int // 到达下层的调用数
		wantTokens int
	}{
		{"累计超过上限后拒绝", 100, []scriptedResult{usageOf(60), usageOf(60), usageOf(60)}, 2, 120},
		{"累计恰好等于上限也拒绝后续", 100, []scriptedResult{usageOf(100), usageOf(1)}, 1, 100},
		{"未到上限继续放行", 100, []scriptedResult{usageOf(30), usageOf(30), usageOf(30)}, 3, 90},
		{"失败但已计费的用量也累计", 100, []scriptedResult{failedButBilled, failedButBilled, okResult("好")}, 2, 140},
		{"上限为0表示不限token", 0, []scriptedResult{usageOf(1000), usageOf(1000)}, 2, 2000},
		{"零用量不累计", 10, []scriptedResult{usageOf(0), usageOf(0), usageOf(0)}, 3, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scripted{results: c.results}
			p := provider.Chain(inner, provider.BudgetMiddleware())
			budget := &provider.Budget{MaxTokens: c.maxTokens}
			ctx := provider.WithBudget(context.Background(), budget)

			for i := range c.results {
				_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
				if kind, ok := provider.KindOf(err); ok && kind == provider.BudgetExceeded {
					if pe := asProviderError(t, err); pe.Code != "tokens_exhausted" {
						t.Fatalf("第 %d 次被拒的错误码 = %q，期望 tokens_exhausted", i+1, pe.Code)
					}
				}
			}
			if inner.Calls() != c.wantPassed {
				t.Fatalf("到达下层的调用数 = %d，期望 %d", inner.Calls(), c.wantPassed)
			}
			if budget.Tokens() != c.wantTokens {
				t.Fatalf("累计 token = %d，期望 %d", budget.Tokens(), c.wantTokens)
			}
		})
	}
}

func Test预算_调用数与token同时设置时先到先拒(t *testing.T) {
	inner := &scripted{
		results: []scriptedResult{{resp: provider.Response{Text: "好", Usage: provider.Usage{Total: 50}}}},
		repeat:  true,
	}
	p := provider.Chain(inner, provider.BudgetMiddleware())
	budget := &provider.Budget{MaxCalls: 5, MaxTokens: 80}
	ctx := provider.WithBudget(context.Background(), budget)

	// 第 1、2 次放行（累计 100 token，超过 80），第 3 次因 token 被拒，而不是因调用数。
	for i := 0; i < 2; i++ {
		if _, err := p.Generate(ctx, provider.Request{Prompt: "x"}); err != nil {
			t.Fatalf("第 %d 次应放行: %v", i+1, err)
		}
	}
	_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
	if pe := asProviderError(t, err); pe.Kind != provider.BudgetExceeded || pe.Code != "tokens_exhausted" {
		t.Fatalf("得到 %+v，期望 BudgetExceeded/tokens_exhausted", pe)
	}
	if inner.Calls() != 2 {
		t.Fatalf("到达下层的调用数 = %d，期望 2", inner.Calls())
	}
}

func Test预算_ctx里没有预算时放行(t *testing.T) {
	inner := &scripted{results: []scriptedResult{okResult("好")}, repeat: true}
	p := provider.Chain(inner, provider.BudgetMiddleware())

	if provider.BudgetFrom(context.Background()) != nil {
		t.Fatal("没有挂预算的 ctx，BudgetFrom 应返回 nil")
	}
	// WithBudget(nil) 不改变 ctx。
	ctx := provider.WithBudget(context.Background(), nil)
	if provider.BudgetFrom(ctx) != nil {
		t.Fatal("WithBudget(nil) 不应挂上预算")
	}
	for i := 0; i < 10; i++ {
		if _, err := p.Generate(ctx, provider.Request{Prompt: "x"}); err != nil {
			t.Fatalf("没有预算时应一律放行: %v", err)
		}
	}
	if inner.Calls() != 10 {
		t.Fatalf("下层调用数 = %d，期望 10", inner.Calls())
	}

	budget := &provider.Budget{MaxCalls: 1}
	if provider.BudgetFrom(provider.WithBudget(context.Background(), budget)) != budget {
		t.Fatal("BudgetFrom 应取回同一个预算")
	}
}

func Test预算_并发下调用数精确(t *testing.T) {
	const (
		workers  = 100
		maxCalls = 37
	)
	var upstream atomic.Int64
	inner := provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		upstream.Add(1)
		return provider.Response{Text: "好", Usage: provider.Usage{Total: 1}}, nil
	})
	p := provider.Chain(inner, provider.BudgetMiddleware())
	budget := &provider.Budget{MaxCalls: maxCalls}
	ctx := provider.WithBudget(context.Background(), budget)

	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		passed   atomic.Int64
		rejected atomic.Int64
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // 一起放开，制造真正的并发竞争
			_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
			if err == nil {
				passed.Add(1)
				return
			}
			if kind, ok := provider.KindOf(err); ok && kind == provider.BudgetExceeded {
				rejected.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if passed.Load() != maxCalls || rejected.Load() != workers-maxCalls {
		t.Fatalf("放行 %d、拒绝 %d，期望 %d 与 %d", passed.Load(), rejected.Load(), maxCalls, workers-maxCalls)
	}
	if upstream.Load() != maxCalls {
		t.Fatalf("上游被调用 %d 次，期望 %d 次", upstream.Load(), maxCalls)
	}
	if budget.Calls() != maxCalls || budget.Tokens() != maxCalls {
		t.Fatalf("Budget 计数 calls=%d tokens=%d，期望都是 %d", budget.Calls(), budget.Tokens(), maxCalls)
	}
}

func Test预算_拒绝时上游0次请求(t *testing.T) {
	// 脚本只有 2 步：第 3 次调用若到达上游，会让 mock 报“脚本耗尽”而使本测试失败。
	srv := providertest.NewServer(t,
		providertest.Success("好", provider.Usage{Prompt: 1, Completion: 1, Total: 2}),
		providertest.Success("好", provider.Usage{Prompt: 1, Completion: 1, Total: 2}),
	)
	p := provider.Chain(newAdapter(t, srv), provider.BudgetMiddleware())
	ctx := provider.WithBudget(context.Background(), &provider.Budget{MaxCalls: 2})

	for i := 0; i < 2; i++ {
		if _, err := p.Generate(ctx, provider.Request{Prompt: "x"}); err != nil {
			t.Fatalf("第 %d 次应成功: %v", i+1, err)
		}
	}
	_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
	if pe := asProviderError(t, err); pe.Kind != provider.BudgetExceeded || pe.HTTPStatus != 0 {
		t.Fatalf("得到 %+v，期望 BudgetExceeded", pe)
	}
	if srv.Count() != 2 {
		t.Fatalf("上游请求数 = %d，期望恰好 2（预算用尽后不得再发请求）", srv.Count())
	}
}

func Test预算_token用尽后上游0次请求(t *testing.T) {
	srv := providertest.NewServer(t, providertest.Success("好", provider.Usage{Prompt: 60, Completion: 40, Total: 100}))
	p := provider.Chain(newAdapter(t, srv), provider.BudgetMiddleware())
	budget := &provider.Budget{MaxTokens: 100}
	ctx := provider.WithBudget(context.Background(), budget)

	if _, err := p.Generate(ctx, provider.Request{Prompt: "x"}); err != nil {
		t.Fatalf("第一次应成功: %v", err)
	}
	_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
	if pe := asProviderError(t, err); pe.Kind != provider.BudgetExceeded || pe.Code != "tokens_exhausted" {
		t.Fatalf("得到 %+v，期望 BudgetExceeded/tokens_exhausted", pe)
	}
	if srv.Count() != 1 {
		t.Fatalf("上游请求数 = %d，期望恰好 1", srv.Count())
	}
	if budget.Tokens() != 100 {
		t.Fatalf("累计 token = %d，期望 100", budget.Tokens())
	}
}
