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

// 两种计划共用原准入控制面和原持久审计；不存在本地默认放行实现。
type AIDossierComparisonExecutionControl struct {
	base *AIDossierSupportExecutionControl
}

var _ AIDossierComparisonControl = (*AIDossierComparisonExecutionControl)(nil)

func NewAIDossierComparisonExecutionControl(registry *AIDossierSupportRegistryService, tasks task.AdmissionStore, full AITaskAdmissionControl, audit *AIReportCallAuditService) (*AIDossierComparisonExecutionControl, error) {
	base, err := NewAIDossierSupportExecutionControl(registry, tasks, full, audit)
	if err != nil {
		return nil, err
	}
	return &AIDossierComparisonExecutionControl{base}, nil
}
func (c *AIDossierComparisonExecutionControl) ExecutionPlan(parent context.Context, uid, id int64) (*dossier.CardComparisonPlan, error) {
	if c == nil || c.base == nil || parent == nil || !conversation.ID(uid) || !conversation.ID(id) {
		return nil, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	var current task.Task
	var admitted task.AdmissionRecord
	err := c.base.tasks.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
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
		if !found || admitted.Plan.ReportProfile != task.DossierComparisonReportProfile {
			return task.ErrAdmissionUnavailable
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = c.base.full.Recheck(ctx, admitted.Plan); err != nil {
		return nil, err
	}
	var out *dossier.CardComparisonPlan
	err = c.base.registry.store.WithinSupport(ctx, false, func(tx dossier.SupportRegistryTx) error {
		if tx == nil || tx.Dossiers() == nil {
			return dossier.ErrInvalidDraft
		}
		reader := &AIDossierSupportRegistryService{store: borrowedSupportRegistry{tx: tx}}
		fields, err := reader.loadForTask(ctx, current, admitted.Plan)
		if err != nil {
			return err
		}
		out, err = reader.loadComparisonForTask(ctx, current, admitted.Plan)
		if err != nil {
			return err
		}
		if fields.DraftDigest() != out.DraftDigest() {
			return points.ErrIdempotency
		}
		// 同一事务锁住两个用途；结束时的数据库时钟不能被较早的字段检查代替。
		uses := []dossier.Approval{}
		for _, component := range []string{admitted.Plan.DossierSupportDigest, admitted.Plan.DossierComparisonDigest} {
			use, err := tx.SupportUse(ctx, component)
			if err != nil {
				return err
			}
			if use.Digest != component || !use.Enabled || !conversation.Text(use.Reference, 128) {
				return dossier.ErrNotFound
			}
			uses = append(uses, use)
		}
		now, err := tx.Dossiers().Now(ctx)
		if err != nil {
			return err
		}
		for _, use := range uses {
			if !use.Until.After(now) {
				return dossier.ErrNotFound
			}
		}
		return ctx.Err()
	})
	if err != nil {
		return nil, err
	}
	if err = c.base.full.Recheck(ctx, admitted.Plan); err != nil {
		return nil, err
	}
	return out, nil
}
func (c *AIDossierComparisonExecutionControl) Recheck(ctx context.Context, v task.Task, p *dossier.CardComparisonPlan) error {
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
func (c *AIDossierComparisonExecutionControl) Start(ctx context.Context, r task.StageRequest, p provider.Request) error {
	if c == nil || c.base == nil {
		return points.ErrInvalid
	}
	return c.base.audit.Start(ctx, r, p)
}
func (c *AIDossierComparisonExecutionControl) Finish(ctx context.Context, r task.StageRequest, p provider.Response, e error) error {
	if c == nil || c.base == nil {
		return points.ErrInvalid
	}
	return c.base.audit.Finish(ctx, r, p, e)
}
