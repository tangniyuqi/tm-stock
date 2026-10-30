//go:build integration

package points_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func localAPIWorkerH5Configuration(mode, depth, origin string) bool {
	u, err := h5TestOrigin(origin)
	return mode == "local-fixture-only" && (depth == "light" || depth == "deep") && err == nil && u.Hostname() == "127.0.0.1"
}

func TestLocalAPIWorkerH5Configuration(t *testing.T) {
	for _, row := range []struct {
		name, mode, depth, origin string
		want                      bool
	}{
		{"轻度", "local-fixture-only", "light", "http://127.0.0.1:5184", true},
		{"深度", "local-fixture-only", "deep", "http://127.0.0.1:5184", true},
		{"缺模式", "", "light", "http://127.0.0.1:5184", false},
		{"缺深度", "local-fixture-only", "", "http://127.0.0.1:5184", false},
		{"错深度", "local-fixture-only", "unknown", "http://127.0.0.1:5184", false},
		{"外部源", "local-fixture-only", "deep", "https://example.test", false},
		{"域名源", "local-fixture-only", "deep", "http://localhost:5184", false},
		{"带路径", "local-fixture-only", "deep", "http://127.0.0.1:5184/app", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if localAPIWorkerH5Configuration(row.mode, row.depth, row.origin) != row.want {
				t.Fatal("正式进程H5模式校验不符")
			}
		})
	}
}

type localAPIWorkerH5Counts struct{ Conversations, Prepares, Confirms, Reports, Streams, Questions, Answers, Feedback, FeedbackReads int }
type localAPIWorkerH5Gateway struct {
	server *httptest.Server
	mu     sync.Mutex
	counts localAPIWorkerH5Counts
	finish chan struct{}
	once   sync.Once
}

func (g *localAPIWorkerH5Gateway) snapshot() localAPIWorkerH5Counts {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.counts
}

func TestLocalAPIWorkerH5Gateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "id: 1\nevent: progress\ndata: {\"seq\":1}\n\n")
			return
		}
		io.WriteString(w, `{"code":0,"data":{"original":true}}`)
	}))
	t.Cleanup(upstream.Close)
	login := &h5MemberLoginBridge{proxy: httputil.NewSingleHostReverseProxy(mustLocalH5URL(t, upstream.URL))}
	completed := 0
	g := newLocalAPIWorkerH5Gateway(t, upstream.URL, upstream.URL, login, func() bool { completed++; return true })
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	call := func(method, path, origin string, want int) string {
		t.Helper()
		r, err := http.NewRequestWithContext(t.Context(), method, g.server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != want {
			t.Fatal("浏览器代理状态不符")
		}
		return string(raw)
	}
	call("POST", "/__test__/complete", g.server.URL, 409)
	call("POST", "/__test__/finish", "http://external.example.test", 403)
	call("POST", "/__test__/finish", g.server.URL, 409)
	call("POST", "/api/client/auth/login", "http://external.example.test", 404)
	call("GET", "/api/client/member/settings", "", 404)
	if completed != 0 {
		t.Fatal("未取得SSE证据前放行了Agent")
	}
	raw := call("GET", "/api/v1/ai/tasks/1/stream", "", 200)
	if raw != "id: 1\nevent: progress\ndata: {\"seq\":1}\n\n" {
		t.Fatal("代理改写原SSE帧")
	}
	call("POST", "/__test__/complete", g.server.URL, 204)
	for _, path := range []string{"/api/v1/ai/conversations", "/api/v1/ai/conversations/1/task-intents", "/api/v1/ai/conversations/1/task-intents/1/confirm"} {
		if raw := call("POST", path, g.server.URL, 200); raw != `{"code":0,"data":{"original":true}}` {
			t.Fatal("代理改写原API响应")
		}
	}
	for i := 0; i < 2; i++ {
		call("GET", "/api/v1/ai/tasks/1/report", "", 200)
	}
	call("POST", "/__test__/finish", g.server.URL, 204)
	select {
	case <-g.finish:
	default:
		t.Fatal("有效流程没有给出结束信号")
	}
	if c := g.snapshot(); c.Conversations != 1 || c.Prepares != 1 || c.Confirms != 1 || c.Reports != 2 || c.Streams != 1 || completed != 1 {
		t.Fatal("代理计数污染或遗漏")
	}
	// 真实范围不能借用替身完成开关；仍须原样转交API和公开SSE。
	g = newLocalAPIWorkerH5Gateway(t, upstream.URL, upstream.URL, login, func() bool { completed++; return true }, true)
	notice := call("GET", "/__test__/", "", 200)
	if !strings.Contains(notice, "真实社区V2及付费模型") || strings.Contains(notice, `id="complete"`) || strings.Contains(notice, "无真实模型") {
		t.Fatal("真实浏览器范围声明或完成按钮不符")
	}
	call("POST", "/__test__/complete", g.server.URL, 404)
	if raw := call("GET", "/api/v1/ai/tasks/1/stream", "", 200); raw != "id: 1\nevent: progress\ndata: {\"seq\":1}\n\n" {
		t.Fatal("真实范围代理改写原SSE")
	}
	call("POST", "/__test__/complete", g.server.URL, 404)
	call("POST", "/__test__/finish", g.server.URL, 409)
	if completed != 1 {
		t.Fatal("真实范围调用了替身完成开关")
	}
}

