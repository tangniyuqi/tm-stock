package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 构造现有worker接口适配，不启动队列或授予模型调用批准。
type AILocalReportPlanner struct{ admission *AILocalAdmissionControl }

func NewAILocalReportPlanner(admission *AILocalAdmissionControl) (*AILocalReportPlanner, error) {
	if admission == nil {
		return nil, points.ErrInvalid
	}
	return &AILocalReportPlanner{admission}, nil
}
func (p *AILocalReportPlanner) Plan(ctx context.Context, v task.Task) (AIL1ReportPlan, error) {
	if p == nil || p.admission == nil {
		return AIL1ReportPlan{}, points.ErrInvalid
	}
	return p.admission.ExecutionPlan(ctx, v)
}
func (p *AILocalReportPlanner) Recheck(ctx context.Context, v task.Task, plan AIL1ReportPlan) error {
	want, err := p.Plan(ctx, v)
	if err != nil {
		return err
	}
	if plan.Agent.ValidateScope(v) != nil {
		return points.ErrIdempotency
	}
	a, err := want.AdmissionDigest()
	if err != nil {
		return err
	}
	b, err := plan.AdmissionDigest()
	if err != nil {
		return err
	}
	if a != b {
		return points.ErrIdempotency
	}
	return ctx.Err()
}
