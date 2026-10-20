package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func writePlan(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validPlan = `{"steps":[{"snapshot_id":42,"operation_key":"recover_operation_0001"}]}`

func TestRecoveryCLIPlanAndOptions(t *testing.T) {
	path := writePlan(t, validPlan)
	if _, err := parseOptions([]string{"recover", "-user", "1", "-task", "2", "-plan", path}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"run"}, {"recover"}, {"recover", "-user", "-1", "-task", "2", "-plan", path}, {"recover", "-user", "1", "-task", "2", "-plan", path, "fake-extra"}, {"recover", "-secret", "fake-sensitive-marker"}} {
		var out bytes.Buffer
		if code := runCommand(context.Background(), args, &out); code != 1 || strings.Contains(out.String(), "fake-sensitive-marker") {
			t.Fatal(code, out.String())
		}
	}
	if specs, err := readPlan(path); err != nil || len(specs) != 1 || specs[0].SnapshotID != 42 {
		t.Fatal(specs, err)
	}
	for _, raw := range []string{`{}`, `null`, `[]`, `{"Steps":[]}`, `{"steps":null}`, `{"steps":[]}`, `{"steps":[],"model":"fake-model"}`, `{"steps":[],"STEPS":[]}`, `{"steps":[{"Snapshot_ID":42,"operation_key":"recover_operation_0001"}]}`, `{"steps":[{"snapshot_id":42,"operation_key":"recover_operation_0001","passed":true}]}`, `{"steps":[{"snapshot_id":"42","operation_key":"recover_operation_0001"}]}`, `{"steps":[{"snapshot_id":0,"operation_key":"short"}]}`, strings.Repeat("a", maxPlanBytes+1), validPlan + `{}`} {
		if _, err := readPlan(writePlan(t, raw)); err == nil {
			t.Fatal("不安全恢复计划被放行")
		}
	}
	if _, err := readPlan(t.TempDir()); err == nil {
		t.Fatal("目录当作计划")
	}
	if _, err := readPlan(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("缺文件")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRecoveryCLIStartupAndOutputBoundaries(t *testing.T) {
	for _, outcome := range []struct {
		result task.RecoveryResult
		err    error
		want   int
	}{
		{task.RecoveryResult{TaskID: 1, TaskStatus: task.Running, CurrentStep: 1, PlannedSteps: 1, Outcome: "report_pending"}, nil, 0},
		{task.RecoveryResult{TaskID: 1, TaskStatus: task.Running, PlannedSteps: 1, Outcome: "response_unknown"}, nil, 2},
		{task.RecoveryResult{TaskID: 1, TaskStatus: task.Cancelled, Outcome: "terminal"}, nil, 0},
		{task.RecoveryResult{}, nil, 1},
		{task.RecoveryResult{TaskID: 1, TaskStatus: task.Running, Outcome: "terminal"}, nil, 1},
		{task.RecoveryResult{TaskID: 1, TaskStatus: task.Running, Outcome: "report_pending"}, nil, 1},
		{task.RecoveryResult{TaskID: 1, TaskStatus: task.Running, Outcome: "response_unknown"}, nil, 1},
		{task.RecoveryResult{TaskID: 1, Outcome: "other"}, nil, 1},
		{task.RecoveryResult{}, errors.New("fake-sensitive-marker"), 1},
	} {
		var out bytes.Buffer
		verifyCalls := 0
		code := execute(context.Background(), &out, func(ctx context.Context) error {
			verifyCalls++
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 20*time.Second {
				t.Fatal("未限制启动探针")
			}
			return nil
		}, func(context.Context) (task.RecoveryResult, error) { return outcome.result, outcome.err })
		if code != outcome.want || verifyCalls != 1 || strings.Contains(out.String(), "fake-sensitive-marker") {
			t.Fatal(code, out.String())
		}
	}
	if code := execute(context.Background(), io.Discard, func(context.Context) error { return errors.New("fake-sensitive-marker") }, func(context.Context) (task.RecoveryResult, error) {
		t.Fatal("探针失败仍恢复")
		return task.RecoveryResult{}, nil
	}); code != 1 {
		t.Fatal(code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := execute(ctx, io.Discard, nil, nil); code != 130 {
		t.Fatal(code)
	}
	if code := runCommand(context.Background(), []string{"-help"}, brokenWriter{}); code != 1 {
		t.Fatal(code)
	}
	var help bytes.Buffer
	if code := runCommand(context.Background(), []string{"-help"}, &help); code != 0 || !strings.Contains(help.String(), "不发起模型") {
		t.Fatal(code)
	}
	path := writePlan(t, validPlan)
	args := []string{"recover", "-user", "1", "-task", "2", "-plan", path}
	for _, dsn := range []string{"", "invalid-dsn?parseTime=true", "test_user@tcp(127.0.0.1:1)/test?parseTime=true&timeout=100ms"} {
		t.Setenv("TM_DB_DSN", dsn)
		t.Setenv("TM_QUOTE_ENABLED", "false")
		var out bytes.Buffer
		if code := runCommand(context.Background(), args, &out); code != 1 || (dsn != "" && strings.Contains(out.String(), dsn)) {
			t.Fatal(code, out.String())
		}
	}
	if code := failure(brokenWriter{}); code != 1 {
		t.Fatal(code)
	}
}
