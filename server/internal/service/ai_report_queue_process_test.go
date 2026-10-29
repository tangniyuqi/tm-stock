package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestAIReportQueuesStopsAndJoinsSibling(t *testing.T) {
	for _, firstNil := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		started, stopped := make(chan struct{}), make(chan struct{})
		failure := points.ErrInvariant
		err := runAIReportQueues(ctx, func(context.Context) error {
			<-started
			if firstNil {
				return nil
			}
			return failure
		}, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			close(stopped)
			return ctx.Err()
		})
		cancel()
		if !errors.Is(err, failure) {
			t.Fatal("未保留首个异常", err)
		}
		select {
		case <-stopped:
		default:
			t.Fatal("另一循环尚未退出")
		}
	}
}

func TestAIReportQueuesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 2)
	run := func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() }
	done := make(chan error, 1)
	go func() { done <- runAIReportQueues(ctx, run, run) }()
	<-started
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("取消后未收尾")
	}
	called := false
	if err := runAIReportQueues(ctx, func(context.Context) error { called = true; return nil }, run); !errors.Is(err, context.Canceled) || called {
		t.Fatal("取消后仍启动", err)
	}
}

func TestAIReportQueuesRejectsInvalidPair(t *testing.T) {
	light, scan, runner, _ := queueFixture(t)
	deep := &AIL1ReportQueue{deep: true, enabled: true}
	if err := RunAIReportQueues(context.Background(), light, deep, 1, time.Second); !errors.Is(err, points.ErrInvalid) || scan.queries != 0 || runner.calls != 0 {
		t.Fatal("深度依赖不完整时启动了轻度队列", err)
	}
	if err := RunAIReportQueues(context.Background(), light, light, 1, time.Second); !errors.Is(err, points.ErrInvalid) || scan.queries != 0 || runner.calls != 0 {
		t.Fatal("同一队列重复启动", err)
	}
	if err := RunAIReportQueues(nil, light, nil, 1, time.Second); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
}
