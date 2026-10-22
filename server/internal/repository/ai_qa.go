package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type AIQARepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAIQARepo(db *sql.DB) *AIQARepo { return NewAIQARepoWithIsolation(db, sql.LevelReadCommitted) }
func NewAIQARepoWithIsolation(db *sql.DB, level sql.IsolationLevel) *AIQARepo {
	return &AIQARepo{db, level}
}

var _ qa.Store = (*AIQARepo)(nil)

type aiQATx struct{ *aiConversationTx }

func (r *AIQARepo) WithinQA(ctx context.Context, write bool, fn func(qa.Tx) error) error {
	if r == nil || r.db == nil || ctx == nil || fn == nil || r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead {
		return qa.ErrInvalid
	}
	return NewAIConversationRepoWithIsolation(r.db, r.isolation).Within(ctx, write, func(tx conversation.Tx) error { return fn(&aiQATx{tx.(*aiConversationTx)}) })
}
func qaDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return qa.ErrNotFound
	}
	var me *mysqldrv.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return qa.ErrConflict
	}
	return fmt.Errorf("问答存储操作未确认: %w", err)
}
func sameQAJSON(a, b any) bool {
	x, e := json.Marshal(a)
	if e != nil {
		return false
	}
	y, e := json.Marshal(b)
	return e == nil && string(x) == string(y)
}
func strictQAJSON(raw []byte, v any) error {
	// 原Stage检查器只接受对象根；数组/空引用包入固定对象检查，仍保留重复键与深度限制。
	if len(raw) > 16384 || task.CheckStageJSON([]byte(`{"data":`+string(raw)+`}`)) != nil || json.Unmarshal(raw, v) != nil {
		return qa.ErrProtocol
	}
	b, err := json.Marshal(v)
	if err != nil || string(b) != string(raw) {
		return qa.ErrProtocol
	}
	return nil
}
func (t *aiQATx) share(query string) string {
	if t.write {
		return query + " FOR SHARE"
	}
	return query
}

