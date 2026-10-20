package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type recoveryTasks interface {
	Inspect(context.Context, int64, int64) (task.Task, error)
	Claim(context.Context, int64, int64, string, time.Duration) (task.Task, error)
	Heartbeat(context.Context, int64, int64, string, int64, time.Duration) (task.Task, error)
	FailOrRetry(context.Context, int64, int64, string, int64, string, time.Duration) (task.Task, error)
	Expire(context.Context, int64, int64) (task.Task, error)
}
type RecoveryPlanner interface {
	Plan(context.Context, task.Task) (task.RecoveryPlan, error)
}

type AIRecoveryWorker struct {
	tasks   recoveryTasks
	store   task.StageStore
	planner RecoveryPlanner
	options task.RecoveryOptions
	owner   string
}

func NewAIRecoveryWorker(tasks recoveryTasks, store task.StageStore, planner RecoveryPlanner, owner string, options task.RecoveryOptions) (*AIRecoveryWorker, error) {
	if tasks == nil || store == nil || planner == nil || !task.ValidOwner(owner) || options.Validate() != nil {
		return nil, points.ErrInvalid
	}
	return &AIRecoveryWorker{tasks: tasks, store: store, planner: planner, options: options, owner: owner}, nil
}

func recoveryResult(v task.Task, count int, outcome string) task.RecoveryResult {
	return task.RecoveryResult{TaskID: v.ID, TaskStatus: v.Status, CurrentStep: v.CurrentStep, PlannedSteps: count, Outcome: outcome}
}

// unconfirmedRecovery 只重读原任务，不清预算、不取得新围栏，也不把终态当报告许可。
func unconfirmedRecovery(ctx context.Context, tasks recoveryTasks, uid, id int64) (task.RecoveryResult, error) {
	v, err := tasks.Inspect(ctx, uid, id)
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if v.ID != id || v.UserID != uid || v.CurrentStep < 0 || v.CurrentStep > task.MaxSteps {
		return task.RecoveryResult{}, points.ErrInvariant
	}
	if v.Terminal() {
		return recoveryResult(v, 0, "terminal"), nil
	}
	if v.Status != task.Running {
		return task.RecoveryResult{}, task.ErrUnconfirmed
	}
	return recoveryResult(v, 0, "response_unknown"), nil
}

