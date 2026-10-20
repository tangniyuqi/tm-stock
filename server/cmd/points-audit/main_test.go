package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func decodeEvents(t *testing.T, output *bytes.Buffer) []auditEvent {
	t.Helper()
	var events []auditEvent
	decoder := json.NewDecoder(output)
	for {
		var e auditEvent
		err := decoder.Decode(&e)
		if errors.Is(err, io.EOF) {
			return events
		}
		if err != nil {
			t.Fatal(err)
		}
		if e.At <= 0 {
			t.Fatal("告警缺少毫秒时间戳")
		}
		events = append(events, e)
	}
}

func TestPointsAuditOptionsAndConfigurationRedaction(t *testing.T) {
	for _, args := range [][]string{{"-page-size", "0"}, {"-page-size", "1001"}, {"-round-timeout", "0"}, {"-round-timeout", "25h"}, {"-interval", "-1s"}, {"-interval", "1ms"}, {"-interval", "25h"}, {"-round-timeout", "bad"}, {"-unknown", "受控敏感标记"}, {"受控敏感标记"}} {
		var output bytes.Buffer
		if code := runCommand(context.Background(), args, &output); code != 1 || strings.Contains(output.String(), "受控敏感标记") {
			t.Fatal(code, output.String())
		}
	}
	for _, args := range [][]string{nil, {"-page-size", "1000", "-round-timeout", "24h", "-interval", "24h"}} {
		if _, err := parseOptions(args); err != nil {
			t.Fatal(err)
		}
	}
	for _, dsn := range []string{"", "受控敏感标记?parseTime=true", "user@tcp(127.0.0.1:1)/test"} {
		t.Setenv("TM_DB_DSN", dsn)
		t.Setenv("TM_QUOTE_ENABLED", "false")
		var output bytes.Buffer
		if code := runCommand(context.Background(), nil, &output); code != 1 || strings.Contains(output.String(), "受控敏感标记") {
			t.Fatal(code, output.String())
		}
	}
	var help bytes.Buffer
	if runCommand(context.Background(), []string{"-help"}, &help) != 0 || !strings.Contains(help.String(), "退出码") {
		t.Fatal(help.String())
	}
}

func TestPointsAuditLoopSingleRoundResults(t *testing.T) {
	fault := errors.New("受控敏感标记")
	for _, tc := range []struct {
		name   string
		result points.AuditSweep
		err    error
		want   int
	}{
		{"健康", points.AuditSweep{Complete: true, Checked: 2}, nil, 0},
		{"账本异常", points.AuditSweep{Complete: true, Checked: 2, Unhealthy: 1, Issues: 1}, nil, 2},
		{"访问失败", points.AuditSweep{Checked: 1}, fault, 1},
		{"未完成但无错误", points.AuditSweep{Checked: 1}, nil, 1},
		{"错误完整标记", points.AuditSweep{Complete: true}, fault, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			var checkContext, roundContext context.Context
			calls := 0
			code := runAuditLoop(context.Background(), options{timeout: time.Minute}, &output, func(ctx context.Context) error {
				checkContext = ctx
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 20*time.Second {
					t.Fatal("自检未限时")
				}
				return nil
			}, func(ctx context.Context, notify func(points.AuditReport) error) (points.AuditSweep, error) {
				calls++
				roundContext = ctx
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > time.Minute {
					t.Fatal("整轮未限时")
				}
				if tc.result.Unhealthy > 0 {
					if err := notify(points.AuditReport{UserID: 7, Issues: []points.AuditIssue{{Code: "account_lots", RecordID: 7}}}); err != nil {
						t.Fatal(err)
					}
				}
				return tc.result, tc.err
			})
			if code != tc.want || calls != 1 || strings.Contains(output.String(), "受控敏感标记") {
				t.Fatal(code, output.String())
			}
			events := decodeEvents(t, &output)
			if tc.result.Unhealthy > 0 && (len(events) != 2 || events[0].Event != "points_audit_issue" || events[0].UserID != 7) {
				t.Fatal(events)
			}
			last := events[len(events)-1]
			if last.Event != "points_audit_round" || last.Failed != (tc.want == 1) || last.Sweep.Complete == (tc.want == 1) {
				t.Fatal(last)
			}
			for _, ctx := range []context.Context{checkContext, roundContext} {
				if !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatal("退出未取消子上下文")
				}
			}
		})
	}
}