// Authorize只认当前显式范围批准，不因来源登记/编号存在或关键词命中就允许使用。
func (t *aiQATx) Authorize(ctx context.Context, p qa.Proof) error {
	if !p.Valid() || p.SubjectType != "theme" {
		return qa.ErrUnavailable
	} // 专题目录尚未接线，不假设为题材。
	var member int64
	if err := t.tx.QueryRowContext(ctx, t.share(`SELECT id FROM addon_member WHERE id=? AND status=1 AND deleted_at IS NULL`), p.Identity.UserID).Scan(&member); err != nil {
		return qaDBError(err)
	}
	var name, ref, policy, expires string
	var enabled int
	var raw []byte
	if err := t.tx.QueryRowContext(ctx, t.share(`SELECT subject_name,approval_ref,policy_digest,evidence_json,CAST(valid_until AS CHAR),enabled FROM tm_ai_qa_scopes WHERE subject_type=? AND subject_id=?`), p.SubjectType, p.SubjectID).Scan(&name, &ref, &policy, &raw, &expires, &enabled); err != nil {
		return qaDBError(err)
	}
	until, err := parsePointsUTC(expires)
	if err != nil {
		return err
	}
	var evidence []qa.EvidenceProof
	if strictQAJSON(raw, &evidence) != nil || enabled != 1 || name != p.SubjectName || ref != p.ScopeApproval || policy != p.PolicyDigest || !until.Equal(p.ValidUntil) || !sameQAJSON(evidence, p.Evidence) {
		return fmt.Errorf("问答范围批准与原计划不符: %w", qa.ErrUnavailable)
	}
	policyLimits, err := t.QuotaPolicy(ctx)
	if err != nil {
		return err
	}
	for _, m := range p.Models {
		if m.BudgetRef != policyLimits.Ref {
			return fmt.Errorf("问答模型预算批准不符: %w", qa.ErrUnavailable)
		}
	}
	// 延用既有来源→快照→题材顺序；只有短事务共享锁，不跨模型请求。
	sourceIDs := []int64{}
	for _, ev := range evidence {
		var id int64
		if err := t.tx.QueryRowContext(ctx, `SELECT source_id FROM tm_ai_source_snapshots WHERE id=?`, ev.ID).Scan(&id); err != nil {
			return qaDBError(err)
		}
		sourceIDs = append(sourceIDs, id)
	}
	slices.Sort(sourceIDs)
	sourceIDs = slices.Compact(sourceIDs)
	current := map[int64]sources.Source{}
	for _, id := range sourceIDs {
		mode := sourceUnlocked
		if t.write {
			mode = sourceShared
		}
		s, err := sourceByIDWithMode(ctx, t.tx, id, mode)
		if err != nil {
			return err
		}
		current[id] = s
	}
	now, err := t.Now(ctx)
	if err != nil {
		return err
	}
	if !now.Before(until) {
		return fmt.Errorf("问答范围期限已失效: %w", qa.ErrUnavailable)
	}
	total, sentences := 0, 0
	for _, ev := range evidence {
		s, err := sourceSnapshotByID(ctx, t.tx, ev.ID, t.write)
		if err != nil {
			return err
		}
		total += len(s.Body)
		sentences += len(s.Sentences)
		registered, ok := current[s.SourceID]
		if !ok || total > 64<<10 || sentences > 512 || s.Hash != ev.Hash || s.FetchedAt.After(now) || !registered.HasValidAuthorization(now) || registered.Category != s.SourceCategory || registered.LicenseTag != s.LicenseTag || registered.LicenseRef != s.LicenseRef || !sameOptionalTime(registered.LicenseExpires, s.LicenseExpires) || registered.LicenseExpires != nil && registered.LicenseExpires.Before(until) {
			return fmt.Errorf("问答来源与当前授权不符: %w", qa.ErrUnavailable)
		}
		url, err := sources.ValidateURL(registered.Domain, s.URL)
		if err != nil || url != s.URL {
			return fmt.Errorf("问答来源链接不符: %w", qa.ErrUnavailable)
		}
	}
	var theme string
	if err := t.tx.QueryRowContext(ctx, t.share(`SELECT name FROM addon_quant_theme WHERE id=? AND deleted_at IS NULL AND IFNULL(status,1)=1`), p.SubjectID).Scan(&theme); err != nil {
		return qaDBError(err)
	}
	// 与原报告目录一致：按固定对象编号复核规范化名称，不把合法全角/空白差异当换题材。
	if guard.Normalize(theme) != name {
		return qa.ErrUnavailable
	}
	return nil
}
func (t *aiQATx) QuotaPolicy(ctx context.Context) (qa.QuotaPolicy, error) {
	var p qa.QuotaPolicy
	var enabled int
	err := t.tx.QueryRowContext(ctx, t.share(`SELECT approval_ref,user_requests,instance_requests,user_tokens,instance_tokens,enabled FROM tm_ai_qa_limits WHERE id=1`)).Scan(&p.Ref, &p.UserRequests, &p.InstanceRequests, &p.UserTokens, &p.InstanceTokens, &enabled)
	if err != nil {
		return p, qaDBError(err)
	}
	if enabled != 1 || !p.Valid() {
		return qa.QuotaPolicy{}, qa.ErrUnavailable
	}
	return p, nil
}
func (t *aiQATx) Quota(ctx context.Context, day string, uid int64) (qa.Quota, error) {
	if !t.write {
		return qa.Quota{}, qa.ErrInvalid
	}
	if _, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_qa_quotas(quota_day,user_id,requests,tokens,created_at,updated_at) VALUES(?,?,0,0,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE user_id=tm_ai_qa_quotas.user_id`, day, uid); err != nil {
		return qa.Quota{}, qaDBError(err)
	}
	q := qa.Quota{Day: day, UserID: uid}
	err := t.tx.QueryRowContext(ctx, `SELECT requests,tokens FROM tm_ai_qa_quotas WHERE quota_day=? AND user_id=? FOR UPDATE`, day, uid).Scan(&q.Requests, &q.Tokens)
	return q, qaDBError(err)
}
func (t *aiQATx) AddQuota(ctx context.Context, q qa.Quota, tokens int64) error {
	if !t.write || tokens < 1 || tokens > 1<<20 {
		return qa.ErrInvalid
	}
	r, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_qa_quotas SET requests=requests+1,tokens=tokens+?,updated_at=GREATEST(updated_at,UTC_TIMESTAMP(3)) WHERE quota_day=? AND user_id=? AND requests=? AND tokens=?`, tokens, q.Day, q.UserID, q.Requests, q.Tokens)
	return qaAffected(r, err)
}
func qaAffected(r sql.Result, err error) error {
	if err != nil {
		return qaDBError(err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return qa.ErrConflict
	}
	return nil
}

const qaColumns = `q.conversation_id,q.seq,q.user_id,q.input_digest,q.message_hash,q.proof_json,q.state,q.reason,q.citations_json,q.calls,q.reserved_tokens,IFNULL(CAST(q.quota_day AS CHAR),''),q.version,CAST(q.created_at AS CHAR),CAST(q.updated_at AS CHAR)`

func scanQA(s pointsScanner) (qa.Record, error) {
	var r qa.Record
	var cid, seq, uid int64
	var input, created, updated string
	var proof, picks []byte
	if err := s.Scan(&cid, &seq, &uid, &input, &r.MessageHash, &proof, &r.State, &r.Reason, &picks, &r.Calls, &r.ReservedTokens, &r.QuotaDay, &r.Version, &created, &updated); err != nil {
		return r, qaDBError(err)
	}
	if strictQAJSON(proof, &r.Proof) != nil || strictQAJSON(picks, &r.Citations) != nil || r.Proof.Identity != (qa.Identity{UserID: uid, ConversationID: cid, MessageSeq: seq}) || r.Proof.InputDigest != input {
		return qa.Record{}, qa.ErrProtocol
	}
	var err error
	r.CreatedAt, err = parsePointsUTC(created)
	if err != nil {
		return qa.Record{}, err
	}
	r.UpdatedAt, err = parsePointsUTC(updated)
	if err != nil {
		return qa.Record{}, err
	}
	if !r.Valid() {
		return qa.Record{}, qa.ErrProtocol
	}
	return r, nil
}
func (t *aiQATx) Question(ctx context.Context, id qa.Identity, lock bool) (qa.Record, error) {
	query := `SELECT ` + qaColumns + ` FROM tm_ai_qa_requests q WHERE q.conversation_id=? AND q.seq=? AND q.user_id=?`
	if lock {
		if !t.write {
			return qa.Record{}, qa.ErrInvalid
		}
		query += ` FOR UPDATE`
	}
	return scanQA(t.tx.QueryRowContext(ctx, query, id.ConversationID, id.MessageSeq, id.UserID))
}
func (t *aiQATx) QuestionByKey(ctx context.Context, uid, cid int64, key string) (qa.Record, error) {
	return scanQA(t.tx.QueryRowContext(ctx, `SELECT `+qaColumns+` FROM tm_ai_qa_requests q JOIN tm_ai_conversation_messages m ON m.conversation_id=q.conversation_id AND m.seq=q.seq AND m.user_id=q.user_id WHERE q.user_id=? AND q.conversation_id=? AND m.idem_key=?`, uid, cid, key))
}
func (t *aiQATx) InsertQuestion(ctx context.Context, r qa.Record) error {
	if !t.write || !r.Valid() || r.State != "queued" {
		return qa.ErrInvalid
	}
	proof, err := json.Marshal(r.Proof)
	if err != nil {
		return qa.ErrInvalid
	}
	picks, err := json.Marshal(r.Citations)
	if err != nil {
		return qa.ErrInvalid
	}
	id := r.Proof.Identity
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_qa_requests(conversation_id,seq,user_id,input_digest,message_hash,proof_json,state,reason,citations_json,calls,reserved_tokens,quota_day,version,created_at,updated_at) VALUES(?,?,?,?,?,?,'queued','',?,0,0,NULL,1,?,?)`, id.ConversationID, id.MessageSeq, id.UserID, r.Proof.InputDigest, r.MessageHash, proof, picks, pointsUTC(r.CreatedAt), pointsUTC(r.UpdatedAt))
	return qaDBError(err)
}
func (t *aiQATx) SetQuestion(ctx context.Context, old, next qa.Record) error {
	if !t.write || !old.Valid() || !next.Valid() || !sameQAJSON(old.Proof, next.Proof) || old.MessageHash != next.MessageHash {
		return qa.ErrInvalid
	}
	picks, err := json.Marshal(next.Citations)
	if err != nil {
		return qa.ErrInvalid
	}
	var day any
	if next.QuotaDay != "" {
		day = next.QuotaDay
	}
	id := old.Proof.Identity
	r, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_qa_requests SET state=?,reason=?,citations_json=?,calls=?,reserved_tokens=?,quota_day=?,version=?,updated_at=? WHERE conversation_id=? AND seq=? AND user_id=? AND version=?`, next.State, next.Reason, picks, next.Calls, next.ReservedTokens, day, next.Version, pointsUTC(next.UpdatedAt), id.ConversationID, id.MessageSeq, id.UserID, old.Version)
	return qaAffected(r, err)
}
func (t *aiQATx) Calls(ctx context.Context, input string) ([]qa.CallRecord, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT step_no,call_json,state,result_json,CAST(created_at AS CHAR),CAST(updated_at AS CHAR) FROM tm_ai_qa_calls WHERE input_digest=? ORDER BY step_no LIMIT 4`, input)
	if err != nil {
		return nil, qaDBError(err)
	}
	defer rows.Close()
	out := []qa.CallRecord{}
	for rows.Next() {
		var c qa.CallRecord
		var step int
		var call, result []byte
		var created, updated string
		if err := rows.Scan(&step, &call, &c.State, &result, &created, &updated); err != nil {
			return nil, qaDBError(err)
		}
		if strictQAJSON(call, &c.Call) != nil || c.Call.InputDigest != input || c.Call.Step != step || step != len(out)+1 {
			return nil, qa.ErrProtocol
		}
		if result != nil && strictQAJSON(result, &c.Result) != nil {
			return nil, qa.ErrProtocol
		}
		c.CreatedAt, err = parsePointsUTC(created)
		if err != nil {
			return nil, err
		}
		c.UpdatedAt, err = parsePointsUTC(updated)
		if err != nil {
			return nil, err
		}
		if !c.Valid() {
			return nil, qa.ErrProtocol
		}
		out = append(out, c)
	}
	return out, qaDBError(rows.Err())
}
func (t *aiQATx) InsertCall(ctx context.Context, c qa.CallRecord) error {
	if !t.write || !c.Valid() || c.State != "reserved" {
		return qa.ErrInvalid
	}
	raw, err := json.Marshal(c.Call)
	if err != nil {
		return qa.ErrInvalid
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_qa_calls(input_digest,step_no,call_json,state,result_json,created_at,updated_at) VALUES(?,? ,?,'reserved',NULL,?,?)`, c.Call.InputDigest, c.Call.Step, raw, pointsUTC(c.CreatedAt), pointsUTC(c.UpdatedAt))
	return qaDBError(err)
}
func (t *aiQATx) FinishCall(ctx context.Context, old qa.CallRecord, result qa.CallResult, now time.Time) error {
	next := old
	next.State = "recorded"
	next.Result = result
	next.UpdatedAt = now
	if !t.write || !old.Valid() || old.State != "reserved" || !next.Valid() {
		return qa.ErrInvalid
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return qa.ErrInvalid
	}
	r, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_qa_calls SET state='recorded',result_json=?,updated_at=? WHERE input_digest=? AND step_no=? AND state='reserved'`, raw, pointsUTC(now), old.Call.InputDigest, old.Call.Step)
	return qaAffected(r, err)
}
