package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type stageScript struct {
	*budgetScript
	results               map[int]task.StageResult
	steps                 map[int]task.StageCheckpoint
	stageEvents           map[int64]task.Event
	inTx, skipProgress    bool
	txCount, failCommitAt int
}

// 通用任务事务也必须暴露本夹具的真实预算/响应，不能降级成“无执行记录”的基础夹具。
func (s *stageScript) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	return s.scriptedTaskStore.WithinTaskTx(ctx, func(task.Tx) error { return fn(s) })
}

// 旧内部阶段夹具无会员准入，实际绑定与缺记录由独立用例/真库验证。
func (s *stageScript) WithinAdmissionTx(ctx context.Context, fn func(task.AdmissionReadTx) error) error {
	return s.WithinStageTx(ctx, func(task.StageTx) error { return fn(s) })
}
func (s *stageScript) ReadAdmission(ctx context.Context, _ task.Task) (task.AdmissionRecord, bool, error) {
	return task.AdmissionRecord{}, false, ctx.Err()
}

func (s *stageScript) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	s.inTx = true
	defer func() { s.inTx = false }()
	s.txCount++
	err := s.WithinTaskTx(ctx, func(task.Tx) error { return fn(s) })
	if err != nil {
		return err
	}
	if s.txCount == s.failCommitAt {
		return errPointsInjected
	}
	return nil
}
func (s *stageScript) InsertStageResult(ctx context.Context, v task.StageResult) (bool, error) {
	if err := s.p.step(ctx); err != nil {
		return false, err
	}
	if _, ok := s.results[v.ReservationSeq]; ok {
		return false, nil
	}
	v.Payload = append([]byte(nil), v.Payload...)
	s.results[v.ReservationSeq] = v
	return true, nil
}
func (s *stageScript) StageResult(ctx context.Context, _, _ int64, seq int) (task.StageResult, error) {
	if err := s.p.step(ctx); err != nil {
		return task.StageResult{}, err
	}
	v, ok := s.results[seq]
	if !ok {
		return task.StageResult{}, points.ErrNotFound
	}
	v.Payload = append([]byte(nil), v.Payload...)
	return v, nil
}
func (s *stageScript) InsertStageCheckpoint(ctx context.Context, c task.StageCheckpoint) error {
	if err := s.p.step(ctx); err != nil {
		return err
	}
	s.steps[c.StepNo] = c
	if !s.skipProgress {
		s.v.CurrentStep = c.StepNo
		s.v.EventSeq = c.EventSeq
		s.stageEvents[c.EventSeq] = task.Event{TaskID: c.TaskID, Seq: c.EventSeq, Type: "step_completed", Status: task.Running, Step: c.StepNo}
	}
	return nil
}
func (s *stageScript) StageCheckpoint(ctx context.Context, _, _ int64, step int) (task.StageCheckpoint, error) {
	if err := s.p.step(ctx); err != nil {
		return task.StageCheckpoint{}, err
	}
	v, ok := s.steps[step]
	if !ok {
		return task.StageCheckpoint{}, points.ErrNotFound
	}
	return v, nil
}
func (s *stageScript) StageEvent(ctx context.Context, _, seq int64) (task.Event, error) {
	if err := s.p.step(ctx); err != nil {
		return task.Event{}, err
	}
	v, ok := s.stageEvents[seq]
	if !ok {
		return task.Event{}, points.ErrNotFound
	}
	return v, nil
}

