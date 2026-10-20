package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

type AIAgentRepo struct{ db *sql.DB }

func NewAIAgentRepo(db *sql.DB) *AIAgentRepo { return &AIAgentRepo{db: db} }

var _ agents.Store = (*AIAgentRepo)(nil)
var _ agents.PayloadReader = (*AIAgentRepo)(nil)

type agentTx struct {
	tx     *sql.Tx
	locked map[int64]agents.Record
}

func (r *AIAgentRepo) WithinAgentTx(ctx context.Context, fn func(agents.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启 Agent 事务失败")
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(&agentTx{tx: tx, locked: map[int64]agents.Record{}}); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return fmt.Errorf("提交 Agent 事务失败；请保留原请求编号核对")
	}
	return nil
}

const agentSelect = `SELECT id,actor_id,request_id,symbol,profile_digest,IFNULL(remote_id,''),state,cancel_requested,
 IFNULL(result_digest,''),error_code,
 (result_payload IS NULL OR (OCTET_LENGTH(result_payload) BETWEEN 1 AND 4194304 AND JSON_VALID(CONVERT(result_payload USING utf8mb4)) AND SHA2(result_payload,256)=result_digest))
 ,event_seq,
 (SELECT COUNT(*) FROM tm_ai_agent_events e WHERE e.run_id=r.id),
 (SELECT IFNULL(MIN(seq),0) FROM tm_ai_agent_events e WHERE e.run_id=r.id),
 (SELECT IFNULL(MAX(seq),0) FROM tm_ai_agent_events e WHERE e.run_id=r.id),
 (SELECT record_json FROM tm_ai_agent_events e WHERE e.run_id=r.id AND e.seq=r.event_seq)
 FROM tm_ai_agent_runs r`

func scanAgentRun(row candidateScanner) (agents.Record, error) {
	var r agents.Record
	var payloadOK bool
	var seq, count, first, last int64
	var head []byte
	err := row.Scan(&r.ID, &r.ActorID, &r.Request.RequestID, &r.Request.Symbol, &r.ProfileDigest, &r.RemoteID, &r.State, &r.CancelRequested, &r.ResultDigest, &r.ErrorCode, &payloadOK, &seq, &count, &first, &last, &head)
	if errors.Is(err, sql.ErrNoRows) {
		return r, agents.ErrNotFound
	}
	if err != nil {
		return agents.Record{}, fmt.Errorf("读取 Agent 记录失败")
	}
	var audited agents.Record
	if !r.Valid() || !payloadOK || seq <= 0 || count != seq || first != 1 || last != seq || len(head) > 4096 || json.Unmarshal(head, &audited) != nil || audited != r {
		return agents.Record{}, agents.ErrInvalid
	}
	return r, nil
}

func (r *AIAgentRepo) ByRequest(ctx context.Context, id string) (agents.Record, error) {
	if !agents.ValidRequestID(id) {
		return agents.Record{}, agents.ErrInvalid
	}
	return scanAgentRun(r.db.QueryRowContext(ctx, agentSelect+` WHERE request_id=?`, id))
}

// Quarantined 先核对不可变事件链，再按操作人/请求/摘要读取终态正文。
// 已隔离正文受数据库不可变保护；并发取消回执只改审计状态，不改本文身份。
func (r *AIAgentRepo) Quarantined(ctx context.Context, actorID int64, id string) (agents.Record, agents.Result, error) {
	if actorID <= 0 || !agents.ValidRequestID(id) {
		return agents.Record{}, agents.Result{}, agents.ErrInvalid
	}
	v, err := r.ByRequest(ctx, id)
	if err != nil {
		return agents.Record{}, agents.Result{}, err
	}
	if v.ActorID != actorID {
		return agents.Record{}, agents.Result{}, agents.ErrNotFound
	}
	if v.State != agents.Quarantined {
		return agents.Record{}, agents.Result{}, agents.ErrUncertain
	}
	result := agents.Result{RemoteID: v.RemoteID}
	err = r.db.QueryRowContext(ctx, `SELECT result_payload FROM tm_ai_agent_runs WHERE id=? AND actor_id=? AND request_id=? AND state=? AND result_digest=?`, v.ID, actorID, id, agents.Quarantined, v.ResultDigest).Scan(&result.Payload)
	if errors.Is(err, sql.ErrNoRows) {
		return agents.Record{}, agents.Result{}, agents.ErrNotFound
	}
	if err != nil {
		return agents.Record{}, agents.Result{}, fmt.Errorf("读取隔离 Agent 结果失败")
	}
	if !result.Valid() || result.Digest() != v.ResultDigest {
		return agents.Record{}, agents.Result{}, agents.ErrInvalid
	}
	return v, result, nil
}

func (t *agentTx) Lock(ctx context.Context, id int64) (agents.Record, error) {
	if id <= 0 {
		return agents.Record{}, agents.ErrInvalid
	}
	// 先取得当前行锁，再以新的 RC 语句读取行与事件；避免等待锁时子查询拿到旧快照。
	var lockedID int64
	if err := t.tx.QueryRowContext(ctx, `SELECT id FROM tm_ai_agent_runs WHERE id=? FOR UPDATE`, id).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.Record{}, agents.ErrNotFound
		}
		return agents.Record{}, fmt.Errorf("锁定 Agent 记录失败")
	}
	r, err := scanAgentRun(t.tx.QueryRowContext(ctx, agentSelect+` WHERE id=?`, id))
	if err == nil {
		t.locked[id] = r
	}
	return r, err
}

