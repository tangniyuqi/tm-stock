package provider_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

// decodeBody 把 mock 记录到的请求体解成 map，便于逐字段断言。
func decodeBody(t *testing.T, rec providertest.Recorded) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body, &m); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v\n%s", err, rec.Body)
	}
	return m
}

func messagesOf(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, ok := body["messages"].([]any)
	if !ok {
		t.Fatalf("messages 缺失或类型不对: %v", body["messages"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("messages 元素类型不对: %v", item)
		}
		out = append(out, m)
	}
	return out
}

func Test适配层_请求体与请求头(t *testing.T) {
	zero := 0.0
	seedZero := int64(0)
	temp := 0.7
	seed := int64(42)
	schema := json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}`)

	cases := []struct {
		name  string
		req   provider.Request
		check func(t *testing.T, rec providertest.Recorded, body map[string]any)
	}{
		{"最小请求只带模型与一条用户消息", provider.Request{Prompt: "你好"},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if body["model"] != "model-a" {
					t.Fatalf("model = %v，期望取 Config.Model", body["model"])
				}
				msgs := messagesOf(t, body)
				if len(msgs) != 1 || msgs[0]["role"] != "user" || msgs[0]["content"] != "你好" {
					t.Fatalf("messages = %v", msgs)
				}
				for _, key := range []string{"max_tokens", "temperature", "seed", "response_format", "stream"} {
					if _, has := body[key]; has {
						t.Fatalf("未指定时不应带 %s，实际请求体: %s", key, rec.Body)
					}
				}
			}},
		{"System非空时放system消息在前", provider.Request{System: "你是助手", Prompt: "你好"},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				msgs := messagesOf(t, body)
				if len(msgs) != 2 || msgs[0]["role"] != "system" || msgs[0]["content"] != "你是助手" ||
					msgs[1]["role"] != "user" || msgs[1]["content"] != "你好" {
					t.Fatalf("messages = %v", msgs)
				}
			}},
		{"MaxTokens为正才携带", provider.Request{Prompt: "你好", MaxTokens: 256},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if body["max_tokens"] != float64(256) {
					t.Fatalf("max_tokens = %v", body["max_tokens"])
				}
			}},
		{"MaxTokens为负不携带", provider.Request{Prompt: "你好", MaxTokens: -5},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if _, has := body["max_tokens"]; has {
					t.Fatalf("负数 MaxTokens 不应被发送: %s", rec.Body)
				}
			}},
		{"Temperature与Seed指向零值也如实发送", provider.Request{Prompt: "你好", Temperature: &zero, Seed: &seedZero},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if v, has := body["temperature"]; !has || v != float64(0) {
					t.Fatalf("temperature = %v, %v，零值应如实发送", v, has)
				}
				if v, has := body["seed"]; !has || v != float64(0) {
					t.Fatalf("seed = %v, %v，零值应如实发送", v, has)
				}
			}},
		{"Temperature与Seed取非零值", provider.Request{Prompt: "你好", Temperature: &temp, Seed: &seed},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if body["temperature"] != 0.7 || body["seed"] != float64(42) {
					t.Fatalf("temperature=%v seed=%v", body["temperature"], body["seed"])
				}
			}},
		{"请求里的Model覆盖配置", provider.Request{Prompt: "你好", Model: "model-b"},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if body["model"] != "model-b" {
					t.Fatalf("model = %v，期望 model-b", body["model"])
				}
			}},
		{"JSON对象格式", provider.Request{Prompt: "你好", Format: provider.FormatJSONObject},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				rf, ok := body["response_format"].(map[string]any)
				if !ok || rf["type"] != "json_object" {
					t.Fatalf("response_format = %v", body["response_format"])
				}
				if _, has := rf["json_schema"]; has {
					t.Fatalf("json_object 不应带 json_schema: %v", rf)
				}
			}},
		{"JSON Schema严格", provider.Request{Prompt: "你好", Format: provider.FormatJSONSchema, Schema: schema, Strict: true},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				rf, ok := body["response_format"].(map[string]any)
				if !ok || rf["type"] != "json_schema" {
					t.Fatalf("response_format = %v", body["response_format"])
				}
				js, ok := rf["json_schema"].(map[string]any)
				if !ok || js["name"] != "result" || js["strict"] != true {
					t.Fatalf("json_schema = %v", rf["json_schema"])
				}
				sch, ok := js["schema"].(map[string]any)
				if !ok || sch["type"] != "object" {
					t.Fatalf("schema = %v", js["schema"])
				}
			}},
		{"JSON Schema非严格也显式带strict=false", provider.Request{Prompt: "你好", Format: provider.FormatJSONSchema, Schema: schema},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				rf := body["response_format"].(map[string]any)
				js := rf["json_schema"].(map[string]any)
				if v, has := js["strict"]; !has || v != false {
					t.Fatalf("strict = %v, %v，期望显式的 false", v, has)
				}
			}},
		{"文本格式不带response_format", provider.Request{Prompt: "你好", Format: provider.FormatText, Schema: schema, Strict: true},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if _, has := body["response_format"]; has {
					t.Fatalf("文本格式不应带 response_format（Schema 此时应被忽略）: %s", rec.Body)
				}
			}},
		{"提示词里的尖括号与&原样发出", provider.Request{Prompt: "a<b&c>d"},
			func(t *testing.T, rec providertest.Recorded, body map[string]any) {
				if !bytes.Contains(rec.Body, []byte("a<b&c>d")) {
					t.Fatalf("请求体应保留未转义的原文，实际: %s", rec.Body)
				}
			}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
			p := newAdapter(t, srv)
			if _, err := p.Generate(context.Background(), c.req); err != nil {
				t.Fatalf("Generate 失败: %v", err)
			}
			if srv.Count() != 1 {
				t.Fatalf("上游请求数 = %d，期望 1", srv.Count())
			}
			rec := srv.Requests()[0]
			if rec.Method != http.MethodPost || rec.Path != "/chat/completions" {
				t.Fatalf("请求 = %s %s，期望 POST /chat/completions", rec.Method, rec.Path)
			}
			if got := rec.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q", got)
			}
			if got := rec.Header.Get("Accept"); got != "application/json" {
				t.Fatalf("Accept = %q", got)
			}
			if got := rec.Header.Get("Authorization"); got != "Bearer <已隐藏>" {
				t.Fatalf("记录里的 Authorization = %q，应已脱敏", got)
			}
			if rec.AuthDigest != providertest.AuthDigest("Bearer "+testKey) {
				t.Fatal("适配层发出的 Authorization 与配置的密钥不一致")
			}
			c.check(t, rec, decodeBody(t, rec))
		})
	}
}

func Test适配层_本地校验失败不发上游请求(t *testing.T) {
	nan := math.NaN()
	cases := []struct {
		name string
		req  provider.Request
		code string
	}{
		{"JSON Schema缺少Schema", provider.Request{Prompt: "x", Format: provider.FormatJSONSchema}, "invalid_schema"},
		{"JSON Schema仅空白", provider.Request{Prompt: "x", Format: provider.FormatJSONSchema, Schema: json.RawMessage("   ")}, "invalid_schema"},
		{"JSON Schema不是合法JSON", provider.Request{Prompt: "x", Format: provider.FormatJSONSchema, Schema: json.RawMessage("{bad")}, "invalid_schema"},
		{"未知的输出格式", provider.Request{Prompt: "x", Format: provider.Format(99)}, "invalid_format"},
		{"温度为NaN无法序列化", provider.Request{Prompt: "x", Temperature: &nan}, "invalid_request"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := providertest.NewServer(t)
			p := newAdapter(t, srv)
			_, err := p.Generate(context.Background(), c.req)
			pe := asProviderError(t, err)
			if pe.Kind != provider.BadRequest || pe.Code != c.code || pe.HTTPStatus != 0 {
				t.Fatalf("得到 %+v，期望 BadRequest/%s 且无 HTTP 状态", pe, c.code)
			}
			if srv.Count() != 0 {
				t.Fatalf("本地校验失败时上游请求数应为 0，实际 %d", srv.Count())
			}
		})
	}
}

func Test适配层_成功响应映射(t *testing.T) {
	usage := provider.Usage{Prompt: 12, Completion: 8, Total: 20, Cached: 5, Reasoning: 3}
	srv := providertest.NewServer(t, providertest.Success("你好，这是一条测试回复", usage))
	p := newAdapter(t, srv)
	resp, err := p.Generate(context.Background(), provider.Request{Prompt: "你好", Model: "model-x"})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if resp.Text != "你好，这是一条测试回复" {
		t.Fatalf("Text = %q", resp.Text)
	}
	if resp.Model != "model-x" {
		t.Fatalf("Model = %q，期望取响应里的 model（mock 回显请求）", resp.Model)
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("FinishReason = %q", resp.FinishReason)
	}
	if resp.Usage != usage || resp.UsageEstimated {
		t.Fatalf("Usage = %+v（估算=%v），期望 %+v 且非估算", resp.Usage, resp.UsageEstimated, usage)
	}
	if resp.RequestID != providertest.RequestIDFor(1) {
		t.Fatalf("RequestID = %q", resp.RequestID)
	}
	if !bytes.Contains(resp.Raw, []byte(`"choices"`)) {
		t.Fatalf("Raw 应是原始响应体: %s", resp.Raw)
	}
	if bytes.Contains(resp.Raw, []byte(testKey)) {
		t.Fatal("Raw 不得包含 API 密钥")
	}
}

func Test适配层_用量解析与估算(t *testing.T) {
	// 带 usage 的响应体：usage 为空串表示不带该字段。
	body := func(usage string) string {
		s := `{"id":"c1","model":"m","choices":[{"message":{"content":"一二三"},"finish_reason":"stop"}]`
		if usage != "" {
			s += `,"usage":` + usage
		}
		return s + `}`
	}
	// 所有估算用例共用的请求：系统词 4 个字符 + 提示词 3 个字符，回复“一二三”3 个字符。
	estimated := provider.Usage{Prompt: 7, Completion: 3, Total: 10}

	cases := []struct {
		name      string
		body      string
		want      provider.Usage
		estimated bool
	}{
		{"标准用量", body(`{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 15}, false},
		{"缺少total时自动相加", body(`{"prompt_tokens":10,"completion_tokens":5}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 15}, false},
		{"缓存与推理token", body(`{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":2}}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 15, Cached: 4, Reasoning: 2}, false},
		{"部分厂商的缓存字段", body(`{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_cache_hit_tokens":6}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 15, Cached: 6}, false},
		{"标准缓存字段优先于部分厂商字段", body(`{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":4},"prompt_cache_hit_tokens":6}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 15, Cached: 4}, false},
		{"详情字段为null", body(`{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":null,"completion_tokens_details":null}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 15}, false},
		{"浮点计数四舍五入", body(`{"prompt_tokens":10.0,"completion_tokens":5.4,"total_tokens":15.6}`),
			provider.Usage{Prompt: 10, Completion: 5, Total: 16}, false},
		{"负数按0处理再相加", body(`{"prompt_tokens":-3,"completion_tokens":5,"total_tokens":0}`),
			provider.Usage{Prompt: 0, Completion: 5, Total: 5}, false},
		{"超大值被截断", body(`{"prompt_tokens":1e15,"completion_tokens":1,"total_tokens":1e15}`),
			provider.Usage{Prompt: 1e12, Completion: 1, Total: 1e12}, false},
		{"缺少usage字段则估算", body(""), estimated, true},
		{"usage为null则估算", body("null"), estimated, true},
		{"usage全为0则估算", body(`{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}`), estimated, true},
		{"usage为空对象则估算", body(`{}`), estimated, true},
		{"usage类型错误则估算", body(`"oops"`), estimated, true},
		{"usage字段类型错误则估算", body(`{"prompt_tokens":"x"}`), estimated, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := providertest.NewServer(t, providertest.Status(200, c.body))
			p := newAdapter(t, srv)
			resp, err := p.Generate(context.Background(), provider.Request{System: "你是助手", Prompt: "请问好"})
			if err != nil {
				t.Fatalf("Generate 失败: %v", err)
			}
			if resp.Usage != c.want || resp.UsageEstimated != c.estimated {
				t.Fatalf("Usage = %+v（估算=%v），期望 %+v（估算=%v）", resp.Usage, resp.UsageEstimated, c.want, c.estimated)
			}
		})
	}

	t.Run("估算按字符数而不是字节数", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("你好"))
		p := newAdapter(t, srv)
		// 提示词是两个字母加一个 4 字节字符（U+1F600），共 3 个字符、6 个字节；
		// 回复“你好”是 2 个字符、6 个字节。
		prompt := "ab" + string(rune(0x1F600))
		resp, err := p.Generate(context.Background(), provider.Request{Prompt: prompt})
		if err != nil {
			t.Fatalf("Generate 失败: %v", err)
		}
		want := provider.Usage{Prompt: 3, Completion: 2, Total: 5}
		if resp.Usage != want || !resp.UsageEstimated {
			t.Fatalf("Usage = %+v（估算=%v），期望 %+v 且为估算", resp.Usage, resp.UsageEstimated, want)
		}
	})
}

func Test适配层_响应字段的回退与优先级(t *testing.T) {
	t.Run("响应缺少model时回退到请求模型再回退到配置模型", func(t *testing.T) {
		noModel := `{"id":"c1","choices":[{"message":{"content":"好"},"finish_reason":"stop"}]}`
		srv := providertest.NewServer(t, providertest.Status(200, noModel), providertest.Status(200, noModel))
		p := newAdapter(t, srv)
		resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x", Model: "m-req"})
		if err != nil || resp.Model != "m-req" {
			t.Fatalf("Model = %q, err=%v，期望 m-req", resp.Model, err)
		}
		resp, err = p.Generate(context.Background(), provider.Request{Prompt: "x"})
		if err != nil || resp.Model != "model-a" {
			t.Fatalf("Model = %q, err=%v，期望 model-a", resp.Model, err)
		}
	})

	t.Run("响应体里的id优先于响应头", func(t *testing.T) {
		body := `{"id":"body-id-7","choices":[{"message":{"content":"好"},"finish_reason":"stop"}]}`
		srv := providertest.NewServer(t, providertest.Status(200, body))
		resp, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "x"})
		if err != nil || resp.RequestID != "body-id-7" {
			t.Fatalf("RequestID = %q, err=%v，期望 body-id-7", resp.RequestID, err)
		}
	})

	t.Run("响应体缺少id时取响应头", func(t *testing.T) {
		body := `{"choices":[{"message":{"content":"好"},"finish_reason":"stop"}]}`
		srv := providertest.NewServer(t, providertest.Status(200, body))
		resp, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "x"})
		if err != nil || resp.RequestID != providertest.RequestIDFor(1) {
			t.Fatalf("RequestID = %q, err=%v，期望取 X-Request-Id", resp.RequestID, err)
		}
	})

	t.Run("2xx里非200的状态码同样视为成功", func(t *testing.T) {
		body := `{"id":"c1","choices":[{"message":{"content":"好"},"finish_reason":"stop"}]}`
		srv := providertest.NewServer(t, providertest.Status(202, body), providertest.Status(299, body))
		p := newAdapter(t, srv)
		for i := 0; i < 2; i++ {
			if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err != nil {
				t.Fatalf("第 %d 次: 2xx 应成功: %v", i+1, err)
			}
		}
	})
}

func Test适配层_密钥不出现在响应里(t *testing.T) {
	// 模拟一个把 Authorization 回显进回复文本的异常供应商。
	srv := providertest.NewServer(t, providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
		echoed := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		body, _ := json.Marshal(map[string]any{
			"id":      "c1",
			"model":   "m",
			"choices": []any{map[string]any{"message": map[string]any{"content": "回显了 " + echoed}, "finish_reason": "stop"}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	resp, err := newAdapter(t, srv).Generate(context.Background(), provider.Request{Prompt: "x"})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if strings.Contains(resp.Text, testKey) || bytes.Contains(resp.Raw, []byte(testKey)) {
		t.Fatalf("Text 或 Raw 里出现了 API 密钥: text=%q raw=%s", resp.Text, resp.Raw)
	}
	if !strings.Contains(resp.Text, "<已隐藏>") {
		t.Fatalf("被抹掉的位置应留下占位符，Text = %q", resp.Text)
	}
}

func Test适配层_构造校验与BaseURL形态(t *testing.T) {
	t.Run("非法BaseURL被拒绝且报错不回显地址与密钥", func(t *testing.T) {
		cases := []struct {
			name string
			url  string
		}{
			{"空", ""},
			{"仅空白", "   "},
			{"非http协议", "ftp://leak-marker.invalid/v1"},
			{"无法解析", "://leak-marker.invalid"},
			{"缺少主机名", "http://"},
			{"内嵌账号", "http://fake-user@leak-marker.invalid/v1"},
			{"带查询串", "https://leak-marker.invalid/v1?x=1"},
			{"带片段", "https://leak-marker.invalid/v1#frag"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				p, err := provider.New(provider.Config{BaseURL: c.url, Model: "m", APIKey: testKey})
				if err == nil || p != nil {
					t.Fatalf("期望构造失败，得到 %v, %v", p, err)
				}
				if strings.Contains(err.Error(), "leak-marker") || strings.Contains(err.Error(), testKey) {
					t.Fatalf("报错回显了地址或密钥: %v", err)
				}
			})
		}
	})

	t.Run("BaseURL形态与路径拼接", func(t *testing.T) {
		cases := []struct {
			name     string
			suffix   string
			wantPath string
		}{
			{"根地址", "", "/chat/completions"},
			{"带尾部斜杠", "/", "/chat/completions"},
			{"带版本前缀", "/v1", "/v1/chat/completions"},
			{"带版本前缀与尾部斜杠", "/v1/", "/v1/chat/completions"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
				p, err := provider.New(provider.Config{BaseURL: srv.URL() + c.suffix, Model: "m", APIKey: testKey})
				if err != nil {
					t.Fatalf("构造失败: %v", err)
				}
				if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err != nil {
					t.Fatalf("Generate 失败: %v", err)
				}
				if got := srv.Requests()[0].Path; got != c.wantPath {
					t.Fatalf("路径 = %q，期望 %q", got, c.wantPath)
				}
			})
		}
	})

	t.Run("未配置密钥时不发送Authorization头", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
		p, err := provider.New(provider.Config{BaseURL: srv.URL(), Model: "m"})
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err != nil {
			t.Fatalf("Generate 失败: %v", err)
		}
		rec := srv.Requests()[0]
		if rec.Header.Get("Authorization") != "" || rec.AuthDigest != "" {
			t.Fatalf("不应发送 Authorization 头: %v", rec.Header)
		}
	})

	t.Run("Name返回配置的厂商名", func(t *testing.T) {
		p, err := provider.New(provider.Config{Name: "vendor-x", BaseURL: "https://example.invalid/v1", Model: "m"})
		if err != nil || p.Name() != "vendor-x" {
			t.Fatalf("Name() = %v, err=%v", p, err)
		}
	})

	t.Run("超时配置的边界值与nil选项", func(t *testing.T) {
		for _, ms := range []int{-1, 0, 1, 1 << 40} {
			if _, err := provider.New(provider.Config{BaseURL: "https://example.invalid", Model: "m", TimeoutMS: ms}, nil); err != nil {
				t.Fatalf("TimeoutMS=%d 构造失败: %v", ms, err)
			}
		}
	})

	t.Run("默认Transport被替换时仍可构造并工作", func(t *testing.T) {
		srv := providertest.NewServer(t, providertest.SuccessNoUsage("好"))
		old := http.DefaultTransport
		http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("适配层不应使用被替换的全局 DefaultTransport")
			return nil, http.ErrNotSupported
		})
		defer func() { http.DefaultTransport = old }()

		p, err := provider.New(provider.Config{BaseURL: srv.URL(), Model: "m", APIKey: testKey})
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		if _, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err != nil {
			t.Fatalf("Generate 失败: %v", err)
		}
	})
}
