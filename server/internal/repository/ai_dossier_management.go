package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

// 管理账号独立使用；不提供模型、任务、报告或积分写入接口。
type AIDossierManagementRepo struct{ db *sql.DB }

func NewAIDossierManagementRepo(db *sql.DB) *AIDossierManagementRepo {
	return &AIDossierManagementRepo{db}
}

type aiDossierManagementTx struct{ *aiDossierTx }

func (r *AIDossierManagementRepo) WithinManagement(ctx context.Context, fn func(dossier.ManagementTx) error) error {
	if r == nil || r.db == nil || ctx == nil || fn == nil {
		return dossier.ErrInvalidManagement
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }() // 所有未确认出口回滚，不把连接信息打印到日志。
	if err = verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	if err = fn(&aiDossierManagementTx{&aiDossierTx{tx: tx}}); err != nil {
		return err
	}
	return dossierDBError(tx.Commit())
}
func (t *aiDossierManagementTx) ManagementHead(ctx context.Context, key dossier.ManagementObject, lock bool) (dossier.ManagementHead, error) {
	var h dossier.ManagementHead
	if !key.Valid() {
		return h, dossier.ErrInvalidManagement
	}
	args := []any{key.Kind, key.UserID, key.ThemeID, key.Version}
	if lock {
		if _, err := t.tx.ExecContext(ctx, `INSERT IGNORE INTO tm_ai_dossier_management_heads(kind,user_id,theme_id,version_no,revision,request_id,created_at,updated_at) VALUES(?,?,?,?,0,NULL,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, args...); err != nil {
			return h, dossierDBError(err)
		}
	}
	query := `SELECT revision,request_id FROM tm_ai_dossier_management_heads WHERE kind=? AND user_id=? AND theme_id=? AND version_no=?`
	if lock {
		query += " FOR UPDATE"
	}
	var request sql.NullString
	err := t.tx.QueryRowContext(ctx, query, args...).Scan(&h.Revision, &request)
	if errors.Is(err, sql.ErrNoRows) && !lock {
		return h, nil
	}
	if err != nil {
		return h, dossierDBError(err)
	}
	h.RequestID = request.String
	if h.Revision < 0 || h.Revision > 9007199254740991 || h.Revision == 0 && request.Valid || h.Revision > 0 && !dossier.ValidManagementRequestID(h.RequestID) {
		return dossier.ManagementHead{}, dossier.ErrManagementEvidence
	}
	return h, nil
}
func (t *aiDossierManagementTx) ManagementEvent(ctx context.Context, id string) (dossier.ManagementEvent, error) {
	var raw []byte
	var digest string
	if !dossier.ValidManagementRequestID(id) {
		return dossier.ManagementEvent{}, dossier.ErrInvalidManagement
	}
	if err := t.tx.QueryRowContext(ctx, `SELECT canonical,content_hash FROM tm_ai_dossier_management_events WHERE request_id=?`, id).Scan(&raw, &digest); err != nil {
		return dossier.ManagementEvent{}, dossierDBError(err)
	}
	e, err := dossier.RestoreManagementEvent(raw, digest)
	if err != nil || e.Request.RequestID != id {
		return dossier.ManagementEvent{}, dossier.ErrManagementEvidence
	}
	return e, nil
}
func (t *aiDossierManagementTx) CurrentPermission(ctx context.Context, key dossier.ManagementObject) (*dossier.ManagementState, error) {
	var a dossier.Approval
	var err error
	switch key.Kind {
	case "publication":
		a, err = scanDossierApproval(t.tx.QueryRowContext(ctx, `SELECT digest,approval_ref,CAST(valid_until AS CHAR),enabled FROM tm_ai_dossier_publications WHERE theme_id=? AND version_no=? FOR UPDATE`, key.ThemeID, key.Version))
	case "member_read":
		a, err = scanDossierApproval(t.tx.QueryRowContext(ctx, `SELECT digest,approval_ref,CAST(valid_until AS CHAR),enabled FROM tm_ai_dossier_grants WHERE user_id=? AND theme_id=? AND version_no=? FOR UPDATE`, key.UserID, key.ThemeID, key.Version))
	default:
		return nil, dossier.ErrInvalidManagement
	}
	if errors.Is(err, dossier.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dossier.ManagementState{Digest: a.Digest, Reference: a.Reference, Enabled: a.Enabled, ValidUntil: a.Until.UnixMilli()}, nil
}
func (t *aiDossierManagementTx) AppendManagementEvent(ctx context.Context, e dossier.ManagementEvent) error {
	raw, digest, err := e.Canonical()
	if err != nil {
		return err
	}
	k := e.Request.Object
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_management_events(request_id,actor_id,kind,user_id,theme_id,version_no,version_digest,revision,canonical,content_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, e.Request.RequestID, e.ActorID, k.Kind, k.UserID, k.ThemeID, k.Version, e.Request.VersionDigest, e.Revision, raw, digest, pointsUTC(time.UnixMilli(e.CreatedAt)), pointsUTC(time.UnixMilli(e.CreatedAt)))
	return dossierDBError(err)
}
func (t *aiDossierManagementTx) AdvanceManagementHead(ctx context.Context, k dossier.ManagementObject, before dossier.ManagementHead, e dossier.ManagementEvent) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_dossier_management_heads SET revision=?,request_id=?,updated_at=? WHERE kind=? AND user_id=? AND theme_id=? AND version_no=? AND revision=? AND request_id<=>?`, e.Revision, e.Request.RequestID, pointsUTC(time.UnixMilli(e.CreatedAt)), k.Kind, k.UserID, k.ThemeID, k.Version, before.Revision, nullableManagementRequest(before.RequestID))
	if err != nil {
		return dossierDBError(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return dossier.ErrManagementConflict
	}
	return nil
}
func nullableManagementRequest(id string) any {
	if id == "" {
		return nil
	}
	return id
}
func (t *aiDossierManagementTx) WritePermission(ctx context.Context, k dossier.ManagementObject, before *dossier.ManagementState, after dossier.ManagementState, at int64) error {
	until, now := pointsUTC(time.UnixMilli(after.ValidUntil)), pointsUTC(time.UnixMilli(at))
	var query string
	var args []any
	switch {
	case k.Kind == "publication" && before == nil:
		query = `INSERT INTO tm_ai_dossier_publications(theme_id,version_no,digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`
		args = []any{k.ThemeID, k.Version, after.Digest, after.Reference, after.Enabled, until, now, now}
	case k.Kind == "publication":
		query = `UPDATE tm_ai_dossier_publications SET digest=?,approval_ref=?,enabled=?,valid_until=?,updated_at=? WHERE theme_id=? AND version_no=?`
		args = []any{after.Digest, after.Reference, after.Enabled, until, now, k.ThemeID, k.Version}
	case k.Kind == "member_read" && before == nil:
		query = `INSERT INTO tm_ai_dossier_grants(user_id,theme_id,version_no,digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`
		args = []any{k.UserID, k.ThemeID, k.Version, after.Digest, after.Reference, after.Enabled, until, now, now}
	case k.Kind == "member_read":
		query = `UPDATE tm_ai_dossier_grants SET digest=?,approval_ref=?,enabled=?,valid_until=?,updated_at=? WHERE user_id=? AND theme_id=? AND version_no=?`
		args = []any{after.Digest, after.Reference, after.Enabled, until, now, k.UserID, k.ThemeID, k.Version}
	default:
		return dossier.ErrInvalidManagement
	}
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return dossierDBError(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return dossier.ErrManagementConflict
	}
	return nil
}

// 原许可行已持共享锁；管理头和不可变事件只读，避免与写入头锁形成反向等待。
// 任何缺失、当前后像不符或缺SELECT权限均失败，不能剥离管理引用回退旧许可。
func (t *aiDossierTx) verifyManagedApproval(ctx context.Context, k dossier.ManagementObject, a dossier.Approval) (dossier.Approval, error) {
	if !dossier.IsManagedApproval(a.Reference) {
		return a, nil
	}
	id := strings.TrimPrefix(a.Reference, dossier.ManagedApprovalPrefix)
	var raw []byte
	var digest string
	var revision int64
	err := t.tx.QueryRowContext(ctx, `SELECT e.canonical,e.content_hash,h.revision FROM tm_ai_dossier_management_heads h JOIN tm_ai_dossier_management_events e ON e.request_id=h.request_id AND e.kind=h.kind AND e.user_id=h.user_id AND e.theme_id=h.theme_id AND e.version_no=h.version_no AND e.revision=h.revision WHERE h.kind=? AND h.user_id=? AND h.theme_id=? AND h.version_no=? AND h.request_id=? AND h.revision>0`, k.Kind, k.UserID, k.ThemeID, k.Version, id).Scan(&raw, &digest, &revision)
	if err != nil {
		return dossier.Approval{}, dossierDBError(err)
	}
	e, err := dossier.RestoreManagementEvent(raw, digest)
	if err != nil || e.Revision != revision || !e.MatchesApproval(k, a) {
		return dossier.Approval{}, dossier.ErrManagementEvidence
	}
	return a, nil
}
