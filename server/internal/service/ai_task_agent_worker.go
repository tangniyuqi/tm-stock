package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type taskAgentClient interface {
	Show(context.Context, int64, string) (agents.Record, error)
	Submit(context.Context, int64, agents.Request) (agents.Record, error)
	Status(context.Context, int64, string) (agents.Record, error)
	Result(context.Context, int64, string) (agents.Record, error)
}

type taskAgentStore interface {
	task.StageStore
	task.BudgetStore
	task.AdmissionStore
}

// AITaskAgentWorker 是内部接线核；没有公开路由、生产规划器或自动启动路径。
type AITaskAgentWorker struct {
	tasks   recoveryTasks
	store   taskAgentStore
	agent   taskAgentClient
	planner agenttask.Planner
	owner   string
	opts    agenttask.Options
}

func NewAITaskAgentWorker(tasks recoveryTasks, store taskAgentStore, agent taskAgentClient, planner agenttask.Planner, owner string, opts agenttask.Options) (*AITaskAgentWorker, error) {
	if tasks == nil || store == nil || !task.ValidOwner(owner) || opts.Validate() != nil || opts.Enabled && (agent == nil || planner == nil) {
		return nil, points.ErrInvalid
	}
	return &AITaskAgentWorker{tasks, store, agent, planner, owner, opts}, nil
}

func (w *AITaskAgentWorker) Run(parent context.Context, uid, id int64) (task.RecoveryResult, error) {
	if w == nil || uid <= 0 || id <= 0 {
		return task.RecoveryResult{}, points.ErrInvalid
	}
	if !w.opts.Enabled {
		return task.RecoveryResult{}, agents.ErrDisabled
	}
	ctx, stop := context.WithTimeout(parent, w.opts.Lease.Timeout)
	defer stop()
	v, err := w.tasks.Inspect(ctx, uid, id)
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if v.Terminal() {
		return recoveryResult(v, 0, "terminal"), nil
	}
	if err := guardTaskAgentAdmission(ctx, w.store, w.planner, v); err != nil {
		return task.RecoveryResult{}, err
	}
	plan, err := w.planner.Plan(ctx, v)
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if err := plan.Validate(v); err != nil {
		return task.RecoveryResult{}, err
	}
	if err := w.planner.Recheck(ctx, v, plan); err != nil {
		return task.RecoveryResult{}, err
	}
	v, err = w.tasks.Claim(ctx, uid, id, w.owner, w.opts.Lease.Lease)
	if err != nil {
		if errors.Is(err, task.ErrUnconfirmed) {
			return unconfirmedRecovery(ctx, w.tasks, uid, id)
		}
		return task.RecoveryResult{}, err
	}
	if v.Terminal() {
		return recoveryResult(v, 0, "terminal"), nil
	}
	if err := plan.Validate(v); err != nil {
		return task.RecoveryResult{}, err
	}
	keeper := &AIRecoveryWorker{tasks: w.tasks, owner: w.owner, options: w.opts.Lease}
	// 无确定业务终检路径，不把网络/取消/提交未知自动解释为可退款的任务失败。
	return keeper.withHeartbeatWork(ctx, v, func(ctx context.Context) (task.RecoveryResult, error) { return w.execute(ctx, keeper, v, plan) })
}

func checkAgentTaskRecord(p agenttask.Plan, r agents.Record) error {
	if !r.Valid() || r.ActorID != p.ActorID || r.Request.RequestID != p.RequestID() || r.Request.Symbol != p.Symbol || r.ProfileDigest != p.ProfileDigest {
		return agents.ErrConflict
	}
	return nil
}

func (w *AITaskAgentWorker) execute(ctx context.Context, keeper *AIRecoveryWorker, v task.Task, p agenttask.Plan) (task.RecoveryResult, error) {
	return w.executeStep(ctx, keeper, v, p, false)
}

