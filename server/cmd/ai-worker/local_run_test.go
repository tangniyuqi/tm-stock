package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLocalRunCommandRequiresExplicitFilesAndContext(t *testing.T) {
	t.Setenv("TM_DB_DSN", "")
	for _, args := range [][]string{{}, {"-config", "private_runtime_path", "-plans", "private_plans_path", "-execution", "private_execution_path"}} {
		var out bytes.Buffer
		if code := runLocalReports(context.Background(), args, nil, &out); code != 1 || strings.Contains(out.String(), "private_") {
			t.Fatal("缺依赖未关闭或泄漏路径", code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if code := runLocalReports(ctx, []string{"-config", "x", "-plans", "y", "-execution", "z"}, nil, &out); code != 130 {
		t.Fatal(code)
	}
	if code := runCommand(context.Background(), []string{"run-local"}, &out); code != 1 {
		t.Fatal("命令未挂载")
	}
}
