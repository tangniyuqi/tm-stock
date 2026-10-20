package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// PrepareCompanies 只准备本人原批准材料支持的资料库公司行，不能发布、结算或调用模型。
// 原引文政策不接纳数据行；须显式采用首次绑定/当前重核的公司政策，准备结果本身不是发布许可。
func (s *AIReportService) PrepareCompanies(ctx context.Context, uid, id int64) (*finalreport.CompanyDraft, error) {
	if s == nil || uid <= 0 || id <= 0 {
		return nil, points.ErrInvalid
	}
	if !s.enabled {
		return nil, finalreport.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var draft *finalreport.CompanyDraft
	err := s.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		if v.Terminal() {
			return points.ErrClosed
		}
		if v.CancelRequested {
			return task.ErrCancelled
		}
		if v.Status != task.Queued && v.Status != task.Running {
			return points.ErrInvariant
		}
		if v.AnalysisType != 1 || v.SubjectType != task.Theme {
			return task.ErrStagePolicy
		}
		a, found, err := reportAdmission(ctx, tx, v)
		if err != nil {
			return err
		}
		if !found {
			return task.ErrAdmissionUnavailable
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if v.NotBefore.After(now) || !v.Deadline.After(now) {
			return task.ErrNotReady
		}
		ids := make([]int64, len(a.Plan.Materials))
		for i, m := range a.Plan.Materials {
			ids[i] = m.SnapshotID
		}
		// 先来源、快照与题材，再取资料库公司；不在模型/网络调用期间持有锁。
		snaps, theme, err := tx.ReportEvidence(ctx, ids, v.SubjectID)
		if err != nil {
			return err
		}
		if theme != a.Plan.SubjectName {
			return finalreport.ErrRejected
		}
		reader, ok := tx.(finalreport.CompanyReader)
		if !ok {
			return task.ErrNotReady
		}
		records, err := reader.ReportCompanies(ctx, v.SubjectID)
		if err != nil {
			return err
		}
		now, err = tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if !v.Deadline.After(now) {
			return task.ErrNotReady
		}
		draft, err = finalreport.NewCompanyDraft(s.policy, a.Plan, snaps, records, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return draft, nil
}
