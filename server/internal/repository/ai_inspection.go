package repository

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
)

type AIInspectionRepo struct{ db *sql.DB }

func NewAIInspectionRepo(db *sql.DB) *AIInspectionRepo { return &AIInspectionRepo{db: db} }

var _ candidate.InspectionStore = (*AIInspectionRepo)(nil)

type inspectionQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (r *AIInspectionRepo) WithinInspectionTx(ctx context.Context, action func(candidate.InspectionTx) error) error {
	if r == nil || r.db == nil || action == nil {
		return candidate.ErrInvalidInspection
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启复核事务失败: %w", err)
	}
	// 成功提交后回滚是空操作，其他路径确保释放连接和行锁。
	defer func() { _ = tx.Rollback() }()
	if err := action(&aiInspectionTx{aiReviewTx: &aiReviewTx{tx: tx}}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交复核事务失败: %w", err)
	}
	return nil
}

type aiInspectionTx struct{ *aiReviewTx }

const inspectionBatchSelect = `SELECT id,request_id,actor_id,after_review_id,window_size,sample_size,population_count,eligible_count,next_review_id,population_digest,population_review_ids,DATE_FORMAT(created_at,'` + databaseUTCSelect + `') FROM tm_ai_inspection_batches`
const inspectionItemSelect = `SELECT id,batch_id,review_id,candidate_id FROM tm_ai_inspection_items`
const inspectionEventSelect = `SELECT id,request_id,item_id,evidence_digest,actor_id,decision,reason,mapping_id,DATE_FORMAT(created_at,'` + databaseUTCSelect + `') FROM tm_ai_inspection_events`

func (r *AIInspectionRepo) BatchByRequest(ctx context.Context, id string) (candidate.InspectionBatch, bool, error) {
	return readInspectionBatch(ctx, r.db, id)
}
func (t *aiInspectionTx) BatchByRequest(ctx context.Context, id string) (candidate.InspectionBatch, bool, error) {
	return readInspectionBatch(ctx, t.tx, id)
}
func readInspectionBatch(ctx context.Context, q inspectionQuerier, id string) (candidate.InspectionBatch, bool, error) {
	b, found, err := scanInspectionBatch(q.QueryRowContext(ctx, inspectionBatchSelect+` WHERE request_id=?`, id))
	if err != nil || !found {
		return b, found, err
	}
	rows, err := q.QueryContext(ctx, inspectionItemSelect+` WHERE batch_id=? ORDER BY id`, b.ID)
	if err != nil {
		return b, false, fmt.Errorf("读取复核批次条目失败: %w", err)
	}
	defer rows.Close()
	b.Items = make([]candidate.InspectionSelection, 0)
	seen := map[int64]bool{}
	population := map[int64]bool{}
	for _, id := range b.PopulationReviewIDs {
		population[id] = true
	}
	for rows.Next() {
		var item candidate.InspectionSelection
		if err := rows.Scan(&item.ID, &item.BatchID, &item.ReviewID, &item.CandidateID); err != nil {
			return b, false, err
		}
		if item.ID <= 0 || item.BatchID != b.ID || item.CandidateID <= 0 || !population[item.ReviewID] || seen[item.ReviewID] {
			return b, false, candidate.ErrInvalidInspection
		}
		seen[item.ReviewID] = true
		b.Items = append(b.Items, item)
	}
	if err := rows.Err(); err != nil {
		return b, false, err
	}
	if len(b.Items) != min(b.Request.SampleSize, b.EligibleCount) {
		return b, false, candidate.ErrInvalidInspection
	}
	return b, true, nil
}

func scanInspectionBatch(row candidateScanner) (candidate.InspectionBatch, bool, error) {
	var b candidate.InspectionBatch
	var created string
	var ids []byte
	if err := row.Scan(&b.ID, &b.Request.RequestID, &b.ActorID, &b.Request.AfterReviewID, &b.Request.WindowSize, &b.Request.SampleSize, &b.PopulationCount, &b.EligibleCount, &b.NextReviewID, &b.PopulationDigest, &ids, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return b, false, nil
		}
		return b, false, err
	}
	var err error
	b.CreatedAt, err = time.Parse(databaseUTCLayout, created)
	if err != nil {
		return b, false, err
	}
	digest, err := hex.DecodeString(b.PopulationDigest)
	if err != nil || len(digest) != 32 || b.ID <= 0 || b.ActorID <= 0 || b.Request.Validate() != nil || b.PopulationCount < 0 || b.PopulationCount > b.Request.WindowSize || b.EligibleCount < 0 || b.EligibleCount > b.PopulationCount || b.NextReviewID < b.Request.AfterReviewID || (b.PopulationCount == 0 && b.NextReviewID != b.Request.AfterReviewID) {
		return b, false, candidate.ErrInvalidInspection
	}
	if json.Unmarshal(ids, &b.PopulationReviewIDs) != nil || b.PopulationReviewIDs == nil || len(b.PopulationReviewIDs) != b.PopulationCount {
		return b, false, candidate.ErrInvalidInspection
	}
	last := b.Request.AfterReviewID
	for _, id := range b.PopulationReviewIDs {
		if id <= last || id > b.NextReviewID {
			return b, false, candidate.ErrInvalidInspection
		}
		last = id
	}
	if last != b.NextReviewID {
		return b, false, candidate.ErrInvalidInspection
	}
	return b, true, nil
}

