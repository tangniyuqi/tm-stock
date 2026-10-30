package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func TestLocalResumeRejectsAmbiguousTargets(t *testing.T) {
	base := []string{"-config", "private_runtime", "-plans", "private_plans", "-execution", "private_execution", "-user", "1", "-task", "9007199254740991"}
	got, err := parseLocalReportTarget(base)
	if err != nil || got.userID != 1 || got.taskID != 9007199254740991 {
		t.Fatal("有效目标未解析")
	}
	for _, row := range []struct{ name, value string }{
		{"零", "0"}, {"负数", "-1"}, {"溢出", "9223372036854775808"}, {"非安全整数", "9007199254740992"},
		{"正号", "+1"}, {"前导零", "01"}, {"空白", " 1"}, {"指数", "1e2"}, {"空值", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			for _, pos := range []int{7, 9} {
				args := append([]string(nil), base...)
				args[pos] = row.value
				if _, err := parseLocalReportTarget(args); err == nil {
					t.Fatal("有歧义的恢复目标被接受")
				}
			}
		})
	}
	for _, args := range [][]string{nil, base[:9], append(append([]string(nil), base...), "-stop-on-stdin")} {
		var out bytes.Buffer
		if code := resumeLocalReport(context.Background(), args, &out); code != 1 || strings.Contains(out.String(), "private_") {
			t.Fatal("恢复参数未关闭或泄漏路径")
		}
	}
	args := append([]string(nil), base...)
	args[6] = "-task"
	if _, err := parseLocalReportTarget(args); err == nil {
		t.Fatal("重复字段被接受")
	}
	t.Setenv("TM_DB_DSN", "")
	var out bytes.Buffer
	if code := runCommand(context.Background(), append([]string{"resume-local"}, base...), &out); code != 1 || strings.Contains(out.String(), "private_") {
		t.Fatal("恢复命令缺依赖未拒绝或泄漏")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := resumeLocalReport(ctx, base, &out); code != 130 {
		t.Fatal("恢复忽略已取消上下文")
	}
	if code := resumeLocalReport(nil, base, &out); code != 1 {
		t.Fatal("空上下文未拒绝")
	}
}

func TestLocalResumeReceiptNeverClaimsUnknownSuccess(t *testing.T) {
	for _, row := range []struct {
		name, outcome string
		status, code  int
		err           error
	}{
		{"成功终态", "terminal", task.Succeeded, 0, nil}, {"失败终态", "terminal", task.Failed, 0, nil},
		{"取消终态", "terminal", task.Cancelled, 0, nil}, {"超期终态", "terminal", task.Expired, 0, nil},
		{"未知响应", "response_unknown", task.Running, 2, nil}, {"候选待提交", "final_prepared", task.Running, 2, nil},
		{"矛盾回执", "response_unknown", task.Succeeded, 2, nil}, {"无效状态", "terminal", 99, 1, nil},
		{"领取拒绝", "", task.Running, 1, task.ErrNotReady}, {"私密错误", "", task.Running, 1, errors.New("private_payload")},
	} {
		t.Run(row.name, func(t *testing.T) {
			out := service.AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: 1, TaskStatus: row.status, Outcome: row.outcome}}
			var log bytes.Buffer
			if code := localResumeReceipt(context.Background(), out, row.err, &log); code != row.code || strings.Contains(log.String(), "private_") {
				t.Fatal("恢复回执误报或泄漏", code)
			}
		})
	}
	var log bytes.Buffer
	if localResumeReceipt(context.Background(), service.AIL1ReportReceipt{}, nil, &log) != 1 {
		t.Fatal("空回执未拒绝")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if localResumeReceipt(ctx, service.AIL1ReportReceipt{}, nil, &log) != 130 {
		t.Fatal("中断回执错误")
	}
	terminal := service.AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: 1, TaskStatus: task.Succeeded, Outcome: "terminal"}}
	if localResumeReceipt(context.Background(), terminal, nil, failingLocalReceiptWriter{}) != 1 {
		t.Fatal("忽略输出失败")
	}
}

type failingLocalReceiptWriter struct{}

func (failingLocalReceiptWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
