package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var _ sources.MutationStore = (*AISourceRepo)(nil)

func (r *AISourceRepo) WithinSourceTx(ctx context.Context, action func(sources.MutationTx) error) error {
	if r == nil || r.db == nil || action == nil {
		return sources.ErrInvalidSourceAudit
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启来源操作事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后 ErrTxDone；异常时释放未提交的事务。
	if err := action(&aiSourceTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交来源操作与审计失败: %w", err)
	}
	return nil
}

type aiSourceTx struct{ tx *sql.Tx }

func (t *aiSourceTx) SourceByID(ctx context.Context, id int64) (sources.Source, error) {
	return sourceByID(ctx, t.tx, id, true)
}
func (t *aiSourceTx) RegisterSource(ctx context.Context, source sources.Source) (sources.Source, error) {
	return registerSource(ctx, t.tx, source)
}
func (t *aiSourceTx) SetSourceEnabled(ctx context.Context, id int64, enabled bool) error {
	return setSourceEnabled(ctx, t.tx, id, enabled)
}
func (t *aiSourceTx) SaveSnapshot(ctx context.Context, snapshot sources.Snapshot) (sources.Snapshot, error) {
	return saveSnapshotTx(ctx, t.tx, snapshot)
}
func (t *aiSourceTx) AppendSourceAudit(ctx context.Context, event sources.SourceAudit) error {
	if err := event.Validate(); err != nil {
		return err
	}
	current, err := t.SourceByID(ctx, event.After.ID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, event.After) {
		return sources.ErrInvalidSourceAudit
	}
	if event.SnapshotID != 0 {
		var found int
		if err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_source_snapshots
		  WHERE id = ? AND source_id = ? AND content_hash = ? AND CAST(url AS BINARY) = CAST(? AS BINARY)`,
			event.SnapshotID, event.After.ID, event.ContentHash, event.URL).Scan(&found); err != nil {
			return fmt.Errorf("复核来源审计快照失败: %w", err)
		}
		if found != 1 {
			return sources.ErrInvalidSourceAudit
		}
	}
	var before any
	if event.Before != nil {
		data, err := json.Marshal(event.Before)
		if err != nil {
			return fmt.Errorf("编码来源变更前副本失败: %w", err)
		}
		before = string(data)
	}
	after, err := json.Marshal(event.After)
	if err != nil {
		return fmt.Errorf("编码来源变更后副本失败: %w", err)
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_source_audits
	  (actor_id, operation, source_id, before_json, after_json, snapshot_id, content_hash, source_url, created_at, updated_at)
	  VALUES (?, ?, ?, ?, ?, NULLIF(?,0), NULLIF(?,''), NULLIF(?,''), UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))`,
		event.ActorID, event.Operation, event.After.ID, before, string(after), event.SnapshotID, event.ContentHash, event.URL)
	if err != nil {
		return fmt.Errorf("追加来源操作审计失败: %w", err)
	}
	return nil
}

// ListSourceAudits 只读历史副本；不连接当前来源状态，避免授权变化改变历史口径。
func (r *AISourceRepo) ListSourceAudits(ctx context.Context, sourceID, afterID int64, limit int) ([]sources.SourceAudit, error) {
	if r == nil || r.db == nil || sourceID <= 0 || afterID < 0 || limit < 1 || limit > 200 {
		return nil, sources.ErrInvalidSourceAudit
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, actor_id, operation, source_id, before_json, after_json,
	  IFNULL(snapshot_id,0), IFNULL(content_hash,''), IFNULL(source_url,''), DATE_FORMAT(created_at, '`+databaseUTCSelect+`')
	  FROM tm_ai_source_audits WHERE source_id = ? AND id > ? ORDER BY id LIMIT ?`, sourceID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询来源操作审计失败: %w", err)
	}
	defer rows.Close()
	out := make([]sources.SourceAudit, 0)
	for rows.Next() {
		event, err := scanSourceAudit(rows)
		if err != nil {
			return nil, fmt.Errorf("读取来源操作审计失败: %w", err)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历来源操作审计失败: %w", err)
	}
	return out, nil
}

func scanSourceAudit(row rowScanner) (sources.SourceAudit, error) {
	var e sources.SourceAudit
	var sourceID int64
	var before, after []byte
	var created string
	if err := row.Scan(&e.ID, &e.ActorID, &e.Operation, &sourceID, &before, &after,
		&e.SnapshotID, &e.ContentHash, &e.URL, &created); err != nil {
		return e, err
	}
	if len(before) > 8192 || len(after) > 8192 {
		return e, sources.ErrInvalidSourceAudit
	}
	if before != nil {
		if err := json.Unmarshal(before, &e.Before); err != nil || e.Before == nil {
			return e, sources.ErrInvalidSourceAudit
		}
	}
	if err := json.Unmarshal(after, &e.After); err != nil {
		return e, sources.ErrInvalidSourceAudit
	}
	var err error
	e.CreatedAt, err = time.Parse(databaseUTCLayout, created)
	if err != nil || e.CreatedAt.IsZero() || e.ID <= 0 || sourceID != e.After.ID {
		return e, sources.ErrInvalidSourceAudit
	}
	if err := e.Validate(); err != nil {
		return e, err
	}
	return e, nil
}