func (t *agentTx) Reserve(ctx context.Context, p agents.Profile, req agents.Request, actorID int64) (agents.Record, bool, error) {
	if !p.Valid() || !req.Valid() || actorID <= 0 {
		return agents.Record{}, false, agents.ErrInvalid
	}
	// 同一配置跨进程串行预留；任何不确定提交也占一个名额，不因失败返还。
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_agent_profiles (profile_digest,max_submissions) VALUES (?,?) ON DUPLICATE KEY UPDATE profile_digest=profile_digest`, p.Digest, p.MaxSubmissions)
	if err != nil {
		return agents.Record{}, false, fmt.Errorf("登记 Agent 配置失败")
	}
	var limit, used int
	if err := t.tx.QueryRowContext(ctx, `SELECT max_submissions,reserved_submissions FROM tm_ai_agent_profiles WHERE profile_digest=? FOR UPDATE`, p.Digest).Scan(&limit, &used); err != nil {
		return agents.Record{}, false, fmt.Errorf("锁定 Agent 限额失败")
	}
	if limit != p.MaxSubmissions || used < 0 || used > limit {
		return agents.Record{}, false, agents.ErrConflict
	}
	r, err := scanAgentRun(t.tx.QueryRowContext(ctx, agentSelect+` WHERE request_id=?`, req.RequestID))
	check := func(r agents.Record) (agents.Record, bool, error) {
		if r.ActorID != actorID || r.Request != req || r.ProfileDigest != p.Digest {
			return agents.Record{}, false, agents.ErrConflict
		}
		return r, false, nil
	}
	if err == nil {
		return check(r)
	}
	if !errors.Is(err, agents.ErrNotFound) {
		return agents.Record{}, false, err
	}
	if used >= limit {
		return agents.Record{}, false, agents.ErrLimit
	}
	insert, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_agent_runs (request_id,actor_id,symbol,profile_digest,created_at,updated_at) VALUES (?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, req.RequestID, actorID, req.Symbol, p.Digest)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			r, err := scanAgentRun(t.tx.QueryRowContext(ctx, agentSelect+` WHERE request_id=?`, req.RequestID))
			if err != nil {
				return agents.Record{}, false, err
			}
			return check(r)
		}
		return agents.Record{}, false, fmt.Errorf("登记 Agent 提交失败")
	}
	id, err := insert.LastInsertId()
	if err != nil {
		return agents.Record{}, false, fmt.Errorf("读取 Agent 提交编号失败")
	}
	r = agents.Record{ID: id, ActorID: actorID, Request: req, ProfileDigest: p.Digest, State: agents.Submitting}
	t.locked[id] = r
	if _, err = t.tx.ExecContext(ctx, `UPDATE tm_ai_agent_profiles SET reserved_submissions=reserved_submissions+1 WHERE profile_digest=?`, p.Digest); err != nil {
		return agents.Record{}, false, fmt.Errorf("预留 Agent 限额失败")
	}
	if err := t.event(ctx, r, "submit_reserved"); err != nil {
		return agents.Record{}, false, err
	}
	return r, true, nil
}

func (t *agentTx) event(ctx context.Context, r agents.Record, event string) error {
	body, err := json.Marshal(r)
	if err != nil {
		return agents.ErrInvalid
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_agent_events (run_id,seq,event,record_json,created_at) SELECT id,event_seq,?,?,UTC_TIMESTAMP(3) FROM tm_ai_agent_runs WHERE id=?`, event, string(body), r.ID)
	if err != nil {
		return fmt.Errorf("写入 Agent 审计失败")
	}
	return nil
}

func (t *agentTx) Save(ctx context.Context, r agents.Record, event string, result *agents.Result) error {
	old, ok := t.locked[r.ID]
	if !ok || !agents.ValidChange(old, r, event, result) {
		return agents.ErrConflict
	}
	var remote, hash, payload any
	if r.RemoteID != "" {
		remote = r.RemoteID
	}
	if r.ResultDigest != "" {
		hash = r.ResultDigest
	}
	if result != nil {
		payload = []byte(result.Payload)
	}
	// 非隔离事件保留隔离正文；已隔离记录只能补完并发中的取消回执。
	_, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_agent_runs SET remote_id=?,state=?,cancel_requested=?,error_code=?,result_digest=?,
 result_payload=COALESCE(?,result_payload),event_seq=event_seq+1,updated_at=UTC_TIMESTAMP(3) WHERE id=?`, remote, r.State, r.CancelRequested, r.ErrorCode, hash, payload, r.ID)
	if err != nil {
		return fmt.Errorf("更新 Agent 状态失败")
	}
	if err := t.event(ctx, r, event); err != nil {
		return err
	}
	t.locked[r.ID] = r
	return nil
}
