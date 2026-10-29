//go:build integration && browser

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 真实H5/原密码登录与真库问答；模型仅回环替身，不运行真实Agent。
func TestIntegrationAIReportMaterialQAH5(t *testing.T) {
	if os.Getenv("TM_TEST_AI_H5_MATERIAL_QA") != "local-fixture-only" {
		t.Fatal("仅显式本地资料问答浏览器验收，不能以未执行记通过")
	}
	target, err := h5TestOrigin(os.Getenv("TM_TEST_AI_H5_ORIGIN"))
	if err != nil || os.Getenv("TM_TEST_AI_H5_LOGIN_RUNNER") == "" {
		t.Fatal("资料问答需要官方回环H5与原登录测试运行器")
	}
	x := newQASQL(t, sql.LevelReadCommitted, "success")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	x.pointsTestEnv.ctx = ctx
	login := startH5MemberLogin(t, ctx, x.database, x.key, os.Getenv("TM_TEST_AI_H5_LOGIN_RUNNER"))
	x.mustExec(`ALTER TABLE addon_quant_theme ADD code varchar(20) NOT NULL DEFAULT '', ADD level int NOT NULL DEFAULT 1, ADD description varchar(100) NOT NULL DEFAULT '仅测试资料', ADD sort int NOT NULL DEFAULT 0, ADD created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3), ADD updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)`)
	x.mustExec(`ALTER TABLE addon_quant_theme ADD parent_id bigint NOT NULL DEFAULT 0`)
	if found, err := repository.NewThemeRepo(x.db).SearchThemes(ctx, "示例", 20); err != nil || len(found) != 1 {
		t.Fatal("原目录查询未通过，不能进入实际问答验收", err)
	}
	mux := x.server.Config.Handler.(*http.ServeMux)
	handler.NewThemeHandler(service.NewThemeService(repository.NewThemeRepo(x.db), nil, service.QuoteConfig{}), handler.DenyAllAccess{}).Register(mux)
	models := &qaHTTPModels{x: x}
	handler.NewAIQAHandler(qaHTTPService(t, x, true, models)).Register(mux, x.auth, true)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil, ResponseHeaderTimeout: 10 * time.Second}
	defer proxy.Transport.(*http.Transport).CloseIdleConnections()
	actions := make(chan h5ReportAction, 4)
	var serial sync.Mutex
	var answered, refused, denied, posts int
	var gateway *httptest.Server
	gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || host != "127.0.0.1" || r.Host != strings.TrimPrefix(gateway.URL, "http://") {
			http.Error(w, "仅限本地测试", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/__test__/" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-material-test'; connect-src 'self'")
			fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>仅测试：资料问答</title><h1>资料问答原H5验收</h1><p>会员、授权资料与账本为同一临时MySQL；原密码登录、回环模型；不联网、不生成报告、不扣真实积分。</p><a href="/__test__/login">本次账号及原登录入口</a><button id="guest">清理测试登录态</button><button id="withdraw">撤回测试来源</button><button id="empty">切换无证据测试范围</button><button id="finish">结束并核对测试</button><p id="result"></p><script nonce="material-test">document.getElementById('guest').onclick=()=>{localStorage.removeItem('token');location.href='/#/pages/ai/index'};for(const id of ['withdraw','empty','finish'])document.getElementById(id).onclick=async()=>{const r=await fetch('/__test__/'+id,{method:'POST'});document.getElementById('result').textContent=r.ok?'测试操作已确认':'测试操作未确认'};</script>`)
			return
		}
		if r.URL.Path == "/__test__/login" && r.Method == "GET" {
			login.proxy.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/__test__/") {
			kind := strings.TrimPrefix(r.URL.Path, "/__test__/")
			if r.Method != "POST" || r.Header.Get("Origin") != gateway.URL || r.ContentLength != 0 || r.URL.RawQuery != "" || kind != "empty" && kind != "withdraw" && kind != "finish" {
				http.Error(w, "测试控制请求无效", 403)
				return
			}
			job := h5ReportAction{kind: kind, done: make(chan struct{})}
			select {
			case actions <- job:
			case <-r.Context().Done():
				return
			}
			select {
			case <-job.done:
				w.WriteHeader(204)
			case <-r.Context().Done():
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/client/") {
			if r.URL.RawQuery != "" || !(r.Method == "POST" && r.Header.Get("Origin") == gateway.URL && (r.URL.Path == "/api/client/auth/login" || r.URL.Path == "/api/client/auth/logout") || r.Method == "GET" && r.URL.Path == "/api/client/member/profile") {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			login.proxy.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			serial.Lock()
			defer serial.Unlock()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, r)
			if strings.HasSuffix(r.URL.Path, "/questions") && r.Method == "POST" {
				posts++
			}
			if strings.HasSuffix(r.URL.Path, "/answer") {
				var out struct {
					Data dto.AIQAAnswer `json:"data"`
				}
				if rec.Code == 200 && json.Unmarshal(rec.Body.Bytes(), &out) == nil {
					if out.Data.State == "answered" && len(out.Data.Quotes) == 1 {
						answered++
					}
					if out.Data.State == "refused" && len(out.Data.Quotes) == 0 && !out.Data.AIGenerated {
						refused++
					}
				} else if rec.Code == 503 {
					denied++
				}
			}
			for k, values := range rec.Header() {
				w.Header()[k] = values
			}
			w.WriteHeader(rec.Code)
			if _, err := w.Write(rec.Body.Bytes()); err != nil {
				return
			}
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer gateway.Close()
	t.Log("本地资料问答入口（原密码登录，回环模型）：", gateway.URL+"/__test__/")
	for {
		select {
		case job := <-actions:
			serial.Lock()
			stop := func() bool {
				defer close(job.done)
				if job.kind == "withdraw" {
					if err := repository.NewAISourceRepo(x.db).SetSourceEnabled(ctx, x.source.ID, false); err != nil {
						t.Fatal(err)
					}
					return false
				}
				if job.kind == "empty" {
					x.mustExec(`UPDATE tm_ai_qa_scopes SET evidence_json='[]',version=version+1,updated_at=UTC_TIMESTAMP(3)`)
					return false
				}
				if posts != 2 || answered < 2 || refused < 1 || denied < 1 || x.provider.Count() != 3 || x.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != 2 || x.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests WHERE state='refused' AND calls=0") != 1 || x.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE kind='question'") != 2 || x.queryInt("SELECT COUNT(*) FROM tm_ai_qa_calls WHERE state='recorded'") != 3 {
					t.Fatal("资料问答正常/历史/拒答/撤回闭环未完整，或发生重复提交/调用", posts, answered, refused, denied)
				}
				x.uncharged(t)
				requirePointAudit(t, x.pointsTestEnv, x.ledger)
				login.finish(t, ctx)
				return true
			}()
			serial.Unlock()
			if stop {
				return
			}
		case <-ctx.Done():
			t.Fatal("资料问答实际H5未在期限内完成")
		}
	}
}
