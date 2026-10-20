package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type l0StoreFake struct {
	mu            sync.Mutex
	record        pipeline.RunRecord
	finishFailure bool
	begins        int
	finishes      int
}

func (s *l0StoreFake) BeginRun(_ context.Context, r pipeline.RunRequest, actor int64, p pipeline.RunPolicy) (pipeline.RunRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.begins++
	if s.record.ID > 0 {
		if s.record.ActorID != actor || s.record.Request != r {
			return pipeline.RunRecord{}, false, pipeline.ErrRunConflict
		}
		return s.record, false, nil
	}
	s.record = pipeline.RunRecord{ID: 1, Request: r, ActorID: actor, Policy: p, State: "running", CreatedAt: time.Now().UTC()}
	return s.record, true, nil
}
func (s *l0StoreFake) RunByRequest(_ context.Context, id string) (pipeline.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.ID == 0 || s.record.Request.RequestID != id {
		return pipeline.RunRecord{}, pipeline.ErrRunNotFound
	}
	return s.record, nil
}
func (s *l0StoreFake) FinishRun(ctx context.Context, id int64, result pipeline.ExtractionResult, failure string) (pipeline.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finishFailure {
		return pipeline.RunRecord{}, pipeline.ErrAuditUnavailable
	}
	if ctx.Err() != nil {
		return pipeline.RunRecord{}, ctx.Err()
	}
	s.finishes++
	s.record.State = "succeeded"
	s.record.Result = result
	if failure != "" {
		s.record.State = "failed"
		s.record.ErrorCode = failure
	}
	now := time.Now().UTC()
	s.record.FinishedAt = &now
	return s.record, nil
}
func (s *l0StoreFake) AppendCallEvent(context.Context, pipeline.CallEvent) error { return nil }

type l0RunnerFake struct {
	calls   atomic.Int64
	failure error
	entered chan struct{}
	release chan struct{}
	cancel  context.CancelFunc
}

func (r *l0RunnerFake) Run(ctx context.Context, snapshot sources.Snapshot) (pipeline.ExtractionResult, error) {
	r.calls.Add(1)
	if pipeline.RunID(ctx) <= 0 {
		return pipeline.ExtractionResult{}, pipeline.ErrInvalidRun
	}
	if r.entered != nil {
		close(r.entered)
		select {
		case <-r.release:
		case <-ctx.Done():
			return pipeline.ExtractionResult{}, ctx.Err()
		}
	}
	if r.cancel != nil {
		r.cancel()
	}
	return pipeline.ExtractionResult{Attempts: 1, Model: "test-model"}, r.failure
}

func l0Fixture(t *testing.T) (*AIL0Service, *l0StoreFake, *l0RunnerFake, pipeline.RunRequest) {
	t.Helper()
	store := &l0StoreFake{}
	runner := &l0RunnerFake{}
	reader := &fakeExtractionSnapshotReader{snapshot: sources.Snapshot{ID: 42}}
	svc, err := NewAIL0Service(store, reader, runner, pipeline.RunPolicy{Provider: "test-provider", Model: "test-model", ApprovalRef: "test-written-approval"})
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, runner, pipeline.RunRequest{SnapshotID: 42, RequestID: "test_l0_request_00001"}
}

func TestL0RunReplayNeverCallsProviderAgain(t *testing.T) {
	svc, store, runner, request := l0Fixture(t)
	first, err := svc.Extract(context.Background(), 7, request)
	if err != nil || first.State != "succeeded" {
		t.Fatalf("抽取失败: %+v %v", first, err)
	}
	second, err := svc.Extract(context.Background(), 7, request)
	if err != nil || second.ID != first.ID || runner.calls.Load() != 1 || store.finishes != 1 {
		t.Fatalf("重放发生二次调用: %v", err)
	}
	if _, err := svc.Extract(context.Background(), 8, request); !errors.Is(err, pipeline.ErrRunConflict) {
		t.Fatal("不同身份复用请求未拒绝")
	}
	changed := request
	changed.SnapshotID++
	if _, err := svc.Extract(context.Background(), 7, changed); !errors.Is(err, pipeline.ErrRunConflict) {
		t.Fatal("不同快照复用请求未拒绝")
	}
	if _, err := svc.RunByRequest(context.Background(), 8, request.RequestID); !errors.Is(err, pipeline.ErrRunConflict) {
		t.Fatal("查询越权未拒绝")
	}
}

