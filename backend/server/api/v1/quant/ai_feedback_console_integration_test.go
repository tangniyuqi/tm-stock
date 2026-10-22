//go:build integration

package quant

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 仅验收进程：实际JWT/Casbin/当前用户核验与原处理接口相连，资产来自显式本地Vite源。
func TestFeedbackConsoleBrowser(t *testing.T) {
	raw, front := os.Getenv("TM_TEST_FEEDBACK_UPSTREAM"), os.Getenv("TM_TEST_FEEDBACK_UI_ORIGIN")
	if raw == "" || front == "" {
		t.Skip("仅显式本地跨层浏览器会话运行")
	}
	for _, s := range []string{raw, front} {
		u, err := url.Parse(s)
		if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() || u.Port() == "" {
			t.Fatal("仅允许测试回环源")
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + listener.Addr().String()
	engine, _, testOnlyFeedbackPassword := feedbackLoginFixture(t, raw, origin, os.Getenv("TM_TEST_FEEDBACK_CREDENTIAL"))
	target, _ := url.Parse(front)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil, ResponseHeaderTimeout: 5 * time.Second}
	defer proxy.Transport.(*http.Transport).CloseIdleConnections()
	var posts, reads, logins, menuWrites, policyWrites atomic.Int32
	finish := make(chan struct{}, 1)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !net.ParseIP(host).IsLoopback() || r.Host != strings.TrimPrefix(origin, "http://") {
			http.Error(w, "仅本地测试", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/__test__/" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-test-only-feedback'; connect-src 'self'")
			fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>仅测试：后台登录与权限</title><h1>仅测试：完整后台登录与反馈</h1><p>账号库是隔离SQLite，反馈是真MySQL；无生产账号、真实模型或费用。内部凭据不进入浏览器。</p><p>权限管理员：feedback_admin；反馈处理人：feedback_operator；未登记人员：feedback_viewer</p><p>仅本次测试口令：%s</p><a href="/#/login">进入实际后台登录页</a><button id="finish">结束并核对</button><p id="result"></p><script nonce="test-only-feedback">document.getElementById('finish').onclick=async()=>{const r=await fetch('/__test__/finish',{method:'POST'});document.getElementById('result').textContent=r.ok?'测试核对完成':'测试核对未完成'};</script>`, testOnlyFeedbackPassword)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/__test__/") {
			if r.URL.Path != "/__test__/finish" || r.Method != "POST" || r.Header.Get("Origin") != origin || r.ContentLength != 0 {
				http.Error(w, "测试控制请求无效", 403)
				return
			}
			if posts.Load() != 2 || reads.Load() < 3 || logins.Load() < 2 || menuWrites.Load() < 1 || policyWrites.Load() < 1 {
				http.Error(w, "尚未完成两次处理与独立查询", 409)
				return
			}
			w.WriteHeader(204)
			finish <- struct{}{}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			if r.URL.Path == "/base/login" && r.Method == "POST" {
				logins.Add(1)
			}
			if r.URL.Path == "/menu/addMenuAuthority" && r.Method == "POST" {
				menuWrites.Add(1)
			}
			if r.URL.Path == "/casbin/updateCasbin" && r.Method == "POST" {
				policyWrites.Add(1)
			}
			if strings.HasPrefix(r.URL.Path, "/quant/ai-feedback") {
				if r.Method == "POST" {
					posts.Add(1)
				} else if r.Method == "GET" {
					reads.Add(1)
				}
			}
			engine.ServeHTTP(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	srv.Listener = listener
	srv.Config.ReadHeaderTimeout = 3 * time.Second
	srv.Start()
	defer srv.Close()
	fmt.Println("FEEDBACK_ADMIN_BROWSER_READY " + origin + "/__test__/")
	select {
	case <-finish:
	case <-time.After(8 * time.Minute):
		t.Fatal("后台反馈浏览器会话超时")
	}
}
