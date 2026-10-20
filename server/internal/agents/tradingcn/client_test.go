package tradingcn

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

func testConfig() Config {
	return Config{Enabled: true, BaseURL: "https://agents.example.test", ApprovedHost: "agents.example.test", Bearer: strings.Repeat("x", 40), ApprovalRef: "test-only:approval", LicenseRef: "test-only:license", QuickModel: "test-quick", DeepModel: "test-deep", Timeout: time.Second, MaxSubmissions: 2}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testClient(t *testing.T, fn func(*http.Request) (*http.Response, error)) *Client {
	t.Helper()
	c, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	c.http.Transport = roundTripFunc(fn)
	return c
}
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestConfigFailClosed(t *testing.T) {
	if err := testConfig().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.Enabled = false }, func(c *Config) { c.BaseURL = "http://agents.example.test" },
		func(c *Config) { c.BaseURL = "https://agents.example.test:444" }, func(c *Config) { c.BaseURL += "/api" },
		func(c *Config) { c.BaseURL += "?" }, func(c *Config) { c.BaseURL += "#a" }, func(c *Config) { c.BaseURL = "https://user@agents.example.test" },
		func(c *Config) { c.BaseURL = "https://127.0.0.1"; c.ApprovedHost = "127.0.0.1" }, func(c *Config) { c.ApprovedHost = "other.example.test" },
		func(c *Config) { c.BaseURL = "https://agents.example.test:" }, func(c *Config) { c.BaseURL = "https://localhost"; c.ApprovedHost = "localhost" },
		func(c *Config) { c.Bearer = "" }, func(c *Config) { c.Bearer = strings.Repeat("x", 40) + "\n" }, func(c *Config) { c.ApprovalRef = "" },
		func(c *Config) { c.LicenseRef = "" }, func(c *Config) { c.QuickModel = "\u200b" }, func(c *Config) { c.DeepModel = " model" },
		func(c *Config) { c.Timeout = 0 }, func(c *Config) { c.Timeout = 31 * time.Second }, func(c *Config) { c.MaxSubmissions = 0 }, func(c *Config) { c.MaxSubmissions = 1001 },
	} {
		c := testConfig()
		change(&c)
		if _, err := New(c); err == nil {
			t.Fatalf("无效配置放行：%s", c.BaseURL)
		}
	}
	c := testConfig()
	p := c.Profile()
	c.Bearer = strings.Repeat("y", 40)
	if p != c.Profile() {
		t.Fatal("凭据轮换改变配置身份")
	}
	c.DeepModel = "new-model"
	if p == c.Profile() {
		t.Fatal("模型变更未隔离旧请求")
	}
}

func TestV2WireContract(t *testing.T) {
	var calls atomic.Int32
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+testConfig().Bearer || r.URL.Host != "agents.example.test" || r.GetBody != nil || r.Header.Get("Idempotency-Key") != "" {
			t.Error("凭据、固定地址或禁止重放契约错误")
		}
		switch r.URL.Path {
		case "/api/analysis/single":
			if r.Method != "POST" {
				t.Error(r.Method)
			}
			var v map[string]any
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				t.Fatal("请求不是 JSON")
			}
			p := v["parameters"].(map[string]any)
			if v["symbol"] != "600001" || p["engine"] != "v2" || p["quick_analysis_model"] != "test-quick" || p["deep_analysis_model"] != "test-deep" || p["include_sentiment"] != false || p["include_risk"] != false || p["enable_trace"] != false || p["custom_prompt"] != nil {
				t.Fatalf("请求失配：%v", v)
			}
			if strings.Join([]string{p["selected_analysts"].([]any)[0].(string), p["selected_analysts"].([]any)[1].(string)}, ",") != "news,fundamentals" {
				t.Fatal("分析师未固定")
			}
			return response(200, `{"success":true,"data":{"task_id":"test-remote-1","status":"pending","created_at":"2026-10-15"},"message":"已入队"}`), nil
		case "/api/analysis/tasks/test-remote-1/status":
			return response(200, `{"success":true,"data":{"task_id":"test-remote-1","status":"processing","progress":33,"tokens_used":0}}`), nil
		case "/api/analysis/tasks/test-remote-1/result":
			return response(200, `{"success":true,"data":{"analysis_id":"test-remote-1","reports":{"news":"未核验内部正文 `+testConfig().Bearer+`"},"tokens_used":9007199254740993}}`), nil
		case "/api/analysis/tasks/test-remote-1/cancel":
			if r.Method != "POST" {
				t.Error(r.Method)
			}
			return response(200, `{"success":true,"message":"已受理"}`), nil
		default:
			t.Fatalf("意外接口：%s", r.URL.Path)
			return nil, nil
		}
	})
	a, err := c.Submit(context.Background(), "600001")
	if err != nil || a.RemoteID != "test-remote-1" {
		t.Fatalf("%v %v", a, err)
	}
	o, err := c.Status(context.Background(), a.RemoteID)
	if err != nil || o.State != agents.Running {
		t.Fatalf("%v %v", o, err)
	}
	r, err := c.Result(context.Background(), a.RemoteID)
	if err != nil || !r.Valid() || strings.Contains(string(r.Payload), testConfig().Bearer) || !strings.Contains(string(r.Payload), "9007199254740993") {
		t.Fatalf("隔离脱敏或整数精度错误：%v", err)
	}
	if c.Cancel(context.Background(), a.RemoteID) != nil || calls.Load() != 4 {
		t.Fatal("操作数量错误")
	}
}

