//go:build integration

package points_test

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

type h5MemberLoginBridge struct {
	proxy   *httputil.ReverseProxy
	url     string
	control string
	done    chan bool
}

func h5MemberLoginConfigurationValid(mode, runner, origin string) bool {
	if mode == "" && runner == "" {
		return true
	}
	_, err := h5TestOrigin(origin)
	return mode == "local-fixture-only" && filepath.IsAbs(runner) && strings.HasSuffix(strings.ToLower(runner), ".exe") && err == nil
}

// 真实会话专项不能静默退回独立创建或测试页令牌注入。
func liveConversationConfigurationValid(mode, intent, login, runner, origin string) bool {
	if mode == "" {
		return intent == "" && login == "" && runner == ""
	}
	return mode == "local-fixture-only" && intent == "local-fixture-only" && login == "local-fixture-only" && h5MemberLoginConfigurationValid(login, runner, origin)
}

func liveDepthConfigurationValid(depth string, conversation bool) bool {
	return depth == "" || depth == "deep" && conversation
}

func TestLiveDepthConfiguration(t *testing.T) {
	for _, row := range []struct {
		name               string
		depth              string
		conversation, want bool
	}{
		{"默认独立轻度", "", false, true},
		{"默认会话轻度", "", true, true},
		{"显式会话深度", "deep", true, true},
		{"深度缺少会话", "deep", false, false},
		{"未知深度类型", "unknown", true, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if liveDepthConfigurationValid(row.depth, row.conversation) != row.want {
				t.Fatal("深度实跑不能静默退回独立轻度入口")
			}
		})
	}
}

func TestLiveConversationConfiguration(t *testing.T) {
	runner := filepath.Join(t.TempDir(), "member-login.exe")
	for _, row := range []struct {
		name, mode, intent, login, runner, origin string
		want                                      bool
	}{
		{"原独立入口", "", "", "", "", "", true},
		{"完整会话", "local-fixture-only", "local-fixture-only", "local-fixture-only", runner, "http://127.0.0.1:5173", true},
		{"缺意图", "local-fixture-only", "", "local-fixture-only", runner, "http://127.0.0.1:5173", false},
		{"缺登录", "local-fixture-only", "local-fixture-only", "", "", "http://127.0.0.1:5173", false},
		{"缺运行器", "local-fixture-only", "local-fixture-only", "local-fixture-only", "", "http://127.0.0.1:5173", false},
		{"隐式会话", "", "local-fixture-only", "local-fixture-only", runner, "http://127.0.0.1:5173", false},
		{"未知模式", "true", "local-fixture-only", "local-fixture-only", runner, "http://127.0.0.1:5173", false},
		{"外部源", "local-fixture-only", "local-fixture-only", "local-fixture-only", runner, "https://example.test", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if liveConversationConfigurationValid(row.mode, row.intent, row.login, row.runner, row.origin) != row.want {
				t.Fatal("真实会话配置检查不符")
			}
		})
	}
}

func TestH5MemberLoginConfiguration(t *testing.T) {
	runner := filepath.Join(t.TempDir(), "member-login.exe")
	for _, row := range []struct {
		name, mode, runner, origin string
		want                       bool
	}{
		{"默认模式", "", "", "", true},
		{"仅开关", "local-fixture-only", "", "http://127.0.0.1:5173", false},
		{"仅运行器", "", runner, "http://127.0.0.1:5173", false},
		{"未知标志", "on", runner, "http://127.0.0.1:5173", false},
		{"相对路径", "local-fixture-only", "member-login.exe", "http://127.0.0.1:5173", false},
		{"无浏览器源", "local-fixture-only", runner, "", false},
		{"外部源", "local-fixture-only", runner, "https://example.test", false},
		{"显式本地", "local-fixture-only", runner, "http://127.0.0.1:5173", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			if h5MemberLoginConfigurationValid(row.mode, row.runner, row.origin) != row.want {
				t.Fatal("原密码登录模式不能静默退回JWT注入模式")
			}
		})
	}
}

func h5MemberLoginPassedLine(line string) bool {
	fields := strings.Fields(line)
	return len(fields) == 4 && fields[0] == "---" && strings.TrimSuffix(fields[1], ":") == "PASS" && fields[2] == "TestAIMemberLoginBridge"
}

func TestH5MemberLoginPassLine(t *testing.T) {
	for _, row := range []struct {
		fields []string
		want   bool
	}{
		{[]string{"---", "PASS:", "TestAIMemberLoginBridge", "(1.00s)"}, true},
		{[]string{"---", "SKIP:", "TestAIMemberLoginBridge", "(0.00s)"}, false},
		{[]string{"---", "FAIL:", "TestAIMemberLoginBridge", "(1.00s)"}, false},
		{[]string{"---", "PASS:", "TestOther", "(1.00s)"}, false},
		{[]string{"===", "RUN", "TestAIMemberLoginBridge"}, false},
	} {
		if h5MemberLoginPassedLine(strings.Join(row.fields, " ")) != row.want {
			t.Fatal("会员登录子程序最终回执识别错误")
		}
	}
}