// Recover 不执行外部调用、不新建预留；未知响应与存储/上下文错误不会被当作可退款的业务失败。
func (w *AIRecoveryWorker) Recover(parent context.Context, uid, id int64) (task.RecoveryResult, error) {
	if w == nil || uid <= 0 || id <= 0 {
		return task.RecoveryResult{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, w.options.Timeout)
	defer cancel()
	v, err := w.tasks.Inspect(ctx, uid, id)
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if v.Terminal() {
		return recoveryResult(v, 0, "terminal"), nil
	}
	// 过期/取消清理不依赖来源计划；尝试耗尽仍由 Claim 核对有无未确认调用。
	if expired, err := w.tasks.Expire(ctx, uid, id); err == nil {
		return recoveryResult(expired, 0, "terminal"), nil
	} else if !errors.Is(err, task.ErrNotReady) {
		return task.RecoveryResult{}, err
	}
	var plan task.RecoveryPlan
	if !v.CancelRequested && v.Attempt < v.Budget.MaxAttempts {
		plan, err = w.planner.Plan(ctx, v)
		if err != nil {
			return task.RecoveryResult{}, err
		}
		if err := plan.Validate(v); err != nil {
			return task.RecoveryResult{}, err
		}
		plan.Steps = append([]task.RecoveryStep(nil), plan.Steps...)
	}
	v, err = w.tasks.Claim(ctx, uid, id, w.owner, w.options.Lease)
	if err != nil {
		if errors.Is(err, task.ErrUnconfirmed) {
			return unconfirmedRecovery(ctx, w.tasks, uid, id)
		}
		return task.RecoveryResult{}, err
	}
	if v.Terminal() {
		return recoveryResult(v, len(plan.Steps), "terminal"), nil
	}
	if err := plan.Validate(v); err != nil {
		return task.RecoveryResult{}, err
	}
	out, err := w.withHeartbeat(ctx, v, plan)
	if err == nil {
		return out, nil
	}
	// 只有已确认的用户取消/确定性拒绝才终结；进程退出或提交未知保留现场。
	cleanup, stop := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer stop()
	code := ""
	if errors.Is(err, task.ErrCancelled) {
		code = "internal_failure" // FailOrRetry 重新锁当前任务并优先处理不可逆取消请求。
	} else if errors.Is(err, task.ErrStageRejected) {
		code = "guard_rejected"
	}
	if code != "" {
		finished, finishErr := w.tasks.FailOrRetry(cleanup, uid, id, w.owner, v.Fence, code, 0)
		if finishErr != nil {
			return task.RecoveryResult{}, finishErr
		}
		return recoveryResult(finished, len(plan.Steps), "terminal"), nil
	}
	if errors.Is(err, task.ErrLease) {
		if expired, expireErr := w.tasks.Expire(cleanup, uid, id); expireErr == nil {
			return recoveryResult(expired, len(plan.Steps), "terminal"), nil
		} else if !errors.Is(expireErr, task.ErrNotReady) {
			return task.RecoveryResult{}, expireErr
		}
	}
	return task.RecoveryResult{}, err
}

func (w *AIRecoveryWorker) beat(ctx context.Context, v task.Task) error {
	// 间隔控制发起频率，不是数据库延迟上限。单次最多租约的三分之一，
	// 与间隔相加仍留出至少三分之一租约余量；父上下文可进一步收紧。
	ctx, cancel := context.WithTimeout(ctx, w.options.Lease/3)
	defer cancel()
	_, err := w.tasks.Heartbeat(ctx, v.UserID, v.ID, w.owner, v.Fence, w.options.Lease)
	return err
}

func (w *AIRecoveryWorker) withHeartbeat(ctx context.Context, v task.Task, plan task.RecoveryPlan) (task.RecoveryResult, error) {
	return w.withHeartbeatWork(ctx, v, func(ctx context.Context) (task.RecoveryResult, error) { return w.resume(ctx, v, plan) })
}

// withHeartbeatWork 共用同一租约续期/取消/回收机制，外部接线也不能在阻塞 HTTP 时丢失心跳。
func (w *AIRecoveryWorker) withHeartbeatWork(ctx context.Context, v task.Task, work func(context.Context) (task.RecoveryResult, error)) (task.RecoveryResult, error) {
	if err := w.beat(ctx, v); err != nil {
		return task.RecoveryResult{}, err
	}
	workCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	failure := make(chan error, 1)
	go func() {
		defer close(done)
		ticker := time.NewTicker(w.options.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				if err := w.beat(workCtx, v); err != nil {
					if workCtx.Err() != nil {
						return
					} // 正常停止/父上下文结束不伪造心跳故障。
					failure <- err
					cancel()
					return
				}
			}
		}
	}()
	out, err := work(workCtx)
	// cancel 是本地停止，不是用户取消；总是等待独立心跳退出再处理终态。
	cancel()
	<-done
	select {
	case heartbeatErr := <-failure:
		return task.RecoveryResult{}, heartbeatErr
	default:
	}
	return out, err
}

func (w *AIRecoveryWorker) resume(ctx context.Context, v task.Task, plan task.RecoveryPlan) (task.RecoveryResult, error) {
	for index, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return task.RecoveryResult{}, err
		}
		if err := step.Recheck(ctx); err != nil {
			return task.RecoveryResult{}, err
		}
		stages, err := NewAITaskStageService(w.store, map[string]task.StagePolicy{step.Stage: step.Policy})
		if err != nil {
			return task.RecoveryResult{}, err
		}
		r := task.StageRequest{UserID: v.UserID, TaskID: v.ID, Owner: w.owner, Fence: v.Fence, StepNo: index + 1, OperationKey: step.OperationKey, Stage: step.Stage, InputDigest: step.InputDigest}
		completed, err := stages.Complete(ctx, r)
		if errors.Is(err, points.ErrNotFound) {
			if index+1 <= v.CurrentStep {
				return task.RecoveryResult{}, points.ErrInvariant
			}
			if err := w.beat(ctx, v); err != nil {
				return task.RecoveryResult{}, err
			}
			return recoveryResult(v, len(plan.Steps), "response_unknown"), nil
		}
		if err != nil {
			return task.RecoveryResult{}, err
		}
		v.CurrentStep = max(v.CurrentStep, completed.Checkpoint.StepNo)
		if err := step.Recheck(ctx); err != nil {
			return task.RecoveryResult{}, err
		}
	}
	if err := w.beat(ctx, v); err != nil {
		return task.RecoveryResult{}, err
	}
	return recoveryResult(v, len(plan.Steps), "report_pending"), nil
}
