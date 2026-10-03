package providertest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

const fakeKey = "sk-fake-0123456789abcdef"

// newClient 返回不复用连接、不跟随重定向的客户端，让每个测试的网络行为互不影响。
func newClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// doPost 向 mock 发一个 POST 并读完响应体。
func doPost(t *testing.T, srv *Server, path, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL()+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+fakeKey)
	resp, err := newClient(10 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读响应体失败: %v", err)
	}
	return resp, data
}

// recordingTB 把 Errorf 记录下来而不是让测试失败，用来验证 mock 会在脚本耗尽时报错。
// 内嵌真实的 testing.TB 以满足接口里的不可导出方法。
type recordingTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
	r.mu.Unlock()
}

func (r *recordingTB) Cleanup(func()) {}

func (r *recordingTB) Helper() {}

func (r *recordingTB) errors() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.errs...)
}

func Test场景_成功响应(t *testing.T) {
	usage := provider.Usage{Prompt: 1, Completion: 2, Total: 3, Cached: 1, Reasoning: 1}
	srv := NewServer(t, Success("你好", usage), Success("再见", provider.Usage{Total: 1}))

	resp, data := doPost(t, srv, "/v1/chat/completions", `{"model":"m1","messages":[]}`)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("状态=%d Content-Type=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Request-Id") != RequestIDFor(1) {
		t.Fatalf("X-Request-Id = %q", resp.Header.Get("X-Request-Id"))
	}
	var body struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
			PromptDetails    struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v\n%s", err, data)
	}
	if body.ID != RequestIDFor(1) || body.Model != "m1" || len(body.Choices) != 1 {
		t.Fatalf("响应体 = %s", data)
	}
	if c := body.Choices[0]; c.Message.Role != "assistant" || c.Message.Content != "你好" || c.FinishReason != "stop" {
		t.Fatalf("choices[0] = %+v", c)
	}
	u := body.Usage
	if u.PromptTokens != 1 || u.CompletionTokens != 2 || u.TotalTokens != 3 || u.PromptDetails.CachedTokens != 1 || u.CompletionDetails.ReasoningTokens != 1 {
		t.Fatalf("usage = %+v", u)
	}

	// 请求里没有 model 时回显默认模型名；没有缓存与推理用量时不带 details。
	resp, data = doPost(t, srv, "/x", `{}`)
	if resp.Header.Get("X-Request-Id") != RequestIDFor(2) {
		t.Fatalf("第二个请求的 X-Request-Id = %q", resp.Header.Get("X-Request-Id"))
	}
	if !strings.Contains(string(data), `"model":"mock-model"`) || strings.Contains(string(data), "prompt_tokens_details") {
		t.Fatalf("第二个响应体 = %s", data)
	}
}

func Test场景_成功响应不带用量(t *testing.T) {
	srv := NewServer(t, SuccessNoUsage("好"))
	_, data := doPost(t, srv, "/", `{"model":"m"}`)
	if strings.Contains(string(data), `"usage"`) || !strings.Contains(string(data), `"content":"好"`) {
		t.Fatalf("响应体 = %s", data)
	}
}

func Test场景_Status与RateLimited(t *testing.T) {
	srv := NewServer(t, Status(418, "我是茶壶"), RateLimited("7"), RateLimited(""))

	resp, data := doPost(t, srv, "/", "{}")
	if resp.StatusCode != 418 || string(data) != "我是茶壶" || resp.Header.Get("X-Request-Id") != RequestIDFor(1) {
		t.Fatalf("Status 步骤: 状态=%d 体=%q 编号=%q", resp.StatusCode, data, resp.Header.Get("X-Request-Id"))
	}

	resp, data = doPost(t, srv, "/", "{}")
	if resp.StatusCode != 429 || resp.Header.Get("Retry-After") != "7" || !strings.Contains(string(data), "rate_limit_exceeded") {
		t.Fatalf("RateLimited(7): 状态=%d Retry-After=%q 体=%s", resp.StatusCode, resp.Header.Get("Retry-After"), data)
	}

	resp, _ = doPost(t, srv, "/", "{}")
	if resp.StatusCode != 429 {
		t.Fatalf("RateLimited(空) 状态 = %d", resp.StatusCode)
	}
	if _, has := resp.Header["Retry-After"]; has {
		t.Fatal("retryAfter 为空时不应带 Retry-After 头")
	}
}

