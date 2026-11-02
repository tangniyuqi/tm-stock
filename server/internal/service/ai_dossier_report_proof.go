package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type borrowedSupportRegistry struct{ tx dossier.SupportRegistryTx }

func (s borrowedSupportRegistry) WithinSupport(ctx context.Context, write bool, fn func(dossier.SupportRegistryTx) error) error {
	if ctx == nil || write || s.tx == nil || fn == nil {
		return dossier.ErrInvalidDraft
	}
	return fn(s.tx)
}

func checkDossierReportProof(ctx context.Context, tx task.AdmissionReadTx, v task.Task, a task.AdmissionPlan, p *finalreport.Policy) error {
	plan, review := p.DossierProof()
	if a.ReportProfile == "" && plan == nil {
		return nil
	}
	if a.ReportProfile != task.DossierReportProfile && a.ReportProfile != task.DossierComparisonReportProfile || !plan.Valid() || !plan.MatchesTask(v) || plan.StepNo() != 4 {
		return points.ErrIdempotency
	}
	registry, ok := tx.(interface {
		DossierRegistry() dossier.SupportRegistryTx
	})
	if !ok {
		return points.ErrInvariant
	}
	reader := &AIDossierSupportRegistryService{store: borrowedSupportRegistry{tx: registry.DossierRegistry()}}
	current, err := reader.loadForTask(ctx, v, a)
	if err != nil {
		return err
	}
	if current.InputDigest() != plan.InputDigest() || current.PolicyDigest() != plan.PolicyDigest() {
		return points.ErrIdempotency
	}
	comparison, compared := p.ComparisonProof()
	if a.ReportProfile == task.DossierComparisonReportProfile {
		if !comparison.Valid() || !comparison.MatchesTask(v) || !comparison.MatchesAdmission(a) {
			return points.ErrIdempotency
		}
		actual, err := reader.loadComparisonForTask(ctx, v, a)
		if err != nil {
			return err
		}
		if actual.InputDigest() != comparison.InputDigest() || actual.PolicyDigest() != comparison.PolicyDigest() {
			return points.ErrIdempotency
		}
	} else if comparison != nil || compared != nil {
		return points.ErrIdempotency
	}
	// 蕴含明确失败可提前结束；缺字段回执的策略本身不能得到成功判定。
	if review == nil {
		return nil
	}
	stages, ok := tx.(task.StageTx)
	if !ok {
		return points.ErrInvariant
	}
	actual, err := dossierCapturedReview(ctx, stages, v, plan)
	if err != nil {
		return err
	}
	if actual.PayloadDigest() != review.PayloadDigest() {
		return points.ErrInvariant
	}
	if compared != nil {
		actualComparison, err := dossierCapturedComparison(ctx, stages, v, comparison)
		if err != nil {
			return err
		}
		if actualComparison.PayloadDigest() != compared.PayloadDigest() || comparison.CheckSupport(current, actual, actualComparison) != nil {
			return points.ErrInvariant
		}
	}
	return nil
}
