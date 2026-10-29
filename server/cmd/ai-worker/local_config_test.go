package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalConfigCommandDoesNotConnectOrRun(t *testing.T) {
	file := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(file, []byte(`{"version":1,"environment":"local-isolated","instance_id":"0123456789abcdef0123456789abcdef","database":"tm_ai_local_0123456789abcdef0123456789abcdef","listen":"127.0.0.1:8099","real_calls":false,"poll_interval_ms":1000,"batch_size":10}`), 0600); err != nil {
		t.Fatal(err)
	}
	// 无数据库监听的目标也应成功：此子命令只检查配置，不建立数据库/模型连接。
	t.Setenv("TM_DB_DSN", "local:fixture_secret@tcp(127.0.0.1:1)/tm_ai_local_0123456789abcdef0123456789abcdef?parseTime=true")
	var output bytes.Buffer
	if code := runCommand(context.Background(), []string{"check-local", "-config", file}, &output); code != 1 {
		t.Fatal("接受受限端口", code)
	}
	t.Setenv("TM_DB_DSN", "local:fixture_secret@tcp(127.0.0.1:65534)/tm_ai_local_0123456789abcdef0123456789abcdef?parseTime=true")
	output.Reset()
	if code := runCommand(context.Background(), []string{"check-local", "-config", file}, &output); code != 0 || !strings.Contains(output.String(), "尚未验证数据库所有权") {
		t.Fatal("预检未正确说明边界", code)
	}
	if strings.Contains(output.String(), "fixture_secret") || strings.Contains(output.String(), file) {
		t.Fatal("泄漏配置")
	}
	for _, args := range [][]string{{"check-local"}, {"check-local", "-config", file, "extra"}, {"check-local", "-config", file, "-unknown"}} {
		output.Reset()
		if code := runCommand(context.Background(), args, &output); code != 1 {
			t.Fatal("接受非法参数", code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := runCommand(ctx, []string{"check-local", "-config", file}, &output); code != 130 {
		t.Fatal("未响应取消", code)
	}
}
