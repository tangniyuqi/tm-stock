package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestReportQueueLoopRescansAndStops(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	var cursors []int64
	waits := 0
	err := runReportQueueLoop(ctx, 1, time.Second, func(_ context.Context, o task.MemberQueueOptions) (task.MemberQueueReceipt, error) {
		cursors = append(cursors, o.AfterID)
		if len(cursors) == 1 {
			return task.MemberQueueReceipt{LastID: 1, UpperID: 2}, nil
		}
		return task.MemberQueueReceipt{LastID: 2, UpperID: 2, Complete: true}, nil
	}, func(context.Context, time.Duration) error {
		waits++
		if waits == 3 {
			stop()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || waits != 3 || !reflect.DeepEqual(cursors, []int64{0, 1, 0}) {
		t.Fatal(cursors, waits, err)
	}
}

func TestReportQueueLoopIdleCancellation(t *testing.T) {
	q, scan, runner, _ := queueFixture(t)
	scan.window.UpperID, scan.page = 0, nil
	ctx, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer stop()
	if err := q.RunLoop(ctx, 1, time.Minute); !errors.Is(err, context.DeadlineExceeded) || scan.queries != 1 || runner.calls != 0 {
		t.Fatal("空闲等待未响应取消或触发执行", err, scan.queries, runner.calls)
	}
	for _, interval := range []time.Duration{0, time.Millisecond, 2 * time.Minute} {
		before := scan.queries
		if err := q.RunLoop(context.Background(), 1, interval); !errors.Is(err, points.ErrInvalid) || scan.queries != before {
			t.Fatal("非法轮询配置访问了队列", err)
		}
	}
}

func TestReportQueueLoopFailsClosed(t *testing.T) {
	for _, mode := range []string{"error", "unknown", "stalled", "invalid_complete", "disabled", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			q, _, runner, _ := queueFixture(t)
			if mode == "disabled" {
				q.enabled = false
				if err := q.RunLoop(context.Background(), 1, time.Second); err == nil || runner.calls != 0 {
					t.Fatal(err)
				}
				return
			}
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			if mode == "cancelled" {
				stop()
			}
			calls, waits := 0, 0
			err := runReportQueueLoop(ctx, 1, time.Second, func(context.Context, task.MemberQueueOptions) (task.MemberQueueReceipt, error) {
				calls++
				if mode == "error" {
					return task.MemberQueueReceipt{}, points.ErrInvariant
				}
				if mode == "unknown" {
					return task.MemberQueueReceipt{Pending: 1}, nil
				}
				if mode == "invalid_complete" {
					return task.MemberQueueReceipt{LastID: 1, UpperID: 2, Complete: true}, nil
				}
				return task.MemberQueueReceipt{UpperID: 1}, nil
			}, func(context.Context, time.Duration) error { waits++; return context.Canceled })
			wantCalls := 1
			if mode == "cancelled" {
				wantCalls = 0
			}
			if err == nil || calls != wantCalls || waits != 0 {
				t.Fatal(calls, waits, err)
			}
		})
	}
}
