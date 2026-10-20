package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AIReviewRepo 与抽取仓储分离，只有审核事务能向归属表写入。
type AIReviewRepo struct{ db *sql.DB }

func NewAIReviewRepo(db *sql.DB) *AIReviewRepo { return &AIReviewRepo{db: db} }

var _ candidate.ReviewStore = (*AIReviewRepo)(nil)

func (r *AIReviewRepo) WithinReviewTx(ctx context.Context, action func(candidate.ReviewTx) error) error {
	if r == nil || r.db == nil || action == nil {
		return fmt.Errorf("审核仓储或事务操作未初始化")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启审核事务失败: %w", err)
	}
	// 提交后 Rollback 返回 ErrTxDone；失败路径回滚由 database/sql 保证连接不复用未完成事务。
	defer func() { _ = tx.Rollback() }()
	if err := action(&aiReviewTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交审核事务失败: %w", err)
	}
	return nil
}

func (r *AIReviewRepo) ListReviewQueue(ctx context.Context, afterID int64, limit int) ([]candidate.ReviewRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("审核仓储未初始化")
	}
	if afterID < 0 || limit < 1 || limit > 200 {
		return nil, candidate.ErrInvalidReview
	}
	rows, err := r.db.QueryContext(ctx, candidateSelect+` WHERE id > ? AND NOT EXISTS
	  (SELECT 1 FROM tm_ai_review_audits a WHERE a.candidate_id = tm_ai_extraction_candidates.id)
	  ORDER BY id LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询待审队列失败: %w", err)
	}
	defer rows.Close()
	out := make([]candidate.ReviewRecord, 0)
	for rows.Next() {
		item, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("读取待审队列失败: %w", err)
		}
		out = append(out, candidate.ReviewRecord{Candidate: item, Status: candidate.Pending})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历待审队列失败: %w", err)
	}
	return out, nil
}

type aiReviewTx struct{ tx *sql.Tx }

func (t *aiReviewTx) LockCandidate(ctx context.Context, id int64) (candidate.ReviewRecord, error) {
	// 与入队相同：先锁来源快照链，后锁候选；停用授权不妨碍驳回。
	var sourceID int64
	err := t.tx.QueryRowContext(ctx, `SELECT src.id FROM tm_ai_sources src
	  JOIN tm_ai_source_snapshots s ON s.source_id = src.id
	  WHERE s.id = (SELECT snapshot_id FROM tm_ai_extraction_candidates WHERE id = ?) FOR UPDATE`, id).Scan(&sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return candidate.ReviewRecord{}, candidate.ErrNotFound
	}
	if err != nil {
		return candidate.ReviewRecord{}, fmt.Errorf("锁定审核来源失败: %w", err)
	}
	item, err := scanCandidate(t.tx.QueryRowContext(ctx, candidateSelect+` WHERE id = ? FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return candidate.ReviewRecord{}, candidate.ErrNotFound
	}
	if err != nil {
		return candidate.ReviewRecord{}, fmt.Errorf("锁定审核候选失败: %w", err)
	}
	record := candidate.ReviewRecord{Candidate: item, Status: candidate.Pending}
	event, found, err := scanAudit(t.tx.QueryRowContext(ctx, auditSelect+` WHERE candidate_id = ?`, id))
	if err != nil {
		return candidate.ReviewRecord{}, err
	}
	if found {
		record.Status, record.MappingID = event.Request.Status(), event.MappingID
		record.ReviewedBy, record.ReviewedAt, record.RejectReason = event.ActorID, &event.CreatedAt, event.Request.Reason
	}
	return record, nil
}

func (t *aiReviewTx) AuditByRequest(ctx context.Context, id string) (candidate.AuditEvent, bool, error) {
	return scanAudit(t.tx.QueryRowContext(ctx, auditSelect+` WHERE request_id = ?`, id))
}

func (t *aiReviewTx) ApprovalSnapshot(ctx context.Context, item candidate.Candidate) (sources.Snapshot, error) {
	return lockAndLoadSnapshotForCandidate(ctx, t.tx, item)
}

