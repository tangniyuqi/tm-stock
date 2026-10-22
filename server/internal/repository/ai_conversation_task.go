package repository

import (
	"context"
	"database/sql"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

type aiConversationIntentTx struct{ *aiConversationTx }

var _ conversation.IntentStore = (*AIConversationRepo)(nil)

func (r *AIConversationRepo) WithinIntents(ctx context.Context, write bool, fn func(conversation.IntentTx) error) error {
	if fn == nil {
		return conversation.ErrInvalid
	}
	return r.Within(ctx, write, func(base conversation.Tx) error { return fn(&aiConversationIntentTx{base.(*aiConversationTx)}) })
}

const intentColumns = `id,conversation_id,user_id,idem_key,req_hash,task_key,analysis_type,subject_type,subject_id,price_ver,price_points,state,rejection_code,version,task_id,message_seq,CAST(created_at AS CHAR),CAST(updated_at AS CHAR)`

func scanConversationIntent(s pointsScanner) (conversation.Intent, error) {
	var i conversation.Intent
	var taskID, messageSeq sql.NullInt64
	var created, updated string
	if err := s.Scan(&i.ID, &i.ConversationID, &i.UserID, &i.Key, &i.RequestHash, &i.TaskKey, &i.Input.AnalysisType, &i.Input.SubjectType, &i.Input.SubjectID, &i.Input.PriceVersion, &i.PricePoints, &i.State, &i.RejectionCode, &i.Version, &taskID, &messageSeq, &created, &updated); err != nil {
		return i, conversationDBError(err)
	}
	if taskID.Valid {
		if !conversation.ID(taskID.Int64) {
			return conversation.Intent{}, conversation.ErrInvariant
		}
		i.TaskID = taskID.Int64
	}
	if messageSeq.Valid {
		if !conversation.ID(messageSeq.Int64) {
			return conversation.Intent{}, conversation.ErrInvariant
		}
		i.MessageSeq = messageSeq.Int64
	}
	var err error
	i.CreatedAt, err = parsePointsUTC(created)
	if err != nil {
		return conversation.Intent{}, err
	}
	i.UpdatedAt, err = parsePointsUTC(updated)
	if err != nil {
		return conversation.Intent{}, err
	}
	if !i.Valid() {
		return conversation.Intent{}, conversation.ErrInvariant
	}
	return i, nil
}
func (t *aiConversationIntentTx) IntentByID(ctx context.Context, uid, cid, id int64, lock bool) (conversation.Intent, error) {
	if lock {
		if !t.write {
			return conversation.Intent{}, conversation.ErrInvalid
		}
		return scanConversationIntent(t.tx.QueryRowContext(ctx, `SELECT `+intentColumns+` FROM tm_ai_conversation_tasks WHERE user_id=? AND conversation_id=? AND id=? FOR UPDATE`, uid, cid, id))
	}
	return scanConversationIntent(t.tx.QueryRowContext(ctx, `SELECT `+intentColumns+` FROM tm_ai_conversation_tasks WHERE user_id=? AND conversation_id=? AND id=?`, uid, cid, id))
}
func (t *aiConversationIntentTx) IntentByKey(ctx context.Context, uid, cid int64, key string) (conversation.Intent, error) {
	return scanConversationIntent(t.tx.QueryRowContext(ctx, `SELECT `+intentColumns+` FROM tm_ai_conversation_tasks WHERE user_id=? AND conversation_id=? AND idem_key=?`, uid, cid, key))
}
func (t *aiConversationIntentTx) InsertIntent(ctx context.Context, i conversation.Intent) (conversation.Intent, error) {
	if !t.write {
		return conversation.Intent{}, conversation.ErrInvalid
	}
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_tasks(conversation_id,user_id,idem_key,req_hash,task_key,analysis_type,subject_type,subject_id,price_ver,price_points,state,rejection_code,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,'prepared','',1,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, i.ConversationID, i.UserID, i.Key, i.RequestHash, i.TaskKey, i.Input.AnalysisType, i.Input.SubjectType, i.Input.SubjectID, i.Input.PriceVersion, i.PricePoints, pointsUTC(i.CreatedAt), pointsUTC(i.UpdatedAt))
	if err != nil {
		return conversation.Intent{}, conversationDBError(err)
	}
	return t.IntentByKey(ctx, i.UserID, i.ConversationID, i.Key)
}
func (t *aiConversationIntentTx) SetIntent(ctx context.Context, old, next conversation.Intent) error {
	if !t.write || !old.Valid() || !next.Valid() || old.ID != next.ID || old.UserID != next.UserID || old.ConversationID != next.ConversationID {
		return conversation.ErrInvalid
	}
	var taskID, seq any
	if next.TaskID > 0 {
		taskID = next.TaskID
		seq = next.MessageSeq
	}
	r, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_conversation_tasks SET state=?,rejection_code=?,version=?,task_id=?,message_seq=?,updated_at=? WHERE id=? AND user_id=? AND conversation_id=? AND version=?`, next.State, next.RejectionCode, next.Version, taskID, seq, pointsUTC(next.UpdatedAt), old.ID, old.UserID, old.ConversationID, old.Version)
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
func (t *aiConversationIntentTx) Intents(ctx context.Context, uid, cid, before int64, limit int) ([]conversation.Intent, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT `+intentColumns+` FROM tm_ai_conversation_tasks WHERE user_id=? AND conversation_id=? AND id<? ORDER BY id DESC LIMIT ?`, uid, cid, before, limit)
	if err != nil {
		return nil, conversationDBError(err)
	}
	defer rows.Close()
	out := []conversation.Intent{}
	for rows.Next() {
		i, err := scanConversationIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, conversationDBError(rows.Err())
}
func (t *aiConversationIntentTx) IntentTask(ctx context.Context, uid int64, key string, lock bool) (conversation.TaskIdentity, error) {
	query := `SELECT id,user_id,idem_key,req_hash,analysis_type,subject_type,subject_id,price_ver,price_points FROM tm_ai_tasks WHERE user_id=? AND idem_key=?`
	if lock {
		if !t.write {
			return conversation.TaskIdentity{}, conversation.ErrInvalid
		}
		query += ` FOR SHARE`
	}
	var v conversation.TaskIdentity
	err := t.tx.QueryRowContext(ctx, query, uid, key).Scan(&v.ID, &v.UserID, &v.Key, &v.RequestHash, &v.AnalysisType, &v.SubjectType, &v.SubjectID, &v.PriceVersion, &v.PricePoints)
	return v, conversationDBError(err)
}
