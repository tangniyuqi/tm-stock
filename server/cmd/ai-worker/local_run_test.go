package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestLocalWorkerFailureCodeDoesNotExposeOriginalError(t *testing.T) {
	for _, row := range []struct {
		name string
		err  error
		want string
	}{
		{"数据库权限", &mysqldrv.MySQLError{Number: 1142, Message: "private_payload"}, "mysql_1142"},
		{"数据库死锁", &mysqldrv.MySQLError{Number: 1213, Message: "private_payload"}, "mysql_1213"},
		{"包装租约", fmt.Errorf("private_payload: %w", task.ErrLease), "lease"},
		{"未知调用", task.ErrUnconfirmed, "unconfirmed"},
		{"超时", context.DeadlineExceeded, "deadline"},
		{"不变量", points.ErrInvariant, "invariant"},
		{"未知错误", fmt.Errorf("private_payload"), "internal"},
		{"无原因退出", nil, "internal"},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := localWorkerFailureCode(row.err); got != row.want || strings.Contains(got, "private_payload") {
				t.Fatal("本地退出诊断类别不符或泄漏原错误")
			}
		})
	}
}

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
