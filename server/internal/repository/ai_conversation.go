package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

type AIConversationRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAIConversationRepo(db *sql.DB) *AIConversationRepo {
	return NewAIConversationRepoWithIsolation(db, sql.LevelReadCommitted)
}
func NewAIConversationRepoWithIsolation(db *sql.DB, level sql.IsolationLevel) *AIConversationRepo {
	return &AIConversationRepo{db, level}
}

var _ conversation.Store = (*AIConversationRepo)(nil)

func (r *AIConversationRepo) Within(ctx context.Context, write bool, fn func(conversation.Tx) error) error {
	if r == nil || r.db == nil || fn == nil || ctx == nil || r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead {
		return conversation.ErrInvalid
	}
	level := r.isolation
	if !write {
		level = sql.LevelRepeatableRead
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: !write, Isolation: level})
	if err != nil {
		return fmt.Errorf("开启会话事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后ErrTxDone可忽略，其余出口一律回滚。
	if err := fn(&aiConversationTx{tx, write}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("会话事务提交未确认，请用原键或当前版本核查: %w", err)
	}
	return nil
}

type aiConversationTx struct {
	tx    *sql.Tx
	write bool
}

func conversationDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrNotFound
	}
	var me *mysqldrv.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return conversation.ErrConflict
	}
	if errors.As(err, &me) && me.Number == 1644 && me.Message == "Conversation has unconfirmed task intents" {
		return conversation.ErrConflict
	}
	return fmt.Errorf("会话存储失败: %w", err)
}
func (t *aiConversationTx) Now(ctx context.Context) (time.Time, error) {
	return (&pointsTx{t.tx}).Now(ctx)
}

const conversationColumns = `id,user_id,idem_key,req_hash,title,archived,version,last_seq,CAST(created_at AS CHAR),CAST(updated_at AS CHAR)`

