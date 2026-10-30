package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

type AIConversationDossierSchemaVerifier struct{ db *sql.DB }
type AIConversationDossierSchemaError struct {
	Stage int
	cause error
}

func (e *AIConversationDossierSchemaError) Error() string {
	return fmt.Sprintf("会话档案保护检查阶段%d失败（详情已隐藏）", e.Stage)
}
func (e *AIConversationDossierSchemaError) Unwrap() error { return e.cause }
func NewAIConversationDossierSchemaVerifier(db *sql.DB) *AIConversationDossierSchemaVerifier {
	return &AIConversationDossierSchemaVerifier{db}
}

// 仅供准备账号；随机探针只回滚，但两个自增序列可能消耗编号。
func (v *AIConversationDossierSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return &AIConversationDossierSchemaError{1, err}
	}
	defer func() { _ = tx.Rollback() }()
	for i, check := range []func(context.Context, *sql.Tx) error{verifyPointsSession, verifyConversationDossierMetadata, probeConversationDossiers} {
		if err = check(ctx, tx); err != nil {
			return &AIConversationDossierSchemaError{i + 2, err}
		}
	}
	return tx.Rollback()
}
func verifyConversationDossierMetadata(ctx context.Context, tx *sql.Tx) error {
	const table = "tm_ai_conversation_dossiers"
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name='chk_tm_ai_conversation_dossier' AND c.constraint_type='CHECK' AND c.enforced='YES'`, table).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return dossier.ErrInvalidDraft
	}
	for _, c := range []struct{ name, kind, nullable, extra string }{
		{"id", "bigint unsigned", "NO", "auto_increment"}, {"conversation_id", "bigint unsigned", "NO", ""}, {"user_id", "bigint unsigned", "NO", ""}, {"theme_id", "bigint unsigned", "NO", ""}, {"version_no", "bigint unsigned", "NO", ""}, {"compare_version", "bigint unsigned", "YES", ""}, {"compare_key", "bigint", "YES", "STORED GENERATED"}, {"created_at", "datetime(3)", "NO", ""}, {"updated_at", "datetime(3)", "NO", ""},
	} {
		var kind, nullable, extra, expr string
		if err := tx.QueryRowContext(ctx, `SELECT column_type,is_nullable,extra,COALESCE(generation_expression,'') FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, c.name).Scan(&kind, &nullable, &extra, &expr); err != nil {
			return err
		}
		if kind != c.kind || nullable != c.nullable || extra != c.extra {
			return dossier.ErrInvalidDraft
		}
		if c.name == "compare_key" {
			if qaCompactSQL(expr) != "coalesce(compare_version,0)" {
				return dossier.ErrInvalidDraft
			}
		} else if expr != "" {
			return dossier.ErrInvalidDraft
		}
	}
	for _, index := range []struct {
		name, columns string
		unique        int
	}{
		{"PRIMARY", "id", 0}, {"uq_tm_ai_conversation_dossier_selection", "conversation_id,theme_id,version_no,compare_key", 0}, {"idx_tm_ai_conversation_dossier_owner", "user_id,conversation_id,id", 1},
	} {
		var columns string
		var nonUnique int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','),''),COALESCE(MAX(non_unique),-1) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND index_type='BTREE' AND is_visible='YES'`, table, index.name).Scan(&columns, &nonUnique); err != nil {
			return err
		}
		if columns != index.columns || nonUnique != index.unique {
			return dossier.ErrInvalidDraft
		}
	}
	for _, fk := range []struct{ name, parent, columns string }{
		{"fk_tm_ai_conversation_dossier_owner", "tm_ai_conversations", "conversation_id:id,user_id:user_id"}, {"fk_tm_ai_conversation_dossier_version", "tm_ai_dossier_versions", "theme_id:theme_id,version_no:version_no"}, {"fk_tm_ai_conversation_dossier_compare", "tm_ai_dossier_versions", "theme_id:theme_id,compare_version:version_no"},
	} {
		var columns string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, table, fk.name, fk.parent).Scan(&columns); err != nil {
			return err
		}
		if columns != fk.columns {
			return dossier.ErrInvalidDraft
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND event_object_table=?`, table).Scan(&count); err != nil {
		return err
	}
	if count != 3 {
		return dossier.ErrInvalidDraft
	}
	for _, trigger := range []struct{ name, event, body string }{
		{"tm_ai_conversation_dossier_insert", "INSERT", `BEGIN DECLARE actual_archived tinyint DEFAULT NULL; SELECT archived INTO actual_archived FROM tm_ai_conversations WHERE id=NEW.conversation_id AND user_id=NEW.user_id FOR SHARE; IF actual_archived IS NULL OR actual_archived<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Dossier reference requires active owned conversation'; END IF; END`},
		{"tm_ai_conversation_dossier_no_update", "UPDATE", `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation dossier references are immutable'`},
		{"tm_ai_conversation_dossier_no_delete", "DELETE", `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation dossier references cannot be deleted'`},
	} {
		var event, timing, body string
		if err := tx.QueryRowContext(ctx, `SELECT event_manipulation,action_timing,action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND event_object_table=? AND trigger_name=?`, table, trigger.name).Scan(&event, &timing, &body); err != nil {
			return err
		}
		if event != trigger.event || timing != "BEFORE" || qaCompactSQL(body) != qaCompactSQL(trigger.body) {
			return dossier.ErrInvalidDraft
		}
	}
	return nil
}
func probeConversationDossiers(ctx context.Context, tx *sql.Tx) error {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return err
	}
	uid := int64(binary.BigEndian.Uint64(seed[:8])&((1<<50)-1)) + 1
	key := "ref_probe_" + hex.EncodeToString(seed[:])
	hash := conversation.CreationHash("仅测试：引用保护")
	r, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversations(user_id,idem_key,req_hash,title,archived,version,last_seq,created_at,updated_at) VALUES(?,?,?,'仅测试：引用保护',0,1,0,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, key, hash)
	if err != nil {
		return err
	}
	cid, err := r.LastInsertId()
	if err != nil {
		return err
	}
	for _, v := range []int64{1, 2} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_versions(theme_id,version_no,digest,canonical,compiled_at,created_at,updated_at) VALUES(?,?,?,'{}',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, v, hash); err != nil {
			return err
		}
	}
	const insert = `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`
	if _, err = tx.ExecContext(ctx, insert, cid, uid, uid, 1, nil); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, insert, cid, uid, uid, 1, 2); err != nil {
		return err
	}
	for _, args := range [][]any{{cid, uid, uid, 1, nil}, {cid, uid, uid, 1, 2}} {
		if err = qaProbeReject(tx, ctx, 1062, insert, args...); err != nil {
			return err
		}
	}
	for _, args := range [][]any{{cid, uid, uid, 1, 1}, {cid, uid, uid, 2, 1}} {
		if err = qaProbeReject(tx, ctx, 3819, insert, args...); err != nil {
			return err
		}
	}
	for _, args := range [][]any{{cid, uid, uid, 3, nil}, {cid, uid, uid, 1, 3}} {
		if err = qaProbeReject(tx, ctx, 1452, insert, args...); err != nil {
			return err
		}
	}
	for _, q := range []string{`UPDATE tm_ai_conversation_dossiers SET version_no=version_no WHERE conversation_id=?`, `DELETE FROM tm_ai_conversation_dossiers WHERE conversation_id=?`} {
		if err = qaProbeReject(tx, ctx, 1644, q, cid); err != nil {
			return err
		}
	}
	if err = qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,created_at,updated_at) VALUES(?,?,?,2,'1970-01-01','1970-01-01')`, cid, uid, uid); err != nil {
		return err
	}
	if err = qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,created_at,updated_at) VALUES(?,?,?,2,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)+INTERVAL 1 SECOND)`, cid, uid, uid); err != nil {
		return err
	}
	if err = qaProbeReject(tx, ctx, 1644, insert, cid, uid+1, uid, 2, nil); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tm_ai_conversations SET archived=1,version=2,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND user_id=?`, cid, uid); err != nil {
		return err
	}
	return qaProbeReject(tx, ctx, 1644, insert, cid, uid, uid, 2, nil)
}

// 只检查列读取与空INSERT权限，不回取正文、不插入行；不能替代准备账号的结构/行为探针。
func (r *AIConversationDossierRepo) VerifyAccess(parent context.Context) error {
	if r == nil || r.db == nil || parent == nil || (r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead) {
		return dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	for _, q := range []string{`SELECT ` + conversationDossierColumns + `,compare_key FROM tm_ai_conversation_dossiers WHERE 1=0 FOR SHARE`, `SELECT ` + conversationColumns + ` FROM tm_ai_conversations WHERE 1=0 FOR UPDATE`} {
		rows, err := tx.QueryContext(ctx, q)
		if err != nil {
			return dossierDBError(err)
		}
		if err = rows.Close(); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) SELECT 0,0,0,0,NULL,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) WHERE 1=0`)
	if err != nil {
		return dossierDBError(err)
	}
	return tx.Rollback()
}
