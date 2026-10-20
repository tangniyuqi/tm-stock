package service

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func checkOriginalAdmission(ctx context.Context, tx task.Tx, v task.Task) error {
	a, ok := tx.(task.AdmissionReadTx)
	if !ok {
		return points.ErrInvariant
	}
	_, _, err := a.ReadAdmission(ctx, v)
	return err
}

// 带首次准入的任务只能由完整L1规划核驱动，不能用单Agent入口绕过其模型/资料绑定。
func guardTaskAgentAdmission(ctx context.Context, store task.AdmissionStore, planner agenttask.Planner, v task.Task) error {
	if store == nil {
		return points.ErrInvariant
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var bound bool
	err := store.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
		current, err := lockOwnedTask(ctx, tx, v.UserID, v.ID)
		if err != nil {
			return err
		}
		_, bound, err = tx.ReadAdmission(ctx, current)
		return err
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if bound {
		if _, ok := planner.(interface{ admittedTaskPlan() }); !ok {
			return task.ErrAdmissionUnavailable
		}
	}
	return nil
}
func cloneL1Plan(p AIL1ReportPlan) AIL1ReportPlan {
	p.Policy.AllowedDomains = slices.Clone(p.Policy.AllowedDomains)
	p.Snapshots = slices.Clone(p.Snapshots)
	for i := range p.Snapshots {
		p.Snapshots[i].Sentences = slices.Clone(p.Snapshots[i].Sentences)
		p.Snapshots[i].FetchedAt = p.Snapshots[i].FetchedAt.UTC()
		if p.Snapshots[i].LicenseExpires != nil {
			at := p.Snapshots[i].LicenseExpires.UTC()
			p.Snapshots[i].LicenseExpires = &at
		}
	}
	return p
}

// AdmissionDigest 是完整执行语义的身份，不授予批准；排除创建后才有的任务号/键/围栏。
func (p AIL1ReportPlan) AdmissionDigest() (string, error) {
	if p.Policy.CompanyDigest != p.Companies.ContentDigest() {
		return "", points.ErrIdempotency
	}
	p = cloneL1Plan(p)
	p.Agent.UserID = 0
	p.Agent.TaskID = 0
	p.Agent.TaskNo = ""
	p.Agent.OperationKey = ""
	slices.Sort(p.Policy.AllowedDomains)
	slices.SortFunc(p.Snapshots, func(a, b sources.Snapshot) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	b, err := json.Marshal(struct {
		Version string
		Plan    AIL1ReportPlan
	}{"l1-admission-execution-v1", p})
	if err != nil {
		return "", points.ErrInvalid
	}
	return task.StageDigest(b), nil
}
func checkAdmittedL1(ctx context.Context, store task.AdmissionStore, v task.Task, p AIL1ReportPlan) error {
	if store == nil {
		return points.ErrInvariant
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var record task.AdmissionRecord
	var found bool
	err := store.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
		current, err := lockOwnedTask(ctx, tx, v.UserID, v.ID)
		if err != nil {
			return err
		}
		if current.No != v.No || current.Hash != v.Hash || current.PriceVersion != v.PriceVersion || current.PricePoints != v.PricePoints || current.Budget != v.Budget || current.AnalysisType != v.AnalysisType || current.SubjectType != v.SubjectType || current.SubjectID != v.SubjectID {
			return points.ErrInvariant
		}
		record, found, err = tx.ReadAdmission(ctx, current)
		return err
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// 只有无标记且无记录的既有内部任务可走旧开发核；新会员创建没有此降级路径。
	if !found {
		if p.Policy.CompanyDigest != "" {
			return task.ErrAdmissionUnavailable
		}
		return nil
	}
	digest, err := p.AdmissionDigest()
	if err != nil {
		return err
	}
	a := record.Plan
	if a.ExecutionDigest != digest || a.CompanyDigest != p.Policy.CompanyDigest || a.SubjectName != p.ThemeName || a.ModelName != p.Model.Model || a.FilingNo != p.Model.FilingNo || a.ProfileDigest != p.Model.ProfileDigest || a.ApprovalRef != p.Model.ApprovalRef || a.QualityRef != p.Model.QualityRef || a.HardBudgetRef != p.Model.HardBudgetRef || len(a.Materials) != len(p.Snapshots) {
		return points.ErrIdempotency
	}
	snaps := slices.Clone(p.Snapshots)
	slices.SortFunc(snaps, func(x, y sources.Snapshot) int {
		if x.ID < y.ID {
			return -1
		}
		if x.ID > y.ID {
			return 1
		}
		return 0
	})
	for i, m := range a.Materials {
		s := snaps[i]
		if s.ID != m.SnapshotID || s.Hash != m.Digest || s.SourceCategory != m.Category || s.URL != m.URL || !s.FetchedAt.Equal(m.CollectedAt) {
			return points.ErrIdempotency
		}
	}
	if p.Policy.CompanyDigest != "" {
		reports, ok := store.(finalreport.Store)
		if !ok {
			return points.ErrInvariant
		}
		policy, err := pendingPolicy(p.Policy, p.Companies)
		if err != nil {
			return err
		}
		svc := &AIReportService{store: reports, policy: policy}
		return reports.WithinReportTx(ctx, func(tx finalreport.Tx) error {
			current, err := lockOwnedTask(ctx, tx, v.UserID, v.ID)
			if err != nil {
				return err
			}
			ids := make([]int64, len(a.Materials))
			for i, m := range a.Materials {
				ids[i] = m.SnapshotID
			}
			_, _, err = svc.reportEvidence(ctx, tx, current, ids)
			return err
		})
	}
	return nil
}

type admittedL1Planner struct {
	base  AIL1ReportPlanner
	store task.AdmissionStore
}

func (p admittedL1Planner) Plan(ctx context.Context, v task.Task) (AIL1ReportPlan, error) {
	a, err := p.base.Plan(ctx, v)
	return cloneL1Plan(a), err
}
func (p admittedL1Planner) Recheck(ctx context.Context, v task.Task, a AIL1ReportPlan) error {
	if err := checkAdmittedL1(ctx, p.store, v, a); err != nil {
		return err
	}
	if err := p.base.Recheck(ctx, v, cloneL1Plan(a)); err != nil {
		return err
	}
	return checkAdmittedL1(ctx, p.store, v, a)
}

type admittedSelectionControl struct {
	AIReportSelectionControl
	check func(context.Context, task.Task) error
}

func (c admittedSelectionControl) Recheck(ctx context.Context, v task.Task, p *finalreport.SelectionModelPlan) error {
	if err := c.check(ctx, v); err != nil {
		return err
	}
	if err := c.AIReportSelectionControl.Recheck(ctx, v, p); err != nil {
		return err
	}
	return c.check(ctx, v)
}

type admittedCheckControl struct {
	AIReportCheckControl
	check func(context.Context, task.Task) error
}

func (c admittedCheckControl) Recheck(ctx context.Context, v task.Task, p *finalreport.CheckPlan) error {
	if err := c.check(ctx, v); err != nil {
		return err
	}
	if err := c.AIReportCheckControl.Recheck(ctx, v, p); err != nil {
		return err
	}
	return c.check(ctx, v)
}

var _ reportModelAudit = (admittedSelectionControl{})
var _ reportModelAudit = (admittedCheckControl{})
