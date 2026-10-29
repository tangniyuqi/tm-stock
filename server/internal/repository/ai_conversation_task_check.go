package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

type AIConversationTaskSchemaVerifier struct {
	db         *sql.DB
	local      bool
	instanceID string
}

func NewAIConversationTaskSchemaVerifier(db *sql.DB) *AIConversationTaskSchemaVerifier {
	return &AIConversationTaskSchemaVerifier{db: db}
}

func NewAILocalConversationTaskSchemaVerifier(db *sql.DB, instanceID string) *AIConversationTaskSchemaVerifier {
	return &AIConversationTaskSchemaVerifier{db: db, local: true, instanceID: instanceID}
}
func (v *AIConversationTaskSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return conversation.ErrInvalid
	}
	base := NewAIConversationSchemaVerifier(v.db)
	if v.local {
		base = NewAILocalConversationSchemaVerifier(v.db, v.instanceID)
	}
	if err := base.Verify(parent); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // 回滚探针不创建任务、不触碰账务；不能代替完整联调。
	metadata := verifyIntentMetadata
	if v.local {
		metadata = verifyLocalIntentMetadata
	}
	for n, fn := range []func(context.Context, *sql.Tx) error{verifyPointsSession, metadata, probeIntents} {
		if err := fn(ctx, tx); err != nil {
			return &AIConversationSchemaError{Stage: n + 1, cause: err}
		}
	}
	return tx.Rollback()
}
func verifyIntentMetadata(ctx context.Context, tx *sql.Tx) error {
	return verifyIntentMetadataFrom(ctx, tx, false)
}
func verifyLocalIntentMetadata(ctx context.Context, tx *sql.Tx) error {
	return verifyIntentMetadataFrom(ctx, tx, true)
}
func verifyIntentMetadataFrom(ctx context.Context, tx *sql.Tx, local bool) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name='tm_ai_conversation_tasks' AND t.engine='InnoDB' AND c.constraint_name='chk_tm_ai_intent' AND c.constraint_type='CHECK' AND c.enforced='YES'`).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return conversation.ErrInvariant
	}
	for _, col := range []struct{ name, kind, nullable, collation string }{
		{"id", "bigint unsigned", "NO", ""}, {"conversation_id", "bigint unsigned", "NO", ""}, {"user_id", "bigint unsigned", "NO", ""},
		{"idem_key", "varchar(80)", "NO", "ascii_bin"}, {"req_hash", "char(64)", "NO", "ascii_bin"}, {"task_key", "varchar(80)", "NO", "ascii_bin"},
		{"analysis_type", "varchar(5)", "NO", "ascii_bin"}, {"subject_type", "varchar(8)", "NO", "ascii_bin"}, {"subject_id", "bigint unsigned", "NO", ""},
		{"price_ver", "varchar(32)", "NO", "ascii_bin"}, {"price_points", "bigint", "NO", ""}, {"state", "varchar(11)", "NO", "ascii_bin"}, {"rejection_code", "varchar(24)", "NO", "ascii_bin"},
		{"version", "bigint", "NO", ""}, {"task_id", "bigint unsigned", "YES", ""}, {"message_seq", "bigint", "YES", ""}, {"created_at", "datetime(3)", "NO", ""}, {"updated_at", "datetime(3)", "NO", ""},
	} {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_conversation_tasks' AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=?`, col.name, col.kind, col.nullable, col.collation).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return conversation.ErrInvariant
		}
	}
	for _, index := range []struct {
		name, columns string
		unique        int
	}{
		{"PRIMARY", "id", 0}, {"uq_tm_ai_intent_key", "conversation_id,idem_key", 0}, {"uq_tm_ai_intent_task_key", "user_id,task_key", 0}, {"uq_tm_ai_intent_task", "task_id", 0}, {"uq_tm_ai_intent_message", "conversation_id,message_seq", 0}, {"idx_tm_ai_intent_history", "user_id,conversation_id,id", 1}, {"idx_tm_ai_intent_archive", "conversation_id,state,id", 1},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tm_ai_conversation_tasks' AND index_name=? AND non_unique=? AND is_visible='YES'`, index.name, index.unique).Scan(&value); err != nil {
			return err
		}
		if value != index.columns {
			return conversation.ErrInvariant
		}
	}
	for _, fk := range []struct{ name, parent, columns string }{
		{"fk_tm_ai_intent_owner", "tm_ai_conversations", "conversation_id:id,user_id:user_id"}, {"fk_tm_ai_intent_task", "tm_ai_tasks", "task_id:id,user_id:user_id"}, {"fk_tm_ai_intent_message", "tm_ai_conversation_messages", "conversation_id:conversation_id,message_seq:seq"},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name='tm_ai_conversation_tasks' AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, fk.name, fk.parent).Scan(&value); err != nil {
			return err
		}
		if value != fk.columns {
			return conversation.ErrInvariant
		}
	}
	localTriggers := map[string]int{}
	if local {
		var err error
		localTriggers, err = readLocalConversationTriggers(ctx, tx, true)
		if err != nil {
			return err
		}
	}
	for _, trigger := range []struct{ table, name, event string }{{"tm_ai_conversation_tasks", "tm_ai_intent_insert", "INSERT"}, {"tm_ai_conversation_tasks", "tm_ai_intent_update", "UPDATE"}, {"tm_ai_conversation_tasks", "tm_ai_intent_no_delete", "DELETE"}, {"tm_ai_conversations", "tm_ai_conversation_intent_archive_guard", "UPDATE"}} {
		if local {
			n = localTriggers[trigger.table+":"+trigger.name+":"+trigger.event]
		} else if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND event_object_table=? AND trigger_name=? AND event_manipulation=? AND action_timing='BEFORE'`, trigger.table, trigger.name, trigger.event).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return conversation.ErrInvariant
		}
	}
	return nil
}
func probeIntents(ctx context.Context, tx *sql.Tx) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	uid := int64(binary.BigEndian.Uint64(random[:8])&((1<<50)-1)) + 1
	key := "intent_probe_" + hex.EncodeToString(random[:])
	t := &aiConversationIntentTx{&aiConversationTx{tx: tx, write: true}}
	now, err := t.Now(ctx)
	if err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversations(user_id,idem_key,req_hash,title,archived,version,last_seq,created_at,updated_at) VALUES(?,?,?,'隔离任务意图探针',0,1,0,?,?)`, uid, key, conversation.CreationHash("隔离任务意图探针"), pointsUTC(now), pointsUTC(now))
	if err != nil {
		return err
	}
	cid, err := r.LastInsertId()
	if err != nil {
		return err
	}
	in := conversation.IntentInput{AnalysisType: "light", SubjectType: "theme", SubjectID: 1, PriceVersion: "probe-v1"}
	hash, err := in.Digest(uid, cid)
	if err != nil {
		return err
	}
	taskKey, err := conversation.NewTaskKey()
	if err != nil {
		return err
	}
	i, err := t.InsertIntent(ctx, conversation.Intent{ConversationID: cid, UserID: uid, Key: key, RequestHash: hash, TaskKey: taskKey, Input: in, PricePoints: 1, State: conversation.Prepared, Version: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return err
	}
	replay, err := t.InsertIntent(ctx, i)
	if err != nil || replay.ID != i.ID {
		return conversation.ErrInvariant
	}
	for _, q := range []string{`DELETE FROM tm_ai_conversation_tasks WHERE id=?`, `UPDATE tm_ai_conversation_tasks SET price_points=2 WHERE id=?`, `UPDATE tm_ai_conversation_tasks SET task_key=CONCAT(task_key,'x') WHERE id=?`, `UPDATE tm_ai_conversation_tasks SET state='not_created',version=3,rejection_code='price_changed' WHERE id=?`} {
		if _, err := tx.ExecContext(ctx, q, i.ID); !pointsProbeError(err, 1644) {
			return conversation.ErrInvariant
		}
	}
	next := i
	next.State = conversation.Unconfirmed
	next.Version = 2
	if err := t.SetIntent(ctx, i, next); err != nil {
		return err
	}
	i = next
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_conversations SET archived=1,version=version+1 WHERE id=?`, cid); !pointsProbeError(err, 1644) {
		return conversation.ErrInvariant
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_conversation_tasks SET state='linked',version=3,task_id=1,message_seq=1 WHERE id=?`, i.ID); !pointsProbeError(err, 1644) {
		return conversation.ErrInvariant
	}
	next = i
	next.State = conversation.NotCreated
	next.Version = 3
	next.RejectionCode = "price_changed"
	if err := t.SetIntent(ctx, i, next); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_conversation_tasks SET state='prepared',version=1,rejection_code='' WHERE id=?`, i.ID); !pointsProbeError(err, 1644) {
		return conversation.ErrInvariant
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_conversations SET archived=1,version=version+1 WHERE id=?`, cid); err != nil {
		return err
	}
	// 有效字段也不能绕过当前归档态；仅检查触发器名字不能识别空实现。
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_tasks(conversation_id,user_id,idem_key,req_hash,task_key,analysis_type,subject_type,subject_id,price_ver,price_points,state,rejection_code,version,created_at,updated_at) VALUES(?,?,?, ?,?,'light','theme',1,'probe-v1',1,'prepared','',1,?,?)`, cid, uid, key+"_archived", hash, taskKey+"x", pointsUTC(now), pointsUTC(now)); !pointsProbeError(err, 1644) {
		return conversation.ErrInvariant
	}
	return nil
}
