package service

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func reportAdmission(ctx context.Context, tx task.AdmissionReadTx, v task.Task) (task.AdmissionRecord, bool, error) {
	record, found, err := tx.ReadAdmission(ctx, v)
	if err != nil {
		return task.AdmissionRecord{}, false, err
	}
	if found {
		if err := record.Bind(v); err != nil {
			return task.AdmissionRecord{}, false, err
		}
	}
	return record, found, nil
}
func checkReportAdmissionScope(ctx context.Context, tx task.AdmissionReadTx, v task.Task, p *finalreport.Policy, raw []byte) error {
	a, found, err := reportAdmission(ctx, tx, v)
	if err != nil || !found {
		return err
	}
	proposal, err := p.Decode(string(raw))
	// 结构拒绝仍沿原终检失败/释放路径；不能用这份正文交付。
	if err != nil {
		return nil
	}
	if !p.MatchesAdmission(a.Plan, proposal) {
		return points.ErrIdempotency
	}
	return nil
}
func checkSelectionAdmissionScope(ctx context.Context, tx task.AdmissionReadTx, v task.Task, s *finalreport.SelectionStage) error {
	a, found, err := reportAdmission(ctx, tx, v)
	if err != nil || !found {
		return err
	}
	if !s.MatchesAdmission(a.Plan) {
		return points.ErrIdempotency
	}
	return nil
}
