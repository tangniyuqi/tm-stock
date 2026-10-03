package providertest

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// hiddenBearer 是 Recorded 里 Authorization 头被替换成的固定值。
const hiddenBearer = "Bearer <已隐藏>"

// Recorded 是上游收到的一个请求的记录。其中的 Authorization 等密钥类头已脱敏。
type Recorded struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
	// AuthDigest 是原始 Authorization 头的 SHA-256 十六进制摘要，没有该头则为空。
	// 测试用它核对“发出的密钥正确”，而不必把密钥留在记录里，比较用 AuthDigest 函数计算期望值。
	AuthDigest string
}

// AuthDigest 计算 Authorization 头原始值的 SHA-256 十六进制摘要，与 Recorded.AuthDigest 同口径。
func AuthDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// RequestIDFor 返回 mock 为第 n 个请求（从 1 起）分配的请求编号，
// 同时写在响应体的 id 与响应头 X-Request-Id 里。
func RequestIDFor(n int) string {
	return "mock-req-" + strconv.Itoa(n)
}

type stepFunc func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request)

// Step 是场景脚本里的一步，决定一个请求如何被应答。用本包的构造函数创建。
type Step struct {
	name string
	run  stepFunc
}

// String 返回步骤名，便于排错。
func (s Step) String() string { return s.name }

// Server 是按请求出队的 mock 上游。第 N 个到达的请求由脚本的第 N 步应答；
// 脚本耗尽后再来的请求得到 500，并让测试记录一次错误。
type Server struct {
	tb  testing.TB
	srv *httptest.Server

	mu    sync.Mutex
	steps []Step
	next  int
	recs  []Recorded

	done chan struct{}
	once sync.Once
}

// NewServer 启动 mock 服务并在 tb.Cleanup 里关闭它。URL() 可直接作为适配层的 BaseURL，
// mock 不挑路径，收到的路径会被记录。
func NewServer(tb testing.TB, steps ...Step) *Server {
	tb.Helper()
	s := &Server{
		tb:    tb,
		steps: slices.Clone(steps),
		done:  make(chan struct{}),
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	tb.Cleanup(s.Close)
	return s
}

// URL 返回服务根地址，形如 http://127.0.0.1:端口，不带路径。
func (s *Server) URL() string { return s.srv.URL }

// Count 返回上游收到的请求总数，含脚本耗尽后多出来的请求。
func (s *Server) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.recs)
}

// Requests 返回按到达顺序排列的请求记录副本。
func (s *Server) Requests() []Recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.recs)
}

// WaitCount 等待上游收到至少 n 个请求，最多等 timeout，返回是否达成。
// 用来让“请求已经到达服务端”成为取消 ctx 之类动作的前提，避免靠 Sleep 猜时机。
func (s *Server) WaitCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if s.Count() >= n {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// Close 关闭服务，可重复调用。会先放行所有阻塞中的 Hang、SlowHeader，再等在途请求结束。
// 关闭后再向它发请求会得到连接被拒。
func (s *Server) Close() {
	s.once.Do(func() {
		close(s.done)
		s.srv.Close()
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.recs = append(s.recs, newRecorded(r, body))
	n := len(s.recs)
	var (
		step Step
		have bool
	)
	if s.next < len(s.steps) {
		step, have = s.steps[s.next], true
	}
	s.next++
	total := len(s.steps)
	s.mu.Unlock()

	if !have {
		s.tb.Errorf("providertest: 脚本已耗尽（共 %d 步），却收到第 %d 个请求 %s %s",
			total, n, r.Method, r.URL.Path)
		http.Error(w, "providertest: 脚本已耗尽", http.StatusInternalServerError)
		return
	}
	step.run(s, n, body, w, r)
}

func newRecorded(r *http.Request, body []byte) Recorded {
	h := r.Header.Clone()
	digest := ""
	if v := h.Get("Authorization"); v != "" {
		digest = AuthDigest(v)
		h.Set("Authorization", hiddenBearer)
	}
	for _, name := range []string{"Proxy-Authorization", "X-Api-Key", "Api-Key"} {
		if h.Get(name) != "" {
			h.Set(name, "<已隐藏>")
		}
	}
	return Recorded{
		Method:     r.Method,
		Path:       r.URL.Path,
		Header:     h,
		Body:       slices.Clone(body),
		AuthDigest: digest,
	}
}

// requestModel 取请求体里的 model 字段，mock 把它原样回显在响应里。
func requestModel(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &req) == nil && req.Model != "" {
		return req.Model
	}
	return "mock-model"
}

// completionJSON 生成 OpenAI 兼容的成功响应体。usage 为 nil 时不带 usage 字段。
func completionJSON(n int, model, text, finish string, usage *provider.Usage) []byte {
	body := map[string]any{
		"id":      RequestIDFor(n),
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": text},
				"finish_reason": finish,
			},
		},
	}
	if usage != nil {
		u := map[string]any{
			"prompt_tokens":     usage.Prompt,
			"completion_tokens": usage.Completion,
			"total_tokens":      usage.Total,
		}
		if usage.Cached > 0 {
			u["prompt_tokens_details"] = map[string]any{"cached_tokens": usage.Cached}
		}
		if usage.Reasoning > 0 {
			u["completion_tokens_details"] = map[string]any{"reasoning_tokens": usage.Reasoning}
		}
		body["usage"] = u
	}
	out, _ := json.Marshal(body)
	return out
}

