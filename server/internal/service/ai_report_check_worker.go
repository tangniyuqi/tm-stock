package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// AIReportCheckControl 必须校验真实模型/来源/质量/远端硬预算证明，并持久化调用前后审计。
// 本仓库尚无生产放行实现；非空引用或测试控制器不是证明。没有该依赖不能启用。
type AIReportCheckControl interface {
	Recheck(context.Context, task.Task, *finalreport.CheckPlan) error
	Start(context.Context, task.StageRequest, provider.Request) error
	Finish(context.Context, task.StageRequest, provider.Response, error) error
}

type AIReportCheckOptions struct {
	Enabled bool
	Lease   task.RecoveryOptions
}
type AIReportCheckReceipt struct {
	Task       task.RecoveryResult        `json:"task"`
	Evidence   *finalreport.CheckEvidence `json:"-"`
	Checkpoint task.StageCheckpoint       `json:"-"`
}

// AIReportCheckWorker 仅执行一个已持有租约的受限判定步骤，不抢占/创建任务，不结束积分冻结。
// 只接受无重试的具体适配器；不得把 Retry/Router 多次尝试藏在一次预留内。
type AIReportCheckWorker struct {
	tasks   recoveryTasks
	store   taskAgentStore
	model   *provider.OpenAICompat
	control AIReportCheckControl
	options AIReportCheckOptions
	owner   string
}

func NewAIReportCheckWorker(tasks recoveryTasks, store taskAgentStore, model *provider.OpenAICompat, control AIReportCheckControl, owner string, o AIReportCheckOptions) (*AIReportCheckWorker, error) {
	if tasks == nil || store == nil || !task.ValidOwner(owner) || o.Lease.Validate() != nil || o.Enabled && (model == nil || model.DirectProfileDigest() == "" || control == nil) {
		return nil, points.ErrInvalid
	}
	return &AIReportCheckWorker{tasks, store, model, control, o, owner}, nil
}

func (w *AIReportCheckWorker) RunOwned(parent context.Context, r task.StageRequest, p *finalreport.CheckPlan) (AIReportCheckReceipt, error) {
	if w == nil {
		return AIReportCheckReceipt{}, points.ErrInvalid
	}
	if !w.options.Enabled {
		return AIReportCheckReceipt{}, finalreport.ErrDisabled
	}
	if !p.Valid() {
		return AIReportCheckReceipt{}, points.ErrInvalid
	}
	core := reportModelExecution{w.tasks, w.store, w.model, w.control, w.owner, w.options}
	out, err := core.runOwned(parent, r, p, func(ctx context.Context, v task.Task) error { return w.control.Recheck(ctx, v, p) })
	if err != nil {
		return AIReportCheckReceipt{}, err
	}
	if out.task.Outcome == "response_unknown" {
		return AIReportCheckReceipt{Task: out.task}, nil
	}
	evidence, err := p.Evidence(string(out.payload))
	if err != nil {
		return AIReportCheckReceipt{}, err
	}
	out.task.Outcome = "check_" + string(evidence.Decision().Verdict)
	return AIReportCheckReceipt{Task: out.task, Evidence: evidence, Checkpoint: out.checkpoint}, nil
}