func startH5MemberLogin(t *testing.T, parent context.Context, database, signing, runner string) *h5MemberLoginBridge {
	t.Helper()
	if os.Getenv("TM_TEST_AI_H5_LOGIN") != "local-fixture-only" || !h5MemberLoginConfigurationValid(os.Getenv("TM_TEST_AI_H5_LOGIN"), runner, os.Getenv("TM_TEST_AI_H5_ORIGIN")) {
		t.Fatal("会员密码验收必须显式启用本地浏览器与预编译测试程序")
	}
	return startIsolatedMemberLogin(t, parent, database, signing, runner)
}

// 供实际进程HTTP联调复用原密码桥；不将HTTP请求冒充H5表单操作。
func startIsolatedMemberLogin(t *testing.T, parent context.Context, database, signing, runner string) *h5MemberLoginBridge {
	t.Helper()
	if !filepath.IsAbs(runner) {
		t.Fatal("登录程序必须是显式本地绝对路径")
	}
	dsn, err := driver.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("会员密码验收DSN无效")
	}
	dsn.DBName = database
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	control := hex.EncodeToString(random[:])
	ctx, cancel := context.WithCancel(parent)
	cmd := exec.CommandContext(ctx, runner, "-test.run=^TestAIMemberLoginBridge$", "-test.count=1", "-test.v")
	// 子程序只继承运行所需环境；不继承供应商凭据、生产配置或原生MySQL口令。
	for _, name := range []string{"PATH", "SystemRoot", "TEMP", "TMP"} {
		cmd.Env = append(cmd.Env, name+"="+os.Getenv(name))
	}
	cmd.Env = append(cmd.Env, "TM_TEST_AI_LOGIN_CHILD=local-fixture-only")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal("建立会员测试配置管道失败")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal("建立会员测试状态管道失败")
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal("启动会员登录测试程序失败")
	}
	bridge := &h5MemberLoginBridge{control: control, done: make(chan bool, 1)}
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		passed := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "AI_MEMBER_LOGIN_READY ") {
				select {
				case ready <- strings.TrimPrefix(line, "AI_MEMBER_LOGIN_READY "):
				default:
				}
			}
			if h5MemberLoginPassedLine(line) {
				passed = true
			}
		}
		err := cmd.Wait()
		bridge.done <- passed && scanner.Err() == nil && err == nil
	}()
	t.Cleanup(func() {
		cancel()
		if bridge.proxy != nil {
			bridge.proxy.Transport.(*http.Transport).CloseIdleConnections()
		}
		select {
		case <-bridge.done:
		case <-time.After(10 * time.Second):
			t.Error("会员登录测试子程序未退出")
		}
	})
	if err := json.NewEncoder(stdin).Encode(map[string]string{"dsn": dsn.FormatDSN(), "signing": signing, "control": control}); err != nil {
		stdin.Close()
		t.Fatal("传递会员测试配置失败")
	}
	if err := stdin.Close(); err != nil {
		t.Fatal("关闭会员测试配置管道失败")
	}
	select {
	case raw := <-ready:
		target, err := h5TestOrigin(raw)
		if err != nil {
			t.Fatal("会员测试程序返回非回环源")
		}
		bridge.url = raw
		bridge.proxy = httputil.NewSingleHostReverseProxy(target)
		direct := bridge.proxy.Director
		bridge.proxy.Director = func(r *http.Request) {
			direct(r)
			r.Header.Set("X-Test-Control", bridge.control)
		}
		bridge.proxy.Transport = &http.Transport{Proxy: nil, ResponseHeaderTimeout: 5 * time.Second}
		bridge.proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "隔离登录服务不可用", 502)
		}
	case passed := <-bridge.done:
		bridge.done <- passed
		t.Fatal("会员测试程序提前结束；未转发可能包含凭据的输出")
	case <-time.After(45 * time.Second):
		t.Fatal("会员测试程序未就绪")
	}
	return bridge
}

func (b *h5MemberLoginBridge) finish(t *testing.T, ctx context.Context) {
	b.finishAs(t, ctx, true)
}
func (b *h5MemberLoginBridge) finishAs(t *testing.T, ctx context.Context, h5 bool) {
	t.Helper()
	r, err := http.NewRequestWithContext(ctx, "POST", b.url+"/__test__/login-finish", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Test-Control", b.control)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
	defer client.CloseIdleConnections()
	res, err := client.Do(r)
	if err != nil {
		t.Fatal("会员原登录结束核验失败")
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatal("两账号原登录、资料或拒绝反例未完成")
	}
	select {
	case passed := <-b.done:
		b.done <- passed // 清理阶段也必须确认子程序已结束。
		if !passed {
			t.Fatal("会员登录子程序未实际通过")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("会员登录子程序未返回最终结果")
	}
	if h5 {
		t.Log("原H5密码表单→GVA bcrypt/JWT→同库会员资料已核验：两会员成功、至少一次拒绝、原登录时间落库")
	} else {
		t.Log("原密码HTTP路由→GVA bcrypt/JWT→同库资料核验通过；不是浏览器操作证据")
	}
}
