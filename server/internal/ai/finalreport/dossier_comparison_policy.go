package finalreport

import (
	"encoding/json"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const DossierComparisonPolicyVersion = "dossier-comparison-report-policy-v1"

func comparisonPolicyInput(c Config, fields *dossier.SupportPlan, comparison *dossier.CardComparisonPlan) bool {
	return c.AnalysisType == task.DeepAnalysisType && comparison.Valid() && comparison.StepNo() == 5 && comparison.MatchesFields(fields) && fields.StepNo() == 4 &&
		c.DossierComparisonDigest == comparison.ComponentDigest() && c.ModelName == comparison.Config().Model.Model && c.FilingNo == comparison.Config().Model.FilingNo && fields.Config().ProfileDigest == comparison.Config().Model.ProfileDigest
}
func NewDossierComparisonPolicy(c Config, fields *dossier.SupportPlan, comparison *dossier.CardComparisonPlan, e EntailmentCheck, s SemanticCheck, companies ...*CompanyDraft) (*Policy, error) {
	if !comparisonPolicyInput(c, fields, comparison) {
		return nil, points.ErrInvalid
	}
	base := c
	base.DossierComparisonDigest = ""
	p, err := NewDossierPolicy(base, fields, e, s, companies...)
	if err != nil {
		return nil, err
	}
	p.cfg.DossierComparisonDigest = c.DossierComparisonDigest
	p.comparisonPlan = comparison
	p.comparisonBaseDigest = p.digest
	if err = p.comparisonDigest(); err != nil {
		return nil, err
	}
	return p, nil
}

// 两份原始捕获都要绑定，不接受“相同结论”的新包替代原调用证据。
func (p *Policy) comparisonDigest() error {
	payload := ""
	if p.comparisonReview != nil {
		payload = p.comparisonReview.PayloadDigest()
	}
	raw, err := json.Marshal([]any{DossierComparisonPolicyVersion, p.comparisonBaseDigest, p.comparisonPlan.InputDigest(), p.comparisonPlan.PolicyDigest(), payload})
	if err != nil {
		return points.ErrInvalid
	}
	p.digest = task.StageDigest(raw)
	return nil
}
func NewDossierComparisonCapturedPolicy(c Config, fields *dossier.SupportPlan, comparison *dossier.CardComparisonPlan, review *dossier.SupportReview, compared *dossier.CardComparisonReview, e, s *CheckEvidence, companies ...*CompanyDraft) (*Policy, error) {
	if !comparisonPolicyInput(c, fields, comparison) || e == nil || !e.plan.Valid() || e.plan.kind != CheckEntailment || e.plan.taskNo != fields.Request().TaskID || c.EntailmentVersion != e.plan.PolicyVersion() {
		return nil, points.ErrInvalid
	}
	if e.Decision().Verdict == Pass {
		if review == nil || review.ResolvedCount() > 0 && compared == nil {
			return nil, points.ErrInvalid
		}
		if s == nil && compared.ResolvedCount() > 0 {
			return nil, points.ErrInvalid
		}
	}
	if s != nil && (!s.plan.Valid() || s.plan.kind != CheckSemantic || s.plan.taskNo != e.plan.taskNo || c.SemanticVersion != s.plan.PolicyVersion()) {
		return nil, points.ErrInvalid
	}
	if review != nil {
		if _, err := fields.VisibleReview(review); err != nil {
			return nil, err
		}
	}
	if compared != nil {
		if comparison.CheckSupport(fields, review, compared) != nil {
			return nil, ErrRejected
		}
	}
	base := c
	base.DossierComparisonDigest = ""
	p, err := NewDossierPolicy(base, fields, e.Entailment, s.Semantic, companies...)
	if err != nil {
		return nil, err
	}
	p.dossierReview = review
	if err = p.dossierDigest(); err != nil {
		return nil, err
	}
	p.cfg.DossierComparisonDigest = c.DossierComparisonDigest
	p.comparisonPlan = comparison
	p.comparisonReview = compared
	p.comparisonBaseDigest = p.digest
	if err = p.comparisonDigest(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *Policy) ComparisonProof() (*dossier.CardComparisonPlan, *dossier.CardComparisonReview) {
	if p == nil {
		return nil, nil
	}
	return p.comparisonPlan, p.comparisonReview
}
func DossierComparisonVisible(visible string, fields *dossier.SupportPlan, review *dossier.SupportReview, comparison *dossier.CardComparisonPlan, compared *dossier.CardComparisonReview) (string, error) {
	if comparison.CheckSupport(fields, review, compared) != nil {
		return "", ErrRejected
	}
	extra, err := comparison.VisibleReview(compared)
	if err != nil {
		return "", err
	}
	return visible + "\n" + extra, nil
}
