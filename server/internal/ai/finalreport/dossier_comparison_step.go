package finalreport

import (
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// DossierComparisonStep只允许封闭的两卡计划进入原单次模型协议。
type DossierComparisonStep struct{ plan *dossier.CardComparisonPlan }

var _ ModelStep = (*DossierComparisonStep)(nil)

func NewDossierComparisonStep(plan *dossier.CardComparisonPlan) (*DossierComparisonStep, error) {
	if !plan.Valid() {
		return nil, points.ErrInvalid
	}
	return &DossierComparisonStep{plan}, nil
}
func (*DossierComparisonStep) modelStep()        {}
func (*DossierComparisonStep) StageName() string { return "judge" }
func (p *DossierComparisonStep) Valid() bool     { return p != nil && p.plan.Valid() }
func (p *DossierComparisonStep) InputDigest() string {
	if !p.Valid() {
		return ""
	}
	return p.plan.InputDigest()
}
func (p *DossierComparisonStep) Config() CheckConfig {
	if !p.Valid() {
		return CheckConfig{}
	}
	c := p.plan.Config().Model
	return CheckConfig{Model: c.Model, Version: c.Version, FilingNo: c.FilingNo, ApprovalRef: c.ApprovalRef, QualityRef: c.QualityRef, HardBudgetRef: c.HardBudgetRef, ProfileDigest: c.ProfileDigest, MaxInputTokens: c.MaxInputTokens, CheckMaxTokens: c.MaxOutputTokens, AnalysisType: task.DeepAnalysisType}
}
func (p *DossierComparisonStep) MatchesTask(v task.Task) bool {
	return p.Valid() && p.plan.MatchesTask(v)
}
func (p *DossierComparisonStep) OperationKey(v task.Task, step int) string {
	if !p.Valid() {
		return ""
	}
	return p.plan.OperationKey(v, step)
}
func (p *DossierComparisonStep) Units() task.Units {
	if !p.Valid() {
		return task.Units{}
	}
	return p.plan.Units()
}
func (p *DossierComparisonStep) Request() provider.Request {
	if !p.Valid() {
		return provider.Request{}
	}
	return p.plan.Request()
}
func (p *DossierComparisonStep) StagePolicy() task.StagePolicy {
	if !p.Valid() {
		return task.StagePolicy{}
	}
	return p.plan.StagePolicy()
}
func (p *DossierComparisonStep) CapturePayload(r provider.Response) ([]byte, error) {
	if !p.Valid() {
		return nil, points.ErrInvalid
	}
	return p.plan.CapturePayload(r)
}
func (DossierComparisonStep) String() string {
	return "两卡比较执行适配（任务与原文已隐藏）"
}
func (p DossierComparisonStep) GoString() string { return p.String() }