func scanConversation(s pointsScanner) (conversation.Record, error) {
	var r conversation.Record
	var archived int
	var created, updated string
	if err := s.Scan(&r.ID, &r.UserID, &r.Key, &r.RequestHash, &r.Title, &archived, &r.Version, &r.LastSeq, &created, &updated); err != nil {
		return r, conversationDBError(err)
	}
	if archived != 0 && archived != 1 {
		return conversation.Record{}, conversation.ErrInvariant
	}
	r.Archived = archived == 1
	var err error
	r.CreatedAt, err = parsePointsUTC(created)
	if err != nil {
		return conversation.Record{}, err
	}
	r.UpdatedAt, err = parsePointsUTC(updated)
	if err != nil {
		return conversation.Record{}, err
	}
	if !r.Valid() {
		return conversation.Record{}, conversation.ErrInvariant
	}
	return r, nil
}
func (t *aiConversationTx) CreateOrGet(ctx context.Context, r conversation.Record) (conversation.Record, error) {
	if !t.write {
		return conversation.Record{}, conversation.ErrInvalid
	}
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_conversations(user_id,idem_key,req_hash,title,archived,version,last_seq,created_at,updated_at) VALUES(?,?,?,?,0,1,0,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, r.UserID, r.Key, r.RequestHash, r.Title, pointsUTC(r.CreatedAt), pointsUTC(r.UpdatedAt))
	if err != nil {
		return conversation.Record{}, conversationDBError(err)
	}
	return scanConversation(t.tx.QueryRowContext(ctx, `SELECT `+conversationColumns+` FROM tm_ai_conversations WHERE user_id=? AND idem_key=? FOR UPDATE`, r.UserID, r.Key))
}
func (t *aiConversationTx) ByID(ctx context.Context, uid, id int64, lock bool) (conversation.Record, error) {
	if lock {
		if !t.write {
			return conversation.Record{}, conversation.ErrInvalid
		}
		return scanConversation(t.tx.QueryRowContext(ctx, `SELECT `+conversationColumns+` FROM tm_ai_conversations WHERE user_id=? AND id=? FOR UPDATE`, uid, id))
	}
	return scanConversation(t.tx.QueryRowContext(ctx, `SELECT `+conversationColumns+` FROM tm_ai_conversations WHERE user_id=? AND id=?`, uid, id))
}
func (t *aiConversationTx) ByKey(ctx context.Context, uid int64, key string) (conversation.Record, error) {
	return scanConversation(t.tx.QueryRowContext(ctx, `SELECT `+conversationColumns+` FROM tm_ai_conversations WHERE user_id=? AND idem_key=?`, uid, key))
}
func (t *aiConversationTx) List(ctx context.Context, uid, before int64, archived bool, limit int) ([]conversation.Record, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT `+conversationColumns+` FROM tm_ai_conversations WHERE user_id=? AND archived=? AND id<? ORDER BY id DESC LIMIT ?`, uid, archived, before, limit)
	if err != nil {
		return nil, conversationDBError(err)
	}
	defer rows.Close()
	out := []conversation.Record{}
	for rows.Next() {
		r, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, conversationDBError(rows.Err())
}
func (t *aiConversationTx) Update(ctx context.Context, old, next conversation.Record) error {
	if !t.write || !old.Valid() || !next.Valid() || old.ID != next.ID || old.UserID != next.UserID {
		return conversation.ErrInvalid
	}
	r, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_conversations SET title=?,archived=?,version=?,last_seq=?,updated_at=? WHERE id=? AND user_id=? AND version=?`, next.Title, next.Archived, next.Version, next.LastSeq, pointsUTC(next.UpdatedAt), old.ID, old.UserID, old.Version)
	if err != nil {
		return conversationDBError(err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return conversation.ErrConflict
	}
	return nil
}
func (t *aiConversationTx) Messages(ctx context.Context, uid, id, after, through int64, limit int) ([]conversation.Message, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT conversation_id,user_id,seq,idem_key,req_hash,kind,subject_type,subject_id,skill,question,task_id,CAST(created_at AS CHAR) FROM tm_ai_conversation_messages WHERE conversation_id=? AND user_id=? AND seq>? AND seq<=? ORDER BY seq LIMIT ?`, id, uid, after, through, limit)
	if err != nil {
		return nil, conversationDBError(err)
	}
	defer rows.Close()
	out := []conversation.Message{}
	for rows.Next() {
		var m conversation.Message
		var taskID sql.NullInt64
		var created string
		if err := rows.Scan(&m.ConversationID, &m.UserID, &m.Seq, &m.Key, &m.RequestHash, &m.Input.Kind, &m.Input.SubjectType, &m.Input.SubjectID, &m.Input.Skill, &m.Input.Question, &taskID, &created); err != nil {
			return nil, conversationDBError(err)
		}
		if taskID.Valid {
			if !conversation.ID(taskID.Int64) {
				return nil, conversation.ErrInvariant
			}
			m.Input.TaskID = taskID.Int64
		}
		m.CreatedAt, err = parsePointsUTC(created)
		if err != nil {
			return nil, err
		}
		if !m.Valid() {
			return nil, conversation.ErrInvariant
		}
		out = append(out, m)
	}
	return out, conversationDBError(rows.Err())
}
func (t *aiConversationTx) Append(ctx context.Context, m conversation.Message) error {
	if !t.write || !m.Valid() {
		return conversation.ErrInvalid
	}
	var taskID any
	if m.Input.TaskID > 0 {
		taskID = m.Input.TaskID
	}
	v := m.Input
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_messages(conversation_id,user_id,seq,idem_key,req_hash,kind,subject_type,subject_id,skill,question,task_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, m.ConversationID, m.UserID, m.Seq, m.Key, m.RequestHash, v.Kind, v.SubjectType, v.SubjectID, v.Skill, v.Question, taskID, pointsUTC(m.CreatedAt), pointsUTC(m.CreatedAt))
	return conversationDBError(err)
}
