package client_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	clientRouter "github.com/flipped-aurora/gin-vue-admin/server/router/client"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 仅测试夹具：复用原路由、bcrypt、JWT及会员资料中间件，不注入登录回包。
func aiMemberLoginFixture(t *testing.T, db *gorm.DB, signing string) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	testutil.InitNopLogger(t)
	oldConfig, oldDB := global.GVA_CONFIG, global.GVA_DB
	t.Cleanup(func() { global.GVA_CONFIG, global.GVA_DB = oldConfig, oldDB })
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: signing, ExpiresTime: "1h", BufferTime: "1m", Issuer: "GVA"}
	if db == nil {
		db = testutil.NewMemoryDB(t, &member.Member{}, &member.MemberAsset{})
	} else if err := db.AutoMigrate(&member.Member{}, &member.MemberAsset{}); err != nil {
		t.Fatal("补齐隔离会员测试表失败；不输出SQL或连接信息")
	}
	global.GVA_DB = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	pool, err := db.DB()
	if err != nil {
		t.Fatal("取得隔离会员连接失败")
	}
	if db.Dialector.Name() == "sqlite" {
		pool.SetMaxOpenConns(1)
	}
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal("生成本次测试口令失败")
	}
	testOnlyMemberPassword := "Local!" + base64.RawURLEncoding.EncodeToString(nonce[:])
	for i, name := range []string{"ai_member_one", "ai_member_two", "ai_member_disabled"} {
		id := uint(i + 1)
		var m member.Member
		if err := db.FirstOrCreate(&m, member.Member{GVA_MODEL_ADDON: global.GVA_MODEL_ADDON{ID: id}}).Error; err != nil {
			t.Fatal("建立隔离会员失败")
		}
		status := 1
		if id == 3 {
			status = 0
		}
		if err := db.Model(&m).Updates(map[string]any{"name": name, "mobile": name, "nickname": "仅测试会员", "password": utils.BcryptHash(testOnlyMemberPassword), "status": status}).Error; err != nil {
			t.Fatal("配置隔离会员失败")
		}
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	public := engine.Group("")
	(&clientRouter.AuthRouter{}).InitAuthRouter(public, public)
	(&clientRouter.MemberRouter{}).InitMemberRouter(public, public)
	return engine, db, testOnlyMemberPassword
}

func TestAIMemberPasswordLogin(t *testing.T) {
	engine, db, testOnlyMemberPassword := aiMemberLoginFixture(t, nil, "test-only-ai-member-signing-key")
	call := func(method, path, body, token string) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal("会员接口未返回JSON")
		}
		return out
	}
	for _, name := range []string{"ai_member_one", "ai_member_two"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]string{"mobile": name, "password": testOnlyMemberPassword})
			out := call("POST", "/client/auth/login", string(raw), "")
			if out["code"] != float64(0) {
				t.Fatal("原密码登录未通过")
			}
			data, ok := out["data"].(map[string]any)
			if !ok {
				t.Fatal("登录结果缺失")
			}
			token, _ := data["token"].(string)
			claims, err := utils.NewJWT().ParseToken(token)
			if err != nil || claims.UserType != system.UserTypeClient {
				t.Fatal("原签发令牌不是会员令牌")
			}
			if call("GET", "/client/member/profile", "", token)["code"] != float64(0) {
				t.Fatal("原会员鉴权/资料读取失败")
			}
			var m member.Member
			if err := db.Where("name = ?", name).First(&m).Error; err != nil || m.LastLoginAt == nil || m.ID != claims.BaseClaims.ID {
				t.Fatal("密码登录未写入原会员或令牌主体错绑")
			}
		})
	}
	for _, name := range []string{"ai_member_one", "ai_member_disabled", "ai_member_missing"} {
		raw, _ := json.Marshal(map[string]string{"mobile": name, "password": "test-only-wrong-password"})
		if name == "ai_member_disabled" {
			raw, _ = json.Marshal(map[string]string{"mobile": name, "password": testOnlyMemberPassword})
		}
		if call("POST", "/client/auth/login", string(raw), "")["code"] == float64(0) {
			t.Fatal("错误密码/停用或不存在账号被放行")
		}
	}
	if call("GET", "/client/member/profile", "", "")["code"] == float64(0) {
		t.Fatal("匿名读取会员资料被放行")
	}
}
