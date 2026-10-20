package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 不包含普通404、缺配置、身份/计划冲突或依赖错误，不能借“拒绝”释放未知调用。
func l1KnownRejection(err error) bool {
	return errors.Is(err, sources.ErrUnauthorized) || errors.Is(err, sources.ErrSourceUnavailable) || errors.Is(err, sources.ErrUnsafeURL) || errors.Is(err, finalreport.ErrRejected) || errors.Is(err, task.ErrStageRejected)
}

func (w *AIL1ReportWorker) ownTask(ctx context.Context, v task.Task) (task.Task, error) {
	if v.Status == task.Running && v.LeaseOwner == w.owner {
		current, err := w.tasks.Heartbeat(ctx, v.UserID, v.ID, w.owner, v.Fence, w.opts.Lease.Lease)
		if errors.Is(err, task.ErrCancelled) {
			// 心跳不为取消请求续租；重新读取后仍须在拒绝事务中核对原有效围栏。
			return w.tasks.Inspect(ctx, v.UserID, v.ID)
		}
		if !errors.Is(err, task.ErrLease) {
			return current, err
		}
	}
	return w.tasks.Claim(ctx, v.UserID, v.ID, w.owner, w.opts.Lease.Lease)
}

// l1CallsCompleted 只核对持久证明，不推断供应商用量或重新执行任何模型。
// 尚有未完成预留时即使来源已撤回，也必须保留原键和冻结供核查。
func l1CallsCompleted(ctx context.Context, tx finalreport.Tx, v task.Task) (bool, error) {
	if v.CurrentStep < 0 || v.CurrentStep > task.MaxSteps || v.Budget.Validate() != nil {
		return false, points.ErrInvariant
	}
	state, budgetErr := tx.LockBudget(ctx, v.UserID, v.ID)
	if budgetErr != nil && !errors.Is(budgetErr, points.ErrNotFound) {
		return false, budgetErr
	}
	rows, err := tx.Reservations(ctx, v.UserID, v.ID)
	if err != nil {
		return false, err
	}
	if budgetErr != nil {
		if v.CurrentStep != 0 || len(rows) != 0 {
			return false, points.ErrInvariant
		}
		return true, nil
	}
	if err := task.CheckBudget(v, state, rows); err != nil {
		return false, err
	}
	if len(rows) < v.CurrentStep {
		return false, points.ErrInvariant
	}
	var previous int64
	for i, reservation := range rows[:v.CurrentStep] {
		checkpoint, err := tx.StageCheckpoint(ctx, v.UserID, v.ID, i+1)
		if err != nil {
			return false, err
		}
		result, err := tx.StageResult(ctx, v.UserID, v.ID, reservation.Seq)
		if err != nil {
			return false, err
		}
		request := task.StageRequest{UserID: v.UserID, TaskID: v.ID, StepNo: i + 1, Stage: checkpoint.Stage, InputDigest: reservation.InputDigest}
		if task.CheckStageResult(request, reservation, result) != nil || checkpoint.TaskID != v.ID || checkpoint.UserID != v.UserID || checkpoint.StepNo != i+1 || checkpoint.ReservationSeq != reservation.Seq || checkpoint.InputDigest != reservation.InputDigest || checkpoint.OutputDigest != result.OutputDigest || !finalreport.ValidDigest(checkpoint.PolicyDigest) || !task.ValidStage(checkpoint.Stage) || !task.StageUnitsMatch(checkpoint.Stage, reservation.Units) || checkpoint.Fence < result.Fence || checkpoint.Fence > v.Fence || !task.ValidOwner(checkpoint.Owner) || checkpoint.CreatedAt.IsZero() || checkpoint.EventSeq <= previous || checkpoint.EventSeq > v.EventSeq {
			return false, points.ErrInvariant
		}
		event, err := tx.StageEvent(ctx, v.ID, checkpoint.EventSeq)
		if err != nil {
			return false, err
		}
		if event != (task.Event{TaskID: v.ID, Seq: checkpoint.EventSeq, Type: "step_completed", Status: task.Running, Step: i + 1}) {
			return false, points.ErrInvariant
		}
		previous = checkpoint.EventSeq
	}
	return len(rows) == v.CurrentStep, nil
}

func l1RejectionState(ctx context.Context, tx finalreport.Tx, uid, id int64) (task.Task, bool, error) {
	v, err := lockOwnedTask(ctx, tx, uid, id)
	if err != nil {
		return task.Task{}, false, err
	}
	if v.Terminal() {
		return v, true, nil
	}
	if err := checkOriginalAdmission(ctx, tx, v); err != nil {
		return task.Task{}, false, err
	}
	if _, err := tx.ReportCommit(ctx, uid, id); !errors.Is(err, points.ErrNotFound) {
		if err == nil {
			err = points.ErrInvariant
		}
		return task.Task{}, false, err
	}
	complete, err := l1CallsCompleted(ctx, tx, v)
	return v, complete, err
}

// rejectKnown 的当前任务、预算/完成证明、冻结单及失败事件必须在同一事务锁下确认。
// 不调用普通FailOrRetry后再补查预算，避免两次事务之间插入新的未知预留。
func (w *AIL1ReportWorker) rejectKnown(parent context.Context, v task.Task) (AIL1ReportReceipt, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	// 领取可能因尝试耗尽而直接终结，必须先排除未完成调用；最终事务仍重查一次防竞争。
	var current task.Task
	var complete bool
	err := w.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		var err error
		current, complete, err = l1RejectionState(ctx, tx, v.UserID, v.ID)
		return err
	})
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if current.Terminal() {
		return AIL1ReportReceipt{Task: recoveryResult(current, 0, "terminal")}, nil
	}
	if !complete {
		if current.Status != task.Running {
			return AIL1ReportReceipt{}, task.ErrNotReady
		}
		return AIL1ReportReceipt{Task: recoveryResult(current, current.CurrentStep, "response_unknown")}, nil
	}
	current, err = w.ownTask(ctx, current)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if current.Terminal() {
		return AIL1ReportReceipt{Task: recoveryResult(current, 0, "terminal")}, nil
	}
	ownedFence := current.Fence
	var out AIL1ReportReceipt
	err = w.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		current, complete, err := l1RejectionState(ctx, tx, current.UserID, current.ID)
		if err != nil {
			return err
		}
		if current.Terminal() {
			out.Task = recoveryResult(current, 0, "terminal")
			return nil
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if err := ownedLease(current, w.owner, ownedFence, now); err != nil {
			return err
		}
		if !complete {
			out.Task = recoveryResult(current, current.CurrentStep, "response_unknown")
			return nil
		}
		hold, err := tx.Points().LockHold(ctx, current.UserID, current.HoldID)
		if err != nil {
			return err
		}
		if hold.ID != current.HoldID || hold.UserID != current.UserID || hold.State != points.Holding || hold.Amount != current.PricePoints || hold.PriceVersion != current.PriceVersion || hold.BizType != "ai_task" || hold.BizID != "task:"+current.No || !hold.ExpiresAt.Equal(current.Deadline) {
			return points.ErrInvariant
		}
		status, code := task.Failed, "guard_rejected"
		if current.CancelRequested {
			status, code = task.Cancelled, "cancelled"
		}
		done, err := (&AITaskService{ledger: w.ledger}).finishTask(ctx, tx, current, status, code)
		if err != nil {
			return err
		}
		out.Task = recoveryResult(done, done.CurrentStep, "terminal")
		return nil
	})
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	return out, nil
}
