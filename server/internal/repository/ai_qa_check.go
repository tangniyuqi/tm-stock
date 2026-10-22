package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/migrations"
)

type AIQASchemaVerifier struct{ db *sql.DB }

func NewAIQASchemaVerifier(db *sql.DB) *AIQASchemaVerifier         { return &AIQASchemaVerifier{db} }
func (v *AIQASchemaVerifier) Verify(ctx context.Context) error     { return v.verify(ctx, false) }
func (v *AIQASchemaVerifier) VerifyRead(ctx context.Context) error { return v.verify(ctx, true) }
func (v *AIQASchemaVerifier) verify(parent context.Context, read bool) error {
	if v == nil || v.db == nil || parent == nil {
		return qa.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: read, Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启问答保护检查失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 只回滚，不运行模型或修改账户/收费任务。
	for _, check := range []func(context.Context, *sql.Tx) error{verifyPointsSession, verifyConversationMetadata, verifyQAMetadata} {
		if err := check(ctx, tx); err != nil {
			return fmt.Errorf("问答保护检查失败: %w", err)
		}
	}
	if !read {
		if err := probeQA(ctx, tx); err != nil {
			return fmt.Errorf("问答回滚探针失败: %w", err)
		}
	}
	return tx.Rollback()
}
func verifyQAMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, v := range []struct{ table, check string }{
		{"tm_ai_qa_scopes", "chk_tm_ai_qa_scope"}, {"tm_ai_qa_limits", "chk_tm_ai_qa_limits"}, {"tm_ai_qa_quotas", "chk_tm_ai_qa_quota"}, {"tm_ai_qa_requests", "chk_tm_ai_qa_request"}, {"tm_ai_qa_calls", "chk_tm_ai_qa_call"},
	} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, v.table, v.check).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("问答表约束%s: %w", v.table, qa.ErrProtocol)
		}
	}
	cols := map[string][]string{
		"tm_ai_qa_scopes":   {"subject_type|varchar(8)|NO|ascii_bin", "subject_id|bigint unsigned|NO|", "subject_name|varchar(128)|NO|utf8mb4_bin", "approval_ref|varchar(128)|NO|utf8mb4_bin", "policy_digest|char(64)|NO|ascii_bin", "evidence_json|text|NO|utf8mb4_bin", "valid_until|datetime(3)|NO|", "enabled|tinyint|NO|", "version|bigint|NO|"},
		"tm_ai_qa_limits":   {"id|tinyint|NO|", "approval_ref|varchar(128)|NO|utf8mb4_bin", "enabled|tinyint|NO|", "user_requests|bigint|NO|", "instance_requests|bigint|NO|", "user_tokens|bigint|NO|", "instance_tokens|bigint|NO|", "version|bigint|NO|"},
		"tm_ai_qa_quotas":   {"quota_day|date|NO|", "user_id|bigint unsigned|NO|", "requests|bigint|NO|", "tokens|bigint|NO|"},
		"tm_ai_qa_requests": {"conversation_id|bigint unsigned|NO|", "seq|bigint|NO|", "user_id|bigint unsigned|NO|", "input_digest|char(64)|NO|ascii_bin", "message_hash|char(64)|NO|ascii_bin", "proof_json|text|NO|utf8mb4_bin", "state|varchar(12)|NO|ascii_bin", "reason|varchar(24)|NO|ascii_bin", "citations_json|text|NO|utf8mb4_bin", "calls|int|NO|", "reserved_tokens|int|NO|", "quota_day|date|YES|", "version|bigint|NO|"},
		"tm_ai_qa_calls":    {"input_digest|char(64)|NO|ascii_bin", "step_no|int|NO|", "call_json|text|NO|utf8mb4_bin", "state|varchar(10)|NO|ascii_bin", "result_json|text|YES|utf8mb4_bin"},
	}
	for table, fields := range cols {
		fields = append(fields, "created_at|datetime(3)|NO|", "updated_at|datetime(3)|NO|")
		for _, spec := range fields {
			f := strings.Split(spec, "|")
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=?`, table, f[0], f[1], f[2], f[3]).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("问答列%s.%s: %w", table, f[0], qa.ErrProtocol)
			}
		}
	}
	for _, table := range []string{"tm_ai_qa_scopes", "tm_ai_qa_limits"} {
		var d string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(column_default,'#null') FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='enabled'`, table).Scan(&d); err != nil {
			return err
		}
		if d != "0" {
			return fmt.Errorf("问答关闭默认值%s: %w", table, qa.ErrProtocol)
		}
	}
	for _, index := range []struct {
		table, name, cols string
		nonUnique         int
	}{
		{"tm_ai_qa_scopes", "PRIMARY", "subject_type,subject_id", 0}, {"tm_ai_qa_limits", "PRIMARY", "id", 0}, {"tm_ai_qa_quotas", "PRIMARY", "quota_day,user_id", 0}, {"tm_ai_qa_requests", "PRIMARY", "conversation_id,seq", 0}, {"tm_ai_qa_requests", "uq_tm_ai_qa_input", "input_digest", 0}, {"tm_ai_qa_requests", "idx_tm_ai_qa_owner", "user_id,conversation_id,seq", 1}, {"tm_ai_qa_calls", "PRIMARY", "input_digest,step_no", 0},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=? AND is_visible='YES'`, index.table, index.name, index.nonUnique).Scan(&value); err != nil {
			return err
		}
		if value != index.cols {
			return fmt.Errorf("问答索引%s: %w", index.name, qa.ErrProtocol)
		}
	}
	for _, fk := range []struct{ table, name, parent, cols string }{
		{"tm_ai_qa_requests", "fk_tm_ai_qa_owner", "tm_ai_conversations", "conversation_id:id,user_id:user_id"}, {"tm_ai_qa_requests", "fk_tm_ai_qa_message", "tm_ai_conversation_messages", "conversation_id:conversation_id,seq:seq"}, {"tm_ai_qa_calls", "fk_tm_ai_qa_call", "tm_ai_qa_requests", "input_digest:input_digest"},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, fk.table, fk.name, fk.parent).Scan(&value); err != nil {
			return err
		}
		if value != fk.cols {
			return fmt.Errorf("问答外键%s: %w", fk.name, qa.ErrProtocol)
		}
	}
	defs, err := qaTriggerDefinitions(migrations.QASchema)
	if err != nil {
		return err
	}
	for _, d := range defs {
		var table, event, timing, body string
		if err := tx.QueryRowContext(ctx, `SELECT event_object_table,event_manipulation,action_timing,action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name=?`, d.name).Scan(&table, &event, &timing, &body); err != nil {
			return err
		}
		if table != d.table || event != d.event || timing != "BEFORE" || qaCompactSQL(body) != qaCompactSQL(d.body) {
			return fmt.Errorf("问答触发器定义%s: %w", d.name, qa.ErrProtocol)
		}
	}
	return nil
}

type qaTriggerDefinition struct{ name, table, event, body string }

// 只解析本仓固定格式DDL，不作为通用SQL解析器；定义与迁移使用同一编译期源。
func qaTriggerDefinitions(raw string) ([]qaTriggerDefinition, error) {
	parts := strings.Split(raw, "CREATE TRIGGER ")
	if len(parts) != 16 {
		return nil, qa.ErrProtocol
	}
	out := []qaTriggerDefinition{}
	for _, part := range parts[1:] {
		pos := strings.Index(part, "FOR EACH ROW")
		if pos < 0 {
			return nil, qa.ErrProtocol
		}
		f := strings.Fields(part[:pos])
		if len(f) != 5 || f[1] != "BEFORE" || f[3] != "ON" {
			return nil, qa.ErrProtocol
		}
		body := strings.TrimSpace(part[pos+len("FOR EACH ROW"):])
		var end int
		if strings.HasPrefix(body, "BEGIN") {
			end = strings.Index(body, "END$$")
			if end < 0 {
				return nil, qa.ErrProtocol
			}
			body = body[:end+3]
		} else {
			end = strings.Index(body, ";")
			if end < 0 {
				return nil, qa.ErrProtocol
			}
			body = body[:end]
		}
		out = append(out, qaTriggerDefinition{f[0], f[4], f[2], body})
	}
	return out, nil
}
func qaCompactSQL(s string) string {
	var out strings.Builder
	quoted := false
	for _, r := range s {
		if r == '\'' {
			quoted = !quoted
			out.WriteRune(r)
			continue
		}
		if !quoted {
			if unicode.IsSpace(r) || r == '`' {
				continue
			}
			r = unicode.ToLower(r)
		}
		out.WriteRune(r)
	}
	return strings.TrimSuffix(out.String(), ";") // 单条触发器在information_schema保留结尾分号，迁移分隔符不属于语义。
}

