package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLocalAuditCommandFailsClosedBeforeDatabase(t *testing.T) {
	t.Setenv("TM_DB_DSN", "")
	for _, args := range [][]string{{"check-local-audit"}, {"check-local-audit", "-plans", "not-a-config"}, {"check-local-audit", "-config", "missing-local-audit-config"}} {
		var out bytes.Buffer
		if code := runCommand(context.Background(), args, &out); code != 1 || !strings.Contains(out.String(), "阶段1") || strings.Contains(out.String(), "missing-local-audit-config") {
			t.Fatal("缺配置没有失败关闭或泄漏路径", code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if code := runCommand(ctx, []string{"check-local-audit", "-config", "never-read"}, &out); code != 130 {
		t.Fatal("取消未退出", code)
	}
	if code := checkLocalAudit(nil, []string{"-config", "never-read"}, &out); code != 1 {
		t.Fatal("空上下文未拒绝")
	}
}
