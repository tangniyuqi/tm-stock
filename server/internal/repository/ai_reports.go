package repository

import (
	"context"
	"database/sql"
	"slices"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var _ finalreport.Store = (*AITaskRepo)(nil)
var _ finalreport.Tx = (*aiTaskTx)(nil)

func (r *AITaskRepo) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	if fn == nil {
		return points.ErrInvalid
	}
	return r.WithinTaskTx(ctx, func(tx task.Tx) error { return fn(tx.(*aiTaskTx)) })
}

func (t *aiTaskTx) ReportCommit(ctx context.Context, uid, id int64) (finalreport.Commit, error) {
	// 同一任务的读写由上层任务行锁串行；不锁不存在的报告键，避免 RR 下不同任务的空区间死锁。
	var c finalreport.Commit
	var raw string
	err := t.tx.QueryRowContext(ctx, `SELECT task_id,user_id,task_no,step_no,fence,event_seq,terminal_txn_id,operation_key,input_digest,content_digest,policy_digest,status,entailment,semantic,reason,payload,CAST(created_at AS CHAR) FROM tm_ai_report_commits WHERE task_id=? AND user_id=?`, id, uid).Scan(&c.TaskID, &c.UserID, &c.TaskNo, &c.StepNo, &c.Fence, &c.EventSeq, &c.TerminalTxnID, &c.OperationKey, &c.InputDigest, &c.ContentDigest, &c.PolicyDigest, &c.Status, &c.Entailment, &c.Semantic, &c.Reason, &c.Payload, &raw)
	if err != nil {
		return finalreport.Commit{}, pointsDBError("读取终检提交", err)
	}
	c.CreatedAt, err = parsePointsUTC(raw)
	if err != nil {
		return finalreport.Commit{}, err
	}
	if err := c.Validate(); err != nil {
		return finalreport.Commit{}, err
	}
	return c, nil
}

func (t *aiTaskTx) InsertReportCommit(ctx context.Context, c finalreport.Commit) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return pointsDBError("保存终检提交", t.insertReportCommit(ctx, c))
}
func (t *aiTaskTx) insertReportCommit(ctx context.Context, c finalreport.Commit) error {
	var payload any
	if len(c.Payload) > 0 {
		payload = c.Payload
	}
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_report_commits(task_id,user_id,task_no,step_no,fence,event_seq,terminal_txn_id,operation_key,input_digest,content_digest,policy_digest,status,entailment,semantic,reason,payload,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.TaskID, c.UserID, c.TaskNo, c.StepNo, c.Fence, c.EventSeq, c.TerminalTxnID, c.OperationKey, c.InputDigest, c.ContentDigest, c.PolicyDigest, c.Status, c.Entailment, c.Semantic, c.Reason, payload, pointsUTC(c.CreatedAt), pointsUTC(c.CreatedAt))
	return err
}

func (t *aiTaskTx) ReportEvidence(ctx context.Context, ids []int64, themeID int64) ([]sources.Snapshot, string, error) {
	if len(ids) < 1 || len(ids) > finalreport.MaxSnapshots || themeID <= 0 {
		return nil, "", points.ErrInvalid
	}
	for i, id := range ids {
		if id <= 0 || i > 0 && id <= ids[i-1] {
			return nil, "", points.ErrInvalid
		}
	}
	sourceIDs := []int64{}
	for _, id := range ids {
		var sourceID int64
		if err := t.tx.QueryRowContext(ctx, `SELECT source_id FROM tm_ai_source_snapshots WHERE id=?`, id).Scan(&sourceID); err != nil {
			if err == sql.ErrNoRows {
				return nil, "", sources.ErrSourceUnavailable
			}
			return nil, "", pointsDBError("读取报告来源编号", err)
		}
		sourceIDs = append(sourceIDs, sourceID)
	}
	slices.Sort(sourceIDs)
	sourceIDs = slices.Compact(sourceIDs)
	// 与既有导入/审核同序：先来源，后快照与题材。所有来源按编号排序，保持撤回与提交互斥。
	current := make(map[int64]sources.Source, len(sourceIDs))
	for _, id := range sourceIDs {
		source, err := sourceByIDWithMode(ctx, t.tx, id, sourceShared)
		if err != nil {
			return nil, "", err
		}
		current[id] = source
	}
	out := make([]sources.Snapshot, 0, len(ids))
	total := 0
	for _, id := range ids {
		s, err := sourceSnapshotByID(ctx, t.tx, id, true)
		if err != nil {
			return nil, "", err
		}
		total += len(s.Body)
		if len(s.Body) > finalreport.MaxEvidenceBodyBytes || total > finalreport.MaxTotalEvidenceBytes || len(s.Sentences) > 10000 {
			return nil, "", finalreport.ErrRejected
		}
		out = append(out, s)
	}
	var theme string
	if err := t.tx.QueryRowContext(ctx, `SELECT name FROM addon_quant_theme WHERE id=? AND deleted_at IS NULL AND IFNULL(status,1)=1 AND name IS NOT NULL AND name<>'' FOR SHARE`, themeID).Scan(&theme); err != nil {
		return nil, "", pointsDBError("复核报告题材", err)
	}
	now, err := t.Points().Now(ctx)
	if err != nil {
		return nil, "", err
	}
	for _, s := range out {
		source, ok := current[s.SourceID]
		if !ok || !source.HasValidAuthorization(now) || source.Category != s.SourceCategory || source.LicenseTag != s.LicenseTag || source.LicenseRef != s.LicenseRef || !sameOptionalTime(source.LicenseExpires, s.LicenseExpires) {
			return nil, "", sources.ErrUnauthorized
		}
		url, err := sources.ValidateURL(source.Domain, s.URL)
		if err != nil || url != s.URL {
			return nil, "", sources.ErrUnsafeURL
		}
	}
	return out, guard.Normalize(theme), nil
}
