//go:build integration

package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 复用报告启动真库夹具；不依赖报告发布或真实模型控制器。
func checkFeedbackStartup(t *testing.T, ctx context.Context, db *sql.DB, read config.AITaskReadConfig) {
	t.Helper()
	for _, on := range []bool{false, true} {
		name := "feedback_off"
		if on {
			name = "feedback_on"
		}
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			if err := registerAIFeedback(ctx, mux, db, read, on); err != nil {
				t.Fatal(err)
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"ID": 1, "UserType": "client", "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte(read.Auth.SigningKey))
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/api/v1/ai/tasks/1/feedback", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			want := 404
			if on {
				want = 422
			}
			if w.Code != want {
				t.Fatal("启动路由未隔离", w.Code, w.Body.String())
			}
			if on {
				r.Header.Del("Authorization")
				w = httptest.NewRecorder()
				mux.ServeHTTP(w, r)
				if w.Code != 401 {
					t.Fatal("启动未装会员认证")
				}
			}
		})
	}
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if err := registerAIFeedback(ended, http.NewServeMux(), db, read, true); err == nil {
		t.Fatal("已取消仍注册反馈")
	}
}
