package quant

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	systemAPI "github.com/flipped-aurora/gin-vue-admin/server/api/v1/system"
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/service/feedbackconsole"
	systemSvc "github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 只在测试中建隔离SQLite账号库；登录/菜单/角色授权均执行原GVA handler和service。
func feedbackLoginFixture(t *testing.T, upstream, origin, credential string) (*gin.Engine, *gorm.DB, string) {
	return feedbackLoginFixtureWithDB(t, upstream, origin, credential, nil)
}

// supplied只由隔离MySQL集成测试注入；产品装配不调用测试夹具。
func feedbackLoginFixtureWithDB(t *testing.T, upstream, origin, credential string, supplied *gorm.DB) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	testOnlyFeedbackPassword := "Local!" + base64.RawURLEncoding.EncodeToString(nonce[:])
	testutil.InitNopLogger(t)
	testutil.InitMemoryCache(t, 0)
	old := global.GVA_CONFIG
	t.Cleanup(func() { global.GVA_CONFIG = old })
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: "test-only-feedback-login-signing", ExpiresTime: "1h", BufferTime: "1m", Issuer: "GVA"}
	global.GVA_CONFIG.System.UseMultipoint = false
	global.GVA_CONFIG.System.UseStrictAuth = true
	models := []any{&system.SysUser{}, &system.SysBaseMenu{}, &system.SysBaseMenuParameter{}, &system.SysBaseMenuBtn{}, &system.SysApi{}, &system.SysAuthorityBtn{}, &system.SysSecurityConfig{}, &system.SysLoginLog{}, &system.JwtBlacklist{}}
	db := supplied
	if db == nil {
		db = testutil.NewMemoryDB(t, models...)
	} else {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatal("建立隔离MySQL登录夹具失败")
		}
		previous := global.GVA_DB
		global.GVA_DB = db
		t.Cleanup(func() { global.GVA_DB = previous })
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// :memory:是每连接独立库；真实页面并发查询必须使用同一测试连接。
	if supplied == nil {
		pool.SetMaxOpenConns(1)
	}
	zero, root := uint(0), uint(888)
	roles := []system.SysAuthority{{AuthorityId: 888, AuthorityName: "测试权限管理员", ParentId: &zero, DefaultRouter: "about"}, {AuthorityId: 7801, AuthorityName: "测试反馈处理人", ParentId: &root, DefaultRouter: "about"}, {AuthorityId: 7802, AuthorityName: "测试普通后台", ParentId: &root, DefaultRouter: "about"}}
	if err := db.Create(&roles).Error; err != nil {
		t.Fatal(err)
	}
	if err := feedbackconsole.EnsureCatalog(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	// 严格授权模式要求被选API也存在于目录；补齐真实会话基础API的测试目录。
	for _, p := range request.DefaultCasbin() {
		if err := db.Create(&system.SysApi{Path: p.Path, Method: p.Method, ApiGroup: "测试后台会话", Description: p.Path}).Error; err != nil {
			t.Fatal(err)
		}
	}
	about := system.SysBaseMenu{Path: "about", Name: "about", Component: "view/about/index.vue", Meta: system.Meta{Title: "测试后台首页", Icon: "house"}}
	permissions := system.SysBaseMenu{Path: "authority", Name: "authority", Component: "view/superAdmin/authority/authority.vue", Meta: system.Meta{Title: "角色权限配置（仅测试）", Icon: "setting"}}
	if err := db.Create(&about).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&permissions).Error; err != nil {
		t.Fatal(err)
	}
	for i := range roles {
		menus := []system.SysBaseMenu{about}
		if roles[i].AuthorityId == 888 {
			menus = append(menus, permissions)
		}
		if err := db.Model(&roles[i]).Association("SysBaseMenus").Replace(menus); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	for _, u := range []system.SysUser{
		{GVA_MODEL: global.GVA_MODEL{ID: 7}, Username: "feedback_operator", NickName: "测试反馈处理人", AuthorityId: 7801, Enable: 1},
		{GVA_MODEL: global.GVA_MODEL{ID: 8}, Username: "feedback_viewer", NickName: "测试普通后台", AuthorityId: 7802, Enable: 1},
		{GVA_MODEL: global.GVA_MODEL{ID: 9}, Username: "feedback_admin", NickName: "测试权限管理员", AuthorityId: 888, Enable: 1},
	} {
		u.UUID = uuid.New()
		u.Password = utils.BcryptHash(testOnlyFeedbackPassword)
		u.PasswordUpdatedAt = &now
		u.HeaderImg = "/favicon.ico"
		if err := db.Omit("Authority", "Authorities").Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&u).Association("Authorities").Append(&system.SysAuthority{AuthorityId: u.AuthorityId}); err != nil {
			t.Fatal(err)
		}
	}
	security := system.DefaultSecurityConfig()
	security.CaptchaOpen = 1000
	security.LockEnable = false
	security.ForceNewUserChangePassword = false
	security.PwdExpireEnable = false
	if err := (&systemSvc.SecurityConfigService{}).Set(context.Background(), security); err != nil {
		t.Fatal(err)
	}
	e := utils.GetCasbin()
	if e == nil {
		t.Fatal("测试Casbin未初始化")
	}
	for _, role := range []string{"888", "7801", "7802"} {
		base := request.DefaultCasbin()
		for _, p := range base {
			if _, err := e.AddPolicy(role, p.Path, p.Method); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, p := range [][2]string{{"/menu/getBaseMenuTree", "POST"}, {"/menu/getMenuAuthority", "POST"}, {"/menu/addMenuAuthority", "POST"}, {"/api/getAllApis", "POST"}, {"/casbin/getPolicyPathByAuthorityId", "POST"}, {"/casbin/updateCasbin", "POST"}, {"/authority/getAuthorityList", "POST"}, {"/authority/updateAuthority", "PUT"}} {
		if _, err := e.AddPolicy("888", p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, role := range []string{"7801", "7802"} {
			if _, err := e.RemoveFilteredPolicy(0, role); err != nil {
				t.Error(err)
			}
		}
	})
	client, err := feedbackconsole.New(config.FeedbackConsoleConfig{Enabled: true, Upstream: upstream, WebOrigin: origin, Reviewers: []config.FeedbackConsoleReviewer{{UserID: 7, ActorID: 7, Token: credential}}}, feedbackconsole.NewAdminIdentity(db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	engine := gin.New()
	engine.Use(gin.Recovery())
	b := &systemAPI.BaseApi{}
	m := &systemAPI.AuthorityMenuApi{}
	c := &systemAPI.CasbinApi{}
	a := &systemAPI.AuthorityApi{}
	apis := &systemAPI.SystemApiApi{}
	engine.POST("/base/login", b.Login)
	engine.POST("/base/captcha", b.Captcha)
	engine.POST("/init/checkdb", (&systemAPI.DBApi{}).CheckDB)
	private := engine.Group("")
	private.Use(middleware.JWTAuth(), middleware.MustChangePwdGuard(), middleware.CasbinHandler())
	private.GET("/user/getUserInfo", b.GetUserInfo)
	private.POST("/menu/getMenu", m.GetMenu)
	private.POST("/jwt/jsonInBlacklist", (&systemAPI.JwtApi{}).JsonInBlacklist)
	private.POST("/menu/getBaseMenuTree", m.GetBaseMenuTree)
	private.POST("/menu/getMenuAuthority", m.GetMenuAuthority)
	private.POST("/menu/addMenuAuthority", m.AddMenuAuthority)
	private.POST("/api/getAllApis", apis.GetAllApis)
	private.POST("/casbin/getPolicyPathByAuthorityId", c.GetPolicyPathByAuthorityId)
	private.POST("/casbin/updateCasbin", c.UpdateCasbin)
	private.POST("/authority/getAuthorityList", a.GetAuthorityList)
	private.PUT("/authority/updateAuthority", a.UpdateAuthority)
	NewFeedbackConsoleAPI(client).Register(private)
	return engine, db, testOnlyFeedbackPassword
}

func TestFeedbackConsoleLogin(t *testing.T) {
	runFeedbackConsoleLogin(t, nil)
}

func runFeedbackConsoleLogin(t *testing.T, supplied *gorm.DB) {
	t.Helper()
	var calls atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"","data":{"items":[],"hasMore":false,"nextAfterId":0}}`))
	}))
	defer up.Close()
	e, db, testOnlyFeedbackPassword := feedbackLoginFixtureWithDB(t, up.URL, "http://127.0.0.1:8080", strings.Repeat("l", 40), supplied)
	requestJSON := func(method, path, token, body string) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-token", token)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal("接口未返回JSON")
		}
		return out
	}
	login := func(username string) string {
		t.Helper()
		out := requestJSON("POST", "/base/login", "", `{"username":"`+username+`","password":"`+testOnlyFeedbackPassword+`"}`)
		if out["code"] != float64(0) {
			t.Fatal("本地真实登录失败")
		}
		data := out["data"].(map[string]any)
		if data["needChangePassword"] == true {
			t.Fatal("测试账号意外强制改密")
		}
		return data["token"].(string)
	}
	if requestJSON("POST", "/base/login", "", `{"username":"feedback_operator","password":"test-only-wrong-password"}`)["code"] == float64(0) {
		t.Fatal("错误密码登录成功")
	}
	admin, operator, viewer := login("feedback_admin"), login("feedback_operator"), login("feedback_viewer")
	// 原角色面板会并行加载目录/已选权限，不能只靠串行夹具通过。
	queries := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		go func(i int) {
			path := "/api/getAllApis"
			if i%2 == 0 {
				path = "/menu/getMenuAuthority"
			}
			r := httptest.NewRequest("POST", path, strings.NewReader(`{"authorityId":7801}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("x-token", admin)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, r)
			var result struct {
				Code int `json:"code"`
			}
			queries <- json.Unmarshal(w.Body.Bytes(), &result) == nil && result.Code == 0
		}(i)
	}
	queryFailed := false
	for i := 0; i < 16; i++ {
		if !<-queries {
			queryFailed = true
		}
	}
	if queryFailed {
		t.Fatal("角色并行查询未成功")
	}
	if requestJSON("GET", "/quant/ai-feedback", operator, "")["code"] != float64(7) || calls.Load() != 0 {
		t.Fatal("未授权处理人仍可转接")
	}
	var menu, about system.SysBaseMenu
	if err := db.Where("name = ?", "AiFeedbackConsole").First(&menu).Error; err != nil {
		t.Fatal("反馈目录缺失")
	}
	if err := db.Where("name = ?", "about").First(&about).Error; err != nil {
		t.Fatal("测试基础菜单缺失")
	}
	marshal := func(v any) string {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal("测试请求编码失败")
		}
		return string(b)
	}
	menuBody := marshal(request.AddMenuAuthorityInfo{AuthorityId: 7801, Menus: []system.SysBaseMenu{about, menu}})
	if requestJSON("POST", "/menu/addMenuAuthority", admin, menuBody)["code"] != float64(0) {
		t.Fatal("原菜单授权接口失败")
	}
	if requestJSON("GET", "/quant/ai-feedback", operator, "")["code"] != float64(7) {
		t.Fatal("仅有菜单即获得处理权")
	}
	grant := func(role uint, feedback bool) {
		t.Helper()
		policies := request.DefaultCasbin()
		if feedback {
			for _, a := range feedbackconsole.CatalogAPIs() {
				policies = append(policies, request.CasbinInfo{Path: a.Path, Method: a.Method})
			}
		}
		body := marshal(request.CasbinInReceive{AuthorityId: role, CasbinInfos: policies})
		if requestJSON("POST", "/casbin/updateCasbin", admin, body)["code"] != float64(0) {
			t.Fatal("原角色接口权限配置失败")
		}
	}
	grant(7802, true)
	if requestJSON("GET", "/quant/ai-feedback", viewer, "")["code"] != float64(40301) || calls.Load() != 0 {
		t.Fatal("未登记人员凭角色权限仍可转接")
	}
	grant(7801, true)
	checkMySQLPolicies := func(want int64) {
		t.Helper()
		if supplied == nil {
			return
		}
		var got int64
		if err := db.Table("casbin_rule").Where("v0 = ? AND v1 IN ?", "7801", []string{"/quant/ai-feedback", "/quant/ai-feedback/:id", "/quant/ai-feedback/:id/decisions"}).Count(&got).Error; err != nil || got != want {
			t.Fatal("角色权限未按预期持久化到真实MySQL")
		}
	}
	checkMySQLPolicies(3)
	if requestJSON("GET", "/quant/ai-feedback", operator, "")["code"] != float64(0) || calls.Load() != 1 {
		t.Fatal("已授权已登记人员无法查询")
	}
	if requestJSON("POST", "/casbin/updateCasbin", operator, `{"authorityId":7801,"casbinInfos":[]}`)["code"] != float64(7) {
		t.Fatal("处理人可以自行配置权限")
	}
	grant(7801, false)
	checkMySQLPolicies(0)
	if requestJSON("GET", "/quant/ai-feedback", operator, "")["code"] != float64(7) || calls.Load() != 1 {
		t.Fatal("原JWT在撤权后仍可转接")
	}
	menuBody = marshal(request.AddMenuAuthorityInfo{AuthorityId: 7801, Menus: []system.SysBaseMenu{about}})
	if requestJSON("POST", "/menu/addMenuAuthority", admin, menuBody)["code"] != float64(0) {
		t.Fatal("菜单撤回失败")
	}
	out := requestJSON("POST", "/menu/getMenu", operator, `{}`)
	raw := marshal(out)
	if out["code"] != float64(0) || strings.Contains(raw, "AiFeedbackConsole") {
		t.Fatal("撤回后动态菜单仍含反馈入口")
	}
}