func TestInvalidResponsesNeverReplay(t *testing.T) {
	for name, body := range map[string]string{
		"重复信封键": `{"success":false,"success":true,"data":{"task_id":"x","status":"pending"}}`,
		"重复嵌套键": `{"success":true,"data":{"task_id":"x","task_id":"y","status":"pending"}}`,
		"多个值":   `{"success":true,"data":{}} {}`, "截断": `{"success":true`, "无成功标记": `{"data":{}}`,
		"字符串布尔": `{"success":"true","data":{}}`, "空对象": `{"success":true,"data":{}}`, "失败信封": `{"success":false,"data":{}}`,
		"空数据": `{"success":true,"data":null}`, "路径注入": `{"success":true,"data":{"task_id":"../x","status":"pending"}}`,
		"错状态": `{"success":true,"data":{"task_id":"x","status":"surprise"}}`, "超大体": strings.Repeat(" ", agents.MaxResultBytes+1),
		"非法UTF8": string([]byte{0xff}), "过深": strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			c := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return response(200, body), nil })
			if _, err := c.Submit(context.Background(), "600001"); err == nil || calls != 1 {
				t.Fatalf("失败未拒绝或重复调用：%v %d", err, calls)
			}
		})
	}
	for _, status := range []int{201, 302, 307, 308, 400, 401, 403, 408, 429, 500, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			c := testClient(t, func(*http.Request) (*http.Response, error) {
				calls++
				return response(status, testConfig().Bearer), nil
			})
			_, err := c.Submit(context.Background(), "600001")
			if err == nil || strings.Contains(err.Error(), testConfig().Bearer) || calls != 1 {
				t.Fatal("状态错误、泄密或重复调用")
			}
		})
	}
}

func TestReadIdentityAndStateGuards(t *testing.T) {
	for _, body := range []string{`{"success":true,"data":{"task_id":"other","status":"running"}}`, `{"success":true,"data":{"task_id":"x","status":"new_state"}}`} {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, err := c.Status(context.Background(), "x"); err == nil {
			t.Fatal("错误身份或状态未拒绝")
		}
	}
	for _, body := range []string{`{"success":true,"data":{"analysis_id":"other","reports":{"news":"x"}}}`, `{"success":true,"data":{"analysis_id":"x","reports":{}}}`} {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, err := c.Result(context.Background(), "x"); err == nil {
			t.Fatal("错误结果未拒绝")
		}
	}
	c := testClient(t, func(*http.Request) (*http.Response, error) { t.Fatal("无效输入不应外联"); return nil, nil })
	if _, err := c.Submit(context.Background(), "bad"); err == nil {
		t.Fatal("非法代码")
	}
	if _, err := c.Status(context.Background(), "../x"); err == nil {
		t.Fatal("非法编号")
	}
	if _, err := c.Result(context.Background(), "../x"); err == nil {
		t.Fatal("非法编号")
	}
	if c.Cancel(context.Background(), "../x") == nil {
		t.Fatal("非法编号")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Submit(ctx, "600001"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestTransportSanitizationAndHeaders(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { return nil, fmtSensitiveError{testConfig().Bearer} })
	_, err := c.Submit(context.Background(), "600001")
	if err == nil || strings.Contains(err.Error(), testConfig().Bearer) || errors.Unwrap(err) != nil {
		t.Fatal("传输错误原文进入错误链")
	}
	c = testClient(t, func(*http.Request) (*http.Response, error) {
		r := response(200, `{}`)
		r.Header.Set("Content-Type", "text/html")
		return r, nil
	})
	if _, err := c.Submit(context.Background(), "600001"); err == nil {
		t.Fatal("HTML 未拒绝")
	}
}

type fmtSensitiveError struct{ secret string }

func (e fmtSensitiveError) Error() string { return e.secret }

func TestTLSRedirectDoesNotLeakCredential(t *testing.T) {
	var second atomic.Int32
	sink := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
	defer sink.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", sink.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	c, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// 仅测试替换私有地址与信任根，生产构造函数仍拒绝 IP/HTTP。
	c.cfg.BaseURL = source.URL
	c.http.Transport = source.Client().Transport
	if _, err := c.Submit(context.Background(), "600001"); err == nil || second.Load() != 0 {
		t.Fatal("发生跳转或泄漏")
	}
}

func TestExplicitLocalEvaluationAllowsOnlyLiteralLoopback(t *testing.T) {
	cfg := testConfig()
	cfg.LocalEvaluation = true
	for _, tc := range []struct {
		url, host string
		valid     bool
	}{
		{"http://127.0.0.1:18000", "127.0.0.1", true}, {"http://[::1]:18000", "::1", true},
		{"http://localhost:18000", "localhost", false}, {"http://192.168.1.10:18000", "192.168.1.10", false},
		{"http://agents.example.test:18000", "agents.example.test", false}, {"http://127.0.0.1", "127.0.0.1", false},
		{"http://127.0.0.1:0", "127.0.0.1", false}, {"http://127.0.0.1:18000?x=1", "127.0.0.1", false},
	} {
		cfg.BaseURL = tc.url
		cfg.ApprovedHost = tc.host
		if (cfg.Validate() == nil) != tc.valid {
			t.Fatalf("本地地址边界错误：%s", tc.url)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":true,"data":{"task_id":"local-eval-1","status":"pending"}}`)
	}))
	defer s.Close()
	cfg.BaseURL = s.URL
	cfg.ApprovedHost = "127.0.0.1"
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Submit(context.Background(), "600001"); err != nil {
		t.Fatal(err)
	}
}