func Test场景_畸形响应(t *testing.T) {
	srv := NewServer(t,
		Malformed(TruncatedJSON), Malformed(EmptyChoices), Malformed(EmptyContent),
		Malformed(NullContent), Malformed(NotJSON), Malformed(MalformedKind(99)),
	)

	_, data := doPost(t, srv, "/", `{"model":"m"}`)
	if json.Valid(data) || len(data) == 0 {
		t.Fatalf("TruncatedJSON 应返回非法 JSON，实际 %q", data)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content *string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	_, data = doPost(t, srv, "/", `{"model":"m"}`)
	if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Choices) != 0 {
		t.Fatalf("EmptyChoices 应是合法 JSON 且 choices 为空: %s (%v)", data, err)
	}

	_, data = doPost(t, srv, "/", `{"model":"m"}`)
	parsed.Choices = nil
	if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Choices) != 1 ||
		parsed.Choices[0].Message.Content == nil || *parsed.Choices[0].Message.Content != "" {
		t.Fatalf("EmptyContent 应是 content 为空串: %s (%v)", data, err)
	}

	_, data = doPost(t, srv, "/", `{"model":"m"}`)
	parsed.Choices = nil
	if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Choices) != 1 || parsed.Choices[0].Message.Content != nil {
		t.Fatalf("NullContent 应是 content 为 null: %s (%v)", data, err)
	}

	for i, name := range []string{"NotJSON", "未知种类"} {
		resp, data := doPost(t, srv, "/", `{"model":"m"}`)
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") || json.Valid(data) {
			t.Fatalf("%s(第 %d 个): 状态=%d 类型=%q 体=%q，期望 200 的 HTML", name, i+1, resp.StatusCode, resp.Header.Get("Content-Type"), data)
		}
	}
}

func Test场景_Hang直到客户端断开(t *testing.T) {
	srv := NewServer(t, Hang())
	start := time.Now()
	req, _ := http.NewRequest(http.MethodPost, srv.URL(), strings.NewReader("{}"))
	if _, err := newClient(300 * time.Millisecond).Do(req); err == nil {
		t.Fatal("Hang 应让客户端等到超时")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("客户端超时没有生效")
	}
	if srv.Count() != 1 {
		t.Fatalf("Count = %d，期望 1", srv.Count())
	}
}

func Test场景_Abort写出半截响应后断开(t *testing.T) {
	srv := NewServer(t, Abort())
	req, _ := http.NewRequest(http.MethodPost, srv.URL(), strings.NewReader(`{"model":"m"}`))
	resp, err := newClient(10 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("Abort 应先让客户端拿到响应头，实际请求就失败了: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("状态 = %d，期望 200", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err == nil {
		t.Fatal("读响应体时应遇到提前结束")
	}
	if len(data) == 0 || int64(len(data)) >= resp.ContentLength {
		t.Fatalf("应收到半截响应体：收到 %d 字节，声明 %d 字节", len(data), resp.ContentLength)
	}
	if srv.Count() != 1 {
		t.Fatalf("Count = %d，期望 1", srv.Count())
	}
}

func Test场景_Drop不写任何响应就断开(t *testing.T) {
	srv := NewServer(t, Drop())
	req, _ := http.NewRequest(http.MethodPost, srv.URL(), strings.NewReader("{}"))
	if resp, err := newClient(10 * time.Second).Do(req); err == nil {
		resp.Body.Close()
		t.Fatal("Drop 应让客户端的请求失败")
	}
	if srv.Count() != 1 {
		t.Fatalf("Count = %d，期望 1", srv.Count())
	}
}

func Test场景_SlowHeader先睡再应答(t *testing.T) {
	srv := NewServer(t, SlowHeader(60*time.Millisecond), SlowHeader(10*time.Millisecond, Status(201, "组合")), SlowHeader(5*time.Second))

	start := time.Now()
	resp, data := doPost(t, srv, "/", `{"model":"m"}`)
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Fatalf("只等了 %v，应先睡满 60ms", elapsed)
	}
	if resp.StatusCode != 200 || !strings.Contains(string(data), "慢响应") {
		t.Fatalf("默认应答: 状态=%d 体=%s", resp.StatusCode, data)
	}

	resp, data = doPost(t, srv, "/", "{}")
	if resp.StatusCode != 201 || string(data) != "组合" {
		t.Fatalf("组合 Status 步骤: 状态=%d 体=%q", resp.StatusCode, data)
	}

	// 客户端在睡眠期间放弃：服务端应立即放弃，而不是睡满 5 秒。
	req, _ := http.NewRequest(http.MethodPost, srv.URL(), strings.NewReader("{}"))
	if _, err := newClient(100 * time.Millisecond).Do(req); err == nil {
		t.Fatal("客户端应超时")
	}
}

