//go:build integration

package points_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"sync"
	"testing"
	"time"
)

type dossierH5Counts struct{ Lists, Reads, Comparisons, Denied, Conversations, Revokes, Restores int }
type dossierH5Gateway struct {
	server *httptest.Server
	mu     sync.Mutex
	counts dossierH5Counts
	finish chan struct{}
	once   sync.Once
}

func (g *dossierH5Gateway) snapshot() dossierH5Counts {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.counts
}

// 网关只转发原进程响应，不创建会话、不拼装目录/档案、不签发会员令牌。
func newDossierH5Gateway(t *testing.T, front, api string, login *h5MemberLoginBridge, publication func(bool) error) *dossierH5Gateway {
	t.Helper()
	g := &dossierH5Gateway{finish: make(chan struct{})}
	proxy := func(raw string) *httputil.ReverseProxy {
		u := mustLocalH5URL(t, raw)
		p := httputil.NewSingleHostReverseProxy(u)
		p.Transport = &http.Transport{Proxy: nil}
		t.Cleanup(p.Transport.(*http.Transport).CloseIdleConnections)
		p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) { http.Error(w, "隔离服务暂不可用", 502) }
		return p
	}
	frontProxy, apiProxy := proxy(front), proxy(api)
	apiProxy.ModifyResponse = func(r *http.Response) error {
		path, method := r.Request.URL.Path, r.Request.Method
		g.mu.Lock()
		defer g.mu.Unlock()
		if method == "POST" && path == "/api/v1/ai/conversations" && r.StatusCode == 200 {
			g.counts.Conversations++
		}
		if method != "GET" || !strings.HasPrefix(path, "/api/v1/ai/themes/") || !strings.Contains(path, "/dossiers") {
			return nil
		}
		if r.StatusCode == 404 || r.StatusCode == 401 {
			g.counts.Denied++
			return nil
		}
		if r.StatusCode != 200 {
			return nil
		}
		if strings.HasSuffix(path, "/dossiers") {
			g.counts.Lists++
		} else {
			g.counts.Reads++
			if strings.Contains(r.Request.URL.RawQuery, "compareVersion=") {
				g.counts.Comparisons++
			}
		}
		return nil
	}
	g.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !net.ParseIP(remote).IsLoopback() || r.Host != strings.TrimPrefix(g.server.URL, "http://") {
			http.Error(w, "仅限本轮本地验收", 403)
			return
		}
		if r.URL.Path == "/__test__/" && r.Method == "GET" {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-dossier-h5'; connect-src 'self'")
			io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>仅测试：专题档案</title><h1>实际H5与独立API：专题档案验收</h1><p>仅本轮随机会员、合成资料和隔离库。查询不运行模型、不创建生成任务、不扣积分。</p><a href="/__test__/login">查看本轮账号，使用原密码表单登录</a><p><button id="guest">清理本轮登录态并进入AI助手</button><button id="revoke">撤回较早版本的测试发布</button><button id="restore">恢复测试发布</button><button id="finish">结束并核验</button></p><p id="result"></p><script nonce="dossier-h5">document.getElementById('guest').onclick=()=>{localStorage.removeItem('token');location.href='/#/pages/ai/index'};for(const id of ['revoke','restore','finish'])document.getElementById(id).onclick=async()=>{const r=await fetch('/__test__/'+id,{method:'POST'});document.getElementById('result').textContent=r.ok?'操作已确认':'尚未满足核验条件'};</script>`)
			return
		}
		if r.URL.Path == "/__test__/login" && r.Method == "GET" {
			login.proxy.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/__test__/") {
			if r.Method != "POST" || r.URL.RawQuery != "" || r.Header.Get("Origin") != g.server.URL {
				http.Error(w, "仅接受同源验收操作", 403)
				return
			}
			c := g.snapshot()
			switch r.URL.Path {
			case "/__test__/revoke":
				if c.Reads < 2 || c.Comparisons < 1 || publication(false) != nil {
					http.Error(w, "先完成实际读取与对照", 409)
					return
				}
				g.mu.Lock()
				g.counts.Revokes++
				g.mu.Unlock()
			case "/__test__/restore":
				if c.Revokes < 1 || c.Denied < 1 || publication(true) != nil {
					http.Error(w, "先核对撤权拒读", 409)
					return
				}
				g.mu.Lock()
				g.counts.Restores++
				g.mu.Unlock()
			case "/__test__/finish":
				if c.Conversations != 1 || c.Lists < 3 || c.Reads < 4 || c.Comparisons < 2 || c.Denied < 1 || c.Revokes != 1 || c.Restores != 1 {
					http.Error(w, "原会话、目录、原文对照与撤权恢复尚未完成", 409)
					return
				}
				g.once.Do(func() { close(g.finish) })
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

func TestDossierH5GatewayBoundary(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/dossiers/1") && r.URL.RawQuery == "denied=true" {
			w.WriteHeader(404)
			io.WriteString(w, `{"code":40404}`)
			return
		}
		io.WriteString(w, `{"code":0,"data":{"original":true}}`)
	}))
	defer upstream.Close()
	login := &h5MemberLoginBridge{proxy: httputil.NewSingleHostReverseProxy(mustLocalH5URL(t, upstream.URL))}
	actions := []bool{}
	g := newDossierH5Gateway(t, upstream.URL, upstream.URL, login, func(enabled bool) error { actions = append(actions, enabled); return nil })
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	defer client.CloseIdleConnections()
	call := func(method, path, origin string, want int) {
		t.Helper()
		r, err := http.NewRequest(method, g.server.URL+path, nil)
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
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("网关边界错误 %s %d", path, res.StatusCode)
		}
		if strings.HasPrefix(path, "/api/v1/") && want == 200 && string(body) != `{"code":0,"data":{"original":true}}` {
			t.Fatal("网关改写原业务响应")
		}
	}
	call("POST", "/__test__/revoke", "http://example.test", 403)
	call("POST", "/__test__/finish", g.server.URL, 409)
	call("GET", "/api/v1/ai/themes/1/dossiers", "", 200)
	call("GET", "/api/v1/ai/themes/1/dossiers/1", "", 200)
	call("GET", "/api/v1/ai/themes/1/dossiers/1?compareVersion=2", "", 200)
	call("POST", "/__test__/revoke", g.server.URL, 204)
	call("POST", "/__test__/restore", g.server.URL, 409)
	call("GET", "/api/v1/ai/themes/1/dossiers/1?denied=true", "", 404)
	call("POST", "/__test__/restore", g.server.URL, 204)
	call("POST", "/api/client/auth/login", "http://example.test", 404)
	call("GET", "/api/client/unknown", "", 404)
	if len(actions) != 2 || actions[0] || !actions[1] {
		t.Fatal(fmt.Sprintf("测试发布操作不符：%d", len(actions)))
	}
}