func stageFixture(t *testing.T) (*AITaskStageService, *stageScript, task.StageRequest, map[string]task.StagePolicy) {
	t.Helper()
	budget, b, r := budgetFixture(t)
	if _, err := budget.Reserve(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	st := &stageScript{budgetScript: b, results: map[int]task.StageResult{}, steps: map[int]task.StageCheckpoint{}, stageEvents: map[int64]task.Event{}}
	policies := map[string]task.StagePolicy{"extract": {Digest: strings.Repeat("c", 64), Check: func(_ context.Context, in task.StageValidationInput) error {
		if st.inTx {
			t.Fatal("校验持有事务")
		}
		if in.Stage != "extract" || in.InputDigest != r.InputDigest {
			return errors.New("测试输入不匹配")
		}
		return nil
	}}}
	s, err := NewAITaskStageService(st, policies)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, task.StageRequest{UserID: r.UserID, TaskID: r.TaskID, Fence: r.Fence, Owner: r.Owner, OperationKey: r.Key, Stage: "extract", InputDigest: r.InputDigest, StepNo: 1}, policies
}
func captureStage(t *testing.T, s *AITaskStageService, r task.StageRequest) {
	t.Helper()
	if _, err := s.Capture(context.Background(), r, []byte(`{"test":"隔离测试数据"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestAITaskStageCaptureCompleteAndReuse(t *testing.T) {
	s, st, r, p := stageFixture(t)
	first, err := s.Capture(context.Background(), r, []byte(`{"test":"隔离测试数据"}`))
	if err != nil || !first.NewlyCaptured {
		t.Fatal(first, err)
	}
	if got, err := s.Capture(context.Background(), r, []byte(`{"test":"隔离测试数据"}`)); err != nil || got.NewlyCaptured || got.OutputDigest != first.OutputDigest {
		t.Fatal(got, err)
	}
	if _, err := s.Capture(context.Background(), r, []byte(`{"test":"修改正文"}`)); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
	if st.v.CurrentStep != 0 || len(st.stageEvents) != 0 {
		t.Fatal("捕获即推进")
	}
	if _, err := s.Load(context.Background(), r); !errors.Is(err, task.ErrNotReady) {
		t.Fatal(err)
	}
	p["extract"] = task.StagePolicy{}
	done, err := s.Complete(context.Background(), r)
	if err != nil || !done.NewlyCompleted || st.v.CurrentStep != 1 || len(st.stageEvents) != 1 {
		t.Fatal(done, err)
	}
	if got, err := s.Complete(context.Background(), r); err != nil || got.NewlyCompleted || got.Checkpoint != done.Checkpoint {
		t.Fatal(got, err)
	}
	loaded, err := s.Load(context.Background(), r)
	if err != nil || loaded.Checkpoint != done.Checkpoint || string(loaded.Result.Payload) != `{"test":"隔离测试数据"}` {
		t.Fatal(loaded.Checkpoint, err)
	}
	loaded.Result.Payload[0] = '!'
	if _, err := s.Load(context.Background(), r); err != nil {
		t.Fatal("读取别名污染存储", err)
	}
	if st.state.Count != 1 || st.p.account.Frozen != 7 {
		t.Fatal("复用重复消耗预算/积分")
	}
	other, err := NewAITaskStageService(st, map[string]task.StagePolicy{"extract": {Digest: strings.Repeat("d", 64), Check: func(context.Context, task.StageValidationInput) error { return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Load(context.Background(), r); !errors.Is(err, task.ErrStagePolicy) {
		t.Fatal(err)
	}
}

func TestAITaskStageTakeoverAndValidationBoundaries(t *testing.T) {
	s, st, r, p := stageFixture(t)
	if _, err := s.Complete(context.Background(), r); !errors.Is(err, points.ErrNotFound) {
		t.Fatal("未知响应被完成", err)
	}
	captureStage(t, s, r)
	old := r
	st.v.Fence++
	st.v.LeaseOwner = "worker_stage_00002"
	r.Fence = st.v.Fence
	r.Owner = st.v.LeaseOwner
	if _, err := s.Capture(context.Background(), r, []byte(`{}`)); !errors.Is(err, task.ErrLease) {
		t.Fatal("接管补造旧响应", err)
	}
	if _, err := s.Complete(context.Background(), old); !errors.Is(err, task.ErrLease) {
		t.Fatal(err)
	}
	if got, err := s.Complete(context.Background(), r); err != nil || !got.NewlyCompleted || got.Checkpoint.Fence != 2 {
		t.Fatal(got, err)
	}
	s, st, r, p = stageFixture(t)
	captureStage(t, s, r)
	p["extract"] = task.StagePolicy{Digest: strings.Repeat("c", 64), Check: func(context.Context, task.StageValidationInput) error {
		return errors.New("不返回原始校验细节")
	}}
	s, _ = NewAITaskStageService(st, p)
	if got, err := s.Complete(context.Background(), r); err != task.ErrStageRejected || got != (task.CompletedStage{}) || st.v.CurrentStep != 0 {
		t.Fatal(got, err)
	}
	for _, kind := range []string{"cancel", "fence", "context"} {
		s, st, r, p := stageFixture(t)
		captureStage(t, s, r)
		ctx, cancel := context.WithCancel(context.Background())
		p["extract"] = task.StagePolicy{Digest: strings.Repeat("c", 64), Check: func(context.Context, task.StageValidationInput) error {
			switch kind {
			case "cancel":
				st.v.CancelRequested = true
			case "fence":
				st.v.Fence++
			case "context":
				cancel()
			}
			return nil
		}}
		s, _ = NewAITaskStageService(st, p)
		if got, err := s.Complete(ctx, r); err == nil || got != (task.CompletedStage{}) || st.v.CurrentStep != 0 {
			t.Fatal(kind, got, err)
		}
		cancel()
	}
}

func TestAITaskStageStorageFailuresAndCorruption(t *testing.T) {
	for _, mode := range []string{"capture", "complete", "load"} {
		prepare := func() (*AITaskStageService, *stageScript, task.StageRequest) {
			s, st, r, _ := stageFixture(t)
			if mode != "capture" {
				captureStage(t, s, r)
			}
			if mode == "load" {
				if _, err := s.Complete(context.Background(), r); err != nil {
					t.Fatal(err)
				}
			}
			st.p.calls = 0
			return s, st, r
		}
		run := func(s *AITaskStageService, r task.StageRequest) error {
			switch mode {
			case "capture":
				out, err := s.Capture(context.Background(), r, []byte(`{}`))
				if err != nil && out != (task.CaptureReceipt{}) {
					t.Fatal(out)
				}
				return err
			case "complete":
				out, err := s.Complete(context.Background(), r)
				if err != nil && out != (task.CompletedStage{}) {
					t.Fatal(out)
				}
				return err
			default:
				out, err := s.Load(context.Background(), r)
				if err != nil && !reflect.DeepEqual(out, task.LoadedStage{}) {
					t.Fatal(out)
				}
				return err
			}
		}
		s, st, r := prepare()
		if err := run(s, r); err != nil {
			t.Fatal(err)
		}
		calls := st.p.calls
		for i := 1; i <= calls; i++ {
			s, st, r := prepare()
			st.p.failAt = i
			if err := run(s, r); err == nil {
				t.Fatal(mode, i)
			}
		}
	}
	s, st, r, _ := stageFixture(t)
	st.commitError = errPointsInjected
	if out, err := s.Capture(context.Background(), r, []byte(`{}`)); err == nil || out != (task.CaptureReceipt{}) {
		t.Fatal(out, err)
	}
	st.commitError = nil
	if out, err := s.Capture(context.Background(), r, []byte(`{}`)); err != nil || out.NewlyCaptured {
		t.Fatal(out, err)
	}
	st.txCount = 0
	st.failCommitAt = 2
	if out, err := s.Complete(context.Background(), r); err == nil || out != (task.CompletedStage{}) {
		t.Fatal(out, err)
	}
	st.failCommitAt = 0
	if out, err := s.Complete(context.Background(), r); err != nil || out.NewlyCompleted {
		t.Fatal(out, err)
	}
	delete(st.stageEvents, st.v.EventSeq)
	if _, err := s.Load(context.Background(), r); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	s, st, r, _ = stageFixture(t)
	captureStage(t, s, r)
	st.skipProgress = true
	if _, err := s.Complete(context.Background(), r); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	s, st, r, _ = stageFixture(t)
	captureStage(t, s, r)
	v := st.results[1]
	v.Payload = []byte(`{"tampered":true}`)
	st.results[1] = v
	if _, err := s.Complete(context.Background(), r); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
}

func TestAITaskStageInvalidSetupAndRequests(t *testing.T) {
	s, st, r, p := stageFixture(t)
	for _, policies := range []map[string]task.StagePolicy{nil, {}, {"unknown": p["extract"]}, {"extract": {}}, {"extract": {Digest: strings.Repeat("c", 64)}}} {
		if _, err := NewAITaskStageService(st, policies); !errors.Is(err, points.ErrInvalid) {
			t.Fatal("无效策略被注册", err)
		}
	}
	if _, err := NewAITaskStageService(nil, p); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	for _, service := range []*AITaskStageService{nil, s} {
		for _, stage := range []string{"extract", "collect"} {
			bad := r
			bad.Stage = stage
			if service != nil && stage == "extract" {
				bad.StepNo = 0
			}
			if _, err := service.Capture(context.Background(), bad, []byte(`{}`)); err == nil {
				t.Fatal("捕获未拒绝无效请求")
			}
			if _, err := service.Complete(context.Background(), bad); err == nil {
				t.Fatal("完成未拒绝无效请求")
			}
			if _, err := service.Load(context.Background(), bad); err == nil {
				t.Fatal("复用未拒绝无效请求")
			}
		}
	}
	if _, err := s.Capture(context.Background(), r, []byte(`[]`)); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestAITaskStageTaskAndReservationGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*stageScript, *task.StageRequest)
		want   error
	}{
		{"跨会员", func(_ *stageScript, r *task.StageRequest) { r.UserID++ }, points.ErrNotFound},
		{"损坏进度", func(s *stageScript, _ *task.StageRequest) { s.v.CurrentStep = -1 }, points.ErrInvariant},
		{"损坏事件序号", func(s *stageScript, _ *task.StageRequest) { s.v.EventSeq = 0 }, points.ErrInvariant},
		{"截止", func(s *stageScript, _ *task.StageRequest) { s.v.Deadline = s.p.now }, task.ErrLease},
		{"取消", func(s *stageScript, _ *task.StageRequest) { s.v.CancelRequested = true }, task.ErrCancelled},
		{"租约过期", func(s *stageScript, _ *task.StageRequest) { s.v.LeaseUntil = &s.p.now }, task.ErrLease},
		{"跳步骤", func(_ *stageScript, r *task.StageRequest) { r.StepNo = 2 }, task.ErrNotReady},
		{"无预留", func(_ *stageScript, r *task.StageRequest) { r.OperationKey = "operation_not_found" }, points.ErrNotFound},
		{"输入改变", func(_ *stageScript, r *task.StageRequest) { r.InputDigest = strings.Repeat("b", 64) }, points.ErrIdempotency},
		{"预算损坏", func(s *stageScript, _ *task.StageRequest) { s.state.Count++ }, points.ErrInvariant},
		{"资源类型错误", func(s *stageScript, _ *task.StageRequest) {
			s.rows[0].Units = task.Units{Pages: 1}
			s.state.Reserved = s.rows[0].Units
		}, points.ErrIdempotency},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, r, _ := stageFixture(t)
			captureStage(t, s, r)
			tc.change(st, &r)
			if out, err := s.Capture(context.Background(), r, []byte(`{}`)); !errors.Is(err, tc.want) || out != (task.CaptureReceipt{}) {
				t.Fatal(out, err)
			}
			if out, err := s.Complete(context.Background(), r); !errors.Is(err, tc.want) || out != (task.CompletedStage{}) {
				t.Fatal(out, err)
			}
		})
	}
	s, st, r, _ := stageFixture(t)
	captureStage(t, s, r)
	st.v.EventSeq = math.MaxInt64
	if _, err := s.Complete(context.Background(), r); !errors.Is(err, points.ErrOverflow) {
		t.Fatal(err)
	}
}

func TestAITaskStageCheckpointIntegrityAndCancelledValidator(t *testing.T) {
	for _, change := range []func(*stageScript){
		func(s *stageScript) { delete(s.steps, 1) },
		func(s *stageScript) { c := s.steps[1]; c.Fence++; s.steps[1] = c },
		func(s *stageScript) { c := s.steps[1]; c.OutputDigest = strings.Repeat("d", 64); s.steps[1] = c },
		func(s *stageScript) { e := s.stageEvents[s.v.EventSeq]; e.Step++; s.stageEvents[s.v.EventSeq] = e },
	} {
		s, st, r, _ := stageFixture(t)
		captureStage(t, s, r)
		if _, err := s.Complete(context.Background(), r); err != nil {
			t.Fatal(err)
		}
		change(st)
		if _, err := s.Load(context.Background(), r); !errors.Is(err, points.ErrInvariant) {
			t.Fatal(err)
		}
	}
	s, st, r, p := stageFixture(t)
	captureStage(t, s, r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p["extract"] = task.StagePolicy{Digest: strings.Repeat("c", 64), Check: func(context.Context, task.StageValidationInput) error {
		cancel()
		return errors.New("不返回测试正文")
	}}
	s, _ = NewAITaskStageService(st, p)
	if out, err := s.Complete(ctx, r); !errors.Is(err, context.Canceled) || out != (task.CompletedStage{}) || st.v.CurrentStep != 0 {
		t.Fatal(out, err)
	}
}
