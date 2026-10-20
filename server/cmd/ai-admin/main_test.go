package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 测试驱动只支持健康检查和空来源清单，不连接任何数据库。
type bootstrapConnector struct {
	pingErr, queryErr error
	onPing            func()
}

func (c bootstrapConnector) Connect(context.Context) (driver.Conn, error) {
	return bootstrapConn{c}, nil
}
func (c bootstrapConnector) Driver() driver.Driver { return bootstrapDriver{} }

type bootstrapDriver struct{}

func (bootstrapDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("测试驱动只允许连接器")
}

type bootstrapConn struct{ bootstrapConnector }

func (bootstrapConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (bootstrapConn) Close() error                        { return nil }
func (bootstrapConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c bootstrapConn) Ping(context.Context) error {
	if c.onPing != nil {
		c.onPing()
	}
	return c.pingErr
}
func (c bootstrapConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return bootstrapRows{}, nil
}

type bootstrapRows struct{}

func (bootstrapRows) Columns() []string         { return []string{"id"} }
func (bootstrapRows) Close() error              { return nil }
func (bootstrapRows) Next([]driver.Value) error { return io.EOF }

func TestAdminStartDefaultDisabledAndShutdown(t *testing.T) {
	ctx := context.Background()
	db := sql.OpenDB(bootstrapConnector{})
	defer db.Close()
	cfg := &config.AIAdminConfig{Addr: "127.0.0.1:0", Reviewers: []config.AIAdminReviewer{{ActorID: 7, Token: strings.Repeat("test_placeholder_", 3)}}}
	served := false
	serve := func(_ context.Context, srv *http.Server) error {
		served = true
		for _, tc := range []struct {
			path, method, body string
			authenticated      bool
			want               int
		}{
			{"/internal/v1/ai-extractions", http.MethodPost, `{"snapshot_id":1,"request_id":"test_request_00001"}`, true, 503},
			{"/internal/v1/ai-review-candidates", http.MethodGet, "", false, 401},
			{"/ai/tasks", http.MethodPost, `{}`, true, 404},
			{"/internal/v1/ai-inspection-decisions", http.MethodPost, `{}`, true, 400},
			{"/internal/v1/ai-sources/1/audits?limit=201", http.MethodGet, "", true, 400},
		} {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.RemoteAddr = "127.0.0.1:12345"
			if tc.authenticated {
				r.Header.Set("Authorization", "Bearer "+cfg.Reviewers[0].Token)
			}
			w := httptest.NewRecorder()
			srv.Handler.ServeHTTP(w, r)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("装配鉴权/路由错误: %s %d", tc.path, w.Code)
			}
		}
		return nil
	}
	if err := startAdmin(ctx, cfg, &config.AIL0Config{}, db, serve); err != nil {
		t.Fatalf("默认关闭启动/退出失败: %v", err)
	}
	if !served {
		t.Fatal("未完成服务装配")
	}
}

func TestAdminStartupFailuresAreSanitized(t *testing.T) {
	for _, tc := range []struct {
		name      string
		connector bootstrapConnector
		cfg       config.AIAdminConfig
		l0        config.AIL0Config
		want      string
	}{
		{"数据库失败", bootstrapConnector{pingErr: errors.New("sensitive-dsn")}, config.AIAdminConfig{}, config.AIL0Config{}, "数据库不可用"},
		{"凭据失败", bootstrapConnector{}, config.AIAdminConfig{}, config.AIL0Config{}, "内部审核凭据"},
		{"来源门禁", bootstrapConnector{queryErr: errors.New("sensitive-source")}, config.AIAdminConfig{Reviewers: []config.AIAdminReviewer{{ActorID: 7, Token: strings.Repeat("test_placeholder_", 3)}}}, config.AIL0Config{Enabled: true}, "来源启动校验失败"},
		{"模型门禁", bootstrapConnector{}, config.AIAdminConfig{Reviewers: []config.AIAdminReviewer{{ActorID: 7, Token: strings.Repeat("test_placeholder_", 3)}}}, config.AIL0Config{Enabled: true}, "调用预算"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := sql.OpenDB(tc.connector)
			defer db.Close()
			err := startAdmin(context.Background(), &tc.cfg, &tc.l0, db, serveAdmin)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("启动失败未封闭: %v", err)
			}
		})
	}
}

func TestAdminServeFailureAndClosedServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := serveAdmin(ctx, &http.Server{Addr: "127.0.0.1:0"}); err != nil {
		t.Fatal(err)
	}
	if err := serveAdmin(context.Background(), &http.Server{Addr: "invalid-address"}); err == nil || err.Error() != "内部审核监听失败" {
		t.Fatal(err)
	}
	srv := &http.Server{Addr: "127.0.0.1:0"}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if err := serveAdmin(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
}

func TestAdminRunRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("TM_DB_DSN", "test_fixture?parseTime=true")
	t.Setenv("TM_AI_ADMIN_ADDR", "0.0.0.0:8081")
	if err := run(); err == nil {
		t.Fatal("公网监听配置未拒绝")
	}
}
