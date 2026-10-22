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

// 复用原报告启动的可丢弃库，新增表的DDL不在生产启动路径执行。
func checkConversationStartup(t *testing.T, ctx context.Context, db *sql.DB, read config.AITaskReadConfig) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261021_create_tm_ai_conversations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(raw), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;")
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		name := "conversation_off"
		if on {
			name = "conversation_on"
		}
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			if err := registerAIConversations(ctx, mux, db, read, on); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/conversations", nil))
			want := 404
			if on {
				want = 401
			}
			if w.Code != want {
				t.Fatal(w.Code)
			}
		})
	}
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if err := registerAIConversations(ended, http.NewServeMux(), db, read, true); err == nil {
		t.Fatal("取消仍装配会话")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tm_ai_conversations").Scan(&count); err != nil || count != 0 {
		t.Fatal("启动探针未回滚", count, err)
	}
}
