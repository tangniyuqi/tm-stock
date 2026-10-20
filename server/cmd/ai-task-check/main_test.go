package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAITaskCheckConfigurationAndFailureRedaction(t *testing.T) {
	for _, tc := range []struct {
		name, dsn, message string
		failure            error
		calls              int
	}{
		{"缺少配置", "", "缺少环境变量", nil, 0},
		{"缺少时间解析", "test_user@tcp(127.0.0.1:1)/test", "parseTime=true", nil, 0},
		{"非法驱动配置", "invalid-dsn?parseTime=true", "数据库配置无效", nil, 0},
		{"约束失败脱敏", "test_user@tcp(127.0.0.1:1)/test?parseTime=true", "驱动详情已隐藏", errors.New("受控任务诊断标记"), 1},
		{"正向装配", "test_user@tcp(127.0.0.1:1)/test?parseTime=true", "", nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TM_DB_DSN", tc.dsn)
			t.Setenv("TM_QUOTE_ENABLED", "false")
			calls := 0
			var used context.Context
			err := runCheck(func(ctx context.Context, db *sql.DB) error {
				calls++
				used = ctx
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 20*time.Second || db.Stats().MaxOpenConnections != 1 {
					t.Fatal("命令未限制期限或连接数")
				}
				return tc.failure
			})
			if calls != tc.calls {
				t.Fatal("校验调用次数不符", calls)
			}
			if tc.message == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.message) || strings.Contains(err.Error(), "受控任务诊断标记") || (tc.dsn != "" && strings.Contains(err.Error(), tc.dsn)) {
				t.Fatal("未失败关闭或诊断泄漏", err)
			}
			if used != nil && !errors.Is(used.Err(), context.Canceled) {
				t.Fatal("命令未取消上下文")
			}
		})
	}
}
