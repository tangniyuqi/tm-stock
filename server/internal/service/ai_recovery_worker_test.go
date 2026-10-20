package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

type recoveryPlannerFunc func(context.Context, task.Task) (task.RecoveryPlan, error)

func (f recoveryPlannerFunc) Plan(ctx context.Context, v task.Task) (task.RecoveryPlan, error) {
	return f(ctx, v)
}

type recoveryControl struct {
	st                                                            *stageScript
	mu                                                            sync.Mutex
	claimed                                                       task.Task
	beats, claims, finishes                                       int
	inspectError, claimError, expireError, beatError, finishError error
	beatFailAt                                                    int
	beatHook                                                      func(context.Context) error
	expiredAfterClaim                                             bool
}

func (f *recoveryControl) Inspect(ctx context.Context, _, _ int64) (task.Task, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	return f.st.v, f.inspectError
}
func (f *recoveryControl) Expire(context.Context, int64, int64) (task.Task, error) {
	if f.expireError != nil {
		return task.Task{}, f.expireError
	}
	if f.expiredAfterClaim && f.claims > 0 {
		v := f.st.v
		v.Status = task.Expired
		return v, nil
	}
	return task.Task{}, task.ErrNotReady
}
func (f *recoveryControl) Claim(_ context.Context, _, _ int64, owner string, _ time.Duration) (task.Task, error) {
	f.claims++
	if f.claimError != nil {
		return task.Task{}, f.claimError
	}
	f.st.v.Fence++
	f.st.v.LeaseOwner = owner
	if f.st.v.CancelRequested {
		f.st.v.Status = task.Cancelled
	}
	if f.st.v.Attempt >= f.st.v.Budget.MaxAttempts {
		f.st.v.Status = task.Failed
	}
	f.claimed = f.st.v
	return f.claimed, nil
}
func (f *recoveryControl) Heartbeat(ctx context.Context, _, _ int64, _ string, _ int64, _ time.Duration) (task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beats++
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	if f.beatError != nil && f.beats >= f.beatFailAt {
		return task.Task{}, f.beatError
	}
	if f.beatHook != nil {
		if err := f.beatHook(ctx); err != nil {
			return task.Task{}, err
		}
	}
	return f.claimed, nil
}
func (f *recoveryControl) FailOrRetry(_ context.Context, _, _ int64, _ string, _ int64, code string, _ time.Duration) (task.Task, error) {
	f.finishes++
	if f.finishError != nil {
		return task.Task{}, f.finishError
	}
	v := f.st.v
	v.Status = task.Failed
	v.ErrorCode = code
	if errors.Is(f.beatError, task.ErrCancelled) || v.CancelRequested {
		v.Status = task.Cancelled
	}
	return v, nil
}
func recoveryFixture(t *testing.T, capture bool) (*AIRecoveryWorker, *recoveryControl, *stageScript, task.RecoveryPlan) {
	t.Helper()
	s, st, r, p := stageFixture(t)
	if capture {
		captureStage(t, s, r)
	}
	plan := task.RecoveryPlan{TaskID: r.TaskID, UserID: r.UserID, TaskNo: st.v.No, Steps: []task.RecoveryStep{{OperationKey: r.OperationKey, Stage: r.Stage, InputDigest: r.InputDigest, Policy: p["extract"], Recheck: func(ctx context.Context) error { return ctx.Err() }}}}
	f := &recoveryControl{st: st}
	w, err := NewAIRecoveryWorker(f, st, recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return plan, nil }), "worker_recovery_0001", task.RecoveryOptions{Lease: time.Second, Heartbeat: 50 * time.Millisecond, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return w, f, st, plan
}

