package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// 完整编排/模型/质量/供应商硬限额仍交原准入控制面亲自核验，无默认放行。
type AIDossierSupportExecutionControl struct {
	registry *AIDossierSupportRegistryService
	tasks    task.AdmissionStore
	full     AITaskAdmissionControl
	audit    *AIReportCallAuditService
}

func NewAIDossierSupportExecutionControl(registry *AIDossierSupportRegistryService, tasks task.AdmissionStore, full AITaskAdmissionControl, audit *AIReportCallAuditService) (*AIDossierSupportExecutionControl, error) {
	if registry == nil || registry.store == nil || tasks == nil || full == nil || audit == nil || audit.store == nil {
		return nil, points.ErrInvalid
	}
	return &AIDossierSupportExecutionControl{registry: registry, tasks: tasks, full: full, audit: audit}, nil
}

var _ AIDossierSupportControl = (*AIDossierSupportExecutionControl)(nil)

func (c *AIDossierSupportExecutionControl) ExecutionPlan(parent context.Context, uid, id int64) (*dossier.SupportPlan, error) {
	if c == nil || c.tasks == nil || c.registry == nil || c.full == nil || parent == nil || !conversation.ID(uid) || !conversation.ID(id) {
		return nil, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	var current task.Task
	var admitted task.AdmissionRecord
	err := c.tasks.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
		var err error
		current, err = lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		var found bool
		admitted, found, err = reportAdmission(ctx, tx, current)
		if err != nil {
			return err
		}
		if !found || admitted.Plan.DossierSupportDigest == "" {
			return task.ErrAdmissionUnavailable
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = c.full.Recheck(ctx, admitted.Plan); err != nil {
		return nil, err
	}
	p, err := c.registry.loadForTask(ctx, current, admitted.Plan)
	if err != nil {
		return nil, err
	}
	// 重建期间完整控制面也可能撤回或替换；返回前再次核查。
	if err = c.full.Recheck(ctx, admitted.Plan); err != nil {
		return nil, err
	}
	return p, nil
}
func (c *AIDossierSupportExecutionControl) Recheck(ctx context.Context, v task.Task, p *dossier.SupportPlan) error {
	if !p.Valid() || !p.MatchesTask(v) {
		return points.ErrInvalid
	}
	current, err := c.ExecutionPlan(ctx, v.UserID, v.ID)
	if err != nil {
		return err
	}
	if current.InputDigest() != p.InputDigest() || current.PolicyDigest() != p.PolicyDigest() {
		return points.ErrIdempotency
	}
	return ctx.Err()
}
func (c *AIDossierSupportExecutionControl) Start(ctx context.Context, r task.StageRequest, p provider.Request) error {
	if c == nil {
		return points.ErrInvalid
	}
	return c.audit.Start(ctx, r, p)
}
func (c *AIDossierSupportExecutionControl) Finish(ctx context.Context, r task.StageRequest, p provider.Response, e error) error {
	if c == nil {
		return points.ErrInvalid
	}
	return c.audit.Finish(ctx, r, p, e)
}
