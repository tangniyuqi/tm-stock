//go:build integration

package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 复用报告启动隔离库，不复制迁移或自造生产配置；真实流在points事件组验收。
func checkTaskStreamStartup(t *testing.T, ctx context.Context, db *sql.DB, read config.AITaskReadConfig) {
	t.Helper()
	for _, on := range []bool{false, true} {
		name := "stream_off"
		if on {
			name = "stream_on"
		}
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			if err := registerAITaskStreams(ctx, mux, db, read, on); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/tasks/1/stream", nil))
			want := 404
			if on {
				want = 401
			}
			if w.Code != want {
				t.Fatal("流启动路由/会员鉴权不一致", w.Code)
			}
		})
	}
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if err := registerAITaskStreams(ended, http.NewServeMux(), db, read, true); err == nil {
		t.Fatal("已取消仍注册流")
	}
}
