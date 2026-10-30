package finalreport

import (
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// DossierSupportStep只适配已经封闭的字段计划，不向外开放ModelStep实现。
type DossierSupportStep struct{ plan *dossier.SupportPlan }

var _ ModelStep = (*DossierSupportStep)(nil)

func NewDossierSupportStep(plan *dossier.SupportPlan) (*DossierSupportStep, error) {
	if !plan.Valid() {
		return nil, points.ErrInvalid
	}
	return &DossierSupportStep{plan: plan}, nil
}
func (*DossierSupportStep) modelStep()        {}
func (*DossierSupportStep) StageName() string { return "judge" }
func (p *DossierSupportStep) Valid() bool     { return p != nil && p.plan.Valid() }
func (p *DossierSupportStep) InputDigest() string {
	if !p.Valid() {
		return ""
	}
	return p.plan.InputDigest()
}
func (p *DossierSupportStep) Config() CheckConfig {
	if !p.Valid() {
		return CheckConfig{}
	}
	c := p.plan.Config()
	kind := 0
	if p.plan.AnalysisType() == 2 {
		kind = 2
	}
	return CheckConfig{Model: c.Model, Version: c.Version, FilingNo: c.FilingNo, ApprovalRef: c.ApprovalRef, QualityRef: c.QualityRef, HardBudgetRef: c.HardBudgetRef,
		ProfileDigest: c.ProfileDigest, MaxInputTokens: c.MaxInputTokens, CheckMaxTokens: c.MaxOutputTokens, AnalysisType: kind}
}
func (p *DossierSupportStep) MatchesTask(v task.Task) bool { return p.Valid() && p.plan.MatchesTask(v) }
func (p *DossierSupportStep) OperationKey(v task.Task, step int) string {
	if !p.Valid() {
		return ""
	}
	return p.plan.OperationKey(v, step)
}
func (p *DossierSupportStep) Units() task.Units {
	if !p.Valid() {
		return task.Units{}
	}
	return p.plan.Units()
}
func (p *DossierSupportStep) Request() provider.Request {
	if !p.Valid() {
		return provider.Request{}
	}
	return p.plan.Request()
}
func (p *DossierSupportStep) StagePolicy() task.StagePolicy {
	if !p.Valid() {
		return task.StagePolicy{}
	}
	return p.plan.StagePolicy()
}
func (p *DossierSupportStep) CapturePayload(r provider.Response) ([]byte, error) {
	if !p.Valid() {
		return nil, points.ErrInvalid
	}
	return p.plan.CapturePayload(r)
}
func (DossierSupportStep) String() string {
	return "专题字段执行适配（内部计划已隐藏）"
}
func (p DossierSupportStep) GoString() string { return p.String() }