func mustLocalH5URL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestLocalAPIWorkerH5AssistantGateway(t *testing.T) {
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	upstream := httptest.NewServer(http.NotFoundHandler())
	defer upstream.Close()
	login := &h5MemberLoginBridge{proxy: httputil.NewSingleHostReverseProxy(mustLocalH5URL(t, upstream.URL))}
	a := &localH5Assistant{}
	g := newLocalAPIWorkerH5GatewayWithAssistant(t, upstream.URL, upstream.URL, login, func() bool { return false }, false, a)
	for _, path := range []string{"finish", "resolve", "empty", "withdraw"} {
		r, err := http.NewRequestWithContext(t.Context(), "POST", g.server.URL+"/__test__/"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Origin", g.server.URL)
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 409 {
			t.Fatal("组合前置条件缺失仍操作", path)
		}
	}
	g.mu.Lock()
	g.counts = localAPIWorkerH5Counts{Conversations: 1, Prepares: 1, Confirms: 1, Reports: 2, Streams: 1, Questions: 3, Answers: 4, Feedback: 1, FeedbackReads: 2}
	g.mu.Unlock()
	r, err := http.NewRequestWithContext(t.Context(), "POST", g.server.URL+"/__test__/finish", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", g.server.URL)
	res, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatal("仅网络计数不能完成组合验收")
	}
	a.mu.Lock()
	a.resolved, a.empty, a.withdrawn = true, true, true
	a.mu.Unlock()
	r, err = http.NewRequestWithContext(t.Context(), "POST", g.server.URL+"/__test__/finish", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", g.server.URL)
	res, err = client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatal("完整组合前置条件未允许收尾")
	}
	select {
	case <-g.finish:
	default:
		t.Fatal("没有完成信号")
	}
}

// 只代理正式API、原密码桥与完整静态H5；这里不调用任务服务，不改写业务响应。
func newLocalAPIWorkerH5Gateway(t *testing.T, front, api string, login *h5MemberLoginBridge, complete func() bool, real ...bool) *localAPIWorkerH5Gateway {
	t.Helper()
	if len(real) > 1 {
		t.Fatal("每轮只能声明一个真实范围")
	}
	live := len(real) == 1 && real[0]
	return newLocalAPIWorkerH5GatewayWithAssistant(t, front, api, login, complete, live, nil)
}
func newLocalAPIWorkerH5GatewayWithAssistant(t *testing.T, front, api string, login *h5MemberLoginBridge, complete func() bool, live bool, assistant *localH5Assistant) *localAPIWorkerH5Gateway {
	t.Helper()
	if live && assistant != nil {
		t.Fatal("组合验收不能混用真实模型")
	}
	g := &localAPIWorkerH5Gateway{finish: make(chan struct{})}
	proxy := func(raw string) *httputil.ReverseProxy {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		p := httputil.NewSingleHostReverseProxy(u)
		p.FlushInterval = -1
		p.Transport = &http.Transport{Proxy: nil}
		t.Cleanup(p.Transport.(*http.Transport).CloseIdleConnections)
		p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) { http.Error(w, "本地代理暂不可用", 502) }
		return p
	}
	frontProxy, apiProxy := proxy(front), proxy(api)
	apiProxy.ModifyResponse = func(r *http.Response) error {
		if r.StatusCode != 200 {
			return nil
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		path, method := r.Request.URL.Path, r.Request.Method
		switch {
		case method == "POST" && path == "/api/v1/ai/conversations":
			g.counts.Conversations++
		case method == "POST" && strings.HasPrefix(path, "/api/v1/ai/conversations/") && strings.HasSuffix(path, "/task-intents"):
			g.counts.Prepares++
		case method == "POST" && strings.HasPrefix(path, "/api/v1/ai/conversations/") && strings.HasSuffix(path, "/confirm"):
			g.counts.Confirms++
		case method == "GET" && strings.HasPrefix(path, "/api/v1/ai/tasks/") && strings.HasSuffix(path, "/report"):
			g.counts.Reports++
		case method == "GET" && strings.HasPrefix(path, "/api/v1/ai/tasks/") && strings.HasSuffix(path, "/stream"):
			g.counts.Streams++
		case method == "POST" && strings.HasSuffix(path, "/questions"):
			g.counts.Questions++
		case method == "GET" && strings.HasSuffix(path, "/answer"):
			g.counts.Answers++
		case method == "POST" && strings.HasSuffix(path, "/feedback"):
			g.counts.Feedback++
		case method == "GET" && strings.HasSuffix(path, "/feedback"):
			g.counts.FeedbackReads++
		}
		return nil
	}
	g.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !net.ParseIP(remote).IsLoopback() || r.Host != strings.TrimPrefix(g.server.URL, "http://") {
			http.Error(w, "仅限本地测试", 403)
			return
		}
		if r.URL.Path == "/__test__/" && r.Method == "GET" {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-local-api-worker'; connect-src 'self'")
			notice := "随机会员、合成资料、测试积分及回环Agent/模型，无真实模型或生产数据；任务预算五分钟。"
			button := `<button id="complete">允许测试Agent完成</button>`
			if assistant != nil {
				button += `<button id="resolve">原管理进程处理反馈</button><button id="empty">改为无证据范围</button><button id="withdraw">撤回测试来源</button>`
			}
			if live {
				notice = "随机会员、合成资料与测试积分；本轮经独立父桥调用真实社区V2及付费模型，任务预算十五分钟；无生产数据，不代表生产批准。"
				button = ""
			}
			fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>仅测试：正式API与独立worker</title><h1>实际H5→API→独立worker</h1><p>%s</p><a href="/__test__/login">查看本轮测试账号，使用原密码表单登录</a><p><button id="guest">清理登录态并进入AI助手</button>%s<button id="finish">结束并核验</button></p><p id="result"></p><script nonce="local-api-worker">document.getElementById('guest').onclick=()=>{localStorage.removeItem('token');location.href='/#/pages/ai/index'};for(const id of ['complete','finish','resolve','empty','withdraw']){const button=document.getElementById(id);if(button)button.onclick=async()=>{const r=await fetch('/__test__/'+id,{method:'POST'});document.getElementById('result').textContent=r.ok?'操作已确认':'尚未满足核验条件'}};</script>`, notice, button)
			return
		}
		if r.URL.Path == "/__test__/login" && r.Method == "GET" {
			login.proxy.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/__test__/") {
			if r.Method != "POST" || r.URL.RawQuery != "" || r.Header.Get("Origin") != g.server.URL {
				http.Error(w, "仅接受同源测试操作", 403)
				return
			}
			switch r.URL.Path {
			case "/__test__/complete":
				if live {
					http.NotFound(w, r)
					return
				}
				if g.snapshot().Streams == 0 || !complete() {
					http.Error(w, "尚未取得原任务运行及SSE证据", 409)
					return
				}
			case "/__test__/finish":
				c := g.snapshot()
				if assistant != nil && !assistant.ready(c) {
					http.Error(w, "组合问答/反馈/撤回尚未完成", 409)
					return
				}
				if c.Conversations != 1 || c.Prepares != 1 || c.Confirms < 1 || c.Reports < 2 || c.Streams < 1 {
					http.Error(w, "原会话和报告读取尚未完成", 409)
					return
				}
				g.once.Do(func() { close(g.finish) })
			case "/__test__/resolve", "/__test__/empty", "/__test__/withdraw":
				if assistant == nil || !assistant.action(r.Context(), r.URL.Path, g.snapshot()) {
					http.Error(w, "组合操作尚未满足条件", 409)
					return
				}
			default:
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(204)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/client/") {
			allowed := r.Method == "POST" && (r.URL.Path == "/api/client/auth/login" || r.URL.Path == "/api/client/auth/logout") || r.Method == "GET" && r.URL.Path == "/api/client/member/profile"
			if !allowed || r.URL.RawQuery != "" || r.Method == "POST" && r.Header.Get("Origin") != g.server.URL {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			login.proxy.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			apiProxy.ServeHTTP(w, r)
			return
		}
		frontProxy.ServeHTTP(w, r)
	}))
	g.server.Config.ReadHeaderTimeout = 3 * time.Second
	g.server.Start()
	t.Cleanup(g.server.Close)
	return g
}
