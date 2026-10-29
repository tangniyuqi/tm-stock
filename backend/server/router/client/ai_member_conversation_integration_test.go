//go:build integration

package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func localConversationAPIURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return false
	}
	host, port, err := net.SplitHostPort(u.Host)
	n, errPort := strconv.Atoi(port)
	return err == nil && errPort == nil && host == "127.0.0.1" && n >= 1024 && n <= 65535
}

// 原密码路由真实签发的JWT直接请求父程序启动的cmd/api；令牌只留在内存。
// 会员/资料表来自本轮专属库，无模型、无证券样例、无积分写入。
func verifyAIMemberConversationAPI(t *testing.T, base string, engine *gin.Engine, db *gorm.DB, password string, tasks bool) {
	t.Helper()
	if !localConversationAPIURL(base) {
		t.Fatal("会话API只能使用明确回环HTTP地址")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	tokens := make([]string, 2)
	for i, name := range []string{"ai_member_one", "ai_member_two"} {
		body, err := json.Marshal(map[string]string{"mobile": name, "password": password})
		if err != nil {
			t.Fatal("登录参数编码失败")
		}
		r := httptest.NewRequest("POST", "/client/auth/login", strings.NewReader(string(body)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		var out struct {
			Code int `json:"code"`
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Code != 0 || out.Data.Token == "" {
			t.Fatal("原会员登录未签发令牌")
		}
		tokens[i] = out.Data.Token
	}
	type apiData struct {
		ID       int64  `json:"id"`
		Version  int64  `json:"version"`
		Title    string `json:"title"`
		Archived bool   `json:"archived"`
		Items    []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	call := func(method, path, token, key, body string, want int) apiData {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal("会话请求构建失败")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("实际会话API请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(raw) > 65536 || res.StatusCode != want {
			t.Fatalf("实际会话响应不符：%s %s，状态%d，预期%d（正文已隐藏）", method, path, res.StatusCode, want)
		}
		var out struct {
			Code int     `json:"code"`
			Data apiData `json:"data"`
		}
		if want == 200 && (json.Unmarshal(raw, &out) != nil || out.Code != 0 || res.Header.Get("Cache-Control") != "no-store") {
			t.Fatal("实际会话成功响应契约不符")
		}
		for _, hidden := range []string{password, tokens[0], tokens[1], "req_hash", "idem_key", "user_id"} {
			if strings.Contains(string(raw), hidden) {
				t.Fatal("会话响应泄漏敏感内容")
			}
		}
		return out.Data
	}
	root := "/api/v1/ai/conversations"
	call("GET", root, "", "", "", 401)
	call("GET", root, "invalid-local-token", "", "", 401)
	first := call("POST", root, tokens[0], "local_conversation_one", `{"title":"仅测试：本地会话"}`, 200)
	if first.ID <= 0 {
		t.Fatal("创建会话未返回标识")
	}
	replay := call("POST", root, tokens[0], "local_conversation_one", `{"title":"仅测试：本地会话"}`, 200)
	if replay.ID != first.ID {
		t.Fatal("会话重放重复创建")
	}
	call("POST", root, tokens[0], "local_conversation_one", `{"title":"冲突标题"}`, 409)
	receipt := call("GET", "/api/v1/ai/conversation-receipt", tokens[0], "local_conversation_one", "", 200)
	if receipt.ID != first.ID {
		t.Fatal("会话回执未恢复原会话")
	}
	call("GET", "/api/v1/ai/conversation-receipt", tokens[1], "local_conversation_one", "", 404)
	path := root + "/" + strconv.FormatInt(first.ID, 10)
	for _, suffix := range []string{"", "/messages"} {
		call("GET", path+suffix, tokens[1], "", "", 404)
	}
	call("PATCH", path, tokens[1], "", `{"title":"越权改名","expectedVersion":1}`, 404)
	call("POST", path+"/archive", tokens[1], "", `{"expectedVersion":1}`, 404)
	if out := call("GET", root, tokens[1], "", "", 200); len(out.Items) != 0 {
		t.Fatal("列表泄漏他人会话")
	}
	second := call("POST", root, tokens[1], "local_conversation_one", `{"title":"仅测试：会员二"}`, 200)
	if second.ID <= 0 || second.ID == first.ID {
		t.Fatal("不同会员同键未隔离")
	}
	if out := call("GET", root, tokens[0], "", "", 200); len(out.Items) != 1 || out.Items[0].ID != first.ID {
		t.Fatal("会员一列表归属不符")
	}
	if out := call("GET", root, tokens[1], "", "", 200); len(out.Items) != 1 || out.Items[0].ID != second.ID {
		t.Fatal("会员二列表归属不符")
	}
	if out := call("GET", path+"/messages", tokens[0], "", "", 200); len(out.Items) != 0 {
		t.Fatal("空会话不应有探针残留或模型消息")
	}
	renamed := call("PATCH", path, tokens[0], "", `{"title":"仅测试：已改名","expectedVersion":1}`, 200)
	if renamed.Version != 2 || renamed.Title != "仅测试：已改名" {
		t.Fatal("改名版本不符")
	}
	call("PATCH", path, tokens[0], "", `{"title":"过期版本","expectedVersion":1}`, 409)
	archived := call("POST", path+"/archive", tokens[0], "", `{"expectedVersion":2}`, 200)
	if !archived.Archived || archived.Version != 3 {
		t.Fatal("归档结果不符")
	}
	call("PATCH", path, tokens[0], "", `{"title":"归档后修改","expectedVersion":3}`, 409)
	if out := call("GET", root, tokens[0], "", "", 200); len(out.Items) != 0 {
		t.Fatal("归档后活跃列表未移除")
	}
	if out := call("GET", root+"?archived=1", tokens[0], "", "", 200); len(out.Items) != 1 || out.Items[0].ID != first.ID {
		t.Fatal("归档列表不符")
	}
	// 状态每次查库：同一有效JWT在会员停用/软删除后必须立即失效。
	if err := db.Exec("UPDATE addon_member SET status=0 WHERE id=1").Error; err != nil {
		t.Fatal("停用反例建立失败")
	}
	defer func() {
		if err := db.Exec("UPDATE addon_member SET status=1,deleted_at=NULL WHERE id=1").Error; err != nil {
			t.Error("测试会员恢复失败")
		}
	}()
	call("GET", root, tokens[0], "", "", 401)
	if err := db.Exec("UPDATE addon_member SET status=1,deleted_at=CURRENT_TIMESTAMP WHERE id=1").Error; err != nil {
		t.Fatal("软删除反例建立失败")
	}
	call("GET", root, tokens[0], "", "", 401)
	if err := db.Exec("UPDATE addon_member SET deleted_at=NULL WHERE id=1").Error; err != nil {
		t.Fatal("会员恢复失败")
	}
	call("GET", path, tokens[0], "", "", 200)
	call("GET", "/api/v1/ai/tasks", tokens[0], "", "", 200)
	if tasks {
		call("POST", "/api/v1/ai/tasks", tokens[0], "", `{}`, 400)
		call("GET", "/api/v1/ai/points/balance", tokens[0], "", "", 200)
	} else {
		call("POST", "/api/v1/ai/tasks", tokens[0], "", `{}`, 405)
		call("GET", "/api/v1/ai/points/balance", tokens[0], "", "", 404)
	}
	var rows int64
	if db.Raw("SELECT (SELECT COUNT(*) FROM tm_ai_conversations)+(SELECT COUNT(*) FROM tm_ai_conversation_messages)").Scan(&rows).Error != nil || rows != 2 {
		t.Fatal("会话幂等或启动回滚探针存证不符")
	}
	t.Log("LOCAL_CONVERSATION_API_VERIFIED：原密码JWT→实际API、双会员归属、幂等恢复、版本冲突、改名归档、停用/软删除即时拒绝；无消息/模型/积分写入")
}

func TestAIMemberConversationAPIURL(t *testing.T) {
	for raw, want := range map[string]bool{"http://127.0.0.1:8099": true, "https://127.0.0.1:8099": false, "http://localhost:8099": false, "http://127.0.0.1:80": false, "http://user@127.0.0.1:8099": false, "http://127.0.0.1:8099/": false, "http://127.0.0.1:8099?": false, "http://127.0.0.1:8099#x": false} {
		if localConversationAPIURL(raw) != want {
			t.Fatal("会话API地址范围不符")
		}
	}
}