type brokenAuditWriter struct{}

func (brokenAuditWriter) Write([]byte) (int, error) { return 0, errors.New("测试输出通道故障") }

func TestPointsAuditLoopStartupOutputAndCancellation(t *testing.T) {
	var output bytes.Buffer
	sweepCalls := 0
	sweep := func(context.Context, func(points.AuditReport) error) (points.AuditSweep, error) {
		sweepCalls++
		return points.AuditSweep{Complete: true}, nil
	}
	if runAuditLoop(context.Background(), options{}, &output, func(context.Context) error { return errors.New("受控敏感标记") }, sweep) != 1 || sweepCalls != 0 || strings.Contains(output.String(), "受控敏感标记") {
		t.Fatal(output.String())
	}
	verify := func(context.Context) error { return nil }
	startupCtx, startupCancel := context.WithCancel(context.Background())
	startupCancel()
	if runAuditLoop(startupCtx, options{}, &output, func(ctx context.Context) error { return ctx.Err() }, sweep) != 130 {
		t.Fatal("启动期间中断退出码错误")
	}
	if runAuditLoop(context.Background(), options{timeout: time.Minute}, brokenAuditWriter{}, verify, sweep) != 1 {
		t.Fatal("汇总输出失败未拒绝")
	}
	if runAuditLoop(context.Background(), options{timeout: time.Minute}, brokenAuditWriter{}, verify, func(_ context.Context, notify func(points.AuditReport) error) (points.AuditSweep, error) {
		return points.AuditSweep{}, notify(points.AuditReport{UserID: 1, Issues: []points.AuditIssue{{Code: "account_lots"}}})
	}) != 1 {
		t.Fatal("异常输出失败未拒绝")
	}
	output.Reset()
	if runAuditLoop(context.Background(), options{timeout: time.Nanosecond}, &output, verify, func(ctx context.Context, _ func(points.AuditReport) error) (points.AuditSweep, error) {
		<-ctx.Done()
		return points.AuditSweep{Complete: true}, nil
	}) != 1 {
		t.Fatal("超时被误判完整")
	}
	ctx, cancel := context.WithCancel(context.Background())
	output.Reset()
	code := runAuditLoop(ctx, options{timeout: time.Minute, interval: time.Hour}, &output, verify, func(context.Context, func(points.AuditReport) error) (points.AuditSweep, error) {
		cancel()
		return points.AuditSweep{}, context.Canceled
	})
	if code != 130 {
		t.Fatal(code)
	}
	events := decodeEvents(t, &output)
	if len(events) != 1 || !events[0].Failed || events[0].Sweep.Complete {
		t.Fatal(events)
	}
}

func TestPointsAuditPeriodicFailureRetryWithoutOverlap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checks, calls := 0, 0
	var output bytes.Buffer
	code := runAuditLoop(ctx, options{timeout: time.Second, interval: time.Millisecond}, &output, func(context.Context) error { checks++; return nil }, func(_ context.Context, _ func(points.AuditReport) error) (points.AuditSweep, error) {
		calls++
		if calls == 1 {
			return points.AuditSweep{Checked: 1}, errors.New("受控敏感标记")
		}
		cancel()
		return points.AuditSweep{Complete: true, Checked: 2}, nil
	})
	if code != 130 || checks != 1 || calls != 2 || strings.Contains(output.String(), "受控敏感标记") {
		t.Fatal(code, checks, calls, output.String())
	}
	events := decodeEvents(t, &output)
	if len(events) != 2 || !events[0].Failed || events[0].Sweep.Complete || !events[1].Failed {
		t.Fatal(events)
	}
}
