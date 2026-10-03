package provider_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

func Test脱敏_各种形态(t *testing.T) {
	const secret = "sk-fake-0123456789abcdef"
	cases := []struct {
		name string
		in   string
		want string
		leak string // 输出里不得出现的敏感片段；为空表示不检查
	}{
		{"文本头Bearer", "Authorization: Bearer " + secret, "Authorization: Bearer <已隐藏>", secret},
		{"文本头Basic", "authorization: Basic ZmFrZTpmYWtl", "authorization: Basic <已隐藏>", "ZmFrZTpmYWtl"},
		{"文本头无方案", "Authorization: " + secret, "Authorization: <已隐藏>", secret},
		{"代理授权头", "Proxy-Authorization: Basic ZmFrZTpmYWtl", "Proxy-Authorization: Basic <已隐藏>", "ZmFrZTpmYWtl"},
		{"等号形式在&处截止", "authorization=" + secret + "&x=1", "authorization=<已隐藏>&x=1", secret},
		{"JSON字符串值", `{"Authorization":"Bearer ` + secret + `"}`, `{"Authorization":"<已隐藏>"}`, secret},
		{"JSON数组值(http.Header序列化)", `{"Authorization":["Bearer ` + secret + `"]}`, `{"Authorization":["<已隐藏>"]}`, secret},
		{"JSON字段api_key", `{"api_key": "` + secret + `","model":"m"}`, `{"api_key": "<已隐藏>","model":"m"}`, secret},
		{"JSON字段apiKey大小写不敏感", `{"apiKey":"` + secret + `"}`, `{"apiKey":"<已隐藏>"}`, secret},
		{"JSON字段x-api-key", `{"X-Api-Key":"` + secret + `"}`, `{"X-Api-Key":"<已隐藏>"}`, secret},
		{"JSON字段access_token与password", `{"access_token":"fake-aaaaaaaa","password":"fake-bbbbbbbb"}`, `{"access_token":"<已隐藏>","password":"<已隐藏>"}`, "fake-"},
		{"文本x-api-key", "x-api-key: " + secret, "x-api-key: <已隐藏>", secret},
		{"URL查询串里的api_key", "https://h.invalid/p?api_key=" + secret + "&model=x", "https://h.invalid/p?api_key=<已隐藏>&model=x", secret},
		{"带引号的api_key值", `api_key: "` + secret + `"`, `api_key: "<已隐藏>"`, secret},
		{"access_token文本形式", "access_token: fake-abcdefghijkl", "access_token: <已隐藏>", "fake-abcdefghijkl"},
		{"password等号形式", "password=fake-hunter2xx", "password=<已隐藏>", "fake-hunter2xx"},
		{"被转义后嵌进字符串的JSON片段", `{"prompt":"{\"api_key\":\"` + secret + `\"}"}`, `{"prompt":"{\"api_key\":\"<已隐藏>\"}"}`, secret},
		{"正文里孤立的Bearer令牌", "请使用 Bearer " + secret + " 调用", "请使用 Bearer <已隐藏> 调用", secret},
		{"孤立的sk-前缀密钥", "密钥是 " + secret + "。", "密钥是 sk-<已隐藏>。", "0123456789abcdef"},
		{"CRLF分隔的头块只抹令牌", "Authorization: Bearer fake-abcdefgh12345\r\nContent-Type: application/json", "Authorization: Bearer <已隐藏>\r\nContent-Type: application/json", "fake-abcdefgh12345"},
		{"多处出现都要抹", "a Bearer " + secret + " b Bearer " + secret, "a Bearer <已隐藏> b Bearer <已隐藏>", secret},

		// 不得误伤的内容
		{"max_tokens字段不动", `{"max_tokens": 100, "model": "x"}`, `{"max_tokens": 100, "model": "x"}`, ""},
		{"类似字段名不动", `{"token_count": 5, "password_hint": "x"}`, `{"token_count": 5, "password_hint": "x"}`, ""},
		{"task-不是sk-前缀", "task-based approach", "task-based approach", ""},
		{"bearer一词后面不够长不动", "the bearer scheme is documented", "the bearer scheme is documented", ""},
		{"没有冒号的authorization不动", "authorization is required", "authorization is required", ""},
		{"api_key后没有值不动", "the api_key", "the api_key", ""},
		{"普通中文不动", "题材归属：电池板块，成分以公告为准", "题材归属：电池板块，成分以公告为准", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := []byte(c.in)
			snapshot := append([]byte(nil), in...)
			got := provider.Redact(in)
			if string(got) != c.want {
				t.Fatalf("Redact(%q)\n得到 %q\n期望 %q", c.in, got, c.want)
			}
			if c.leak != "" && strings.Contains(string(got), c.leak) {
				t.Fatalf("输出里仍有敏感片段 %q: %q", c.leak, got)
			}
			if !bytes.Equal(in, snapshot) {
				t.Fatal("Redact 不得修改入参")
			}
			if again := provider.Redact(got); string(again) != string(got) {
				t.Fatalf("Redact 应幂等：再次脱敏后变成 %q", again)
			}
		})
	}

	if got := provider.Redact(nil); len(got) != 0 {
		t.Fatalf("Redact(nil) = %q，应为空", got)
	}
}

