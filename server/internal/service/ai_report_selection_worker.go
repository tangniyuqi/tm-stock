package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// AIReportSelectionControl 必须证明当前来源、模型准入、质量与硬预算并持久审计。
// 没有生产实现；测试控制器不能装入产品启动路径。
type AIReportSelectionControl interface {
	Recheck(context.Context, task.Task, *finalreport.SelectionModelPlan) error
	Start(context.Context, task.StageRequest, provider.Request) error
	Finish(context.Context, task.StageRequest, provider.Response, error) error
}
type AIReportSelectionReceipt struct {
	Task       task.RecoveryResult  `json:"task"`
	Checkpoint task.StageCheckpoint `json:"-"`
}
type AIReportSelectionWorker struct {
	core    reportModelExecution
	control AIReportSelectionControl
}

func NewAIReportSelectionWorker(tasks recoveryTasks, store taskAgentStore, model *provider.OpenAICompat, control AIReportSelectionControl, owner string, o AIReportCheckOptions) (*AIReportSelectionWorker, error) {
	if tasks == nil || store == nil || !task.ValidOwner(owner) || o.Lease.Validate() != nil || o.Enabled && (model == nil || model.DirectProfileDigest() == "" || control == nil) {
		return nil, points.ErrInvalid
	}
	return &AIReportSelectionWorker{reportModelExecution{tasks, store, model, control, owner, o}, control}, nil
}
func (w *AIReportSelectionWorker) RunOwned(ctx context.Context, r task.StageRequest, p *finalreport.SelectionModelPlan) (AIReportSelectionReceipt, error) {
	if w == nil {
		return AIReportSelectionReceipt{}, points.ErrInvalid
	}
	if !w.core.options.Enabled {
		return AIReportSelectionReceipt{}, finalreport.ErrDisabled
	}
	if !p.Valid() {
		return AIReportSelectionReceipt{}, points.ErrInvalid
	}
	out, err := w.core.runOwned(ctx, r, p, func(ctx context.Context, v task.Task) error {
		proof, policy, err := p.AgentBinding(v, w.core.owner)
		if err != nil {
			return err
		}
		stages, err := NewAITaskStageService(w.core.store, map[string]task.StagePolicy{"agent_call": policy})
		if err != nil {
			return err
		}
		if _, err := stages.Load(ctx, proof); err != nil {
			return err
		}
		return w.control.Recheck(ctx, v, p)
	})
	if err != nil {
		return AIReportSelectionReceipt{}, err
	}
	if out.task.Outcome != "response_unknown" {
		out.task.Outcome = "selection_captured"
	}
	return AIReportSelectionReceipt{out.task, out.checkpoint}, nil
}
