package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

// 后台与 C 端共用同一把签名密钥与 claims 结构，令牌类型（UserType）是唯一的隔离手段。
// 这里用真实签发、真实解析的令牌，验证两个方向都被挡住：
//   - 会员令牌（claims.ID 是会员 ID）不得通过后台鉴权；
//   - 后台令牌（claims.ID 是后台账号 ID）不得通过 C 端鉴权，也不能冒充同号会员；
//   - 类型为空（旧令牌或手工拼的令牌）与未知类型一律拒绝，不做兼容放行。

func useTestJWT(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	old := global.GVA_CONFIG.JWT
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: "test-signing-key", ExpiresTime: "7d", BufferTime: "1d", Issuer: "GVA"}
	t.Cleanup(func() { global.GVA_CONFIG.JWT = old })
	testutil.InitNopLogger(t)
	testutil.InitMemoryCache(t, 0)
}

func mintToken(t *testing.T, base systemReq.BaseClaims) string {
	t.Helper()
	j := utils.NewJWT()
	token, err := j.CreateToken(j.CreateClaims(base))
	if err != nil {
		t.Fatalf("签发测试令牌失败：%v", err)
	}
	return token
}

func serve(engine *gin.Engine, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("x-token", token)
	}
	engine.ServeHTTP(w, req)
	return w
}

func TestJWTAuthOnlyAcceptsAdminTokens(t *testing.T) {
	useTestJWT(t)
	reached := 0
	engine := gin.New()
	engine.Use(JWTAuth())
	engine.GET("/admin/ping", func(c *gin.Context) {
		reached++
		c.JSON(http.StatusOK, gin.H{"id": utils.GetUserID(c)})
	})

	cases := []struct {
		name string
		base systemReq.BaseClaims
		want int
	}{
		{"后台令牌放行", systemReq.BaseClaims{ID: 1, AuthorityId: 888, UserType: system.UserTypeAdmin}, http.StatusOK},
		{"会员令牌拒绝", systemReq.BaseClaims{ID: 1, UserType: system.UserTypeClient}, http.StatusUnauthorized},
		{"类型为空拒绝（长得像旧后台令牌）", systemReq.BaseClaims{ID: 1, AuthorityId: 888}, http.StatusUnauthorized},
		{"未知类型拒绝", systemReq.BaseClaims{ID: 1, AuthorityId: 888, UserType: "service"}, http.StatusUnauthorized},
		{"大小写不同的类型拒绝", systemReq.BaseClaims{ID: 1, AuthorityId: 888, UserType: "Admin"}, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := reached
			w := serve(engine, "/admin/ping", mintToken(t, tc.base))
			if w.Code != tc.want {
				t.Fatalf("状态码 = %d，期望 %d；响应：%s", w.Code, tc.want, w.Body.String())
			}
			if tc.want != http.StatusOK {
				if reached != before {
					t.Error("被拒绝的令牌不应进入业务处理函数")
				}
				if len(w.Result().Cookies()) != 0 {
					t.Error("类型不匹配不应改动 cookie（它不是后台会话，不能误清别的会话）")
				}
			}
		})
	}

	if w := serve(engine, "/admin/ping", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("无令牌应为 401，实际 %d", w.Code)
	}
}

func TestClientJWTAuthOnlyAcceptsClientTokens(t *testing.T) {
	useTestJWT(t)
	db := testutil.NewMemoryDB(t, &member.Member{})
	active := member.Member{Mobile: "13800000001", Status: 1}
	disabled := member.Member{Mobile: "13800000002", Status: 2}
	for _, m := range []*member.Member{&active, &disabled} {
		if err := db.Create(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 后台账号与会员可能同号：用 active 会员的 ID 去造后台令牌，才能证明"同号冒充"被挡住
	if active.ID == 0 {
		t.Fatal("测试会员未写入")
	}

	var gotMember uint
	reached := 0
	engine := gin.New()
	engine.Use(ClientJWTAuth())
	engine.GET("/client/ping", func(c *gin.Context) {
		reached++
		gotMember = c.GetUint("member_id")
		c.Status(http.StatusOK)
	})

	cases := []struct {
		name string
		base systemReq.BaseClaims
		want int
	}{
		{"会员令牌放行", systemReq.BaseClaims{ID: active.ID, UserType: system.UserTypeClient}, http.StatusOK},
		{"后台令牌同号冒充会员被拒", systemReq.BaseClaims{ID: active.ID, AuthorityId: 888, UserType: system.UserTypeAdmin}, http.StatusUnauthorized},
		{"类型为空拒绝", systemReq.BaseClaims{ID: active.ID}, http.StatusUnauthorized},
		{"未知类型拒绝", systemReq.BaseClaims{ID: active.ID, UserType: "service"}, http.StatusUnauthorized},
		{"会员不存在拒绝", systemReq.BaseClaims{ID: 9999, UserType: system.UserTypeClient}, http.StatusUnauthorized},
		{"会员被禁用拒绝", systemReq.BaseClaims{ID: disabled.ID, UserType: system.UserTypeClient}, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := reached
			gotMember = 0
			w := serve(engine, "/client/ping", mintToken(t, tc.base))
			if w.Code != tc.want {
				t.Fatalf("状态码 = %d，期望 %d；响应：%s", w.Code, tc.want, w.Body.String())
			}
			if tc.want == http.StatusOK && gotMember != active.ID {
				t.Errorf("放行后应写入 member_id=%d，实际 %d", active.ID, gotMember)
			}
			if tc.want != http.StatusOK && reached != before {
				t.Error("被拒绝的令牌不应进入业务处理函数")
			}
		})
	}

	if w := serve(engine, "/client/ping", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("无令牌应为 401，实际 %d", w.Code)
	}
	if w := serve(engine, "/client/ping", "not-a-jwt"); w.Code != http.StatusUnauthorized {
		t.Errorf("乱写的令牌应为 401，实际 %d", w.Code)
	}
}

// 类型检查必须先于会员表查询：后台令牌不应触达会员表。
// 这里让 GVA_DB 为 nil——若实现把查库放在类型检查前面，会 panic 而不是返回 401。
func TestClientJWTAuthChecksTokenTypeBeforeTouchingMemberTable(t *testing.T) {
	useTestJWT(t)
	old := global.GVA_DB
	global.GVA_DB = nil
	t.Cleanup(func() { global.GVA_DB = old })

	engine := gin.New()
	engine.Use(ClientJWTAuth())
	engine.GET("/client/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := serve(engine, "/client/ping", mintToken(t, systemReq.BaseClaims{ID: 1, AuthorityId: 888, UserType: system.UserTypeAdmin}))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("后台令牌应在类型检查处被拒（401），实际 %d", w.Code)
	}
}