func writeJSON(w http.ResponseWriter, n, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", RequestIDFor(n))
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

// Success 返回 200 与 OpenAI 兼容的成功响应：finish_reason 为 stop，带 usage，
// id 与 X-Request-Id 取 RequestIDFor(第几个请求)，model 回显请求里的 model。
func Success(text string, usage provider.Usage) Step {
	u := usage
	return Step{name: "Success", run: func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, n, http.StatusOK, completionJSON(n, requestModel(body), text, "stop", &u))
	}}
}

// SuccessNoUsage 同 Success，但响应里没有 usage 字段，用来测试用量估算。
func SuccessNoUsage(text string) Step {
	return Step{name: "SuccessNoUsage", run: func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, n, http.StatusOK, completionJSON(n, requestModel(body), text, "stop", nil))
	}}
}

// Status 返回指定状态码与响应体（Content-Type 为 application/json），
// 用来构造各种供应商错误体、或 2xx 的自定义响应体。
func Status(code int, body string) Step {
	return Step{name: "Status(" + strconv.Itoa(code) + ")", run: func(s *Server, n int, _ []byte, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, n, code, []byte(body))
	}}
}

// RateLimited 返回 429 与限流错误体；retryAfter 非空时带 Retry-After 头，原样写入，
// 所以既可以是秒数也可以是 HTTP-date。
func RateLimited(retryAfter string) Step {
	const body = `{"error":{"message":"请求过于频繁","type":"rate_limit_error","code":"rate_limit_exceeded"}}`
	return Step{name: "RateLimited", run: func(s *Server, n int, _ []byte, w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		writeJSON(w, n, http.StatusTooManyRequests, []byte(body))
	}}
}

// MalformedKind 是 Malformed 步骤制造的畸形响应种类。
type MalformedKind int

const (
	// TruncatedJSON 状态 200，响应体是一段被截断的 JSON（长度头与实际一致，所以客户端读得完整却解析不了）。
	TruncatedJSON MalformedKind = iota + 1
	// EmptyChoices 状态 200，choices 为空数组。
	EmptyChoices
	// EmptyContent 状态 200，content 为空字符串。
	EmptyContent
	// NullContent 状态 200，content 为 null。
	NullContent
	// NotJSON 状态 200，响应体是一段 HTML，不是 JSON。
	NotJSON
)

