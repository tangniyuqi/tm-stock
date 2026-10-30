package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type AIDossierRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAIDossierRepo(db *sql.DB, level sql.IsolationLevel) *AIDossierRepo {
	return &AIDossierRepo{db, level}
}

type aiDossierTx struct {
	tx    *sql.Tx
	write bool
}

var _ dossier.Store = (*AIDossierRepo)(nil)

func (r *AIDossierRepo) Within(ctx context.Context, write bool, fn func(dossier.Tx) error) error {
	if r == nil || r.db == nil || ctx == nil || fn == nil || (r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead) {
		return dossier.ErrInvalidDraft
	}
	// 读取也用共享锁实现撤回互斥，不能设为数据库READ ONLY事务；方法不提供许可写入。
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后ErrTxDone可忽略，其余出口全部回滚。
	if err = fn(&aiDossierTx{tx, write}); err != nil {
		return err
	}
	return dossierDBError(tx.Commit())
}
func dossierDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return dossier.ErrNotFound
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return dossier.ErrConflict
	}
	return fmt.Errorf("专题存储操作未确认: %w", err)
}
func (t *aiDossierTx) Now(ctx context.Context) (time.Time, error) {
	var raw string
	if err := t.tx.QueryRowContext(ctx, `SELECT CAST(UTC_TIMESTAMP(3) AS CHAR)`).Scan(&raw); err != nil {
		return time.Time{}, dossierDBError(err)
	}
	return parsePointsUTC(raw)
}
func (t *aiDossierTx) Member(ctx context.Context, id int64) error {
	var got int64
	return dossierDBError(t.tx.QueryRowContext(ctx, `SELECT id FROM addon_member WHERE id=? AND status=1 AND deleted_at IS NULL FOR SHARE`, id).Scan(&got))
}
func (t *aiDossierTx) Theme(ctx context.Context, id int64) error {
	query := `SELECT id FROM addon_quant_theme WHERE id=? AND status=1`
	if t.write {
		query += ` FOR UPDATE`
	} else {
		query += ` FOR SHARE`
	}
	var got int64
	return dossierDBError(t.tx.QueryRowContext(ctx, query, id).Scan(&got))
}
func (t *aiDossierTx) Version(ctx context.Context, theme, version int64) (dossier.StoredVersion, error) {
	v := dossier.StoredVersion{Identity: dossier.DraftIdentity{ThemeID: theme, Version: version}}
	var compiled string
	err := t.tx.QueryRowContext(ctx, `SELECT digest,canonical,CAST(compiled_at AS CHAR) FROM tm_ai_dossier_versions WHERE theme_id=? AND version_no=? FOR SHARE`, theme, version).Scan(&v.Digest, &v.Canonical, &compiled)
	if err != nil {
		return dossier.StoredVersion{}, dossierDBError(err)
	}
	at, err := parsePointsUTC(compiled)
	if err != nil {
		return dossier.StoredVersion{}, err
	}
	v.Identity.CreatedAt = at.UnixMilli()
	rows, err := t.tx.QueryContext(ctx, `SELECT snapshot_id,source_id,content_hash FROM tm_ai_dossier_evidence WHERE theme_id=? AND version_no=? ORDER BY snapshot_id LIMIT 33 FOR SHARE`, theme, version)
	if err != nil {
		return dossier.StoredVersion{}, dossierDBError(err)
	}
	defer rows.Close()
	v.Evidence = []dossier.EvidenceRef{}
	for rows.Next() {
		var ref dossier.EvidenceRef
		if err = rows.Scan(&ref.ID, &ref.SourceID, &ref.Hash); err != nil {
			return dossier.StoredVersion{}, dossierDBError(err)
		}
		v.Evidence = append(v.Evidence, ref)
	}
	if err = rows.Err(); err != nil {
		return dossier.StoredVersion{}, dossierDBError(err)
	}
	return v, nil
}
func (t *aiDossierTx) Latest(ctx context.Context, theme int64) (dossier.DraftIdentity, error) {
	v := dossier.DraftIdentity{ThemeID: theme}
	var raw string
	err := t.tx.QueryRowContext(ctx, `SELECT version_no,CAST(compiled_at AS CHAR) FROM tm_ai_dossier_versions WHERE theme_id=? ORDER BY version_no DESC LIMIT 1 FOR SHARE`, theme).Scan(&v.Version, &raw)
	if err != nil {
		return dossier.DraftIdentity{}, dossierDBError(err)
	}
	at, err := parsePointsUTC(raw)
	if err != nil {
		return dossier.DraftIdentity{}, err
	}
	v.CreatedAt = at.UnixMilli()
	return v, nil
}
func (t *aiDossierTx) Insert(ctx context.Context, v dossier.StoredVersion) error {
	if !t.write {
		return dossier.ErrInvalidDraft
	}
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_versions(theme_id,version_no,digest,canonical,compiled_at,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.Identity.ThemeID, v.Identity.Version, v.Digest, v.Canonical, pointsUTC(time.UnixMilli(v.Identity.CreatedAt)))
	if err != nil {
		return dossierDBError(err)
	}
	for _, ref := range v.Evidence {
		_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_evidence(theme_id,version_no,snapshot_id,source_id,content_hash,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.Identity.ThemeID, v.Identity.Version, ref.ID, ref.SourceID, ref.Hash)
		if err != nil {
			return dossierDBError(err)
		}
	}
	return nil
}

