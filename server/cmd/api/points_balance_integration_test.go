//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 使用正式注册函数、JWT校验及数据库会员状态；令牌仅为隔离测试签发，不冒充原登录页验收。
func checkPointsBalanceStartup(t *testing.T, ctx context.Context, db *sql.DB, read config.AITaskReadConfig) {
	t.Helper()
	t.Run("points_balance_auth", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, `INSERT INTO addon_member VALUES(902,1,NULL),(903,0,NULL),(904,1,UTC_TIMESTAMP(3))`); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := db.ExecContext(ctx, `DELETE FROM addon_member WHERE id IN (902,903,904)`); err != nil {
				t.Error(err)
			}
		}()
		mux := http.NewServeMux()
		if err := registerPointsBalance(ctx, mux, db, read, true); err != nil {
			t.Fatal(err)
		}
		var before int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_points_accounts`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name              string
			uid               int
			kind              string
			want              int
			available, frozen int64
		}{
			{"owner", 1, "client", 200, 13, 7}, {"other_empty", 902, "client", 200, 0, 0},
			{"disabled", 903, "client", 401, 0, 0}, {"deleted", 904, "client", 401, 0, 0},
			{"missing", 905, "client", 401, 0, 0}, {"admin", 1, "admin", 401, 0, 0},
			{"expired", 1, "client", 401, 0, 0}, {"bad_signature", 1, "client", 401, 0, 0},
			{"anonymous", 1, "client", 401, 0, 0}, {"other_query", 1, "client", 400, 0, 0},
		} {
			t.Run(tc.name, func(t *testing.T) {
				exp := time.Now().Add(time.Hour)
				if tc.name == "expired" {
					exp = time.Now().Add(-time.Hour)
				}
				claims := jwt.MapClaims{"ID": tc.uid, "UserType": tc.kind, "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": exp.Unix(), "nbf": time.Now().Add(-2 * time.Hour).Unix()}
				key := read.Auth.SigningKey
				if tc.name == "bad_signature" {
					key += "invalid"
				}
				token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
				if err != nil {
					t.Fatal(err)
				}
				url := "/api/v1/ai/points/balance"
				if tc.name == "other_query" {
					url += "?user_id=902"
				}
				r := httptest.NewRequest("GET", url, nil)
				if tc.name != "anonymous" {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("状态=%d，预期=%d", w.Code, tc.want)
				}
				if tc.want == 200 {
					var body struct {
						Code int `json:"code"`
						Data struct {
							Available int64 `json:"availablePoints"`
							Frozen    int64 `json:"frozenPoints"`
							At        int64 `json:"checkedAt"`
						} `json:"data"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if body.Code != 0 || body.Data.Available != tc.available || body.Data.Frozen != tc.frozen || body.Data.At <= 0 || w.Header().Get("Cache-Control") != "no-store" {
						t.Fatal("余额或缓存契约不符", body)
					}
				}
			})
		}
		var after int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_points_accounts`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatal("只读接口创建了账户")
		}
		closed := http.NewServeMux()
		if err := registerPointsBalance(ctx, closed, db, read, false); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		closed.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/points/balance", nil))
		if w.Code != 404 {
			t.Fatal(w.Code)
		}
		ended, cancel := context.WithCancel(ctx)
		cancel()
		if err := registerPointsBalance(ended, http.NewServeMux(), db, read, true); err == nil {
			t.Fatal("已取消仍完成余额启动")
		}
	})
}
