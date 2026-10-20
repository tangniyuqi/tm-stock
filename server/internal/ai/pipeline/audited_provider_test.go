package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type auditStoreFake struct {
	mu        sync.Mutex
	events    []CallEvent
	failPhase string
	cancelled bool
}

func (s *auditStoreFake) BeginRun(context.Context, RunRequest, int64, RunPolicy) (RunRecord, bool, error) {
	return RunRecord{}, false, ErrInvalidRun
}
func (s *auditStoreFake) RunByRequest(context.Context, string) (RunRecord, error) {
	return RunRecord{}, ErrRunNotFound
}
func (s *auditStoreFake) FinishRun(context.Context, int64, ExtractionResult, string) (RunRecord, error) {
	return RunRecord{}, ErrInvalidRun
}
func (s *auditStoreFake) AppendCallEvent(ctx context.Context, event CallEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		s.cancelled = true
		return ctx.Err()
	}
	if event.Phase == s.failPhase {
		return ErrAuditUnavailable
	}
	s.events = append(s.events, event)
	return nil
}

func TestAuditedProviderGuardsBeforeNetwork(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*provider.Request)
		noRun  bool
	}{
		{"无运行编号", func(r *provider.Request) {}, true}, {"错模型", func(r *provider.Request) { r.Model = "other" }, false},
		{"无封闭结构", func(r *provider.Request) { r.Format = provider.FormatText }, false}, {"非严格结构", func(r *provider.Request) { r.Strict = false }, false},
		{"尝试越界", func(r *provider.Request) { r.Attempt = 3 }, false}, {"输出超限", func(r *provider.Request) { r.MaxTokens = 4097 }, false},
		{"输入超限", func(r *provider.Request) { r.Prompt = strings.Repeat("x", 2000) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			store := &auditStoreFake{}
			base := provider.ProviderFunc(func(context.Context, provider.Request) (provider.Response, error) {
				calls++
				return provider.Response{}, nil
			})
			p, err := NewAuditedProvider(base, store, "test-model", "test_secret", 1000)
			if err != nil {
				t.Fatal(err)
			}
			req := provider.Request{Attempt: 1, Prompt: "测试原文", MaxTokens: 4096, Format: provider.FormatJSONSchema, Strict: true}
			tc.change(&req)
			ctx := WithRunID(context.Background(), 1)
			if tc.noRun {
				ctx = context.Background()
			}
			if _, err := p.Generate(ctx, req); err == nil || calls != 0 || len(store.events) != 0 {
				t.Fatalf("拒绝前仍调用了模型: %v 次数=%d", err, calls)
			}
		})
	}
}

func TestAuditedProviderRedactsRawAndFinishesAfterCancellation(t *testing.T) {
	secret := "test_private_provider_value"
	ctx, cancel := context.WithCancel(WithRunID(context.Background(), 7))
	store := &auditStoreFake{}
	base := provider.ProviderFunc(func(ctx context.Context, r provider.Request) (provider.Response, error) {
		if r.Model != "test-model" {
			t.Fatal("未绑定模型")
		}
		cancel()
		return provider.Response{Model: "test-model", Text: "事实内容", Raw: []byte(`{"api_key":"` + secret + `"}`), Usage: provider.Usage{Total: 12}}, nil
	})
	p, err := NewAuditedProvider(base, store, "test-model", secret, 4096)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Generate(ctx, provider.Request{Attempt: 1, Prompt: secret, MaxTokens: 10, Format: provider.FormatJSONSchema, Strict: true})
	if err != nil || len(store.events) != 2 || store.cancelled {
		t.Fatalf("取消后的完成审计缺失: %v", err)
	}
	var finish struct {
		Raw []byte `json:"raw"`
	}
	if err := json.Unmarshal(store.events[1].Payload, &finish); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(finish.Raw), secret) || strings.Contains(string(store.events[0].Payload), secret) {
		t.Fatal("审计泄露密钥")
	}
}

func TestAuditedProviderFailsClosedOnAuditFailures(t *testing.T) {
	for _, phase := range []string{"start", "finish"} {
		t.Run(phase, func(t *testing.T) {
			calls := 0
			store := &auditStoreFake{failPhase: phase}
			p, err := NewAuditedProvider(provider.ProviderFunc(func(context.Context, provider.Request) (provider.Response, error) {
				calls++
				return provider.Response{Model: "test-model"}, nil
			}), store, "test-model", "", 4096)
			if err != nil {
				t.Fatal(err)
			}
			response, err := p.Generate(WithRunID(context.Background(), 1), provider.Request{Attempt: 1, Prompt: "原文", MaxTokens: 10, Format: provider.FormatJSONSchema, Strict: true})
			wantCalls := 1
			if phase == "start" {
				wantCalls = 0
			}
			if !errors.Is(err, ErrAuditUnavailable) || calls != wantCalls || response.Model != "" {
				t.Fatalf("审计失败仍放行: %v 次数=%d", err, calls)
			}
		})
	}
}

func TestAuditedProviderRecordsBilledErrorAndRejectsModelSwitch(t *testing.T) {
	for _, failure := range []bool{false, true} {
		store := &auditStoreFake{}
		base := provider.ProviderFunc(func(context.Context, provider.Request) (provider.Response, error) {
			if failure {
				return provider.Response{}, &provider.Error{Kind: provider.Billing, Usage: provider.Usage{Total: 20}, Detail: "Bearer test_private_value_long"}
			}
			return provider.Response{Model: "unregistered-model"}, nil
		})
		p, err := NewAuditedProvider(base, store, "test-model", "", 4096)
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.Generate(WithRunID(context.Background(), 1), provider.Request{Attempt: 1, Prompt: "原文", MaxTokens: 10, Format: provider.FormatJSONSchema, Strict: true})
		if err == nil || len(store.events) != 2 {
			t.Fatal("失败未被审计/拒绝")
		}
		if strings.Contains(string(store.events[1].Payload), "test_private_value_long") {
			t.Fatal("错误原文中的凭据未脱敏")
		}
	}
}