func TestL0UncertainCommitDoesNotRetryProvider(t *testing.T) {
	svc, store, runner, request := l0Fixture(t)
	store.finishFailure = true
	if _, err := svc.Extract(context.Background(), 7, request); !errors.Is(err, pipeline.ErrRunBusy) {
		t.Fatal(err)
	}
	store.finishFailure = false
	replay, err := svc.Extract(context.Background(), 7, request)
	if err != nil || replay.State != "running" || runner.calls.Load() != 1 {
		t.Fatalf("不确定结果被重跑: %+v %v", replay, err)
	}
}

func TestL0SourceFailureAndAuditFailureAreTerminal(t *testing.T) {
	for _, sourceFailure := range []bool{false, true} {
		svc, store, runner, request := l0Fixture(t)
		if sourceFailure {
			svc.snapshots = &fakeExtractionSnapshotReader{err: sources.ErrUnauthorized}
		} else {
			runner.failure = pipeline.ErrAuditUnavailable
		}
		record, err := svc.Extract(context.Background(), 7, request)
		if err != nil || record.State != "failed" || store.finishes != 1 {
			t.Fatalf("失败没有终态: %+v %v", record, err)
		}
		if sourceFailure && runner.calls.Load() != 0 {
			t.Fatal("授权失败仍调用模型")
		}
		if !sourceFailure && record.ErrorCode != "audit_unavailable" {
			t.Fatal("审计失败分类丢失")
		}
		if _, err := svc.Extract(context.Background(), 7, request); err != nil {
			t.Fatal(err)
		}
		want := int64(1)
		if sourceFailure {
			want = 0
		}
		if runner.calls.Load() != want {
			t.Fatal("失败请求被重跑")
		}
	}
}

func TestL0CancelledCallerStillFinalizesBoundedRun(t *testing.T) {
	svc, store, runner, request := l0Fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.cancel = cancel
	if _, err := svc.Extract(ctx, 7, request); err != nil || store.finishes != 1 {
		t.Fatalf("取消后丢失终态: %v", err)
	}
}

func TestL0SingleFlightAndDisabledGate(t *testing.T) {
	svc, store, runner, request := l0Fixture(t)
	runner.entered = make(chan struct{})
	runner.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { _, err := svc.Extract(context.Background(), 7, request); done <- err }()
	<-runner.entered
	active, err := svc.Extract(context.Background(), 7, request)
	if err != nil || active.State != "running" {
		t.Fatalf("在途重放未返回状态: %v", err)
	}
	other := request
	other.RequestID = "test_l0_request_00002"
	if _, err := svc.Extract(context.Background(), 7, other); !errors.Is(err, pipeline.ErrRunBusy) {
		t.Fatalf("并发新请求未限流: %v", err)
	}
	close(runner.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	svc.runner = nil
	if _, err := svc.Extract(context.Background(), 7, request); !errors.Is(err, pipeline.ErrDisabled) {
		t.Fatal("关闭后仍启动抽取")
	}
	if record, err := svc.RunByRequest(context.Background(), 7, request.RequestID); err != nil || record.ID != store.record.ID {
		t.Fatal("关闭后无法查历史")
	}
	if _, err := svc.Extract(context.Background(), 0, request); !errors.Is(err, pipeline.ErrInvalidRun) {
		t.Fatal(err)
	}
	if _, err := svc.RunByRequest(context.Background(), 7, "bad"); !errors.Is(err, pipeline.ErrInvalidRun) {
		t.Fatal(err)
	}
}
