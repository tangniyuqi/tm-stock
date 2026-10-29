//go:build integration

package client_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 仅由隔离H5测试父程序通过stdin传入；不读产品配置、不输出凭据。
type aiMemberLoginBridgeConfig struct {
	DSN       string `json:"dsn"`
	Signing   string `json:"signing"`
	Control   string `json:"control"`
	APIURL    string `json:"api_url,omitempty"`
	TaskAPI   bool   `json:"task_api,omitempty"`
	PlansPath string `json:"plans_path,omitempty"`
}

func TestAIMemberLoginBridge(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LOGIN_CHILD") != "local-fixture-only" {
		t.Skip("仅显式本地会员登录跨层验收运行")
	}
	var cfg aiMemberLoginBridgeConfig
	if json.NewDecoder(io.LimitReader(os.Stdin, 16384)).Decode(&cfg) != nil || len(cfg.Signing) < 32 || len(cfg.Control) < 32 {
		t.Fatal("隔离会员登录配置无效")
	}
	dsn, err := driver.ParseDSN(cfg.DSN)
	if err != nil {
		t.Fatal("隔离会员登录DSN无效")
	}
	host, port, err := net.SplitHostPort(dsn.Addr)
	if err != nil || host != "127.0.0.1" || port == "" || dsn.Net != "tcp" || !aiMemberTestDatabaseName(dsn.DBName) {
		t.Fatal("仅允许父测试本次创建的回环随机库")
	}
	dsn.Timeout, dsn.ReadTimeout, dsn.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	db, err := gorm.Open(mysql.Open(dsn.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("连接隔离会员库失败")
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal("取得隔离会员库连接失败")
	}
	defer pool.Close()
	if err := verifyAIMemberLocalMarker(pool, dsn.DBName); err != nil {
		t.Fatal("本地会员库实例标记未确认，未初始化会员")
	}
	engine, _, testOnlyMemberPassword := aiMemberLoginFixture(t, db, cfg.Signing)
	var mu sync.Mutex
	logins, profiles := map[uint]int{}, map[uint]int{}
	rejections := 0
	finished := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || remote != "127.0.0.1" || r.Header.Get("X-Test-Control") != cfg.Control {
			http.Error(w, "仅限本地测试", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/__test__/login" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'")
			fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>仅测试：会员密码登录</title><h1>本次隔离会员登录</h1><p>原H5登录表单、GVA bcrypt及JWT；会员、报告和积分均为同一临时MySQL。无生产账号，积分仅用于隔离测试；模型调用范围和费用以本次测试控制页说明为准。</p><p>会员一：ai_member_one；会员二：ai_member_two；停用会员：ai_member_disabled</p><p>仅本次测试口令：%s</p><a href="/#/pages/ai/index">进入AI助手，再从原登录入口登录</a>`, testOnlyMemberPassword)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/__test__/login-finish" && r.Method == "POST" && r.Header.Get("X-Test-Control") == cfg.Control {
			if logins[1] < 1 || logins[2] < 1 || profiles[1] < 1 || profiles[2] < 1 || rejections < 1 {
				http.Error(w, "两账号原登录/资料及错误密码反例未完成", 409)
				return
			}
			for _, id := range []uint{1, 2} {
				var m member.Member
				if err := db.First(&m, id).Error; err != nil || m.LastLoginAt == nil {
					http.Error(w, "原登录记录未落库", 409)
					return
				}
			}
			w.WriteHeader(http.StatusNoContent)
			finished <- struct{}{}
			return
		}
		allowed := r.Method == "POST" && (r.URL.Path == "/client/auth/login" || r.URL.Path == "/client/auth/logout") || r.Method == "GET" && r.URL.Path == "/client/member/profile"
		if !allowed || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, r)
		var response struct {
			Code int `json:"code"`
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if json.Unmarshal(rec.Body.Bytes(), &response) == nil {
			if r.URL.Path == "/client/auth/login" {
				if response.Code == 0 {
					if claims, err := utils.NewJWT().ParseToken(response.Data.Token); err == nil {
						logins[claims.BaseClaims.ID]++
					}
				} else {
					rejections++
				}
			}
			if r.URL.Path == "/client/member/profile" && response.Code == 0 {
				if claims, err := utils.NewJWT().ParseToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); err == nil {
					profiles[claims.BaseClaims.ID]++
				}
			}
		}
		for key, values := range rec.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(rec.Code)
		if _, err := w.Write(rec.Body.Bytes()); err != nil {
			return
		}
	}))
	defer srv.Close()
	fmt.Println("AI_MEMBER_LOGIN_READY " + srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("会员原登录跨层验收未结束")
	}
}

func aiMemberTestDatabaseName(name string) bool {
	return regexp.MustCompile(`^tm_points_it_[a-f0-9]{24}$`).MatchString(name) || regexp.MustCompile(`^tm_ai_local_[a-f0-9]{32}$`).MatchString(name)
}

// 仅测试桥支持新增本地装配库；必须在AutoMigrate和会员写入之前核验标记。
func verifyAIMemberLocalMarker(db *sql.DB, name string) error {
	if !strings.HasPrefix(name, "tm_ai_local_") {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var total, matched int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(instance_id=? AND database_name=DATABASE() AND server_uuid=@@server_uuid AND environment='local-isolated' AND contains_real_users=0),0) FROM tm_ai_local_instance`, strings.TrimPrefix(name, "tm_ai_local_")).Scan(&total, &matched)
	if err != nil || total != 1 || matched != 1 {
		return fmt.Errorf("本地会员库隔离标记无效")
	}
	return nil
}