func Test场景_Redirect与Handler(t *testing.T) {
	type seen struct {
		body string
		auth string
	}
	captured := make(chan seen, 1) // 用通道把处理函数里看到的内容交给测试，避免跨 goroutine 共享变量
	srv := NewServer(t,
		Redirect(302, "http://127.0.0.1:1/elsewhere"),
		Handler(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			captured <- seen{body: string(b), auth: r.Header.Get("Authorization")}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("自定义"))
		}),
	)

	resp, _ := doPost(t, srv, "/", "{}")
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "http://127.0.0.1:1/elsewhere" {
		t.Fatalf("Redirect: 状态=%d Location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp, data := doPost(t, srv, "/", `{"k":"v"}`)
	if resp.StatusCode != 202 || string(data) != "自定义" {
		t.Fatalf("Handler: 状态=%d 体=%q", resp.StatusCode, data)
	}
	got := <-captured
	if got.body != `{"k":"v"}` || got.auth != "Bearer "+fakeKey {
		t.Fatalf("Handler 应能读到完整请求体与请求头: %+v", got)
	}
}

func Test记录_请求与脱敏(t *testing.T) {
	srv := NewServer(t, Status(200, "{}"), Status(200, "{}"))

	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/v1/chat/completions?x=1", strings.NewReader(`{"model":"m"}`))
	req.Header.Set("Authorization", "Bearer "+fakeKey)
	req.Header.Set("Proxy-Authorization", "Basic ZmFrZTpmYWtl")
	req.Header.Set("X-Api-Key", fakeKey)
	req.Header.Set("Api-Key", fakeKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := newClient(10 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()

	req2, _ := http.NewRequest(http.MethodPost, srv.URL()+"/second", strings.NewReader("{}"))
	resp, err = newClient(10 * time.Second).Do(req2)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()

	recs := srv.Requests()
	if len(recs) != 2 || srv.Count() != 2 {
		t.Fatalf("记录数 = %d，Count = %d，期望都是 2", len(recs), srv.Count())
	}
	r := recs[0]
	if r.Method != http.MethodPost || r.Path != "/v1/chat/completions" || string(r.Body) != `{"model":"m"}` {
		t.Fatalf("记录 = %s %s %q", r.Method, r.Path, r.Body)
	}
	if r.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("普通请求头应原样记录: %v", r.Header)
	}
	for _, name := range []string{"Authorization", "Proxy-Authorization", "X-Api-Key", "Api-Key"} {
		got := r.Header.Get(name)
		if strings.Contains(got, fakeKey) || strings.Contains(got, "ZmFrZTpmYWtl") {
			t.Fatalf("%s 头没有脱敏: %q", name, got)
		}
	}
	if r.Header.Get("Authorization") != "Bearer <已隐藏>" {
		t.Fatalf("Authorization = %q，期望 Bearer <已隐藏>", r.Header.Get("Authorization"))
	}
	if r.AuthDigest != AuthDigest("Bearer "+fakeKey) || r.AuthDigest == AuthDigest("Bearer other") {
		t.Fatalf("AuthDigest = %q", r.AuthDigest)
	}
	if strings.Contains(r.AuthDigest, fakeKey) {
		t.Fatal("摘要里不得出现密钥")
	}

	second := recs[1]
	if second.Header.Get("Authorization") != "" || second.AuthDigest != "" {
		t.Fatalf("没有 Authorization 头的请求不应凭空出现该头: %v digest=%q", second.Header, second.AuthDigest)
	}

	// Requests 返回副本：修改它不影响内部记录。
	recs[0] = Recorded{}
	if again := srv.Requests(); again[0].Path != "/v1/chat/completions" {
		t.Fatalf("Requests() 应返回副本，内部记录被改了: %+v", again[0])
	}
}

func Test脚本耗尽_让测试记录错误且仍计数(t *testing.T) {
	fake := &recordingTB{TB: t}
	srv := NewServer(fake, Status(200, "{}"))
	defer srv.Close()

	resp, _ := doPost(t, srv, "/ok", "{}")
	if resp.StatusCode != 200 {
		t.Fatalf("脚本内的请求状态 = %d", resp.StatusCode)
	}
	if len(fake.errors()) != 0 {
		t.Fatalf("脚本未耗尽时不应记录错误: %v", fake.errors())
	}

	resp, _ = doPost(t, srv, "/extra", "{}")
	if resp.StatusCode != 500 {
		t.Fatalf("脚本耗尽后的请求状态 = %d，期望 500", resp.StatusCode)
	}
	errs := fake.errors()
	if len(errs) != 1 || !strings.Contains(errs[0], "脚本已耗尽") || !strings.Contains(errs[0], "/extra") {
		t.Fatalf("应记录一次说明脚本已耗尽的错误，实际 %v", errs)
	}

	doPost(t, srv, "/extra2", "{}")
	if len(fake.errors()) != 2 {
		t.Fatalf("每个多出来的请求都应记录一次错误，实际 %v", fake.errors())
	}
	if srv.Count() != 3 {
		t.Fatalf("Count = %d，期望 3（含脚本耗尽后多出来的请求）", srv.Count())
	}
}

func Test服务_Close幂等且关闭后连接被拒(t *testing.T) {
	srv := NewServer(t)
	srv.Close()
	srv.Close() // 再关一次不应 panic，t.Cleanup 里还会关第三次

	req, _ := http.NewRequest(http.MethodPost, srv.URL(), strings.NewReader("{}"))
	if resp, err := newClient(5 * time.Second).Do(req); err == nil {
		resp.Body.Close()
		t.Fatal("关闭后的服务不应再接受连接")
	}
}

func Test服务_WaitCount(t *testing.T) {
	srv := NewServer(t, Status(200, "{}"))
	if srv.WaitCount(1, 20*time.Millisecond) {
		t.Fatal("还没有请求时 WaitCount(1) 应在超时后返回 false")
	}
	if !srv.WaitCount(0, 0) {
		t.Fatal("WaitCount(0) 应立即返回 true")
	}
	doPost(t, srv, "/", "{}")
	if !srv.WaitCount(1, time.Second) {
		t.Fatal("已有 1 个请求时 WaitCount(1) 应返回 true")
	}
}

func Test服务_并发请求按序出队(t *testing.T) {
	const workers = 20
	steps := make([]Step, workers)
	for i := range steps {
		steps[i] = Status(200, "{}")
	}
	srv := NewServer(t, steps...)

	var wg sync.WaitGroup
	codes := make(chan int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, srv.URL(), strings.NewReader("{}"))
			resp, err := newClient(10 * time.Second).Do(req)
			if err != nil {
				codes <- -1
				return
			}
			resp.Body.Close()
			codes <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Errorf("并发请求得到状态 %d，期望 200", code)
		}
	}
	if srv.Count() != workers {
		t.Fatalf("Count = %d，期望 %d", srv.Count(), workers)
	}
}

func Test请求编号与步骤名(t *testing.T) {
	if RequestIDFor(3) != "mock-req-3" {
		t.Fatalf("RequestIDFor(3) = %q", RequestIDFor(3))
	}
	if got := Status(404, "").String(); got != "Status(404)" {
		t.Fatalf("Status(404).String() = %q", got)
	}
	if got := Hang().String(); got != "Hang" {
		t.Fatalf("Hang().String() = %q", got)
	}
}
