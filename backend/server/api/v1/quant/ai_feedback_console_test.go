package quant

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/service/feedbackconsole"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

func feedbackAdminFixture(t *testing.T, upstream, origin, credential string) (*gin.Engine, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	testutil.InitNopLogger(t)
	testutil.InitMemoryCache(t, 0)
	old := global.GVA_CONFIG.JWT
	t.Cleanup(func() { global.GVA_CONFIG.JWT = old })
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: "test-only-feedback-admin-signing", ExpiresTime: "1h", BufferTime: "1m", Issuer: "GVA"}
	db := testutil.NewMemoryDB(t)
	if err := db.Exec(`CREATE TABLE sys_users(id INTEGER PRIMARY KEY,enable INTEGER,deleted_at DATETIME);INSERT INTO sys_users VALUES(7,1,NULL),(8,1,NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	cfg := config.FeedbackConsoleConfig{Enabled: true, Upstream: upstream, WebOrigin: origin, Reviewers: []config.FeedbackConsoleReviewer{{UserID: 7, ActorID: 7, Token: credential}}}
	client, err := feedbackconsole.New(cfg, feedbackconsole.NewAdminIdentity(db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	e := gin.New()
	private := e.Group("")
	private.Use(middleware.JWTAuth(), middleware.MustChangePwdGuard(), middleware.CasbinHandler())
	enforcer := utils.GetCasbin()
	if enforcer == nil {
		t.Fatal("测试权限初始化失败")
	}
	for _, rule := range [][]string{{"888", "/quant/ai-feedback", "GET"}, {"888", "/quant/ai-feedback/:id", "GET"}, {"888", "/quant/ai-feedback/:id/decisions", "POST"}} {
		if _, err := enforcer.AddPolicy(rule); err != nil {
			t.Fatal(err)
		}
	}
	NewFeedbackConsoleAPI(client).Register(private)
	mint := func(uid uint, kind system.UserType) string {
		j := utils.NewJWT()
		token, err := j.CreateToken(j.CreateClaims(systemReq.BaseClaims{ID: uid, AuthorityId: 888, UserType: kind}))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	return e, mint(7, system.UserTypeAdmin), mint(7, system.UserTypeClient)
}

func TestFeedbackConsoleHTTP(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("f", 40) || r.Header.Get("Origin") != "" || r.Header.Get("x-token") != "" {
			t.Error("转接泄漏浏览器或内部身份")
		}
		switch r.URL.Path {
		case "/internal/v1/ai-feedback":
			fmt.Fprint(w, `{"code":0,"msg":"","data":{"items":[],"nextAfterId":0,"hasMore":false}}`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"code":40403,"msg":"不能泄漏的内部文字"}`)
		}
	}))
	defer upstream.Close()
	e, admin, member := feedbackAdminFixture(t, upstream.URL, "http://127.0.0.1:8080", strings.Repeat("f", 40))
	for _, tc := range []struct {
		method, path, body, token, origin string
		status                            int
	}{
		{"GET", "/quant/ai-feedback", "", admin, "", 200},
		{"GET", "/quant/ai-feedback", "", member, "", 401},
		{"GET", "/quant/ai-feedback", "", "", "", 401},
		{"GET", "/quant/ai-feedback?actorId=7", "", admin, "", 400},
		{"GET", "/quant/ai-feedback?limit=1&limit=2", "", admin, "", 400},
		{"GET", "/quant/ai-feedback?limit=101", "", admin, "", 400},
		{"GET", "/quant/ai-feedback?", "", admin, "", 400},
		{"GET", "/quant/ai-feedback", "x", admin, "", 400},
		{"GET", "/quant/ai-feedback/01", "", admin, "", 400},
		{"GET", "/quant/ai-feedback/1?limit=20", "", admin, "", 400},
		{"GET", "/quant/ai-feedback/1", "", admin, "", 404},
		{"GET", "/quant/ai-feedback", "", admin, "https://untrusted.example.test", 403},
		{"POST", "/quant/ai-feedback/1/decisions", `{"expectedVersion":1,"status":"processing","resolution":"","actorId":7}`, admin, "", 400},
		{"POST", "/quant/ai-feedback/1/decisions", `{"expectedVersion":1,"status":"processing","status":"resolved","resolution":""}`, admin, "", 400},
		{"POST", "/quant/ai-feedback/1/decisions", `{"expectedVersion":1,"status":"resolved","resolution":"reviewed"}`, admin, "", 400},
		{"POST", "/quant/ai-feedback/1/decisions", strings.Repeat(" ", 2049) + `{}`, admin, "", 400},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("x-token", tc.token)
		r.Header.Set("x-user-id", "7")
		r.Header.Set("Content-Type", "application/json")
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		before := calls.Load()
		e.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s %s %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if (tc.status == 400 || tc.status == 401 || tc.status == 403) && before != calls.Load() {
			t.Fatal("拒绝请求仍转接")
		}
		if strings.Contains(w.Body.String(), "不能泄漏") {
			t.Fatal("内部错误外泄")
		}
	}
	// 持有同一有效后台JWT时，撤回任一角色接口权限也不得继续转接。
	// 延长测试缓存，稳定复现参数化/筛选删策略与具体URL缓存键不一致。
	enforcer := utils.GetCasbin()
	enforcer.SetExpireTime(time.Hour)
	if err := enforcer.InvalidateCache(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// 恢复当前GVA初始化的默认duration，不在产品中调整全站缓存。
		enforcer.SetExpireTime(60 * 60)
		if err := enforcer.InvalidateCache(); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct{ name, method, path, policyPath, body string }{
		{"permission_revoked_list", "GET", "/quant/ai-feedback", "/quant/ai-feedback", ""},
		{"permission_revoked_detail", "GET", "/quant/ai-feedback/1", "/quant/ai-feedback/:id", ""},
		{"permission_revoked_decision", "POST", "/quant/ai-feedback/1/decisions", "/quant/ai-feedback/:id/decisions", `{"expectedVersion":1,"status":"processing","resolution":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if allowed, err := enforcer.Enforce("888", tc.path, tc.method); err != nil || !allowed {
				t.Fatal("未预热实际URL的权限缓存")
			}
			// 原角色更新接口也使用筛选删除；此操作不会清理具体请求的缓存键。
			if removed, err := enforcer.RemoveFilteredPolicy(0, "888", tc.policyPath, tc.method); err != nil || !removed {
				t.Fatal("未撤回测试接口权限")
			}
			t.Cleanup(func() {
				if _, err := enforcer.AddPolicy("888", tc.policyPath, tc.method); err != nil {
					t.Error("未恢复测试接口权限")
				}
			})
			if stale, err := enforcer.Enforce("888", tc.path, tc.method); err != nil || !stale {
				t.Fatal("未复现残留放行缓存，不能证明独立复核有效")
			}
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("x-token", admin)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			before := calls.Load()
			e.ServeHTTP(w, r)
			var envelope struct {
				Code int `json:"code"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &envelope) != nil || envelope.Code != 7 || calls.Load() != before {
				t.Fatal("撤权后仍转接，或未遵守原Casbin拒绝契约")
			}
		})
	}
	// 即使用户仍持有效JWT，停用也必须即时拒绝；x-user-id不能冒充获准处理人。
	if err := global.GVA_DB.Exec("UPDATE sys_users SET enable=2 WHERE id=7").Error; err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/quant/ai-feedback", nil)
	r.Header.Set("x-token", admin)
	w := httptest.NewRecorder()
	before := calls.Load()
	e.ServeHTTP(w, r)
	if w.Code != 403 || calls.Load() != before {
		t.Fatal("停用后台用户仍可调用")
	}
	j := utils.NewJWT()
	other, err := j.CreateToken(j.CreateClaims(systemReq.BaseClaims{ID: 8, AuthorityId: 888, UserType: system.UserTypeAdmin}))
	if err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest("GET", "/quant/ai-feedback", nil)
	r.Header.Set("x-token", other)
	r.Header.Set("x-user-id", "7")
	w = httptest.NewRecorder()
	e.ServeHTTP(w, r)
	if w.Code != 403 || calls.Load() != before {
		t.Fatal("未登记后台人员通过或身份头生效")
	}
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body["data"] != nil {
		t.Fatal("拒绝响应含内部数据")
	}
}
