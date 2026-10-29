//go:build integration

package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 仅用原密码路由签发，不直接构造JWT；测试令牌只留在本进程内存。
func originalLocalMemberTokens(t *testing.T, engine *gin.Engine, password string) []string {
	t.Helper()
	tokens := make([]string, 2)
	for i, name := range []string{"ai_member_one", "ai_member_two"} {
		body, err := json.Marshal(map[string]string{"mobile": name, "password": password})
		if err != nil {
			t.Fatal("原登录参数编码失败")
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
			t.Fatal("原密码路由未签发会员JWT")
		}
		tokens[i] = out.Data.Token
	}
	return tokens
}

// 原登录JWT→实际cmd/api→会话意图→原任务/账本；只消费本轮合成测试积分。
// 模型、报告交付及线上数据不参与本用例；取消终态也不等于报告终检通过。
func verifyAIMemberTaskAPI(t *testing.T, base string, engine *gin.Engine, db *gorm.DB, password, plansPath string) {
	t.Helper()
	if !localConversationAPIURL(base) {
		t.Fatal("任务API必须是明确回环地址")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	tokens := originalLocalMemberTokens(t, engine, password)
	type item struct {
		Seq    int64  `json:"seq"`
		TaskID int64  `json:"taskId"`
		Kind   string `json:"kind"`
		Skill  string `json:"skill"`
		Type   string `json:"type"`
		Status int    `json:"status"`
	}
	type data struct {
		ID              int64  `json:"id"`
		TaskID          int64  `json:"taskId"`
		MessageSeq      int64  `json:"messageSeq"`
		State           string `json:"state"`
		RejectionCode   string `json:"rejectionCode"`
		Status          int    `json:"status"`
		EventSeq        int64  `json:"eventSeq"`
		ReportAvailable bool   `json:"reportAvailable"`
		PricePoints     int64  `json:"pricePoints"`
		PriceVersion    string `json:"priceVersion"`
		AvailablePoints int64  `json:"availablePoints"`
		FrozenPoints    int64  `json:"frozenPoints"`
		Items           []item `json:"items"`
	}
	hidden := func(raw []byte) {
		t.Helper()
		for _, value := range []string{password, tokens[0], tokens[1], "req_hash", "idem_key", "task_key", "lease_owner", "budget_json", "profile_digest", "local-eval:approval"} {
			if strings.Contains(string(raw), value) {
				t.Fatal("任务响应或SSE泄漏内部信息")
			}
		}
	}
	call := func(method, path, token, key, body string, want int) data {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal("任务请求构建失败")
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
			t.Fatal("实际任务API请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(raw) > 65536 || res.StatusCode != want {
			t.Fatalf("实际任务响应不符：%s %s，状态%d，预期%d（正文隐藏）", method, path, res.StatusCode, want)
		}
		hidden(raw)
		var out struct {
			Code int  `json:"code"`
			Data data `json:"data"`
		}
		if json.Unmarshal(raw, &out) != nil || want == 200 && (out.Code != 0 || res.Header.Get("Cache-Control") != "no-store") || want != 200 && out.Code == 0 {
			t.Fatal("实际任务响应契约不符")
		}
		return out.Data
	}
	assertBalance := func(token string, available, frozen int64) {
		t.Helper()
		out := call("GET", "/api/v1/ai/points/balance", token, "", "", 200)
		if out.AvailablePoints != available || out.FrozenPoints != frozen {
			t.Fatal("测试积分可消费/冻结余额不符")
		}
	}
	assertBalance(tokens[0], 50, 0)
	assertBalance(tokens[1], 50, 0)
	conv := call("POST", "/api/v1/ai/conversations", tokens[0], "local_task_conversation", `{"title":"仅测试:任务HTTP闭环"}`, 200)
	if conv.ID <= 0 {
		t.Fatal("任务会话未建立")
	}
	root := "/api/v1/ai/conversations/" + strconv.FormatInt(conv.ID, 10)
	intents := root + "/task-intents"
	for index, kind := range []string{"light", "deep"} {
		price := int64(7)
		if kind == "deep" {
			price = 30
		}
		quoteBody := `{"analysis_type":"` + kind + `","subject_type":"theme","subject_id":1}`
		call("POST", "/api/v1/ai/tasks/quote", "", "", quoteBody, 401)
		q := call("POST", "/api/v1/ai/tasks/quote", tokens[0], "", quoteBody, 200)
		if q.PricePoints != price || q.PriceVersion != "local-v1" || q.AvailablePoints != 50 {
			t.Fatal("轻深报价与本轮计划不符")
		}
		key := "local_intent_" + kind + "_key"
		body := `{"analysis_type":"` + kind + `","subject_type":"theme","subject_id":1,"price_version":"local-v1"}`
		call("POST", intents, tokens[0], key+"_wrong", strings.Replace(body, "local-v1", "obsolete-v1", 1), 409)
		prepared := call("POST", intents, tokens[0], key, body, 200)
		if prepared.ID <= 0 || prepared.TaskID != 0 || prepared.State != "prepared" || prepared.PricePoints != price {
			t.Fatal("准备意图前创建了任务或价格不符")
		}
		if again := call("POST", intents, tokens[0], key, body, 200); again.ID != prepared.ID {
			t.Fatal("准备重放产生新意图")
		}
		call("POST", intents, tokens[0], key, strings.Replace(body, `"subject_id":1`, `"subject_id":2`, 1), 409)
		assertBalance(tokens[0], 50, 0)
		intent := intents + "/" + strconv.FormatInt(prepared.ID, 10)
		call("GET", intent, tokens[1], "", "", 404)
		call("POST", intent+"/confirm", tokens[1], "", `{"confirmed":true}`, 404)
		call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":false}`, 400)
		call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true,"price_points":1}`, 400)
		linked := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200)
		if linked.State != "linked" || linked.TaskID <= 0 || linked.MessageSeq != int64(index+1) {
			t.Fatal("确认后任务/消息关联不符")
		}
		if again := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200); again.TaskID != linked.TaskID || again.MessageSeq != linked.MessageSeq {
			t.Fatal("确认重放重复创建/写消息")
		}
		if again := call("POST", intent+"/reconcile", tokens[0], "", `{}`, 200); again.TaskID != linked.TaskID {
			t.Fatal("原意图核查未恢复同一任务")
		}
		if again := call("GET", root+"/task-intent-receipt", tokens[0], key, "", 200); again.TaskID != linked.TaskID {
			t.Fatal("原键回执未恢复同一任务")
		}
		assertBalance(tokens[0], 50-price, price)
		assertBalance(tokens[1], 50, 0)
		taskPath := "/api/v1/ai/tasks/" + strconv.FormatInt(linked.TaskID, 10)
		progress := call("GET", taskPath, tokens[0], "", "", 200)
		if progress.Status != 1 || progress.ReportAvailable {
			t.Fatal("排队任务被当作已交付报告")
		}
		for _, suffix := range []string{"", "/events", "/stream"} {
			call("GET", taskPath+suffix, tokens[1], "", "", 404)
		}
		call("POST", taskPath+"/cancel", tokens[1], "", `{}`, 404)
		assertBalance(tokens[0], 50-price, price)
		messages := call("GET", root+"/messages", tokens[0], "", "", 200)
		if len(messages.Items) != index+1 {
			t.Fatal("会话消息重复或缺失")
		}
		last := messages.Items[index]
		if last.Seq != linked.MessageSeq || last.TaskID != linked.TaskID || last.Kind != "task" || last.Skill != kind {
			t.Fatal("会话消息关联了其它任务")
		}
		for replay := 0; replay < 2; replay++ {
			out := call("POST", taskPath+"/cancel", tokens[0], "", `{}`, 200)
			if out.Status != 5 || out.EventSeq != 2 || out.ReportAvailable {
				t.Fatal("取消终态或事件序号不符")
			}
		}
		assertBalance(tokens[0], 50, 0)
		events := call("GET", taskPath+"/events", tokens[0], "", "", 200)
		if len(events.Items) != 2 || events.Items[0].Type != "created" || events.Items[1].Type != "cancelled" {
			t.Fatal("重放产生重复事件")
		}
		for _, after := range []int{0, 1, 2} {
			r, err := http.NewRequestWithContext(ctx, "GET", base+taskPath+"/stream?after_seq="+strconv.Itoa(after), nil)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Authorization", "Bearer "+tokens[0])
			r.Header.Set("Last-Event-ID", strconv.Itoa(after))
			res, err := client.Do(r)
			if err != nil {
				t.Fatal("终态SSE请求失败")
			}
			raw, readErr := io.ReadAll(io.LimitReader(res.Body, 65537))
			closeErr := res.Body.Close()
			if readErr != nil || closeErr != nil || len(raw) > 65536 || res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
				t.Fatal("SSE未按真实终态有界结束")
			}
			hidden(raw)
			if strings.Count(string(raw), "event: progress\n") != 2-after {
				t.Fatal("SSE断点重复或漏事件")
			}
			for seq := 1; seq <= 2; seq++ {
				if strings.Contains(string(raw), "id: "+strconv.Itoa(seq)+"\n") != (seq > after) {
					t.Fatal("SSE游标身份不符")
				}
			}
		}
		if again := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200); again.TaskID != linked.TaskID {
			t.Fatal("取消后的原意图重放创建了新任务")
		}
		assertBalance(tokens[0], 50, 0)
	}
	assertBalance(tokens[0], 50, 0)
	assertBalance(tokens[1], 50, 0)
	// 计划在准备后撤回必须明确未创建；恢复文件也不能偷偷重提原意图。
	if !filepath.IsAbs(plansPath) || filepath.Base(plansPath) != "local-plans.json" {
		t.Fatal("计划变更反例文件范围无效")
	}
	original, err := os.ReadFile(plansPath)
	if err != nil {
		t.Fatal("读取本轮无密钥计划失败")
	}
	var scope struct {
		InstanceID string `json:"instance_id"`
		Scope      string `json:"scope"`
	}
	var database string
	if json.Unmarshal(original, &scope) != nil || scope.Scope != "synthetic-local-only" || db.WithContext(ctx).Raw("SELECT DATABASE()").Scan(&database).Error != nil || database != "tm_ai_local_"+scope.InstanceID {
		t.Fatal("计划文件不是本轮隔离库")
	}
	var changed map[string]json.RawMessage
	if json.Unmarshal(original, &changed) != nil {
		t.Fatal("计划文件编码无效")
	}
	changed["revision"] = json.RawMessage(`"withdrawn-for-local-test"`)
	withdrawn, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"analysis_type":"light","subject_type":"theme","subject_id":1,"price_version":"local-v1"}`
	prepared := call("POST", intents, tokens[0], "local_withdrawn_intent", body, 200)
	if prepared.ID <= 0 || prepared.State != "prepared" {
		t.Fatal("计划变更前意图未准备")
	}
	if err := os.WriteFile(plansPath, withdrawn, 0600); err != nil {
		t.Fatal("本轮计划变更反例建立失败")
	}
	restored := false
	defer func() {
		if !restored {
			if err := os.WriteFile(plansPath, original, 0600); err != nil {
				t.Error("本轮原计划恢复失败")
			}
		}
	}()
	intent := intents + "/" + strconv.FormatInt(prepared.ID, 10)
	denied := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200)
	if denied.State != "not_created" || denied.TaskID != 0 || denied.MessageSeq != 0 || denied.RejectionCode != "admission_unavailable" {
		t.Fatal("创建前计划撤回滞留未知或创建了任务")
	}
	assertBalance(tokens[0], 50, 0)
	if err := os.WriteFile(plansPath, original, 0600); err != nil {
		t.Fatal("原计划恢复失败")
	}
	restored = true
	if again := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200); again.State != "not_created" || again.TaskID != 0 {
		t.Fatal("恢复计划后原拒绝意图被自动重提")
	}
	// HTTP证据之外核验持久数据；启动回滚探针、幂等重放和越权均不得留下额外记录。
	for query, want := range map[string]int64{
		"SELECT COUNT(*) FROM tm_ai_tasks":                                        2,
		"SELECT COUNT(*) FROM tm_ai_conversation_messages":                        2,
		"SELECT COUNT(*) FROM tm_ai_conversation_tasks WHERE state='linked'":      2,
		"SELECT COUNT(*) FROM tm_ai_conversation_tasks WHERE state='not_created'": 1,
		"SELECT COUNT(*) FROM tm_ai_task_events":                                  4,
		"SELECT COUNT(*) FROM tm_points_txns WHERE kind='grant'":                  2,
		"SELECT COUNT(*) FROM tm_points_txns WHERE kind='freeze'":                 2,
		"SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'":                2,
		"SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'":                 0,
		"SELECT COUNT(*) FROM tm_ai_agent_runs":                                   0,
		"SELECT COUNT(*) FROM tm_ai_report_commits":                               0,
		"SELECT COUNT(*) FROM tm_ai_task_reservations":                            0,
	} {
		var n int64
		if db.WithContext(ctx).Raw(query).Scan(&n).Error != nil || n != want {
			t.Fatal("持久任务/账务/执行计数不符，查询内容已隐藏")
		}
	}
	t.Log("LOCAL_TASK_API_VERIFIED：原登录JWT→实际API轻深报价/准备确认/冻结/同键重放/会话关联/双会员隔离/取消一次释放/SSE续读通过；零Agent调用与报告结算")
}
