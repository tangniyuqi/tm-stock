package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func buildL0Runner(cfg config.AIL0Config, store pipeline.RunStore, catalog pipeline.Catalog, options ...provider.Option) (*pipeline.Runner, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.MaxCalls < 1 || cfg.MaxCalls > 1000 || cfg.MaxPromptBytes < 4096 || cfg.MaxPromptBytes > 256<<10 || !cfg.Provider.Enabled {
		return nil, fmt.Errorf("内部抽取调用预算或启用状态无效")
	}
	file, err := os.Open(cfg.RegistryPath)
	if err != nil {
		return nil, fmt.Errorf("内部模型注册表无法读取")
	}
	registry, loadErr := provider.LoadRegistry(file)
	closeErr := file.Close()
	if loadErr != nil || closeErr != nil {
		return nil, fmt.Errorf("内部模型注册表无效")
	}
	if err := cfg.ValidateModel(registry); err != nil {
		return nil, err
	}
	base, err := provider.New(cfg.Provider, options...)
	if err != nil {
		return nil, fmt.Errorf("内部模型适配配置无效")
	}
	audited, err := pipeline.NewAuditedProvider(base, store, cfg.Provider.Model, cfg.Provider.APIKey, cfg.MaxPromptBytes)
	if err != nil {
		return nil, err
	}
	breaker := provider.NewBreaker(provider.DefaultBreakerConfig(), nil)
	guarded := provider.Chain(audited, provider.BudgetMiddleware(), breaker.Middleware(cfg.Provider.Name))
	budget := &provider.Budget{MaxCalls: cfg.MaxCalls}
	// 不安装传输重试/路由回退；仅流水线允许一次畸形输出修复。
	p := provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		return guarded.Generate(provider.WithBudget(ctx, budget), req)
	})
	return pipeline.NewRunner(p, catalog, cfg.Provider.Name)
}