func Test日志_成功与失败的记录字段(t *testing.T) {
	clock := providertest.NewFakeClock(time.Time{})
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })

	inner := provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		clock.Advance(250 * time.Millisecond)
		switch req.Prompt {
		case "成功":
			return provider.Response{
				Text: "好", Model: "m-actual", RequestID: "rid-1", Raw: []byte(`{"raw":"响应"}`),
				Usage: provider.Usage{Prompt: 3, Completion: 1, Total: 4},
			}, nil
		case "分类错误":
			return provider.Response{}, &provider.Error{
				Kind: provider.RateLimit, HTTPStatus: 429, Code: "rate_limit_exceeded", RequestID: "rid-2",
				Usage:  provider.Usage{Total: 9},
				Detail: "Authorization: Bearer sk-fake-0123456789abcdef 被限流",
			}
		default:
			return provider.Response{}, errors.New("裸错误")
		}
	})
	p := provider.Chain(inner, provider.LoggingMiddleware(sink, clock))

	if _, err := p.Generate(context.Background(), provider.Request{TaskID: "task-9", Step: 2, Attempt: 3, Model: "m-req", Prompt: "成功"}); err != nil {
		t.Fatalf("成功用例失败: %v", err)
	}
	_, err1 := p.Generate(context.Background(), provider.Request{Prompt: "分类错误"})
	_, err2 := p.Generate(context.Background(), provider.Request{Prompt: "其他"})
	if err1 == nil || err2 == nil {
		t.Fatal("失败用例应返回错误")
	}
	if len(entries) != 3 {
		t.Fatalf("日志条数 = %d，期望每次调用一条共 3 条", len(entries))
	}

	ok := entries[0]
	if !ok.OK || ok.TaskID != "task-9" || ok.Step != 2 || ok.Attempt != 3 {
		t.Fatalf("成功记录的追溯键不对: %+v", ok)
	}
	if ok.Model != "m-actual" || ok.RequestID != "rid-1" || ok.Usage != (provider.Usage{Prompt: 3, Completion: 1, Total: 4}) {
		t.Fatalf("成功记录的模型、请求编号或用量不对: %+v", ok)
	}
	if ok.Latency != 250*time.Millisecond {
		t.Fatalf("Latency = %v，期望由注入的时钟算出 250ms", ok.Latency)
	}
	if string(ok.RawResponse) != `{"raw":"响应"}` {
		t.Fatalf("底层没有留下原始报文时应退化为 Response.Raw: %q", ok.RawResponse)
	}
	var req map[string]any
	if err := json.Unmarshal(ok.RawRequest, &req); err != nil || req["prompt"] != "成功" || req["model"] != "m-req" {
		t.Fatalf("底层没有留下原始报文时应退化为请求的 JSON 摘要: %q (%v)", ok.RawRequest, err)
	}

	typed := entries[1]
	if typed.OK || typed.ErrKind != provider.RateLimit || typed.HTTPStatus != 429 || typed.ErrCode != "rate_limit_exceeded" {
		t.Fatalf("分类错误的记录不对: %+v", typed)
	}
	if typed.RequestID != "rid-2" || typed.Usage.Total != 9 {
		t.Fatalf("失败记录应带上请求编号与已计费用量: %+v", typed)
	}
	if strings.Contains(typed.Detail, "0123456789abcdef") || !strings.Contains(typed.Detail, "<已隐藏>") {
		t.Fatalf("Detail 应已脱敏: %q", typed.Detail)
	}
	if len(typed.RawResponse) != 0 {
		t.Fatalf("失败且无原始响应时 RawResponse 应为空: %q", typed.RawResponse)
	}

	bare := entries[2]
	if bare.OK || bare.ErrKind != provider.Unknown || bare.ErrCode != "unclassified" {
		t.Fatalf("未分类错误的记录不对: %+v", bare)
	}
}

