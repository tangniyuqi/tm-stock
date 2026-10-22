package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

type AIConversationSchemaVerifier struct{ db *sql.DB }

func NewAIConversationSchemaVerifier(db *sql.DB) *AIConversationSchemaVerifier {
	return &AIConversationSchemaVerifier{db}
}
func (v *AIConversationSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启会话保护探针失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 探针从不提交，不能创建真实会话或触碰账务。
	for index, check := range []func(context.Context, *sql.Tx) error{verifyPointsSession, verifyConversationMetadata, probeConversations} {
		if err := check(ctx, tx); err != nil {
			return fmt.Errorf("会话保护检查阶段%d失败: %w", index+1, err)
		}
	}
	return tx.Rollback()
}

func verifyConversationMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []struct{ name, check string }{
		{"tm_ai_conversations", "chk_tm_ai_conversation"}, {"tm_ai_conversation_messages", "chk_tm_ai_conversation_message"},
	} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, table.name, table.check).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return conversation.ErrInvariant
		}
	}
	for _, col := range []struct{ table, name, kind, nullable, collation string }{
		{"tm_ai_conversations", "id", "bigint unsigned", "NO", ""}, {"tm_ai_conversations", "user_id", "bigint unsigned", "NO", ""},
		{"tm_ai_conversations", "idem_key", "varchar(80)", "NO", "ascii_bin"}, {"tm_ai_conversations", "req_hash", "char(64)", "NO", "ascii_bin"}, {"tm_ai_conversations", "title", "varchar(40)", "NO", "utf8mb4_bin"},
		{"tm_ai_conversations", "archived", "tinyint", "NO", ""}, {"tm_ai_conversations", "version", "bigint", "NO", ""}, {"tm_ai_conversations", "last_seq", "bigint", "NO", ""},
		{"tm_ai_conversations", "created_at", "datetime(3)", "NO", ""}, {"tm_ai_conversations", "updated_at", "datetime(3)", "NO", ""},
		{"tm_ai_conversation_messages", "conversation_id", "bigint unsigned", "NO", ""}, {"tm_ai_conversation_messages", "user_id", "bigint unsigned", "NO", ""}, {"tm_ai_conversation_messages", "seq", "bigint", "NO", ""},
		{"tm_ai_conversation_messages", "idem_key", "varchar(80)", "NO", "ascii_bin"}, {"tm_ai_conversation_messages", "req_hash", "char(64)", "NO", "ascii_bin"}, {"tm_ai_conversation_messages", "kind", "varchar(8)", "NO", "ascii_bin"},
		{"tm_ai_conversation_messages", "subject_type", "varchar(8)", "NO", "ascii_bin"}, {"tm_ai_conversation_messages", "subject_id", "bigint unsigned", "NO", ""},
		{"tm_ai_conversation_messages", "skill", "varchar(12)", "NO", "ascii_bin"}, {"tm_ai_conversation_messages", "question", "varchar(1000)", "NO", "utf8mb4_bin"}, {"tm_ai_conversation_messages", "task_id", "bigint unsigned", "YES", ""},
		{"tm_ai_conversation_messages", "created_at", "datetime(3)", "NO", ""}, {"tm_ai_conversation_messages", "updated_at", "datetime(3)", "NO", ""},
	} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=?`, col.table, col.name, col.kind, col.nullable, col.collation).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return conversation.ErrInvariant
		}
	}
	for _, index := range []struct {
		table, name, columns string
		unique               int
	}{
		{"tm_ai_conversations", "PRIMARY", "id", 0}, {"tm_ai_conversations", "uq_tm_ai_conversation_owner", "id,user_id", 0}, {"tm_ai_conversations", "uq_tm_ai_conversation_key", "user_id,idem_key", 0}, {"tm_ai_conversations", "idx_tm_ai_conversation_list", "user_id,archived,id", 1},
		{"tm_ai_conversation_messages", "PRIMARY", "conversation_id,seq", 0}, {"tm_ai_conversation_messages", "uq_tm_ai_conversation_message_key", "conversation_id,idem_key", 0},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=? AND is_visible='YES'`, index.table, index.name, index.unique).Scan(&value); err != nil {
			return err
		}
		if value != index.columns {
			return conversation.ErrInvariant
		}
	}
	for _, fk := range []struct{ name, parent, columns string }{
		{"fk_tm_ai_conversation_message_owner", "tm_ai_conversations", "conversation_id:id,user_id:user_id"}, {"fk_tm_ai_conversation_message_task", "tm_ai_tasks", "task_id:id,user_id:user_id"},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name='tm_ai_conversation_messages' AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, fk.name, fk.parent).Scan(&value); err != nil {
			return err
		}
		if value != fk.columns {
			return conversation.ErrInvariant
		}
	}
	for _, trigger := range []struct{ table, name, event string }{
		{"tm_ai_conversations", "tm_ai_conversation_insert", "INSERT"}, {"tm_ai_conversations", "tm_ai_conversation_update", "UPDATE"}, {"tm_ai_conversations", "tm_ai_conversation_no_delete", "DELETE"},
		{"tm_ai_conversation_messages", "tm_ai_conversation_message_insert", "INSERT"}, {"tm_ai_conversation_messages", "tm_ai_conversation_message_no_update", "UPDATE"}, {"tm_ai_conversation_messages", "tm_ai_conversation_message_no_delete", "DELETE"},
	} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND event_object_table=? AND trigger_name=? AND event_manipulation=? AND action_timing='BEFORE'`, trigger.table, trigger.name, trigger.event).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return conversation.ErrInvariant
		}
	}
	return nil
}

func probeConversations(ctx context.Context, tx *sql.Tx) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	uid := int64(binary.BigEndian.Uint64(random[:8])&((1<<50)-1)) + 1
	key := "conv_probe_" + hex.EncodeToString(random[:])
	t := &aiConversationTx{tx: tx, write: true}
	now, err := t.Now(ctx)
	if err != nil {
		return err
	}
	// 直接INSERT：极小概率撞键也必须失败，不能接管任何已有记录。
	res, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversations(user_id,idem_key,req_hash,title,archived,version,last_seq,created_at,updated_at) VALUES(?,?,?,'隔离回滚探针',0,1,0,?,?)`, uid, key, conversation.CreationHash("隔离回滚探针"), pointsUTC(now), pointsUTC(now))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	r, err := t.ByID(ctx, uid, id, true)
	if err != nil {
		return err
	}
	replay, err := t.CreateOrGet(ctx, r)
	if err != nil || replay.ID != id {
		return conversation.ErrInvariant
	}
	check := func(err error, number uint16) error {
		if !pointsProbeError(err, number) {
			return conversation.ErrInvariant
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversations(user_id,idem_key,req_hash,title,archived,version,last_seq,created_at,updated_at) VALUES(?,?,?,'非法初态探针',0,2,0,?,?)`, uid, key+"_initial", conversation.CreationHash("非法初态探针"), pointsUTC(now), pointsUTC(now)); check(err, 1644) != nil {
		return conversation.ErrInvariant
	}
	for _, query := range []string{
		`DELETE FROM tm_ai_conversations WHERE id=?`,
		`UPDATE tm_ai_conversations SET user_id=user_id+1 WHERE id=?`,
		`UPDATE tm_ai_conversations SET title='越过版本' WHERE id=?`,
		`UPDATE tm_ai_conversations SET version=version+2,title='跳过版本' WHERE id=?`,
	} {
		if _, err := tx.ExecContext(ctx, query, id); check(err, 1644) != nil {
			return conversation.ErrInvariant
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_conversations SET title='',version=version+1 WHERE id=?`, id); check(err, 3819) != nil {
		return conversation.ErrInvariant
	}
	next := r
	next.Title = "隔离改名探针"
	next.Version++
	if err := t.Update(ctx, r, next); err != nil {
		return err
	}
	r = next
	next = r
	next.LastSeq++
	next.Version++
	if err := t.Update(ctx, r, next); err != nil {
		return err
	}
	r = next
	input := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "隔离探针，仅验证存储，不是模型回答"}
	hash, err := input.Digest(uid, id)
	if err != nil {
		return err
	}
	m := conversation.Message{ConversationID: id, UserID: uid, Seq: 1, Key: key, RequestHash: hash, Input: input, CreatedAt: now}
	if err := t.Append(ctx, m); err != nil {
		return err
	}
	if err := t.Append(ctx, m); !errors.Is(err, conversation.ErrConflict) {
		return conversation.ErrInvariant
	}
	for _, query := range []string{`UPDATE tm_ai_conversation_messages SET question=question WHERE conversation_id=?`, `DELETE FROM tm_ai_conversation_messages WHERE conversation_id=?`} {
		if _, err := tx.ExecContext(ctx, query, id); check(err, 1644) != nil {
			return conversation.ErrInvariant
		}
	}
	// 证明消息CHECK不是同名恒真式：先合法推进序号，再尝试非白名单消息。
	next = r
	next.LastSeq++
	next.Version++
	if err := t.Update(ctx, r, next); err != nil {
		return err
	}
	r = next
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_messages(conversation_id,user_id,seq,idem_key,req_hash,kind,subject_type,subject_id,skill,question,task_id,created_at,updated_at) VALUES(?,?,2,?,?,'task','theme',1,'light','',NULL,?,?)`, id, uid, key+"_null", strings.Repeat("a", 64), pointsUTC(now), pointsUTC(now)); check(err, 3819) != nil {
		return conversation.ErrInvariant
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_messages(conversation_id,user_id,seq,idem_key,req_hash,kind,subject_type,subject_id,skill,question,created_at,updated_at) VALUES(?,?,2,?,?,'raw','theme',1,'material_qa','非白名单探针',?,?)`, id, uid, key+"_bad", strings.Repeat("a", 64), pointsUTC(now), pointsUTC(now)); check(err, 3819) != nil {
		return conversation.ErrInvariant
	}
	m.Seq = 2
	m.Key = key + "_second"
	if err := t.Append(ctx, m); err != nil {
		return err
	}
	wrong := m
	wrong.Seq = 3
	wrong.Key = key + "_wrong"
	if err := t.Append(ctx, wrong); check(err, 1644) != nil {
		return conversation.ErrInvariant
	}
	next = r
	next.Archived = true
	next.Version++
	if err := t.Update(ctx, r, next); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_ai_conversations SET title='归档后修改',version=version+1 WHERE id=?`, id); check(err, 1644) != nil {
		return conversation.ErrInvariant
	}
	return nil
}
