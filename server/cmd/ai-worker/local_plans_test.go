package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLocalPlansCommandFailsClosed(t *testing.T) {
	t.Setenv("TM_DB_DSN", "forbidden_private_dsn")
	for _, args := range [][]string{{"check-local-plans"}, {"check-local-plans", "-config", "relative.json", "-plans", "relative.json"}, {"check-local-plans", "-config", "private_path", "-config", "private_path"}, {"check-local-plans", "-config", "private_path", "-plans", "private_path", "extra"}} {
		var output bytes.Buffer
		if code := runCommand(context.Background(), args, &output); code != 1 || !strings.Contains(output.String(), "未启动任务") {
			t.Fatal("缺参数/缺配置未拒绝")
		}
		if strings.Contains(output.String(), "private_path") || strings.Contains(output.String(), "forbidden_private_dsn") {
			t.Fatal("错误泄漏配置")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if code := runCommand(ctx, []string{"check-local-plans", "-config", "private_path", "-plans", "private_path"}, &output); code != 130 {
		t.Fatal("未响应取消")
	}
}
