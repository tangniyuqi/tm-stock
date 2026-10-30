package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func localQAFileFixture(t *testing.T) (AILocalQA, *AILocalRuntime) {
	t.Helper()
	var runtime AILocalRuntime
	if err := json.Unmarshal([]byte(localRuntimeFixture), &runtime); err != nil {
		t.Fatal(err)
	}
	ep := AILocalExecutionModel{Name: "本地问答协议", BaseURL: "http://127.0.0.1:19001/v1", Model: "qa-local-test", TimeoutMS: 20000}
	model := AILocalQAModel{Endpoint: ep, FilingNo: "local-eval:filing", ApprovalRef: "local-eval:model", QualityRef: "local-eval:quality", BudgetRef: "local-eval:budget"}
	client, err := provider.NewDirect(model.ProviderConfig(), true)
	if err != nil {
		t.Fatal(err)
	}
	model.ProfileDigest = client.DirectProfileDigest()
	now := time.Now().UnixMilli()
	return AILocalQA{Version: 1, InstanceID: runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: now - 1000, ExpiresAtMS: now + 3600000, Members: []int64{1}, Subjects: []int64{1}, Models: []AILocalQAModel{model, model, model}, AllowedDomains: []string{"qa.example.test"}, Limits: AILocalQALimits{MaxSources: 4, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, TimeoutMS: 20000}}, &runtime
}
func writeLocalQA(t *testing.T, path string, c AILocalQA) []byte {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestAILocalQAClosedConfiguration(t *testing.T) {
	for _, mode := range []string{"valid", "instance", "scope", "expired", "future", "long_window", "member", "subject", "models", "external", "credentials", "port", "profile", "approval", "budget", "timeout", "limit", "domain", "nil_runtime", "real_calls"} {
		t.Run(mode, func(t *testing.T) {
			c, runtime := localQAFileFixture(t)
			switch mode {
			case "instance":
				c.InstanceID = strings.Repeat("f", 32)
			case "scope":
				c.Scope = "production"
			case "expired":
				c.ExpiresAtMS = c.IssuedAtMS
			case "future":
				c.IssuedAtMS = c.ExpiresAtMS - 1
			case "long_window":
				c.ExpiresAtMS = c.IssuedAtMS + 86400001
			case "member":
				c.Members = []int64{1, 1}
			case "subject":
				c.Subjects = []int64{0}
			case "models":
				c.Models = c.Models[:2]
			case "external":
				c.Models[0].Endpoint.BaseURL = "https://example.test/v1"
			case "credentials":
				c.Models[0].Endpoint.BaseURL = "http://user:forbidden_value@127.0.0.1:19001/v1"
			case "port":
				c.Models[0].Endpoint.BaseURL = "http://127.0.0.1:080/v1"
			case "profile":
				c.Models[0].ProfileDigest = strings.Repeat("f", 64)
			case "approval":
				c.Models[0].ApprovalRef = "production"
			case "budget":
				c.Models[0].BudgetRef = ""
			case "timeout":
				c.Limits.TimeoutMS = 1 << 62
			case "limit":
				c.Limits.MaxTotalTokens = 1
			case "domain":
				c.AllowedDomains = []string{"qa.example.test", "qa.example.test"}
			case "nil_runtime":
				runtime = nil
			case "real_calls":
				runtime.RealCalls = true
			}
			path := filepath.Join(t.TempDir(), "qa.json")
			writeLocalQA(t, path, c)
			s, err := NewAILocalQASource(path, runtime)
			if (err == nil) != (mode == "valid") || err != nil && strings.Contains(err.Error(), "forbidden_value") {
				t.Fatal("本地问答配置边界不符或泄漏内容")
			}
			if err == nil {
				if _, err := s.Current(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestAILocalQAStrictFileAndRevocation(t *testing.T) {
	c, runtime := localQAFileFixture(t)
	path := filepath.Join(t.TempDir(), "qa.json")
	raw := string(writeLocalQA(t, path, c))
	for _, mode := range []string{"duplicate", "alias", "unknown", "null", "missing", "nested", "trailing", "large"} {
		t.Run(mode, func(t *testing.T) {
			bad := raw
			switch mode {
			case "duplicate":
				bad = strings.Replace(raw, `"version":1`, `"version":1,"version":1`, 1)
			case "alias":
				bad = strings.Replace(raw, `"version"`, `"Version"`, 1)
			case "unknown":
				bad = strings.Replace(raw, `"version":1`, `"version":1,"api_key":"forbidden_value"`, 1)
			case "null":
				bad = strings.Replace(raw, `"members":[1]`, `"members":null`, 1)
			case "missing":
				bad = strings.Replace(raw, `"version":1,`, ``, 1)
			case "nested":
				bad = strings.Replace(raw, `"timeout_ms":20000`, `"timeout_ms":20000,"unexpected":false`, 1)
			case "trailing":
				bad = raw + `{}`
			case "large":
				bad = strings.Repeat(" ", 32769)
			}
			if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewAILocalQASource(path, runtime); err == nil {
				t.Fatal("严格文件保护未拒绝")
			}
		})
	}
	writeLocalQA(t, path, c)
	s, err := NewAILocalQASource(path, runtime)
	if err != nil {
		t.Fatal(err)
	}
	c.Revision = "v2"
	writeLocalQA(t, path, c)
	if _, err := s.Current(context.Background()); err == nil {
		t.Fatal("静默更换问答配置")
	}
	c.Revision = "v1"
	writeLocalQA(t, path, c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Current(ctx); err == nil {
		t.Fatal("取消后仍取得配置")
	}
	if _, err := s.Current(nil); err == nil {
		t.Fatal("空上下文仍取得配置")
	}
}
