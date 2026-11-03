package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

type AIDossierCorrectionRepo struct{ db *sql.DB }

func NewAIDossierCorrectionRepo(db *sql.DB) *AIDossierCorrectionRepo {
	return &AIDossierCorrectionRepo{db}
}

type aiDossierCorrectionTx struct {
	*aiDossierManagementTx
	correctionWrite bool
}

func (r *AIDossierCorrectionRepo) WithinCorrection(ctx context.Context, write bool, fn func(dossier.CorrectionTx) error) error {
	if r == nil || r.db == nil || ctx == nil || fn == nil {
		return dossier.ErrInvalidManagement
	}
	// 当前授权读取须持共享锁，与原档案仓储一致；MySQL READ ONLY不允许此锁。
	// 写能力由专属SQL账号及correctionWrite限制，不以数据库只读标记替代。
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }() // 未确认出口统一回滚，提交未知由原请求核查。
	if err = verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	a := &aiDossierCorrectionTx{&aiDossierManagementTx{&aiDossierTx{tx: tx}}, write}
	if err = fn(a); err != nil {
		return err
	}
	return dossierDBError(tx.Commit())
}
func (t *aiDossierCorrectionTx) CorrectionReadLock(ctx context.Context, p dossier.CorrectionPair) (bool, error) {
	if !p.Valid() {
		return false, dossier.ErrInvalidManagement
	}
	var revision int64
	err := t.tx.QueryRowContext(ctx, `SELECT revision FROM tm_ai_dossier_management_heads WHERE kind='publication' AND user_id=0 AND theme_id=? AND version_no=? FOR SHARE`, p.ThemeID, p.AfterVersion).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, dossierDBError(err)
}
func (t *aiDossierCorrectionTx) LatestCorrection(ctx context.Context, p dossier.CorrectionPair) (dossier.CorrectionEvent, string, error) {
	if !p.Valid() {
		return dossier.CorrectionEvent{}, "", dossier.ErrInvalidManagement
	}
	var raw []byte
	var digest string
	err := t.tx.QueryRowContext(ctx, `SELECT canonical,content_hash FROM tm_ai_dossier_correction_events WHERE theme_id=? AND before_version=? AND after_version=? ORDER BY revision DESC LIMIT 1 FOR SHARE`, p.ThemeID, p.BeforeVersion, p.AfterVersion).Scan(&raw, &digest)
	if err != nil {
		return dossier.CorrectionEvent{}, "", dossierDBError(err)
	}
	e, err := dossier.RestoreCorrectionEvent(raw, digest)
	if err != nil || e.Request.Object != p {
		return dossier.CorrectionEvent{}, "", dossier.ErrManagementEvidence
	}
	return e, digest, nil
}
func (t *aiDossierCorrectionTx) CorrectionEvent(ctx context.Context, id string) (dossier.CorrectionEvent, error) {
	if !dossier.ValidManagementRequestID(id) {
		return dossier.CorrectionEvent{}, dossier.ErrInvalidManagement
	}
	var raw []byte
	var digest string
	err := t.tx.QueryRowContext(ctx, `SELECT canonical,content_hash FROM tm_ai_dossier_correction_events WHERE request_id=?`, id).Scan(&raw, &digest)
	if err != nil {
		return dossier.CorrectionEvent{}, dossierDBError(err)
	}
	e, err := dossier.RestoreCorrectionEvent(raw, digest)
	if err != nil || e.Request.RequestID != id {
		return dossier.CorrectionEvent{}, dossier.ErrManagementEvidence
	}
	return e, nil
}
func (t *aiDossierCorrectionTx) AppendCorrection(ctx context.Context, e dossier.CorrectionEvent) error {
	if !t.correctionWrite {
		return dossier.ErrInvalidManagement
	}
	raw, digest, err := e.Canonical()
	if err != nil {
		return err
	}
	p := e.Request.Object
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_correction_events(request_id,actor_id,theme_id,before_version,after_version,before_digest,after_digest,revision,canonical,content_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, e.Request.RequestID, e.ActorID, p.ThemeID, p.BeforeVersion, p.AfterVersion, e.Request.BeforeDigest, e.Request.AfterDigest, e.Revision, raw, digest, pointsUTC(time.UnixMilli(e.CreatedAt)), pointsUTC(time.UnixMilli(e.CreatedAt)))
	return dossierDBError(err)
}
func (r *AIDossierCorrectionRepo) VerifyAccess(ctx context.Context, write bool) error {
	if r == nil || r.db == nil || ctx == nil {
		return dossier.ErrInvalidManagement
	}
	if err := NewAIDossierRepo(r.db, sql.LevelReadCommitted).VerifyReadAccess(ctx); err != nil {
		return err
	}
	return r.WithinCorrection(ctx, write, func(tx dossier.CorrectionTx) error {
		a := tx.(*aiDossierCorrectionTx)
		for _, q := range []string{`SELECT request_id,actor_id,theme_id,before_version,after_version,before_digest,after_digest,revision,canonical,content_hash,created_at,updated_at FROM tm_ai_dossier_correction_events WHERE 1=0`, `SELECT revision FROM tm_ai_dossier_management_heads WHERE 1=0`, `SELECT canonical FROM tm_ai_dossier_management_events WHERE 1=0`} {
			rows, err := a.tx.QueryContext(ctx, q)
			if err != nil {
				return err
			}
			if err = rows.Close(); err != nil {
				return err
			}
		}
		if write {
			_, err := a.tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_correction_events(request_id,actor_id,theme_id,before_version,after_version,before_digest,after_digest,revision,canonical,content_hash,created_at,updated_at) SELECT request_id,actor_id,theme_id,before_version,after_version,before_digest,after_digest,revision,canonical,content_hash,created_at,updated_at FROM tm_ai_dossier_correction_events WHERE 1=0`)
			return err
		}
		return nil
	})
}
