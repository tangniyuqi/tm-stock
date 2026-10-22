//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 该宿主只在本地联调时启用：真实 MySQL + 真实问答服务 + 回环模型，
// 供官方 H5 页面经 Vite 代理访问；不装配生产开关，也不写真实用户数据。
func TestIntegrationAIConversationsQABrowserHost(t *testing.T) {
	if os.Getenv("TM_TEST_BROWSER_HOST") != "1" {
		t.Skip("未设置 TM_TEST_BROWSER_HOST，跳过浏览器宿主")
	}
	readyPath, donePath := os.Getenv("TM_TEST_BROWSER_READY"), os.Getenv("TM_TEST_BROWSER_DONE")
	if readyPath == "" || donePath == "" {
		t.Fatal("浏览器宿主缺少 ready/done 文件路径")
	}
	x := newQASQL(t, sql.LevelReadCommitted, "success")
	models := &qaHTTPModels{x: x}
	qaSvc := qaHTTPService(t, x, true, models)
	convRepo := repository.NewAIConversationRepoWithIsolation(x.db, sql.LevelReadCommitted)
	convSvc, err := service.NewAIConversationService(convRepo)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.NewAIConversationHandler(convSvc).Register(mux, x.auth)
	handler.NewAIQAHandler(qaSvc).Register(mux, x.auth, true)
	mux.HandleFunc("/api/v1/theme/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":[{"id":1,"name":"示例题材","description":"本地联调测试目录"}]}`))
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatal("浏览器宿主端口不可用（需要 Vite 代理目标 127.0.0.1:8080）", err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	ready := map[string]any{
		"token":        x.token(t, 1, "client", x.key),
		"conversation": x.conv.ID,
		"subjectId":    1,
		"subjectName":  x.scope.SubjectName,
		"question":     x.input.Question,
		"excerpt":      x.scope.Snapshots[0].Sentences[0].Text,
	}
	raw, err := json.Marshal(ready)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Minute)
	for {
		if _, err := os.Stat(donePath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("等待浏览器完成超时")
		}
		time.Sleep(200 * time.Millisecond)
	}
	// 浏览器只通过真实 HTTP 写入；这里核对恰好一次执行与一份答案。
	if got := x.provider.Count(); got != 3 {
		t.Fatalf("回环模型调用次数应为3，实际%d", got)
	}
	if got := x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`); got != 1 {
		t.Fatalf("问答记录应恰好1条，实际%d", got)
	}
	if got := x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE kind='question'`); got != 1 {
		t.Fatalf("问题消息应恰好1条，实际%d", got)
	}
	if got := x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls WHERE state='recorded'`); got != 3 {
		t.Fatalf("模型调用存证应恰好3条，实际%d", got)
	}
	var state string
	if err := x.db.QueryRowContext(x.ctx, `SELECT state FROM tm_ai_qa_requests LIMIT 1`).Scan(&state); err != nil || state != "answered" {
		t.Fatalf("问答终态应为answered，实际%q err=%v", state, err)
	}
	x.uncharged(t)
	t.Log("BROWSER_HOST_PASS: 官方H5经Vite代理→真实HTTP→MySQL→回环模型恰好一次执行")
}