func TestAIRecoveryWorkerResumeUnknownAndReplay(t *testing.T) {
	w, f, st, _ := recoveryFixture(t, true)
	out, err := w.Recover(context.Background(), 1, 40)
	if err != nil || out.Outcome != "report_pending" || out.CurrentStep != 1 || out.ReportAvailable || f.finishes != 0 || len(st.rows) != 1 || st.p.account.Frozen != 7 {
		t.Fatal(out, err)
	}
	out, err = w.Recover(context.Background(), 1, 40)
	if err != nil || out.Outcome != "report_pending" || len(st.steps) != 1 || len(st.stageEvents) != 1 {
		t.Fatal(out, err)
	}
	w, f, st, _ = recoveryFixture(t, false)
	out, err = w.Recover(context.Background(), 1, 40)
	if err != nil || out.Outcome != "response_unknown" || out.CurrentStep != 0 || f.finishes != 0 || len(st.results) != 0 || len(st.rows) != 1 {
		t.Fatal(out, err)
	}
	st.v.Status = task.Cancelled
	out, err = w.Recover(context.Background(), 1, 40)
	if err != nil || out.Outcome != "terminal" {
		t.Fatal(out, err)
	}
}

func TestAIRecoveryWorkerHeartbeatCancelsAndJoins(t *testing.T) {
	for _, failure := range []error{task.ErrCancelled, task.ErrLease, errPointsInjected} {
		w, f, st, plan := recoveryFixture(t, true)
		f.beatError = failure
		f.beatFailAt = 2
		plan.Steps[0].Policy.Check = func(ctx context.Context, _ task.StageValidationInput) error { <-ctx.Done(); return ctx.Err() }
		w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return plan, nil })
		out, err := w.Recover(context.Background(), 1, 40)
		if errors.Is(failure, task.ErrCancelled) {
			if err != nil || out.TaskStatus != task.Cancelled || f.finishes != 1 {
				t.Fatal(out, err)
			}
		} else if err == nil || out != (task.RecoveryResult{}) || f.finishes != 0 {
			t.Fatal(out, err)
		}
		if st.v.CurrentStep != 0 {
			t.Fatal("心跳失败仍推进")
		}
		f.mu.Lock()
		beats := f.beats
		f.mu.Unlock()
		time.Sleep(70 * time.Millisecond)
		f.mu.Lock()
		after := f.beats
		f.mu.Unlock()
		if after != beats {
			t.Fatal("返回后心跳未退出")
		}
	}
}

func TestAIRecoveryWorkerHeartbeatDeadline(t *testing.T) {
	w, f, st, _ := recoveryFixture(t, true)
	w.options.Timeout = 3 * time.Second
	f.beatHook = func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining <= 2*w.options.Heartbeat || remaining > w.options.Lease/3 {
			return errPointsInjected
		}
		// 模拟慢于发起间隔、仍在租约安全窗口内的 SQL；旧的间隔超时会失败。
		timer := time.NewTimer(2 * w.options.Heartbeat)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if out, err := w.Recover(context.Background(), 1, 40); err != nil || out.Outcome != "report_pending" || st.v.CurrentStep != 1 || f.finishes != 0 {
		t.Fatal(out, err)
	}
	w, f, st, _ = recoveryFixture(t, true)
	f.beatHook = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	if out, err := w.Recover(context.Background(), 1, 40); !errors.Is(err, context.DeadlineExceeded) || out != (task.RecoveryResult{}) || st.v.CurrentStep != 0 || f.finishes != 0 || st.p.account.Frozen != 7 {
		t.Fatal("心跳超时未失败关闭", out, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := w.beat(ctx, st.v); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("未遵守父上下文期限", err)
	}
}

func TestAIRecoveryWorkerCleanupAndRecheckBoundaries(t *testing.T) {
	for _, cancelRequested := range []bool{true, false} {
		w, f, st, _ := recoveryFixture(t, true)
		st.v.CancelRequested = cancelRequested
		if !cancelRequested {
			st.v.Attempt = st.v.Budget.MaxAttempts
		}
		w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) {
			t.Fatal("已取消/耗尽任务仍要求来源计划")
			return task.RecoveryPlan{}, errPointsInjected
		})
		if out, err := w.Recover(context.Background(), 1, 40); err != nil || out.Outcome != "terminal" || f.claims != 1 {
			t.Fatal(out, err)
		}
	}
	w, f, _, _ := recoveryFixture(t, true)
	f.beatError, f.beatFailAt, f.expiredAfterClaim = task.ErrLease, 1, true
	if out, err := w.Recover(context.Background(), 1, 40); err != nil || out.TaskStatus != task.Expired || f.finishes != 0 {
		t.Fatal(out, err)
	}
	w, f, st, plan := recoveryFixture(t, true)
	checks := 0
	plan.Steps[0].Recheck = func(context.Context) error {
		checks++
		if checks == 2 {
			return task.ErrStagePolicy
		}
		return nil
	}
	w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return plan, nil })
	if out, err := w.Recover(context.Background(), 1, 40); !errors.Is(err, task.ErrStagePolicy) || out != (task.RecoveryResult{}) || st.v.CurrentStep != 1 || f.finishes != 0 || st.p.account.Frozen != 7 {
		t.Fatal("提交后来源变化被当成功/退款", out, err)
	}
}