// 受控初始化程序通过stdin注入本轮库/签名配置；不读取产品配置或输出凭据。
func TestAIMemberLocalBootstrap(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LOGIN_CHILD") != "local-fixture-only" {
		t.Skip("仅受控本地初始化运行")
	}
	var cfg aiMemberLoginBridgeConfig
	if json.NewDecoder(io.LimitReader(os.Stdin, 16384)).Decode(&cfg) != nil || len(cfg.Signing) < 32 {
		t.Fatal("隔离会员初始化配置无效")
	}
	dsn, err := driver.ParseDSN(cfg.DSN)
	if err != nil || !regexp.MustCompile(`^tm_ai_local_[a-f0-9]{32}$`).MatchString(dsn.DBName) {
		t.Fatal("本地库名无效")
	}
	host, port, err := net.SplitHostPort(dsn.Addr)
	if err != nil || host != "127.0.0.1" || port == "" || dsn.Net != "tcp" {
		t.Fatal("本地库目标无效")
	}
	dsn.Timeout, dsn.ReadTimeout, dsn.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	db, err := gorm.Open(mysql.Open(dsn.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("本地会员库连接失败")
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal("本地会员连接未就绪")
	}
	defer pool.Close()
	if err := verifyAIMemberLocalMarker(pool, dsn.DBName); err != nil {
		t.Fatal("实例标记未确认，未初始化会员")
	}
	engine, _, password := aiMemberLoginFixture(t, db, cfg.Signing)
	call := func(method, path, body, token string) struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	} {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		var out struct {
			Code int `json:"code"`
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal("原会员响应无效")
		}
		return out
	}
	wrong, _ := json.Marshal(map[string]string{"mobile": "ai_member_one", "password": "wrong-local-only"})
	if call("POST", "/client/auth/login", string(wrong), "").Code == 0 {
		t.Fatal("错误密码未拒绝")
	}
	for i, name := range []string{"ai_member_one", "ai_member_two"} {
		body, _ := json.Marshal(map[string]string{"mobile": name, "password": password})
		login := call("POST", "/client/auth/login", string(body), "")
		if login.Code != 0 || login.Data.Token == "" {
			t.Fatal("原密码登录失败")
		}
		claims, err := utils.NewJWT().ParseToken(login.Data.Token)
		if err != nil || claims.BaseClaims.ID != uint(i+1) || claims.UserType != system.UserTypeClient {
			t.Fatal("原JWT身份不符")
		}
		if call("GET", "/client/member/profile", "", login.Data.Token).Code != 0 {
			t.Fatal("原会员资料读取失败")
		}
	}
	var count int64
	if db.Model(&member.Member{}).Where("last_login_at IS NOT NULL").Count(&count).Error != nil || count != 2 {
		t.Fatal("原登录记录未落库")
	}
	// 验证错标记拒绝后恢复原值，不能让后续实际API借用错误实例。
	original := strings.TrimPrefix(dsn.DBName, "tm_ai_local_")
	wrongID := strings.Repeat("0", 32)
	if wrongID == original {
		wrongID = strings.Repeat("f", 32)
	}
	if _, err := pool.Exec("UPDATE tm_ai_local_instance SET instance_id=?", wrongID); err != nil {
		t.Fatal("错标记反例未建立")
	}
	rejected := verifyAIMemberLocalMarker(pool, dsn.DBName) != nil
	if _, err := pool.Exec("UPDATE tm_ai_local_instance SET instance_id=?", original); err != nil {
		t.Fatal("原实例标记未恢复")
	}
	if !rejected || verifyAIMemberLocalMarker(pool, dsn.DBName) != nil {
		t.Fatal("错标记拒绝/恢复边界失败")
	}
	if cfg.APIURL != "" {
		verifyAIMemberConversationAPI(t, cfg.APIURL, engine, db, password, cfg.TaskAPI)
		if cfg.TaskAPI {
			verifyAIMemberTaskAPI(t, cfg.APIURL, engine, db, password, cfg.PlansPath)
		}
	}
	t.Log("LOCAL_MEMBER_BOOTSTRAP_VERIFIED：原bcrypt密码登录/JWT/两会员资料/错误密码拒绝及登录存证通过；仅本轮隔离会员")
}

func TestAIMemberLocalDatabaseNames(t *testing.T) {
	for name, want := range map[string]bool{"tm_points_it_" + strings.Repeat("a", 24): true, "tm_ai_local_" + strings.Repeat("b", 32): true, "tm_ai_local_" + strings.Repeat("B", 32): false, "production": false, "tm_ai_local_": false, "tm_ai_local_" + strings.Repeat("a", 32) + ";": false} {
		if aiMemberTestDatabaseName(name) != want {
			t.Fatal("本地会员库名称边界不符")
		}
	}
}
