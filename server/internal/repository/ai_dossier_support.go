package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

type AIDossierSupportRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAIDossierSupportRepo(db *sql.DB, level sql.IsolationLevel) *AIDossierSupportRepo {
	return &AIDossierSupportRepo{db: db, isolation: level}
}

var _ dossier.SupportRegistry = (*AIDossierSupportRepo)(nil)

type aiDossierSupportTx struct {
	tx    *sql.Tx
	write bool
}

func (r *AIDossierSupportRepo) WithinSupport(ctx context.Context, write bool, fn func(dossier.SupportRegistryTx) error) error {
	if r == nil || r.db == nil || ctx == nil || fn == nil || r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead {
		return dossier.ErrInvalidDraft
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后ErrTxDone可忽略，其他出口回滚。
	if err = fn(&aiDossierSupportTx{tx: tx, write: write}); err != nil {
		return err
	}
	return dossierDBError(tx.Commit())
}
func (t *aiDossierSupportTx) Dossiers() dossier.Tx { return &aiDossierTx{tx: t.tx, write: false} }
func (t *aiDossierSupportTx) LockSupportTheme(ctx context.Context, id int64) error {
	if !t.write {
		return dossier.ErrInvalidDraft
	}
	var got int64
	return dossierDBError(t.tx.QueryRowContext(ctx, `SELECT id FROM addon_quant_theme WHERE id=? AND status=1 FOR UPDATE`, id).Scan(&got))
}
func (t *aiDossierSupportTx) SupportManifest(ctx context.Context, digest string) (dossier.SupportRegistration, error) {
	var r dossier.SupportRegistration
	var compiled, created, versionDigest string
	err := t.tx.QueryRowContext(ctx, `SELECT component_digest,theme_id,version_no,version_digest,analysis_type,manifest_digest,canonical,CAST(compiled_at AS CHAR),CAST(created_at AS CHAR) FROM tm_ai_dossier_support_plans WHERE component_digest=? FOR SHARE`, digest).Scan(&r.ComponentDigest, &r.Identity.ThemeID, &r.Identity.Version, &versionDigest, &r.AnalysisType, &r.ManifestDigest, &r.Canonical, &compiled, &created)
	if err != nil {
		return dossier.SupportRegistration{}, dossierDBError(err)
	}
	at, err := parsePointsUTC(compiled)
	if err != nil {
		return dossier.SupportRegistration{}, err
	}
	r.Identity.CreatedAt = at.UnixMilli()
	r.CreatedAt, err = parsePointsUTC(created)
	if err != nil {
		return dossier.SupportRegistration{}, err
	}
	m, err := r.Manifest()
	if err != nil || m.DraftDigest() != versionDigest {
		return dossier.SupportRegistration{}, dossier.ErrSupportReview
	}
	return r, nil
}
func (t *aiDossierSupportTx) InsertSupportManifest(ctx context.Context, r dossier.SupportRegistration) error {
	if !t.write {
		return dossier.ErrInvalidDraft
	}
	m, err := r.Manifest()
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_support_plans(component_digest,theme_id,version_no,version_digest,analysis_type,manifest_digest,canonical,compiled_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, r.ComponentDigest, r.Identity.ThemeID, r.Identity.Version, m.DraftDigest(), r.AnalysisType, r.ManifestDigest, r.Canonical, pointsUTC(time.UnixMilli(r.Identity.CreatedAt)), pointsUTC(r.CreatedAt), pointsUTC(r.CreatedAt))
	return dossierDBError(err)
}
func (t *aiDossierSupportTx) SupportUse(ctx context.Context, digest string) (dossier.Approval, error) {
	return scanDossierApproval(t.tx.QueryRowContext(ctx, `SELECT component_digest,approval_ref,CAST(valid_until AS CHAR),enabled FROM tm_ai_dossier_support_uses WHERE component_digest=? FOR SHARE`, digest))
}
