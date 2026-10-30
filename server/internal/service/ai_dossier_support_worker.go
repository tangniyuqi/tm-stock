package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// Recheck必须亲自重建当前字段计划/来源处理许可、模型/质量/远端硬限额证明。
// Start/Finish必须接原持久审计；没有生产默认放行实现。
type AIDossierSupportControl interface {
	Recheck(context.Context, task.Task, *dossier.SupportPlan) error
	Start(context.Context, task.StageRequest, provider.Request) error
	Finish(context.Context, task.StageRequest, provider.Response, error) error
}
type AIDossierSupportReceipt struct {
	Task       task.RecoveryResult    `json:"task"`
	Review     *dossier.SupportReview `json:"-"`
	Checkpoint task.StageCheckpoint   `json:"-"`
}

func (AIDossierSupportReceipt) String() string {
	return "专题字段执行回执（内部核验已隐藏）"
}
func (r AIDossierSupportReceipt) GoString() string { return r.String() }

type AIDossierSupportWorker struct {
	core    reportModelExecution
	control AIDossierSupportControl
}

func NewAIDossierSupportWorker(tasks recoveryTasks, store taskAgentStore, model *provider.OpenAICompat, control AIDossierSupportControl, owner string, o AIReportCheckOptions) (*AIDossierSupportWorker, error) {
	if tasks == nil || store == nil || !task.ValidOwner(owner) || o.Lease.Validate() != nil || o.Enabled && (model == nil || model.DirectProfileDigest() == "" || control == nil) {
		return nil, points.ErrInvalid
	}
	return &AIDossierSupportWorker{core: reportModelExecution{tasks: tasks, store: store, model: model, audit: control, owner: owner, options: o}, control: control}, nil
}

// RunOwned只运行一个已获准入、已持有租约的核验步骤，不创建任务/报告或结算/释放积分。
func (w *AIDossierSupportWorker) RunOwned(parent context.Context, r task.StageRequest, p *dossier.SupportPlan) (AIDossierSupportReceipt, error) {
	if w == nil || parent == nil {
		return AIDossierSupportReceipt{}, points.ErrInvalid
	}
	if !w.core.options.Enabled {
		return AIDossierSupportReceipt{}, finalreport.ErrDisabled
	}
	// 当前原调用审计仅允许第3/4步judge；不借该适配改写旧报告的执行语义。
	if !p.Valid() || r.Stage != "judge" || r.StepNo != p.StepNo() {
		return AIDossierSupportReceipt{}, points.ErrInvalid
	}
	step, err := finalreport.NewDossierSupportStep(p)
	if err != nil {
		return AIDossierSupportReceipt{}, err
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
		return AIDossierSupportReceipt{}, err
	}
	if out.task.Outcome == "response_unknown" {
		return AIDossierSupportReceipt{Task: out.task}, nil
	}
	review, err := p.Review(parent, out.payload)
	if err != nil {
		return AIDossierSupportReceipt{}, err
	}
	out.task.Outcome = "dossier_support_recorded"
	return AIDossierSupportReceipt{Task: out.task, Review: review, Checkpoint: out.checkpoint}, nil
}
