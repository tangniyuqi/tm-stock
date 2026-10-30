package repository

import (
	"context"
	"database/sql"
	"errors"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

type AIConversationDossierRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAIConversationDossierRepo(db *sql.DB, level sql.IsolationLevel) *AIConversationDossierRepo {
	return &AIConversationDossierRepo{db, level}
}

var _ dossier.ConversationStore = (*AIConversationDossierRepo)(nil)

type aiConversationDossierTx struct {
	tx    *sql.Tx
	write bool
}

func (r *AIConversationDossierRepo) WithinReferences(ctx context.Context, write bool, fn func(dossier.ConversationTx) error) error {
	if r == nil || r.db == nil || ctx == nil || fn == nil || r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead {
		return dossier.ErrInvalidDraft
	}
	// 当前许可与会话归属使用共享锁；数据库事务不设READ ONLY，接口仍禁止非引用写入。
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }() // 已提交后可忽略ErrTxDone；其余出口全部回滚。
	if err = fn(&aiConversationDossierTx{tx, write}); err != nil {
		return err
	}
	return dossierDBError(tx.Commit())
}

func (t *aiConversationDossierTx) Dossiers() dossier.Tx { return &aiDossierTx{t.tx, false} }
func (t *aiConversationDossierTx) Conversation(ctx context.Context, uid, cid int64, lock bool) (conversation.Record, error) {
	return (&aiConversationTx{t.tx, t.write}).ByID(ctx, uid, cid, lock)
}

const conversationDossierColumns = `id,user_id,conversation_id,theme_id,version_no,COALESCE(compare_version,0),CAST(created_at AS CHAR)`

func scanConversationDossier(row rowScanner) (dossier.ConversationReference, error) {
	var r dossier.ConversationReference
	var created string
	if err := row.Scan(&r.ID, &r.UserID, &r.ConversationID, &r.ThemeID, &r.Version, &r.CompareVersion, &created); err != nil {
		return dossier.ConversationReference{}, dossierDBError(err)
	}
	at, err := parsePointsUTC(created)
	if err != nil {
		return dossier.ConversationReference{}, err
	}
	r.CreatedAt = at
	if !r.Valid() {
		return dossier.ConversationReference{}, dossier.ErrInvalidDraft
	}
	return r, nil
}

func (t *aiConversationDossierTx) Reference(ctx context.Context, uid, cid, id int64) (dossier.ConversationReference, error) {
	return scanConversationDossier(t.tx.QueryRowContext(ctx, `SELECT `+conversationDossierColumns+` FROM tm_ai_conversation_dossiers WHERE user_id=? AND conversation_id=? AND id=? FOR SHARE`, uid, cid, id))
}
func (t *aiConversationDossierTx) ReferenceBySelection(ctx context.Context, uid, cid int64, s dossier.Selection) (dossier.ConversationReference, error) {
	query := `SELECT ` + conversationDossierColumns + ` FROM tm_ai_conversation_dossiers WHERE user_id=? AND conversation_id=? AND theme_id=? AND version_no=? AND compare_key=?`
	if !t.write {
		query += ` FOR SHARE`
	}
	// 保存已先锁本人会话，首次一致性读发生在锁后；避免不同会话的空组合共享间隙锁互相阻塞插入。
	return scanConversationDossier(t.tx.QueryRowContext(ctx, query, uid, cid, s.ThemeID, s.Version, s.CompareVersion))
}
func (t *aiConversationDossierTx) References(ctx context.Context, uid, cid, before int64, limit int) ([]dossier.ConversationReference, error) {
	if limit < 1 || limit > 11 {
		return nil, dossier.ErrInvalidDraft
	}
	rows, err := t.tx.QueryContext(ctx, `SELECT `+conversationDossierColumns+` FROM tm_ai_conversation_dossiers WHERE user_id=? AND conversation_id=? AND (?=0 OR id<?) ORDER BY id DESC LIMIT ? FOR SHARE`, uid, cid, before, before, limit)
	if err != nil {
		return nil, dossierDBError(err)
	}
	defer rows.Close()
	out := []dossier.ConversationReference{}
	for rows.Next() {
		r, err := scanConversationDossier(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err = rows.Err(); err != nil {
		return nil, dossierDBError(err)
	}
	return out, nil
}
func (t *aiConversationDossierTx) ReferenceCount(ctx context.Context, uid, cid int64) (int, error) {
	var count int
	err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_conversation_dossiers WHERE user_id=? AND conversation_id=?`, uid, cid).Scan(&count)
	return count, dossierDBError(err)
}
func (t *aiConversationDossierTx) InsertReference(ctx context.Context, r dossier.ConversationReference) (dossier.ConversationReference, error) {
	if !t.write || r.ID != 0 || !conversation.ID(r.UserID) || !conversation.ID(r.ConversationID) || !r.Selection.Valid() || !conversation.Time(r.CreatedAt) {
		return dossier.ConversationReference{}, dossier.ErrInvalidDraft
	}
	var compare any
	if r.CompareVersion != 0 {
		compare = r.CompareVersion
	}
	result, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, r.ConversationID, r.UserID, r.ThemeID, r.Version, compare, pointsUTC(r.CreatedAt), pointsUTC(r.CreatedAt))
	if err != nil {
		var foreignKey *mysql.MySQLError
		if errors.As(err, &foreignKey) && foreignKey.Number == 1452 {
			// 父会话已锁定；不存在的版本是明确拒绝，不伪装为提交结果未知。
			return dossier.ConversationReference{}, dossier.ErrNotFound
		}
		return dossier.ConversationReference{}, dossierDBError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return dossier.ConversationReference{}, dossierDBError(err)
	}
	return t.Reference(ctx, r.UserID, r.ConversationID, id)
}