// Malformed 返回一个 2xx 但不可用的响应，种类见 MalformedKind。未知种类按 NotJSON 处理。
func Malformed(kind MalformedKind) Step {
	return Step{name: "Malformed", run: func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request) {
		model := requestModel(body)
		usage := &provider.Usage{Prompt: 3, Completion: 0, Total: 3}
		switch kind {
		case TruncatedJSON:
			full := completionJSON(n, model, "你好，这是一条测试回复", "stop", usage)
			writeJSON(w, n, http.StatusOK, full[:len(full)/2])
		case EmptyChoices:
			writeJSON(w, n, http.StatusOK, []byte(`{"id":"`+RequestIDFor(n)+`","object":"chat.completion","model":"`+model+`","choices":[]}`))
		case EmptyContent:
			writeJSON(w, n, http.StatusOK, completionJSON(n, model, "", "stop", usage))
		case NullContent:
			writeJSON(w, n, http.StatusOK, []byte(`{"id":"`+RequestIDFor(n)+`","object":"chat.completion","model":"`+model+`","choices":[{"index":0,"message":{"role":"assistant","content":null},"finish_reason":"stop"}]}`))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body>gateway error</body></html>"))
		}
	}}
}

// Hang 让请求一直阻塞，直到客户端断开（ctx 取消或超时）或服务被关闭。
func Hang() Step {
	return Step{name: "Hang", run: func(s *Server, n int, _ []byte, w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-s.done:
		}
	}}
}

// SlowHeader 先睡 d 再应答，期间客户端断开或服务关闭则直接放弃。
// then 指定睡醒后用哪一步应答，不给则返回一个普通成功响应。
func SlowHeader(d time.Duration, then ...Step) Step {
	return Step{name: "SlowHeader", run: func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		}
		next := Success("慢响应", provider.Usage{Prompt: 1, Completion: 1, Total: 2})
		if len(then) > 0 {
			next = then[0]
		}
		next.run(s, n, body, w, r)
	}}
}

// Abort 先写出状态行、响应头和半截响应体（长度头声明的比实际发出的多），然后劫持并断开连接。
// 客户端会拿到 200 的响应头，随后读响应体时遇到提前结束。
func Abort() Step {
	return Step{name: "Abort", run: func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request) {
		conn, rw, err := hijack(w)
		if err != nil {
			s.tb.Errorf("providertest: Abort 需要可劫持的连接: %v", err)
			return
		}
		full := completionJSON(n, requestModel(body), "你好，这是一条测试回复", "stop", nil)
		part := full[:len(full)/2]
		_, _ = rw.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: " +
			strconv.Itoa(len(part)+100) + "\r\n\r\n")
		_, _ = rw.Write(part)
		_ = rw.Flush()
		gracefulClose(conn)
	}}
}

// Drop 收到请求后劫持连接并直接断开，不写出任何响应字节。
func Drop() Step {
	return Step{name: "Drop", run: func(s *Server, n int, _ []byte, w http.ResponseWriter, r *http.Request) {
		conn, _, err := hijack(w)
		if err != nil {
			s.tb.Errorf("providertest: Drop 需要可劫持的连接: %v", err)
			return
		}
		gracefulClose(conn)
	}}
}

// Redirect 返回重定向响应（code 通常是 301、302、307、308）。
// 用来证明客户端没有跟随重定向：目标地址不应收到任何请求。
func Redirect(code int, location string) Step {
	return Step{name: "Redirect(" + strconv.Itoa(code) + ")", run: func(s *Server, n int, _ []byte, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(code)
	}}
}

// Handler 用自定义处理函数应答，处理函数里仍可读取完整的请求体。
// 用来构造脚本之外的特殊响应，例如把请求头回显进响应体。
func Handler(h http.HandlerFunc) Step {
	return Step{name: "Handler", run: func(s *Server, n int, body []byte, w http.ResponseWriter, r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(body))
		h(w, r)
	}}
}

func hijack(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("ResponseWriter 不支持 Hijack")
	}
	return hj.Hijack()
}

// gracefulClose 先半关闭写端（客户端会读到已写出的数据再遇到 EOF），
// 等客户端关闭连接后再整体关闭，避免带着未读数据关闭而触发 RST 把已写出的数据冲掉。
func gracefulClose(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, _ = io.Copy(io.Discard, conn)
	_ = conn.Close()
}