func qaProbeReject(tx *sql.Tx, ctx context.Context, code uint16, query string, args ...any) error {
	_, err := tx.ExecContext(ctx, query, args...)
	var m *mysqldrv.MySQLError
	if !errors.As(err, &m) || m.Number != code {
		return qa.ErrProtocol
	}
	return nil
}
func probeQA(ctx context.Context, tx *sql.Tx) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	uid := int64(binary.BigEndian.Uint64(random[:8])&((1<<50)-1)) + 1
	key := "qa_probe_" + hex.EncodeToString(random[:])
	hash := task.StageDigest(random[:])
	base := &aiConversationTx{tx: tx, write: true}
	qtx := &aiQATx{base}
	now, err := base.Now(ctx)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_conversations(user_id,idem_key,req_hash,title,archived,version,last_seq,created_at,updated_at) VALUES(?,?,?,'问答回滚探针',0,1,0,?,?)`, uid, key, conversation.CreationHash("问答回滚探针"), pointsUTC(now), pointsUTC(now))
	if err != nil {
		return err
	}
	cid, err := result.LastInsertId()
	if err != nil {
		return err
	}
	conv, err := base.ByID(ctx, uid, cid, true)
	if err != nil {
		return err
	}
	next := conv
	next.Version++
	next.LastSeq++
	if err := base.Update(ctx, conv, next); err != nil {
		return err
	}
	in := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: uid, Skill: "material_qa", Question: "仅检查问答存储保护"}
	mh, err := in.Digest(uid, cid)
	if err != nil {
		return err
	}
	if err := base.Append(ctx, conversation.Message{UserID: uid, ConversationID: cid, Seq: 1, Key: key, RequestHash: mh, Input: in, CreatedAt: now}); err != nil {
		return err
	}
	m := qa.Model{Name: "schema-probe", FilingNo: "schema-probe", ApprovalRef: "schema-probe", QualityRef: "schema-probe", BudgetRef: "schema-probe", ProfileDigest: hash}
	p, err := qa.NewPlan(qa.Identity{UserID: uid, ConversationID: cid, MessageSeq: 1}, in, qa.Scope{SubjectType: "theme", SubjectID: uid, SubjectName: "问答存储探针", ApprovalRef: key, ValidUntil: now.Add(time.Hour)}, qa.Config{Selector: m, Entailment: m, Semantic: m, AllowedDomains: []string{"example.test"}, Limits: qa.Limits{MaxSources: 1, MaxSentences: 1, MaxEvidenceBytes: 1, MaxPromptBytes: 8192, MaxInputTokens: 8192, MaxOutputTokens: 128, MaxTotalTokens: 24960, Timeout: time.Second}}, now)
	if err != nil {
		return err
	}
	r := qa.Record{Proof: p.Proof(), MessageHash: mh, State: "queued", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := qtx.InsertQuestion(ctx, r); err != nil {
		return err
	}
	for _, query := range []string{`UPDATE tm_ai_qa_requests SET message_hash=REPEAT('a',64) WHERE conversation_id=?`, `DELETE FROM tm_ai_qa_requests WHERE conversation_id=?`} {
		if err := qaProbeReject(tx, ctx, 1644, query, cid); err != nil {
			return err
		}
	}
	day := now.Format("2006-01-02")
	quota, err := qtx.Quota(ctx, day, uid)
	if err != nil {
		return err
	}
	if err := qtx.AddQuota(ctx, quota, 24960); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 1644, `UPDATE tm_ai_qa_quotas SET tokens=tokens-1 WHERE quota_day=? AND user_id=?`, day, uid); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_qa_quotas(quota_day,user_id,requests,tokens,created_at,updated_at) VALUES(?,9007199254740992,0,0,?,?)`, day, pointsUTC(now), pointsUTC(now)); err != nil {
		return err
	}
	old := r
	r.State = "running"
	r.QuotaDay = day
	r.Version++
	if err := qtx.SetQuestion(ctx, old, r); err != nil {
		return err
	}
	call := qa.CallRecord{Call: qa.Call{InputDigest: p.Proof().InputDigest, Stage: "select", Step: 1, RequestDigest: hash, ProfileDigest: hash, Model: m.Name, MaxTokens: 8320}, State: "reserved", CreatedAt: now, UpdatedAt: now}
	if err := qtx.InsertCall(ctx, call); err != nil {
		return err
	}
	old = r
	r.Calls = 1
	r.ReservedTokens = 8320
	r.Version++
	if err := qtx.SetQuestion(ctx, old, r); err != nil {
		return err
	}
	if err := qtx.FinishCall(ctx, call, qa.CallResult{Failure: "not_sent"}, now); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 1644, `UPDATE tm_ai_qa_calls SET result_json='{}' WHERE input_digest=?`, r.Proof.InputDigest); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 1644, `DELETE FROM tm_ai_qa_calls WHERE input_digest=?`, r.Proof.InputDigest); err != nil {
		return err
	}
	badCall := call.Call
	badCall.Step = 2
	badCall.Stage = "entailment"
	rawCall, err := json.Marshal(badCall)
	if err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_qa_calls(input_digest,step_no,call_json,state,result_json,created_at,updated_at) VALUES(?,2,?,'reserved','{}',?,?)`, r.Proof.InputDigest, rawCall, pointsUTC(now), pointsUTC(now)); err != nil {
		return err
	}
	old = r
	r.State = "failed"
	r.Reason = "execution_failed"
	r.Version++
	if err := qtx.SetQuestion(ctx, old, r); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 1644, `UPDATE tm_ai_qa_requests SET state='running',reason='',version=version+1 WHERE conversation_id=?`, cid); err != nil {
		return err
	}
	second := next
	second.LastSeq++
	second.Version++
	if err := base.Update(ctx, next, second); err != nil {
		return err
	}
	if err := base.Append(ctx, conversation.Message{UserID: uid, ConversationID: cid, Seq: 2, Key: key + "_2", RequestHash: mh, Input: in, CreatedAt: now}); err != nil {
		return err
	}
	badProof := p.Proof()
	badProof.Identity.MessageSeq = 2
	badProof.InputDigest = hash
	proofJSON, err := json.Marshal(badProof)
	if err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_qa_requests(conversation_id,seq,user_id,input_digest,message_hash,proof_json,state,reason,citations_json,calls,reserved_tokens,quota_day,version,created_at,updated_at) VALUES(?,2,?,?,?,?,'queued','invalid','null',0,0,NULL,1,?,?)`, cid, uid, hash, mh, proofJSON, pointsUTC(now), pointsUTC(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_qa_scopes(subject_type,subject_id,subject_name,approval_ref,policy_digest,evidence_json,valid_until,enabled,version,created_at,updated_at) VALUES('theme',?,'回滚探针',?,?,'[]',?,0,1,?,?)`, uid, key, hash, pointsUTC(now.Add(time.Hour)), pointsUTC(now), pointsUTC(now)); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 3819, `UPDATE tm_ai_qa_scopes SET subject_name='',version=version+1 WHERE subject_type='theme' AND subject_id=?`, uid); err != nil {
		return err
	}
	if err := qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(2,?,0,1,1,1,1,1,?,?)`, key, pointsUTC(now), pointsUTC(now)); err != nil {
		return err
	}
	// 单例政策若已有则不更新/删除；定义比对保护触发器，CHECK用非法id反例，不接管运营配置。
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_qa_limits WHERE id=1`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,?,0,1,1,1,1,1,?,?)`, key, pointsUTC(now), pointsUTC(now)); err != nil {
			return err
		}
	}
	return nil
}
