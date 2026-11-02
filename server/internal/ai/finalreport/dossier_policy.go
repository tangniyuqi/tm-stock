package finalreport

import (
	"encoding/json"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const DossierReportPolicyVersion = "dossier-report-policy-v1"

// NewDossierPolicy绑定新编排的真实任务计划；未捕获字段结果不能交付。
func NewDossierPolicy(c Config, p *dossier.SupportPlan, e EntailmentCheck, s SemanticCheck, companies ...*CompanyDraft) (*Policy, error) {
	if c.DossierComparisonDigest != "" || !p.Valid() || p.StepNo() != 4 || c.DossierSupportDigest != p.ComponentDigest() || c.ModelName != p.Config().Model || c.FilingNo != p.Config().FilingNo || len(companies) > 1 {
		return nil, points.ErrInvalid
	}
	want := 0
	if p.AnalysisType() == 2 {
		want = 2
	}
	if c.AnalysisType != want {
		return nil, points.ErrInvalid
	}
	base := c
	base.DossierSupportDigest = ""
	var policy *Policy
	var err error
	if len(companies) == 1 && companies[0] != nil {
		policy, err = NewCompanyPolicy(base, companies[0], e, s)
	} else {
		policy, err = NewPolicy(base, e, s)
	}
	if err != nil {
		return nil, err
	}
	policy.cfg.DossierSupportDigest = c.DossierSupportDigest
	policy.dossierPlan = p
	policy.dossierBaseDigest = policy.digest
	if err = policy.dossierDigest(); err != nil {
		return nil, err
	}
	return policy, nil
}

func (p *Policy) dossierDigest() error {
	payload := ""
	if p.dossierReview != nil {
		payload = p.dossierReview.PayloadDigest()
	}
	raw, err := json.Marshal([]any{DossierReportPolicyVersion, p.dossierBaseDigest, p.dossierPlan.InputDigest(), p.dossierPlan.PolicyDigest(), payload})
	if err != nil {
		return points.ErrInvalid
	}
	p.digest = task.StageDigest(raw)
	return nil
}

// NewDossierCapturedPolicy只消费实际捕获的纯判定；不存在替补模型调用或默认支持。
func NewDossierCapturedPolicy(c Config, p *dossier.SupportPlan, review *dossier.SupportReview, e, s *CheckEvidence, companies ...*CompanyDraft) (*Policy, error) {
	if e == nil || !e.plan.Valid() || e.plan.kind != CheckEntailment || e.plan.taskNo != p.Request().TaskID || c.EntailmentVersion != e.plan.PolicyVersion() {
		return nil, points.ErrInvalid
	}
	if e.Decision().Verdict == Pass && review == nil {
		return nil, points.ErrInvalid
	}
	if s == nil {
		if e.Decision().Verdict == Pass && review.ResolvedCount() > 0 {
			return nil, points.ErrInvalid
		}
	} else if !s.plan.Valid() || s.plan.kind != CheckSemantic || s.plan.taskNo != e.plan.taskNo || c.SemanticVersion != s.plan.PolicyVersion() {
		return nil, points.ErrInvalid
	}
	if review != nil {
		if _, err := p.VisibleReview(review); err != nil {
			return nil, err
		}
	}
	policy, err := NewDossierPolicy(c, p, e.Entailment, s.Semantic, companies...)
	if err != nil {
		return nil, err
	}
	policy.dossierReview = review
	if err = policy.dossierDigest(); err != nil {
		return nil, err
	}
	return policy, nil
}

func (p *Policy) dossierDeliverable() bool {
	if p == nil || p.dossierPlan == nil || p.dossierReview == nil || p.dossierReview.ResolvedCount() < 1 {
		return false
	}
	_, err := p.dossierPlan.VisibleReview(p.dossierReview)
	if err != nil {
		return false
	}
	if p.comparisonPlan != nil {
		return p.comparisonReview != nil && p.comparisonReview.ResolvedCount() > 0 && p.comparisonPlan.CheckSupport(p.dossierPlan, p.dossierReview, p.comparisonReview) == nil
	}
	return true
}
func (p *Policy) DossierProof() (*dossier.SupportPlan, *dossier.SupportReview) {
	if p == nil {
		return nil, nil
	}
	return p.dossierPlan, p.dossierReview
}
func (p *Policy) dossierAdmission(a task.AdmissionPlan) bool {
	if p.comparisonPlan != nil {
		return a.ReportProfile == task.DossierComparisonReportProfile && a.DossierSupportDigest == p.cfg.DossierSupportDigest && a.DossierComparisonDigest == p.cfg.DossierComparisonDigest && p.dossierPlan.MatchesAdmission(a) && p.comparisonPlan.MatchesAdmission(a)
	}
	if p.dossierPlan == nil {
		return a.ReportProfile == "" && a.DossierSupportDigest == "" && p.cfg.DossierSupportDigest == "" && a.DossierComparisonDigest == "" && p.cfg.DossierComparisonDigest == ""
	}
	return a.ReportProfile == task.DossierReportProfile && a.DossierComparisonDigest == "" && p.cfg.DossierComparisonDigest == "" && a.DossierSupportDigest == p.cfg.DossierSupportDigest && p.dossierPlan.MatchesAdmission(a)
}

func DossierVisible(visible string, p *dossier.SupportPlan, r *dossier.SupportReview) (string, error) {
	extra, err := p.VisibleReview(r)
	if err != nil {
		return "", err
	}
	return visible + "\n" + extra, nil
}
func (p *Policy) dossierTask(taskNo string) bool {
	return p.dossierPlan == nil || p.dossierPlan.Request().TaskID == taskNo
}
func (s *SelectionStage) HasDossierSupport() bool {
	return s != nil && s.policy != nil && s.policy.dossierPlan != nil
}
