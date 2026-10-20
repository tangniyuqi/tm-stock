package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestExpiryCommandOptionsAndRedaction(t *testing.T) {
	for _, args := range [][]string{nil, {"-after", "9", "-page-size", "1000", "-max-tasks", "100000", "-timeout", "5m"}} {
		if _, err := parseOptions(args); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"-after", "-1"}, {"-page-size", "0"}, {"-max-tasks", "100001"}, {"-timeout", "500ms"}, {"-timeout", "6m"}, {"extra"}, {"-private-marker"}} {
		var out strings.Builder
		if code := runCommand(context.Background(), args, &out); code != 1 || strings.Contains(out.String(), "private-marker") {
			t.Fatal(code, out.String())
		}
	}
	for _, w := range []io.Writer{new(strings.Builder), brokenWriter{}} {
		want := 0
		if _, ok := w.(brokenWriter); ok {
			want = 1
		}
		if code := runCommand(context.Background(), []string{"-help"}, w); code != want {
			t.Fatal(code)
		}
	}
	for _, dsn := range []string{"", "fake-private-marker", "bad?parseTime=true", "test_user@tcp(127.0.0.1:1)/test?parseTime=true&timeout=100ms"} {
		t.Setenv("TM_DB_DSN", dsn)
		t.Setenv("TM_QUOTE_ENABLED", "false")
		var out strings.Builder
		if code := runCommand(context.Background(), nil, &out); code != 1 || strings.Contains(out.String(), "private-marker") || strings.Contains(out.String(), "127.0.0.1") {
			t.Fatal(code, out.String())
		}
	}
	if failure(brokenWriter{}, "固定失败") != 1 {
		t.Fatal("输出失败")
	}
}

func TestExpiryCommandSweepGateAndExit(t *testing.T) {
	fault := errors.New("fake-private-marker")
	for _, mode := range []string{"complete", "bounded", "verify", "verify_cancel", "sweep", "cancel", "output"} {
		t.Run(mode, func(t *testing.T) {
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			opts, _ := parseOptions(nil)
			var buf strings.Builder
			var output io.Writer = &buf
			if mode == "output" {
				output = brokenWriter{}
			}
			called := 0
			var verifyCtx, sweepCtx context.Context
			code := runSweep(ctx, opts, output, func(ctx context.Context) error {
				verifyCtx = ctx
				if d, ok := ctx.Deadline(); !ok || time.Until(d) > 20*time.Second {
					t.Fatal("自检期限")
				}
				if mode == "verify_cancel" {
					stop()
				}
				if mode == "verify" {
					return fault
				}
				return nil
			}, func(ctx context.Context, o task.ExpiryOptions) (task.ExpirySweep, error) {
				called++
				sweepCtx = ctx
				if d, ok := ctx.Deadline(); !ok || time.Until(d) > opts.timeout || o != opts.sweep {
					t.Fatal("清扫期限/参数")
				}
				v := task.ExpirySweep{UpperID: 9, LastID: 3, Visited: 2, ExpiredObserved: 1, Skipped: 1, Complete: true}
				if mode == "bounded" {
					v.Complete = false
				}
				if mode == "cancel" {
					stop()
				}
				if mode == "sweep" {
					return v, fault
				}
				return v, nil
			})
			want := 0
			if mode == "bounded" {
				want = 2
			}
			if mode == "verify" || mode == "sweep" || mode == "output" {
				want = 1
			}
			if mode == "cancel" || mode == "verify_cancel" {
				want = 130
			}
			if code != want || strings.Contains(buf.String(), "private-marker") {
				t.Fatal(code, want, buf.String())
			}
			if !errors.Is(verifyCtx.Err(), context.Canceled) {
				t.Fatal("自检上下文未释放")
			}
			if mode == "verify" || mode == "verify_cancel" {
				if called != 0 {
					t.Fatal("失败自检后仍清扫")
				}
				return
			}
			if called != 1 || !errors.Is(sweepCtx.Err(), context.Canceled) {
				t.Fatal("清扫次数或上下文")
			}
			if mode != "output" {
				var r receipt
				if err := json.Unmarshal([]byte(buf.String()), &r); err != nil || r.Sweep == nil || r.Sweep.LastID != 3 {
					t.Fatal(r, err)
				}
				if mode == "sweep" || mode == "cancel" {
					if r.Sweep.Complete || !r.Failed {
						t.Fatal("错误结果被当作完整", r)
					}
				}
			}
		})
	}
}