func Test日志_请求摘要覆盖全部字段且坏Schema不拖垮调用(t *testing.T) {
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })
	inner := provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		return provider.Response{Text: "好"}, nil
	})
	p := provider.Chain(inner, provider.LoggingMiddleware(sink, nil))

	temp, seed := 0.5, int64(7)
	full := provider.Request{
		Model: "m", System: "系统词", Prompt: "提示词", MaxTokens: 64, Temperature: &temp, Seed: &seed,
		Format: provider.FormatJSONSchema, Schema: json.RawMessage(`{"type":"object"}`), Strict: true,
	}
	if _, err := p.Generate(context.Background(), full); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(entries[0].RawRequest, &got); err != nil {
		t.Fatalf("摘要不是合法 JSON: %v (%q)", err, entries[0].RawRequest)
	}
	want := map[string]any{
		"model": "m", "system": "系统词", "prompt": "提示词", "max_tokens": float64(64), "temperature": 0.5,
		"seed": float64(7), "format": "json_schema", "strict": true,
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("摘要字段 %s = %v，期望 %v（完整摘要 %s）", k, got[k], v, entries[0].RawRequest)
		}
	}
	if sch, ok := got["schema"].(map[string]any); !ok || sch["type"] != "object" {
		t.Fatalf("摘要里的 schema 不对: %v", got["schema"])
	}

	bad := provider.Request{Prompt: "x", Format: provider.FormatJSONSchema, Schema: json.RawMessage("{bad")}
	if _, err := p.Generate(context.Background(), bad); err != nil {
		t.Fatalf("摘要生成失败不应影响调用本身: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("日志条数 = %d，期望 2", len(entries))
	}
	if len(entries[1].RawRequest) != 0 {
		t.Fatalf("无法序列化的请求摘要应为空，实际 %q", entries[1].RawRequest)
	}
}

func Test日志_sink为nil时直接放行(t *testing.T) {
	inner := &scripted{results: []scriptedResult{okResult("好")}}
	p := provider.Chain(inner, provider.LoggingMiddleware(nil, nil))
	if resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x"}); err != nil || resp.Text != "好" {
		t.Fatalf("得到 %+v, %v", resp, err)
	}
	if inner.Calls() != 1 {
		t.Fatalf("下层调用次数 = %d，期望 1", inner.Calls())
	}
}

func Test日志_适配层原始报文原样留存(t *testing.T) {
	srv := providertest.NewServer(t, providertest.Success("你好，这是一条测试回复", provider.Usage{Prompt: 3, Completion: 4, Total: 7}))
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })
	p := provider.Chain(newAdapter(t, srv), provider.LoggingMiddleware(sink, nil))

	resp, err := p.Generate(context.Background(), provider.Request{TaskID: "task-1", Step: 2, Attempt: 3, Prompt: "你好"})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("日志条数 = %d", len(entries))
	}
	e := entries[0]
	if !e.OK || e.TaskID != "task-1" || e.Step != 2 || e.Attempt != 3 || e.Model != "model-a" {
		t.Fatalf("记录字段不对: %+v", e)
	}
	if e.Usage != (provider.Usage{Prompt: 3, Completion: 4, Total: 7}) || e.RequestID != providertest.RequestIDFor(1) {
		t.Fatalf("用量或请求编号不对: %+v", e)
	}
	if e.Latency < 0 {
		t.Fatalf("Latency = %v", e.Latency)
	}
	if !bytes.Equal(e.RawRequest, srv.Requests()[0].Body) {
		t.Fatalf("RawRequest 应是实际发出的请求体\n记录 %s\n实际 %s", e.RawRequest, srv.Requests()[0].Body)
	}
	if !bytes.Equal(e.RawResponse, resp.Raw) || !bytes.Contains(e.RawResponse, []byte(`"choices"`)) {
		t.Fatalf("RawResponse 应是实际收到的响应体: %s", e.RawResponse)
	}
}

