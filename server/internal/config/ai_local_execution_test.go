package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func localExecutionFixture() (*AILocalRuntime, AILocalExecution) {
	r, _ := localPlansFixture()
	now := time.Now().UnixMilli()
	return r, AILocalExecution{Version: 1, InstanceID: r.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: now - 1000, ExpiresAtMS: now + 60000, Model: AILocalExecutionModel{Name: "local-eval", BaseURL: "http://127.0.0.1:8123", Model: "仅测试:模型", TimeoutMS: 3000}, Agent: AILocalExecutionAgent{BaseURL: "http://127.0.0.1:8124", ApprovalRef: "local-eval:agent", LicenseRef: "local-eval:license", QuickModel: "local-quick", DeepModel: "local-deep", TimeoutMS: 3000, MaxSubmissions: 10}, LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 30000, AgentPollMS: 50}
}

func TestLocalModelSessionParsing(t *testing.T) {
	for _, row := range []struct {
		name, value string
		want        bool
	}{
		{"未配置", "", true}, {"父桥会话", strings.Repeat("a1", 32), true},
		{"太短", strings.Repeat("a", 63), false}, {"太长", strings.Repeat("a", 65), false},
		{"大写", strings.Repeat("A", 64), false}, {"非十六进制", strings.Repeat("z", 64), false},
		{"前空白", " " + strings.Repeat("a", 63), false}, {"换行", strings.Repeat("a", 63) + "\n", false},
		{"供应商前缀", "sk-" + strings.Repeat("a", 61), false}, {"宽字符", strings.Repeat("ａ", 64), false},
	} {
		t.Run(row.name, func(t *testing.T) {
			out, err := ParseAILocalModelSession(row.value)
			if (err == nil) != row.want || err != nil && out != "" || err == nil && out != row.value {
				t.Fatal("会话解析边界不符")
			}
		})
	}
}
func TestLocalExecutionRejectsUnsafeOrAmbiguousConfig(t *testing.T) {
	r, c := localExecutionFixture()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for name, body := range map[string]string{
		"正常":        raw,
		"未知凭据":      strings.Replace(raw, `"version":1`, `"version":1,"api_key":"forbidden_value"`, 1),
		"重复字段":      strings.Replace(raw, `"lease_ms":60000`, `"lease_ms":60000,"lease_ms":60000`, 1),
		"嵌套大小写":     strings.Replace(raw, `"max_submissions"`, `"Max_Submissions"`, 1),
		"缺字段":       strings.Replace(raw, `"heartbeat_ms":1000,`, "", 1),
		"空字段":       strings.Replace(raw, `"heartbeat_ms":1000`, `"heartbeat_ms":null`, 1),
		"跨实例":       strings.Replace(raw, r.InstanceID, strings.Repeat("b", 32), 1),
		"过期":        strings.Replace(raw, `"expires_at_ms":`+strconv.FormatInt(c.ExpiresAtMS, 10), `"expires_at_ms":1`, 1),
		"范围":        strings.Replace(raw, "synthetic-loopback-only", "production", 1),
		"模型远端":      strings.Replace(raw, "http://127.0.0.1:8123", "https://api.example.test", 1),
		"Agent远端":   strings.Replace(raw, "http://127.0.0.1:8124", "http://192.0.2.1:8124", 1),
		"域名":        strings.Replace(raw, "127.0.0.1:8123", "localhost:8123", 1),
		"路径":        strings.Replace(raw, "127.0.0.1:8124", "127.0.0.1:8124/other", 1),
		"查询":        strings.Replace(raw, "127.0.0.1:8123", "127.0.0.1:8123?other=1", 1),
		"端口":        strings.Replace(raw, "127.0.0.1:8123", "127.0.0.1:80", 1),
		"超大时限":      strings.Replace(raw, `"worker_timeout_ms":30000`, `"worker_timeout_ms":999999999999`, 1),
		"超大Agent时限": strings.Replace(raw, `"deep_model":"local-deep","timeout_ms":3000`, `"deep_model":"local-deep","timeout_ms":9223372036854775807`, 1),
		"短租约":       strings.Replace(raw, `"lease_ms":60000`, `"lease_ms":1000`, 1),
		"非本地许可":     strings.Replace(raw, "local-eval:license", "production-license", 1),
		"尾随":        raw + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "execution.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewAILocalExecutionSource(path, r); (err == nil) != (name == "正常") {
				t.Fatal("执行边界不符", err)
			}
		})
	}
}
func TestLocalExecutionSourceCannotHotReplace(t *testing.T) {
	r, c := localExecutionFixture()
	path := filepath.Join(t.TempDir(), "execution.json")
	write := func(v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(c)
	s, err := NewAILocalExecutionSource(path, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Current(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.Revision = "v2"
	write(c)
	if _, err := s.Current(context.Background()); err == nil {
		t.Fatal("运行中静默换配置")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Current(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.Current(nil); err == nil {
		t.Fatal("空上下文未拒绝")
	}
	if strings.Contains(c.String(), c.Model.BaseURL) || c.GoString() != c.String() {
		t.Fatal("配置格式化泄漏端点")
	}
}