func TestAIRecoveryWorkerFailureBoundaries(t *testing.T) {
	for _, setup := range []func(*AIRecoveryWorker, *recoveryControl, *stageScript){
		func(_ *AIRecoveryWorker, f *recoveryControl, _ *stageScript) { f.inspectError = errPointsInjected },
		func(_ *AIRecoveryWorker, f *recoveryControl, _ *stageScript) { f.expireError = errPointsInjected },
		func(_ *AIRecoveryWorker, f *recoveryControl, _ *stageScript) { f.claimError = errPointsInjected },
		func(w *AIRecoveryWorker, _ *recoveryControl, _ *stageScript) {
			w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) {
				return task.RecoveryPlan{}, errPointsInjected
			})
		},
		func(w *AIRecoveryWorker, _ *recoveryControl, _ *stageScript) {
			w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return task.RecoveryPlan{}, nil })
		},
		func(_ *AIRecoveryWorker, f *recoveryControl, _ *stageScript) {
			f.beatError = errPointsInjected
			f.beatFailAt = 1
		},
		func(_ *AIRecoveryWorker, _ *recoveryControl, st *stageScript) { st.commitError = errPointsInjected },
	} {
		w, f, st, _ := recoveryFixture(t, true)
		setup(w, f, st)
		if out, err := w.Recover(context.Background(), 1, 40); err == nil || out != (task.RecoveryResult{}) || f.finishes != 0 {
			t.Fatal(out, err)
		}
	}
	w, f, _, plan := recoveryFixture(t, true)
	plan.Steps[0].Policy.Check = func(context.Context, task.StageValidationInput) error { return task.ErrStageRejected }
	w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return plan, nil })
	if out, err := w.Recover(context.Background(), 1, 40); err != nil || out.TaskStatus != task.Failed || f.finishes != 1 {
		t.Fatal(out, err)
	}
	f.finishError = errPointsInjected
	if out, err := w.Recover(context.Background(), 1, 40); err == nil || out != (task.RecoveryResult{}) {
		t.Fatal(out, err)
	}
	w, f, _, plan = recoveryFixture(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	plan.Steps[0].Recheck = func(context.Context) error { cancel(); return context.Canceled }
	w.planner = recoveryPlannerFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) { return plan, nil })
	if out, err := w.Recover(ctx, 1, 40); !errors.Is(err, context.Canceled) || out != (task.RecoveryResult{}) || f.finishes != 0 {
		t.Fatal(out, err)
	}
	var absent *AIRecoveryWorker
	if _, err := absent.Recover(context.Background(), 1, 40); err == nil {
		t.Fatal("空恢复服务")
	}
	if _, err := w.Recover(context.Background(), 0, 40); err == nil {
		t.Fatal("无效用户")
	}
	if _, err := NewAIRecoveryWorker(nil, nil, nil, strings.Repeat("x", 16), task.RecoveryOptions{}); err == nil {
		t.Fatal("无效装配")
	}
}