// RunOwned 用于统一编排的固定第一步；已完成前缀只核对/回放，不重新提交。
func (w *AITaskAgentWorker) RunOwned(parent context.Context, v task.Task, p agenttask.Plan) (task.RecoveryResult, error) {
	if w == nil {
		return task.RecoveryResult{}, points.ErrInvalid
	}
	if !w.opts.Enabled {
		return task.RecoveryResult{}, agents.ErrDisabled
	}
	if p.ValidateScope(v) != nil || v.LeaseOwner != w.owner {
		return task.RecoveryResult{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, w.opts.Lease.Timeout)
	defer cancel()
	if err := guardTaskAgentAdmission(ctx, w.store, w.planner, v); err != nil {
		return task.RecoveryResult{}, err
	}
	keeper := &AIRecoveryWorker{tasks: w.tasks, owner: w.owner, options: w.opts.Lease}
	return keeper.withHeartbeatWork(ctx, v, func(ctx context.Context) (task.RecoveryResult, error) { return w.executeStep(ctx, keeper, v, p, true) })
}

func (w *AITaskAgentWorker) executeStep(ctx context.Context, keeper *AIRecoveryWorker, v task.Task, p agenttask.Plan, prefix bool) (task.RecoveryResult, error) {
	input := p.InputDigest(v)
	budget, err := NewAITaskBudgetService(w.store)
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if err := w.planner.Recheck(ctx, v, p); err != nil {
		return task.RecoveryResult{}, err
	}
	request := task.ReserveRequest{UserID: v.UserID, TaskID: v.ID, Fence: v.Fence, Owner: w.owner, Key: p.OperationKey, InputDigest: input, Units: p.Units}
	var reserved task.ReservationReceipt
	if prefix {
		reserved, err = budget.ReserveStep(ctx, request, 1)
	} else {
		reserved, err = budget.ReserveSingleOperation(ctx, request)
	}
	if err != nil {
		return task.RecoveryResult{}, err
	}
	record, err := w.agent.Show(ctx, p.ActorID, p.RequestID())
	if errors.Is(err, agents.ErrNotFound) {
		if !reserved.NewlyReserved {
			return recoveryResult(v, 1, "response_unknown"), nil
		}
		if err := keeper.beat(ctx, v); err != nil {
			return task.RecoveryResult{}, err
		}
		if err := w.planner.Recheck(ctx, v, p); err != nil {
			return task.RecoveryResult{}, err
		}
		record, err = w.agent.Submit(ctx, p.ActorID, agents.Request{RequestID: p.RequestID(), Symbol: p.Symbol})
	}
	if errors.Is(err, agents.ErrUncertain) {
		return recoveryResult(v, 1, "response_unknown"), nil
	}
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if err := checkAgentTaskRecord(p, record); err != nil {
		return task.RecoveryResult{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return task.RecoveryResult{}, err
		}
		if err := w.planner.Recheck(ctx, v, p); err != nil {
			return task.RecoveryResult{}, err
		}
		switch record.State {
		case agents.Submitting, agents.SubmissionUnknown:
			return recoveryResult(v, 1, "response_unknown"), nil
		case agents.Failed, agents.Cancelled:
			// 先捕获并确认封闭终态回执；不能仅凭内存返回值绕过失败前的未知调用核查。
			return w.checkpoint(ctx, keeper, v, p, input, reserved, record)
		case agents.Quarantined:
			return w.checkpoint(ctx, keeper, v, p, input, reserved, record)
		case agents.Completed:
			record, err = w.agent.Result(ctx, p.ActorID, p.RequestID())
			if err == nil && record.State != agents.Quarantined {
				return task.RecoveryResult{}, agents.ErrConflict
			}
		default:
			record, err = w.agent.Status(ctx, p.ActorID, p.RequestID())
		}
		if err != nil {
			return task.RecoveryResult{}, err
		}
		if err := checkAgentTaskRecord(p, record); err != nil {
			return task.RecoveryResult{}, err
		}
		if record.State == agents.Quarantined || record.State == agents.Completed || agents.Terminal(record.State) {
			continue
		}
		timer := time.NewTimer(w.opts.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return task.RecoveryResult{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func (w *AITaskAgentWorker) checkpoint(ctx context.Context, keeper *AIRecoveryWorker, v task.Task, p agenttask.Plan, input string, reserved task.ReservationReceipt, record agents.Record) (task.RecoveryResult, error) {
	terminal := record.State == agents.Failed || record.State == agents.Cancelled
	var payload []byte
	var policy task.StagePolicy
	var err error
	if terminal {
		payload, policy, err = agenttask.TerminalReceiptPolicy(p, input, record)
	} else {
		payload, policy, err = agenttask.ReceiptPolicy(p, input, record)
	}
	if err != nil {
		return task.RecoveryResult{}, err
	}
	stages, err := NewAITaskStageService(w.store, map[string]task.StagePolicy{"agent_call": policy})
	if err != nil {
		return task.RecoveryResult{}, err
	}
	r := task.StageRequest{UserID: v.UserID, TaskID: v.ID, Fence: v.Fence, Owner: w.owner, OperationKey: p.OperationKey, Stage: "agent_call", InputDigest: input, StepNo: 1}
	done, err := stages.Complete(ctx, r)
	if errors.Is(err, points.ErrNotFound) {
		// 不补造旧围栏的响应。远端已隔离但旧租约没捕获回执，只标记待核对，不新提交。
		if reserved.Reservation.Owner != w.owner || reserved.Reservation.Fence != v.Fence {
			return recoveryResult(v, 1, "capture_pending"), nil
		}
		if err := w.planner.Recheck(ctx, v, p); err != nil {
			return task.RecoveryResult{}, err
		}
		if _, err := stages.Capture(ctx, r, payload); err != nil {
			return task.RecoveryResult{}, err
		}
		done, err = stages.Complete(ctx, r)
	}
	if err != nil {
		return task.RecoveryResult{}, err
	}
	if err := w.planner.Recheck(ctx, v, p); err != nil {
		return task.RecoveryResult{}, err
	}
	if err := keeper.beat(ctx, v); err != nil {
		return task.RecoveryResult{}, err
	}
	v.CurrentStep = max(v.CurrentStep, done.Checkpoint.StepNo)
	if terminal {
		return recoveryResult(v, 1, "upstream_terminal"), nil
	}
	return recoveryResult(v, 1, "evidence_pending"), nil
}