// Versions只枚举本人当前启用且摘要匹配的批准，来源及规范记录仍由服务逐版复核。
func (t *aiDossierTx) Versions(ctx context.Context, user, theme, before int64, limit int) ([]int64, error) {
	if limit < 1 || limit > 8 {
		return nil, dossier.ErrInvalidDraft
	}
	rows, err := t.tx.QueryContext(ctx, `SELECT g.version_no FROM tm_ai_dossier_grants g JOIN tm_ai_dossier_publications p ON p.theme_id=g.theme_id AND p.version_no=g.version_no AND p.digest=g.digest WHERE g.user_id=? AND g.theme_id=? AND (?=0 OR g.version_no<?) AND g.enabled=1 AND p.enabled=1 AND g.valid_until>UTC_TIMESTAMP(3) AND p.valid_until>UTC_TIMESTAMP(3) ORDER BY g.version_no DESC LIMIT ? FOR SHARE`, user, theme, before, before, limit)
	if err != nil {
		return nil, dossierDBError(err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, dossierDBError(err)
		}
		out = append(out, id)
	}
	if err = rows.Err(); err != nil {
		return nil, dossierDBError(err)
	}
	return out, nil
}
func (t *aiDossierTx) Publication(ctx context.Context, theme, version int64) (dossier.Approval, error) {
	return scanDossierApproval(t.tx.QueryRowContext(ctx, `SELECT digest,approval_ref,CAST(valid_until AS CHAR),enabled FROM tm_ai_dossier_publications WHERE theme_id=? AND version_no=? FOR SHARE`, theme, version))
}
func (t *aiDossierTx) Grant(ctx context.Context, user, theme, version int64) (dossier.Approval, error) {
	return scanDossierApproval(t.tx.QueryRowContext(ctx, `SELECT digest,approval_ref,CAST(valid_until AS CHAR),enabled FROM tm_ai_dossier_grants WHERE user_id=? AND theme_id=? AND version_no=? FOR SHARE`, user, theme, version))
}
func scanDossierApproval(row rowScanner) (dossier.Approval, error) {
	var a dossier.Approval
	var raw string
	var enabled int
	if err := row.Scan(&a.Digest, &a.Reference, &raw, &enabled); err != nil {
		return a, dossierDBError(err)
	}
	until, err := parsePointsUTC(raw)
	if err != nil {
		return dossier.Approval{}, err
	}
	a.Until = until
	a.Enabled = enabled == 1
	return a, nil
}
func (t *aiDossierTx) Source(ctx context.Context, id int64) (sources.Source, error) {
	return sourceByIDWithMode(ctx, t.tx, id, sourceShared)
}
func (t *aiDossierTx) Snapshot(ctx context.Context, id int64) (sources.Snapshot, error) {
	return sourceSnapshotByID(ctx, t.tx, id, false)
}
