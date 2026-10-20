package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// reportEvidence 保留原锁序；公司政策必须在同一当前读事务中重建首次批准集合。
func (s *AIReportService) reportEvidence(ctx context.Context, tx finalreport.Tx, v task.Task, ids []int64) ([]sources.Snapshot, string, error) {
	snaps, theme, err := tx.ReportEvidence(ctx, ids, v.SubjectID)
	if err != nil || s.policy.CompanyDigest() == "" {
		return snaps, theme, err
	}
	a, found, err := reportAdmission(ctx, tx, v)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", task.ErrAdmissionUnavailable
	}
	reader, ok := tx.(finalreport.CompanyReader)
	if !ok {
		return nil, "", points.ErrInvariant
	}
	records, err := reader.ReportCompanies(ctx, v.SubjectID)
	if err != nil {
		return nil, "", err
	}
	now, err := tx.Points().Now(ctx)
	if err != nil {
		return nil, "", err
	}
	if !v.Terminal() && !v.Deadline.After(now) {
		return nil, "", task.ErrLease
	}
	draft, err := finalreport.NewCompanyDraft(s.policy, a.Plan, snaps, records, now)
	if err != nil {
		return nil, "", err
	}
	if theme != a.Plan.SubjectName || draft.ContentDigest() != s.policy.CompanyDigest() {
		return nil, "", finalreport.ErrRejected
	}
	return snaps, theme, nil
}
