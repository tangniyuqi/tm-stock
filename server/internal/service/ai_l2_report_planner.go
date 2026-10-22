package service

import (
	"context"
	"net/url"
	"slices"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AIL2ReportPlan 是同一可信来源的深度准入/执行计划；摘要排除创建后才有的任务号/键/围栏。
type AIL2ReportPlan struct {
	template      AIL1ReportPlan
	price         points.Price
	budget        task.Budget
	approvalRef   string
	qualityRef    string
	hardBudgetRef string
	priceRef      string
	sourceRef     string
	digest        string
}

func NewAIL2ReportPlan(template AIL1ReportPlan, price points.Price, budget task.Budget, approvalRef, qualityRef, hardBudgetRef, priceRef, sourceRef string) (*AIL2ReportPlan, error) {
	if template.AnalysisType != task.DeepAnalysisType || template.Policy.AnalysisType != task.DeepAnalysisType || template.Policy.MinQuotes != task.DeepMinQuotes || template.Model.AnalysisType != task.DeepAnalysisType || template.Agent.SubjectID <= 0 || template.Agent.SubjectType != task.Theme || len(template.Snapshots) < 1 || len(template.Snapshots) > 32 || price.Validate() != nil || price.Points != task.DeepPricePoints || budget.Validate() != nil {
		return nil, points.ErrInvalid
	}
	digest, err := template.AdmissionDigest()
	if err != nil {
		return nil, err
	}
	return &AIL2ReportPlan{template: cloneL1Plan(template), price: price, budget: budget, approvalRef: approvalRef, qualityRef: qualityRef, hardBudgetRef: hardBudgetRef, priceRef: priceRef, sourceRef: sourceRef, digest: digest}, nil
}

func (p *AIL2ReportPlan) materials() []task.AdmissionMaterial {
	out := make([]task.AdmissionMaterial, 0, len(p.template.Snapshots))
	for _, s := range p.template.Snapshots {
		out = append(out, task.AdmissionMaterial{SnapshotID: s.ID, Digest: s.Hash, Domain: urlDomain(s.URL), Category: s.SourceCategory, URL: s.URL, CollectedAt: s.FetchedAt})
	}
	slices.SortFunc(out, func(a, b task.AdmissionMaterial) int {
		if a.SnapshotID < b.SnapshotID {
			return -1
		}
		if a.SnapshotID > b.SnapshotID {
			return 1
		}
		return 0
	})
	return out
}

func (p *AIL2ReportPlan) Admission(scope task.AdmissionScope) (task.AdmissionPlan, error) {
	if scope.Validate() != nil || scope.AnalysisType != task.DeepAnalysisType || scope.SubjectType != task.Theme || scope.SubjectID != p.template.Agent.SubjectID {
		return task.AdmissionPlan{}, points.ErrInvalid
	}
	out := task.AdmissionPlan{Scope: scope, Price: p.price, Budget: p.budget, SubjectName: p.template.ThemeName, ModelName: p.template.Model.Model, FilingNo: p.template.Model.FilingNo, ProfileDigest: p.template.Model.ProfileDigest, ExecutionDigest: p.digest, ApprovalRef: p.approvalRef, QualityRef: p.qualityRef, HardBudgetRef: p.hardBudgetRef, PriceRef: p.priceRef, SourceRef: p.sourceRef, Materials: p.materials()}
	if out.Validate(scope) != nil {
		return task.AdmissionPlan{}, points.ErrInvalid
	}
	return out, nil
}

func (p *AIL2ReportPlan) Execution(v task.Task) (AIL1ReportPlan, error) {
	if v.ID <= 0 || v.UserID <= 0 || v.AnalysisType != task.DeepAnalysisType || v.SubjectType != task.Theme || v.SubjectID != p.template.Agent.SubjectID {
		return AIL1ReportPlan{}, points.ErrInvalid
	}
	q := cloneL1Plan(p.template)
	q.Agent.UserID, q.Agent.TaskID, q.Agent.TaskNo = v.UserID, v.ID, v.No
	q.Agent.OperationKey = ""
	q.Agent.OperationKey = q.Agent.RequestID()
	if _, err := q.AdmissionDigest(); err != nil {
		return AIL1ReportPlan{}, err
	}
	return q, nil
}

func urlDomain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

type AIL2AdmissionControl struct{ plan *AIL2ReportPlan }

func NewAIL2AdmissionControl(plan *AIL2ReportPlan) (*AIL2AdmissionControl, error) {
	if plan == nil {
		return nil, points.ErrInvalid
	}
	return &AIL2AdmissionControl{plan: plan}, nil
}
func (c *AIL2AdmissionControl) Ready(context.Context) error     { return nil }
func (c *AIL2AdmissionControl) ReadyDeep(context.Context) error { return nil }
func (c *AIL2AdmissionControl) Plan(_ context.Context, scope task.AdmissionScope) (task.AdmissionPlan, error) {
	return c.plan.Admission(scope)
}
func (c *AIL2AdmissionControl) Recheck(ctx context.Context, p task.AdmissionPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	want, err := c.plan.Admission(p.Scope)
	if err != nil {
		return err
	}
	a, err := points.RequestHash("ai-deep-admission", want)
	if err != nil {
		return err
	}
	b, err := points.RequestHash("ai-deep-admission", p)
	if err != nil {
		return err
	}
	if a != b {
		return points.ErrInvariant
	}
	return nil
}

type AIL2ExecutionPlanner struct{ plan *AIL2ReportPlan }

func NewAIL2ExecutionPlanner(plan *AIL2ReportPlan) (*AIL2ExecutionPlanner, error) {
	if plan == nil {
		return nil, points.ErrInvalid
	}
	return &AIL2ExecutionPlanner{plan: plan}, nil
}
func (c *AIL2ExecutionPlanner) Plan(_ context.Context, v task.Task) (AIL1ReportPlan, error) {
	return c.plan.Execution(v)
}
func (c *AIL2ExecutionPlanner) Recheck(_ context.Context, v task.Task, p AIL1ReportPlan) error {
	if p.AnalysisType != task.DeepAnalysisType || v.AnalysisType != task.DeepAnalysisType {
		return points.ErrInvalid
	}
	digest, err := p.AdmissionDigest()
	if err != nil || digest != c.plan.digest {
		return points.ErrIdempotency
	}
	return nil
}
