package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type AIDossierComparisonControl interface {
	Recheck(context.Context, task.Task, *dossier.CardComparisonPlan) error
	Start(context.Context, task.StageRequest, provider.Request) error
	Finish(context.Context, task.StageRequest, provider.Response, error) error
}
type AIDossierComparisonReceipt struct {
	Task       task.RecoveryResult           `json:"task"`
	Review     *dossier.CardComparisonReview `json:"-"`
	Checkpoint task.StageCheckpoint          `json:"-"`
}

func (AIDossierComparisonReceipt) String() string {
	return "两卡比较执行回执（内部核验已隐藏）"
}
func (r AIDossierComparisonReceipt) GoString() string { return r.String() }

type AIDossierComparisonWorker struct {
	core    reportModelExecution
	control AIDossierComparisonControl
}

func NewAIDossierComparisonWorker(tasks recoveryTasks, store taskAgentStore, model *provider.OpenAICompat, control AIDossierComparisonControl, owner string, o AIReportCheckOptions) (*AIDossierComparisonWorker, error) {
	if tasks == nil || store == nil || !task.ValidOwner(owner) || o.Lease.Validate() != nil || o.Enabled && (model == nil || model.DirectProfileDigest() == "" || control == nil) {
		return nil, points.ErrInvalid
	}
	return &AIDossierComparisonWorker{core: reportModelExecution{tasks: tasks, store: store, model: model, audit: control, owner: owner, options: o}, control: control}, nil
}

// RunOwned沿原预留/心跳/捕获/检查点运行一步，不创建任务、报告或积分流水。
func (w *AIDossierComparisonWorker) RunOwned(parent context.Context, r task.StageRequest, p *dossier.CardComparisonPlan) (AIDossierComparisonReceipt, error) {
	if w == nil || parent == nil {
		return AIDossierComparisonReceipt{}, points.ErrInvalid
	}
	if !w.core.options.Enabled {
		return AIDossierComparisonReceipt{}, finalreport.ErrDisabled
	}
	if !p.Valid() || r.Stage != "judge" || r.StepNo != 5 || r.StepNo != p.StepNo() {
		return AIDossierComparisonReceipt{}, points.ErrInvalid
	}
	step, err := finalreport.NewDossierComparisonStep(p)
	if err != nil {
		return AIDossierComparisonReceipt{}, err
	}
	out, err := w.core.runOwned(parent, r, step, func(ctx context.Context, v task.Task) error {
		if err := w.core.store.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
			current, err := lockOwnedTask(ctx, tx, v.UserID, v.ID)
			if err != nil {
				return err
			}
			a, found, err := reportAdmission(ctx, tx, current)
			if err != nil {
				return err
			}
			if !found {
				return task.ErrAdmissionUnavailable
			}
			if !p.MatchesTask(current) || !p.MatchesAdmission(a.Plan) {
				return points.ErrIdempotency
			}
			return nil
		}); err != nil {
			return err
		}
		return w.control.Recheck(ctx, v, p)
	})
	if err != nil {
		return AIDossierComparisonReceipt{}, err
	}
	if out.task.Outcome == "response_unknown" {
		return AIDossierComparisonReceipt{Task: out.task}, nil
	}
	review, err := p.Review(parent, out.payload)
	if err != nil {
		return AIDossierComparisonReceipt{}, err
	}
	out.task.Outcome = "dossier_comparison_recorded"
	return AIDossierComparisonReceipt{Task: out.task, Review: review, Checkpoint: out.checkpoint}, nil
}
