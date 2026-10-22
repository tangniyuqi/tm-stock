//go:build integration

package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 只装配关闭原创建的真实收据服务；启动探针不产生任务或账务。
func checkConversationTaskStartup(t *testing.T, ctx context.Context, db *sql.DB, read config.AITaskReadConfig) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261021_create_tm_ai_conversations_tasks.sql"))
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(raw), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;")
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	creator, err := prepareAITaskCreates(ctx, db, read, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		name := "conversation_tasks_off"
		if on {
			name = "conversation_tasks_on"
		}
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			if err := registerAIConversationTasks(ctx, mux, db, read, creator, on, false); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/conversations/1/task-intents", nil))
			want := 404
			if on {
				want = 401
			}
			if w.Code != want {
				t.Fatal(w.Code)
			}
			for _, path := range []string{"/task-intents", "/task-intents/1/confirm"} {
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/ai/conversations/1"+path, nil))
				if w.Code != 404 && w.Code != 405 {
					t.Fatal("关闭创建却有写路由", w.Code)
				}
			}
		})
	}
	if err := registerAIConversationTasks(ctx, http.NewServeMux(), db, read, nil, true, true); err == nil {
		t.Fatal("空原创建器仍装配")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_conversation_tasks`).Scan(&count); err != nil || count != 0 {
		t.Fatal("启动探针未回滚", count, err)
	}
}