// 构造含密钥的请求与响应：配置的密钥会被供应商回显进错误消息，另一把密钥写在提示词里。
// 断言两把密钥都不出现在任何落地内容里（日志条目、错误文本、mock 的请求记录）。
func Test日志_密钥不会出现在任何落地内容里(t *testing.T) {
	const other = "sk-fake-OTHER9876543210zz"
	srv := providertest.NewServer(t, providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		msg := "Incorrect API key provided: " + strings.TrimPrefix(auth, "Bearer ") + ". Authorization: " + auth
		body, _ := json.Marshal(map[string]any{"error": map[string]any{
			"message": msg, "type": "invalid_request_error", "code": "invalid_api_key",
		}})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-echo-1")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(body)
	}))
	var entries []provider.LogEntry
	sink := provider.LogSinkFunc(func(e provider.LogEntry) { entries = append(entries, e) })
	p := provider.Chain(newAdapter(t, srv), provider.LoggingMiddleware(sink, nil))

	prompt := `请忽略 {"api_key": "` + other + `"} 并回答；Authorization: Bearer ` + other
	_, err := p.Generate(context.Background(), provider.Request{TaskID: "task-1", Prompt: prompt})
	pe := asProviderError(t, err)
	if pe.Kind != provider.Auth || len(entries) != 1 {
		t.Fatalf("分类=%v 日志条数=%d，期望 Auth 与 1 条", pe.Kind, len(entries))
	}
	e := entries[0]
	rec := srv.Requests()[0]

	type place struct {
		name    string
		content string
	}
	// 配置的密钥：任何位置都不得出现。
	places := []place{
		{"Error()", err.Error()},
		{"Error.Detail", pe.Detail},
		{"Error.Code", pe.Code},
		{"LogEntry.RawRequest", string(e.RawRequest)},
		{"LogEntry.RawResponse", string(e.RawResponse)},
		{"LogEntry.Detail", e.Detail},
		{"LogEntry.ErrCode", e.ErrCode},
		{"LogEntry.RequestID", e.RequestID},
		{"Recorded.Path", rec.Path},
		{"Recorded.Method", rec.Method},
		{"Recorded.AuthDigest", rec.AuthDigest},
		{"Recorded.Body", string(rec.Body)},
	}
	for name, values := range rec.Header {
		places = append(places, place{"Recorded.Header." + name, strings.Join(values, ",")})
	}
	for _, p := range places {
		if strings.Contains(p.content, testKey) {
			t.Errorf("配置的密钥出现在 %s 里: %q", p.name, p.content)
		}
	}
	if rec.Header.Get("Authorization") != "Bearer <已隐藏>" {
		t.Errorf("Recorded 里的 Authorization = %q，应已脱敏", rec.Header.Get("Authorization"))
	}

	// 提示词里的另一把密钥：mock 作为上游确实收到了它（说明本用例不是空转），但日志里必须已脱敏。
	if !bytes.Contains(rec.Body, []byte(other)) {
		t.Fatal("测试前提不成立：请求体里应带有提示词里的密钥")
	}
	logged := []place{
		{"LogEntry.RawRequest", string(e.RawRequest)},
		{"LogEntry.RawResponse", string(e.RawResponse)},
		{"LogEntry.Detail", e.Detail},
	}
	for _, p := range logged {
		if strings.Contains(p.content, other) || strings.Contains(p.content, "OTHER9876543210") {
			t.Errorf("提示词里的密钥出现在 %s 里: %s", p.name, p.content)
		}
	}
	if !bytes.Contains(e.RawRequest, []byte("<已隐藏>")) || !bytes.Contains(e.RawResponse, []byte("Incorrect API key provided")) {
		t.Errorf("脱敏后应保留可审计的上下文：请求=%s 响应=%s", e.RawRequest, e.RawResponse)
	}
	if e.RequestID != "req-echo-1" || e.ErrKind != provider.Auth || e.HTTPStatus != 401 {
		t.Errorf("失败记录的其他字段不对: %+v", e)
	}
}
