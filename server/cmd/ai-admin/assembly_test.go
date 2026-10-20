package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type assemblyStore struct{ events []pipeline.CallEvent }

func (s *assemblyStore) BeginRun(context.Context, pipeline.RunRequest, int64, pipeline.RunPolicy) (pipeline.RunRecord, bool, error) {
	return pipeline.RunRecord{}, false, pipeline.ErrInvalidRun
}
func (s *assemblyStore) RunByRequest(context.Context, string) (pipeline.RunRecord, error) {
	return pipeline.RunRecord{}, pipeline.ErrRunNotFound
}
func (s *assemblyStore) FinishRun(context.Context, int64, pipeline.ExtractionResult, string) (pipeline.RunRecord, error) {
	return pipeline.RunRecord{}, pipeline.ErrInvalidRun
}
func (s *assemblyStore) AppendCallEvent(_ context.Context, e pipeline.CallEvent) error {
	s.events = append(s.events, e)
	return nil
}

type assemblyCatalog struct{}

func (assemblyCatalog) Vocabulary(context.Context) (guard.Vocabulary, error) {
	return guard.Vocabulary{Predicates: map[string]bool{"DISCLOSES_BUSINESS": true}, Themes: map[string][]string{"算力": {"算力"}}}, nil
}
func (assemblyCatalog) StockByAlias(context.Context, string) (pipeline.StockRef, bool, error) {
	return pipeline.StockRef{ID: 2, TsCode: "688001.SH"}, true, nil
}
func (assemblyCatalog) ThemeByKey(context.Context, string) (pipeline.ThemeRef, bool, error) {
	return pipeline.ThemeRef{ID: 3}, true, nil
}

type assemblyTransport func(*http.Request) (*http.Response, error)

func (f assemblyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAssemblyDefaultDisabledAndStrictRegistry(t *testing.T) {
	if runner, err := buildL0Runner(config.AIL0Config{}, nil, nil); err != nil || runner != nil {
		t.Fatal("默认状态启用了模型")
	}
	cfg := config.AIL0Config{Enabled: true, Provider: provider.Config{Enabled: true, Name: "test-vendor", BaseURL: "https://api.example.test/v1", Model: "test-model", APIKey: "test_placeholder_key"}, MaxCalls: 1, MaxPromptBytes: 65536, ModelID: "test-model-id"}
	if _, err := buildL0Runner(cfg, &assemblyStore{}, assemblyCatalog{}); err == nil {
		t.Fatal("缺注册表仍构造成功")
	}
	cfg.RegistryPath = filepath.Join(t.TempDir(), "registry.json")
	// 只在测试临时目录写入明确的测试注册项，不接触用户配置或真实备案记录。
	if err := os.WriteFile(cfg.RegistryPath, []byte(`[{"id":"test-model-id","vendor":"test-vendor","name":"test-model","filing_no":"test-filing","caps":["json_schema"]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	invalid := cfg
	invalid.ModelID = "absent"
	if _, err := buildL0Runner(invalid, &assemblyStore{}, assemblyCatalog{}); err == nil {
		t.Fatal("错注册项未拒绝")
	}
	invalid = cfg
	invalid.MaxCalls = 0
	if _, err := buildL0Runner(invalid, &assemblyStore{}, assemblyCatalog{}); err == nil {
		t.Fatal("无限调用预算未拒绝")
	}
}

func TestAssemblyRunsClosedSchemaWithPersistentAuditAndProcessBudget(t *testing.T) {
	registry := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(registry, []byte(`[{"id":"test-model-id","vendor":"test-vendor","name":"test-model","filing_no":"test-filing","caps":["json_schema"]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.AIL0Config{Enabled: true, Provider: provider.Config{Enabled: true, Name: "test-vendor", BaseURL: "https://api.example.test/v1", Model: "test-model", APIKey: "test_placeholder_key"}, MaxCalls: 1, MaxPromptBytes: 65536, ModelID: "test-model-id", RegistryPath: registry}
	var calls atomic.Int64
	transport := assemblyTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Hostname() != "api.example.test" || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer "+cfg.Provider.APIKey {
			t.Fatal("模型线路或鉴权未固定")
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var request struct {
			Model          string `json:"model"`
			ResponseFormat struct {
				Type string `json:"type"`
			} `json:"response_format"`
		}
		if err := json.Unmarshal(raw, &request); err != nil || request.Model != "test-model" || request.ResponseFormat.Type != "json_schema" {
			t.Fatalf("未使用封闭结构: %v", err)
		}
		content := `{"claims":[{"predicate":"DISCLOSES_BUSINESS","entity":"星河科技","theme":"算力","sentence_ids":[1]}]}`
		body, err := json.Marshal(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": map[string]any{"content": content}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}})
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	store := &assemblyStore{}
	runner, err := buildL0Runner(cfg, store, assemblyCatalog{}, provider.WithTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	source := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "test-license", LicenseRef: "test-written-authorization"}
	snapshot, err := sources.BuildSnapshot(source, "https://example.test/report", "text/plain; charset=utf-8", []byte("星河科技主营算力服务。"), time.Now().UTC(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = 1
	result, err := runner.Run(pipeline.WithRunID(context.Background(), 1), snapshot)
	if err != nil || len(result.Candidates) != 1 || len(store.events) != 2 || calls.Load() != 1 {
		t.Fatalf("装配抽取/审计失败: %+v %v", result, err)
	}
	if _, err := runner.Run(pipeline.WithRunID(context.Background(), 2), snapshot); err == nil || calls.Load() != 1 || len(store.events) != 2 {
		t.Fatal("进程调用上限未阻止第二次网络请求")
	}
}
