package client

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

// /client/member 下的每一条路由都必须走 ClientJWTAuth：无令牌、后台令牌、类型为空的令牌一律 401。
// 这里把 GVA_DB 置为 nil——类型不对的令牌若走到会员表查询会直接 panic，所以同时证明了
// "类型检查先于查库"。路由是遍历 engine.Routes() 得到的，之后新增的成员路由也会被自动覆盖；
// 若有人把 Use 挪到路由注册之后（只对后面的路由生效），无令牌请求会拿到非 401，测试会失败。
func TestMemberRoutesRequireClientToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := global.GVA_CONFIG.JWT
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: "test-signing-key", ExpiresTime: "7d", BufferTime: "1d", Issuer: "GVA"}
	t.Cleanup(func() { global.GVA_CONFIG.JWT = old })
	oldDB := global.GVA_DB
	global.GVA_DB = nil
	t.Cleanup(func() { global.GVA_DB = oldDB })
	testutil.InitNopLogger(t)

	engine := gin.New()
	(&MemberRouter{}).InitMemberRouter(engine.Group("/private"), engine.Group("/public"))

	var routes []gin.RouteInfo
	for _, r := range engine.Routes() {
		if strings.HasPrefix(r.Path, "/public/client/member/") {
			routes = append(routes, r)
		}
	}
	if len(routes) < 5 {
		t.Fatalf("只找到 %d 条 /client/member 路由，注册可能没生效", len(routes))
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Path+routes[i].Method < routes[j].Path+routes[j].Method })

	j := utils.NewJWT()
	mint := func(base systemReq.BaseClaims) string {
		token, err := j.CreateToken(j.CreateClaims(base))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	tokens := map[string]string{
		"无令牌":    "",
		"后台令牌":   mint(systemReq.BaseClaims{ID: 1, AuthorityId: 888, UserType: system.UserTypeAdmin}),
		"类型为空令牌": mint(systemReq.BaseClaims{ID: 1}),
	}

	for _, r := range routes {
		for name, token := range tokens {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(r.Method, r.Path, nil)
			if token != "" {
				req.Header.Set("x-token", token)
			}
			engine.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s 用【%s】访问应为 401，实际 %d", r.Method, r.Path, name, w.Code)
			}
		}
	}
}