func (t *aiInspectionTx) SamplingWindow(ctx context.Context, afterID int64, limit int) ([]candidate.AuditEvent, error) {
	if afterID < 0 || limit < 1 || limit > 2000 {
		return nil, candidate.ErrInvalidInspection
	}
	rows, err := t.tx.QueryContext(ctx, auditSelect+` WHERE id>? AND decision='approve' ORDER BY id LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("读取复核抽样窗口失败: %w", err)
	}
	defer rows.Close()
	out := make([]candidate.AuditEvent, 0)
	for rows.Next() {
		e, _, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (t *aiInspectionTx) InsertBatch(ctx context.Context, b candidate.InspectionBatch, selected []candidate.AuditEvent) (candidate.InspectionBatch, error) {
	ids, err := json.Marshal(b.PopulationReviewIDs)
	if err != nil {
		return b, err
	}
	result, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_inspection_batches(request_id,actor_id,after_review_id,window_size,sample_size,population_count,eligible_count,next_review_id,population_digest,population_review_ids,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, b.Request.RequestID, b.ActorID, b.Request.AfterReviewID, b.Request.WindowSize, b.Request.SampleSize, b.PopulationCount, b.EligibleCount, b.NextReviewID, b.PopulationDigest, string(ids))
	if err != nil {
		return b, reviewWriteError("保存复核批次失败", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return b, err
	}
	for _, e := range selected {
		if _, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_inspection_items(batch_id,review_id,candidate_id,created_at,updated_at) VALUES(?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, e.ID, e.Request.CandidateID); err != nil {
			return b, reviewWriteError("保存复核分派失败", err)
		}
	}
	out, found, err := t.BatchByRequest(ctx, b.Request.RequestID)
	if err != nil {
		return b, err
	}
	if !found {
		return b, candidate.ErrInspectionNotFound
	}
	return out, nil
}

func (r *AIInspectionRepo) ItemByID(ctx context.Context, id int64) (candidate.InspectionItem, error) {
	return readInspectionItem(ctx, r.db, id, false, nil)
}
func (t *aiInspectionTx) LockItem(ctx context.Context, id int64) (candidate.InspectionItem, error) {
	return readInspectionItem(ctx, t.tx, id, true, t.aiReviewTx)
}
func readInspectionItem(ctx context.Context, q inspectionQuerier, id int64, lock bool, review *aiReviewTx) (candidate.InspectionItem, error) {
	var item candidate.InspectionItem
	err := q.QueryRowContext(ctx, inspectionItemSelect+` WHERE id=?`, id).Scan(&item.Selection.ID, &item.Selection.BatchID, &item.Selection.ReviewID, &item.Selection.CandidateID)
	if errors.Is(err, sql.ErrNoRows) {
		return item, candidate.ErrInspectionNotFound
	}
	if err != nil {
		return item, err
	}
	// 与首次审核相同锁顺序：来源→候选→复核条目→题材→归属。
	if lock {
		record, err := review.LockCandidate(ctx, item.Selection.CandidateID)
		if err != nil {
			return item, err
		}
		item.Candidate = record.Candidate
		var lockedID int64
		if err := q.QueryRowContext(ctx, `SELECT id FROM tm_ai_inspection_items WHERE id=? FOR UPDATE`, id).Scan(&lockedID); err != nil {
			return item, err
		}
	} else {
		item.Candidate, err = scanCandidate(q.QueryRowContext(ctx, candidateSelect+` WHERE id=?`, item.Selection.CandidateID))
		if err != nil {
			return item, err
		}
	}
	var populationJSON []byte
	if err := q.QueryRowContext(ctx, `SELECT actor_id,population_review_ids FROM tm_ai_inspection_batches WHERE id=?`, item.Selection.BatchID).Scan(&item.ActorID, &populationJSON); err != nil {
		return item, err
	}
	var population []int64
	if json.Unmarshal(populationJSON, &population) != nil || len(population) > 2000 {
		return item, candidate.ErrInvalidInspection
	}
	selected := false
	for _, reviewID := range population {
		if reviewID == item.Selection.ReviewID {
			selected = true
			break
		}
	}
	if !selected {
		return item, candidate.ErrInvalidInspection
	}
	original, found, err := scanAudit(q.QueryRowContext(ctx, auditSelect+` WHERE id=?`, item.Selection.ReviewID))
	if err != nil {
		return item, err
	}
	if !found {
		return item, candidate.ErrInspectionNotFound
	}
	item.Original = original
	event, hasResult, err := scanInspectionEvent(q.QueryRowContext(ctx, inspectionEventSelect+` WHERE item_id=?`, id))
	if err != nil {
		return item, err
	}
	if hasResult {
		if event.ActorID != item.ActorID || event.MappingID != original.MappingID {
			return item, candidate.ErrInvalidInspection
		}
		item.Result = &event
	}
	digest, err := candidate.InspectionDigest(item)
	if err != nil {
		return item, err
	}
	if item.Result != nil && item.Result.Request.EvidenceDigest != digest {
		return item, candidate.ErrEvidenceChanged
	}
	return item, nil
}

func (t *aiInspectionTx) EventByRequest(ctx context.Context, id string) (candidate.InspectionEvent, bool, error) {
	return scanInspectionEvent(t.tx.QueryRowContext(ctx, inspectionEventSelect+` WHERE request_id=?`, id))
}
func (t *aiInspectionTx) EventByItem(ctx context.Context, id int64) (candidate.InspectionEvent, bool, error) {
	return scanInspectionEvent(t.tx.QueryRowContext(ctx, inspectionEventSelect+` WHERE item_id=?`, id))
}
func scanInspectionEvent(row candidateScanner) (candidate.InspectionEvent, bool, error) {
	var e candidate.InspectionEvent
	var created string
	if err := row.Scan(&e.ID, &e.Request.RequestID, &e.Request.ItemID, &e.Request.EvidenceDigest, &e.ActorID, &e.Request.Decision, &e.Request.Reason, &e.MappingID, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e, false, nil
		}
		return e, false, err
	}
	var err error
	e.CreatedAt, err = time.Parse(databaseUTCLayout, created)
	if err != nil {
		return e, false, err
	}
	if e.ID <= 0 || e.ActorID <= 0 || e.MappingID <= 0 || e.Request.Validate() != nil {
		return e, false, candidate.ErrInvalidInspection
	}
	return e, true, nil
}

func (t *aiInspectionTx) LockMapping(ctx context.Context, id, themeID int64) (candidate.InspectionMapping, error) {
	var m candidate.InspectionMapping
	var lockedTheme int64
	if err := t.tx.QueryRowContext(ctx, `SELECT id FROM addon_quant_theme WHERE id=? FOR UPDATE`, themeID).Scan(&lockedTheme); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return m, candidate.ErrEvidenceChanged
		}
		return m, err
	}
	var collected string
	var deleted sql.NullString
	err := t.tx.QueryRowContext(ctx, `SELECT id,theme_id,stock_id,ts_code,source_type,source_excerpt,excerpt_trimmed,source_url,DATE_FORMAT(collected_at,'`+databaseUTCSelect+`'),audit_status,status,DATE_FORMAT(deleted_at,'`+databaseUTCSelect+`') FROM addon_quant_theme_stock WHERE id=? FOR UPDATE`, id).Scan(&m.ID, &m.ThemeID, &m.StockID, &m.TsCode, &m.SourceType, &m.SourceExcerpt, &m.ExcerptTrimmed, &m.SourceURL, &collected, &m.AuditStatus, &m.Status, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return m, candidate.ErrEvidenceChanged
	}
	if err != nil {
		return m, fmt.Errorf("锁定复核归属失败: %w", err)
	}
	m.CollectedAt, err = time.Parse(databaseUTCLayout, collected)
	if err != nil {
		return m, err
	}
	m.Deleted = deleted.Valid
	return m, nil
}

func (t *aiInspectionTx) Quarantine(ctx context.Context, id, actorID int64) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE addon_quant_theme_stock SET status=0,updated_by=?,updated_at=GREATEST(UTC_TIMESTAMP(3),updated_at + INTERVAL 1000 MICROSECOND) WHERE id=? AND deleted_at IS NULL AND audit_status=2`, actorID, id)
	if err != nil {
		return fmt.Errorf("暂停问题归属失败: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return candidate.ErrConflict
	}
	return nil
}

func (t *aiInspectionTx) AppendInspection(ctx context.Context, request candidate.InspectionRequest, actorID, mappingID int64) (candidate.InspectionEvent, error) {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_inspection_events(request_id,item_id,evidence_digest,actor_id,decision,reason,mapping_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, request.RequestID, request.ItemID, request.EvidenceDigest, actorID, request.Decision, request.Reason, mappingID)
	if err != nil {
		return candidate.InspectionEvent{}, reviewWriteError("追加复核事件失败", err)
	}
	e, found, err := t.EventByRequest(ctx, request.RequestID)
	if err != nil {
		return e, err
	}
	if !found {
		return e, candidate.ErrInspectionNotFound
	}
	return e, nil
}

func (r *AIInspectionRepo) ListIssues(ctx context.Context, afterID int64, limit int) ([]candidate.InspectionEvent, error) {
	if afterID < 0 || limit < 1 || limit > 200 {
		return nil, candidate.ErrInvalidInspection
	}
	rows, err := r.db.QueryContext(ctx, inspectionEventSelect+` WHERE decision='issue' AND id>? ORDER BY id LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("读取复核问题历史失败: %w", err)
	}
	defer rows.Close()
	out := make([]candidate.InspectionEvent, 0)
	for rows.Next() {
		e, _, err := scanInspectionEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