func (t *aiReviewTx) LockCatalog(ctx context.Context, item candidate.Candidate) (candidate.ReviewCatalog, error) {
	var result candidate.ReviewCatalog
	err := t.tx.QueryRowContext(ctx, `SELECT IFNULL(name, '') FROM addon_quant_theme
	  WHERE id = ? AND deleted_at IS NULL AND IFNULL(status, 1) = 1 FOR UPDATE`, item.ThemeID).Scan(&result.ThemeName)
	if errors.Is(err, sql.ErrNoRows) {
		return result, candidate.ErrEvidenceChanged
	}
	if err != nil {
		return result, fmt.Errorf("锁定审核题材失败: %w", err)
	}
	err = t.tx.QueryRowContext(ctx, `SELECT IFNULL(name, ''), IFNULL(ts_code, '') FROM addon_quant_base_stock
	  WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, item.StockID).Scan(&result.StockName, &result.TsCode)
	if errors.Is(err, sql.ErrNoRows) {
		return result, candidate.ErrEvidenceChanged
	}
	if err != nil {
		return result, fmt.Errorf("锁定审核股票失败: %w", err)
	}
	return result, nil
}

func (t *aiReviewTx) InsertMapping(ctx context.Context, item candidate.Candidate, actorID int64) (int64, error) {
	result, err := t.tx.ExecContext(ctx, `INSERT INTO addon_quant_theme_stock
	  (theme_id, stock_id, ts_code, source_type, source_excerpt, excerpt_trimmed, source_url, collected_at,
	   audit_status, audit_by, audit_at, status, created_at, updated_at, created_by, updated_by)
	  VALUES (?, ?, ?, ?, ?, ?, ?, ?, 2, ?, UTC_TIMESTAMP(3), 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3), ?, ?)`,
		item.ThemeID, item.StockID, item.TsCode, item.SourceType, item.SourceExcerpt, item.ExcerptTrimmed, item.SourceURL,
		item.CollectedAt.UTC().Format(databaseUTCLayout), actorID, actorID, actorID)
	if err != nil {
		return 0, reviewWriteError("写入审核归属失败", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("读取审核归属编号失败: %w", err)
	}
	return id, nil
}

func (t *aiReviewTx) AppendAudit(ctx context.Context, request candidate.ReviewRequest, actorID, mappingID int64) (candidate.AuditEvent, error) {
	var mapping any
	if mappingID > 0 {
		mapping = mappingID
	}
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_review_audits
	  (request_id, candidate_id, evidence_digest, actor_id, decision, reason, mapping_id, created_at)
	  VALUES (?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(3))`, request.RequestID, request.CandidateID, request.EvidenceDigest,
		actorID, request.Decision, request.Reason, mapping)
	if err != nil {
		return candidate.AuditEvent{}, reviewWriteError("写入审核审计事件失败", err)
	}
	event, found, err := t.AuditByRequest(ctx, request.RequestID)
	if err != nil {
		return candidate.AuditEvent{}, err
	}
	if !found {
		return candidate.AuditEvent{}, fmt.Errorf("新写入的审核审计事件缺失")
	}
	return event, nil
}

func (t *aiReviewTx) RefreshThemeCount(ctx context.Context, themeID int64) error {
	// 共库最小测试形态可能没有此列；生产形态有列时必须同事务刷新，不能留下旧计数。
	var exists int
	err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
	  WHERE table_schema = DATABASE() AND table_name = 'addon_quant_theme' AND column_name = 'stock_count'`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("检查题材计数列失败: %w", err)
	}
	if exists == 0 {
		return nil
	}
	_, err = t.tx.ExecContext(ctx, `UPDATE addon_quant_theme SET stock_count = (
	  SELECT COUNT(*) FROM addon_quant_theme_stock
	  WHERE theme_id = ? AND deleted_at IS NULL AND audit_status = 2 AND status = 1) WHERE id = ?`, themeID, themeID)
	if err != nil {
		return fmt.Errorf("刷新题材可见股票数失败: %w", err)
	}
	return nil
}

const auditSelect = `SELECT id, request_id, candidate_id, evidence_digest, actor_id, decision, reason, mapping_id,
  DATE_FORMAT(created_at, '` + databaseUTCSelect + `') FROM tm_ai_review_audits`

func scanAudit(row candidateScanner) (candidate.AuditEvent, bool, error) {
	var event candidate.AuditEvent
	var mapping sql.NullInt64
	var created string
	err := row.Scan(&event.ID, &event.Request.RequestID, &event.Request.CandidateID, &event.Request.EvidenceDigest,
		&event.ActorID, &event.Request.Decision, &event.Request.Reason, &mapping, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return candidate.AuditEvent{}, false, nil
	}
	if err != nil {
		return candidate.AuditEvent{}, false, fmt.Errorf("读取审核审计事件失败: %w", err)
	}
	event.MappingID = mapping.Int64
	event.CreatedAt, err = time.Parse(databaseUTCLayout, created)
	if err != nil {
		return candidate.AuditEvent{}, false, fmt.Errorf("解析审核审计时点失败: %w", err)
	}
	if err := event.Request.Validate(); err != nil {
		return candidate.AuditEvent{}, false, fmt.Errorf("审核审计事件损坏: %w", err)
	}
	if event.ID <= 0 || event.ActorID <= 0 ||
		(event.Request.Status() == candidate.Approved && (!mapping.Valid || event.MappingID <= 0)) ||
		(event.Request.Status() == candidate.Rejected && mapping.Valid) {
		return candidate.AuditEvent{}, false, fmt.Errorf("审核审计身份或归属编号损坏")
	}
	return event, true, nil
}

func reviewWriteError(where string, err error) error {
	var mysqlErr *mysqldrv.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return candidate.ErrConflict
	}
	return fmt.Errorf("%s: %w", where, err)
}
