package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type reportModelAudit interface {
	Start(context.Context, task.StageRequest, provider.Request) error
	Finish(context.Context, task.StageRequest, provider.Response, error) error
}
type reportModelExecution struct {
	tasks   recoveryTasks
	store   taskAgentStore
	model   *provider.OpenAICompat
	audit   reportModelAudit
	owner   string
	options AIReportCheckOptions
}
type reportModelResult struct {
	task       task.RecoveryResult
	payload    []byte
	checkpoint task.StageCheckpoint
}

// runOwned 复用原判定步骤的一次预留协议；仅允许封闭计划，不封装重试/路由/自动放行。
func (w reportModelExecution) runOwned(parent context.Context, r task.StageRequest, p finalreport.ModelStep, recheck func(context.Context, task.Task) error) (reportModelResult, error) {
	if !w.options.Enabled {
		return reportModelResult{}, finalreport.ErrDisabled
	}
	if r.Validate() != nil || r.Owner != w.owner || p == nil || !p.Valid() || r.Stage != p.StageName() || r.InputDigest != p.InputDigest() || w.model == nil || !w.model.MatchesDirect(p.Config().ProfileDigest, p.Config().Model) || w.audit == nil || recheck == nil {
		return reportModelResult{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, w.options.Lease.Timeout)
	defer cancel()
	v, err := w.tasks.Inspect(ctx, r.UserID, r.TaskID)
	if err != nil {
		return reportModelResult{}, err
	}
	if v.ID != r.TaskID || v.UserID != r.UserID || !p.MatchesTask(v) || r.OperationKey != p.OperationKey(v, r.StepNo) || v.Fence != r.Fence || v.LeaseOwner != r.Owner || v.CurrentStep < r.StepNo-1 {
		return reportModelResult{}, points.ErrIdempotency
	}
	if err := recheck(ctx, v); err != nil {
		return reportModelResult{}, err
	}
	keeper := &AIRecoveryWorker{tasks: w.tasks, owner: w.owner, options: w.options.Lease}
	var out reportModelResult
	result, err := keeper.withHeartbeatWork(ctx, v, func(ctx context.Context) (task.RecoveryResult, error) {
		budget, err := NewAITaskBudgetService(w.store)
		if err != nil {
			return task.RecoveryResult{}, err
		}
		reserved, err := budget.ReserveStep(ctx, task.ReserveRequest{UserID: r.UserID, TaskID: r.TaskID, Owner: r.Owner, Fence: r.Fence, Key: r.OperationKey, InputDigest: r.InputDigest, Units: p.Units()}, r.StepNo)
		if err != nil {
			return task.RecoveryResult{}, err
		}
		stages, err := NewAITaskStageService(w.store, map[string]task.StagePolicy{r.Stage: p.StagePolicy()})
		if err != nil {
			return task.RecoveryResult{}, err
		}
		if reserved.NewlyReserved {
			if err := keeper.beat(ctx, v); err != nil {
				return task.RecoveryResult{}, err
			}
			if err := recheck(ctx, v); err != nil {
				return task.RecoveryResult{}, err
			}
			req := p.Request()
			req.TaskID = v.No
			req.Step = r.StepNo
			if err := w.audit.Start(ctx, r, req); err != nil {
				return task.RecoveryResult{}, err
			}
			response, callErr := w.model.Generate(ctx, req)
			finishCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			auditErr := w.audit.Finish(finishCtx, r, response, callErr)
			stop()
			if auditErr != nil {
				return task.RecoveryResult{}, auditErr
			}
			if callErr != nil {
				return task.RecoveryResult{}, callErr
			}
			if err := ctx.Err(); err != nil {
				return task.RecoveryResult{}, err
			}
			if err := recheck(ctx, v); err != nil {
				return task.RecoveryResult{}, err
			}
			payload, err := p.CapturePayload(response)
			if err != nil {
				return task.RecoveryResult{}, err
			}
			if _, err := stages.Capture(ctx, r, payload); err != nil {
				return task.RecoveryResult{}, err
			}
		}
		completed, err := stages.Complete(ctx, r)
		if errors.Is(err, points.ErrNotFound) {
			return recoveryResult(v, r.StepNo, "response_unknown"), nil
		}
		if err != nil {
			return task.RecoveryResult{}, err
		}
		loaded, err := stages.Load(ctx, r)
		if err != nil {
			return task.RecoveryResult{}, err
		}
		if err := recheck(ctx, v); err != nil {
			return task.RecoveryResult{}, err
		}
		if err := keeper.beat(ctx, v); err != nil {
			return task.RecoveryResult{}, err
		}
		v.CurrentStep = max(v.CurrentStep, completed.Checkpoint.StepNo)
		out.payload = loaded.Result.Payload
		out.checkpoint = completed.Checkpoint
		return recoveryResult(v, r.StepNo, "model_step_captured"), nil
	})
	if err != nil {
		return reportModelResult{}, err
	}
	out.task = result
	return out, nil
}
