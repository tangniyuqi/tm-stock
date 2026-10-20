package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPointsCheckConfigurationAndFailureRedaction(t *testing.T) {
	for _, tc := range []struct {
		name, dsn, message string
		verifyError        error
		wantCalls          int
	}{
		{"缺少配置", "", "缺少环境变量", nil, 0},
		{"未配置时间解析", "test_user@tcp(127.0.0.1:1)/test", "parseTime=true", nil, 0},
		{"非法驱动配置", "invalid-dsn?parseTime=true", "数据库配置无效", nil, 0},
		{"约束失败脱敏", "test_user@tcp(127.0.0.1:1)/test?parseTime=true", "驱动详情已隐藏", errors.New("受控诊断标记"), 1},
		{"正向装配", "test_user@tcp(127.0.0.1:1)/test?parseTime=true", "", nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TM_DB_DSN", tc.dsn)
			t.Setenv("TM_QUOTE_ENABLED", "false")
			calls := 0
			var usedContext context.Context
			err := runCheck(func(ctx context.Context, db *sql.DB) error {
				calls++
				usedContext = ctx
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 20*time.Second || db.Stats().MaxOpenConnections != 1 {
					t.Fatal("装配缺少限时或单连接限制")
				}
				return tc.verifyError
			})
			if calls != tc.wantCalls {
				t.Fatalf("校验调用次数=%d，期望 %d", calls, tc.wantCalls)
			}
			if tc.message == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.message) || strings.Contains(err.Error(), "受控诊断标记") || (tc.dsn != "" && strings.Contains(err.Error(), tc.dsn)) {
				t.Fatalf("失败未关闭或详情泄露: %v", err)
			}
			if usedContext != nil && !errors.Is(usedContext.Err(), context.Canceled) {
				t.Fatal("退出未取消上下文")
			}
		})
	}
}
