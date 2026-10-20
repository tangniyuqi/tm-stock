package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

// VerifySchema 在单个回滚事务中实测关键保护，不建表、不修表、不提交业务数据。
// 这是有限反例校验，不是任意结构篡改均可检出的证明。
func (r *AIAgentRepo) VerifySchema(ctx context.Context) error {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return fmt.Errorf("生成 Agent 自检编号失败")
	}
	digest := hex.EncodeToString(seed)
	req := "probe_" + digest[:40]
	err := r.WithinAgentTx(ctx, func(raw agents.Tx) error {
		t := raw.(*agentTx)
		exec := func(q string, args ...any) error { _, err := t.tx.ExecContext(ctx, q, args...); return err }
		reject := func(number uint16, contains, q string, args ...any) error {
			err := exec(q, args...)
			var me *mysql.MySQLError
			if !errors.As(err, &me) || me.Number != number || !strings.Contains(me.Message, contains) {
				return fmt.Errorf("Agent 数据库关键保护未按预期拒写")
			}
			return nil
		}
		if err := exec(`INSERT INTO tm_ai_agent_profiles (profile_digest,max_submissions) VALUES (?,1)`, digest); err != nil {
			return fmt.Errorf("Agent 自检配置插入失败")
		}
		if err := reject(3819, "chk_tm_ai_agent_profile", `UPDATE tm_ai_agent_profiles SET reserved_submissions=2 WHERE profile_digest=?`, digest); err != nil {
			return err
		}
		if err := exec(`INSERT INTO tm_ai_agent_runs (request_id,actor_id,symbol,profile_digest) VALUES (?,1,'600001',?)`, req, digest); err != nil {
			return fmt.Errorf("Agent 自检请求插入失败")
		}
		r := agents.Record{ActorID: 1, Request: agents.Request{RequestID: req, Symbol: "600001"}, ProfileDigest: digest}
		if err := t.tx.QueryRowContext(ctx, `SELECT id,state,cancel_requested FROM tm_ai_agent_runs WHERE request_id=?`, req).Scan(&r.ID, &r.State, &r.CancelRequested); err != nil || !r.Valid() || r.State != agents.Submitting || r.CancelRequested {
			return fmt.Errorf("Agent 默认状态或结构无效")
		}
		if err := reject(1062, "uk_tm_ai_agent_request", `INSERT INTO tm_ai_agent_runs (request_id,actor_id,symbol,profile_digest) VALUES (?,1,'600001',?)`, req, digest); err != nil {
			return err
		}
		if err := reject(3819, "chk_tm_ai_agent_run", `UPDATE tm_ai_agent_runs SET state='unknown' WHERE id=?`, r.ID); err != nil {
			return err
		}
		if err := exec(`UPDATE tm_ai_agent_runs SET state='accepted',remote_id=? WHERE id=?`, req, r.ID); err != nil {
			return fmt.Errorf("Agent 自检远端映射失败")
		}
		r.State = agents.Accepted
		r.RemoteID = req
		if err := reject(1062, "uk_tm_ai_agent_remote", `INSERT INTO tm_ai_agent_runs (request_id,actor_id,symbol,profile_digest,state,remote_id) VALUES (?,1,'600001',?,'accepted',?)`, req+"b", digest, req); err != nil {
			return err
		}
		if err := reject(3819, "chk_tm_ai_agent_run", `UPDATE tm_ai_agent_runs SET state='result_quarantined' WHERE id=?`, r.ID); err != nil {
			return err
		}
		head, _ := json.Marshal(r)
		if err := exec(`INSERT INTO tm_ai_agent_events (run_id,seq,event,record_json) VALUES (?,1,'submitted',?)`, r.ID, string(head)); err != nil {
			return fmt.Errorf("Agent 自检事件插入失败")
		}
		if _, err := scanAgentRun(t.tx.QueryRowContext(ctx, agentSelect+` WHERE id=?`, r.ID)); err != nil {
			return fmt.Errorf("Agent 自检记录与事件不一致")
		}
		if err := reject(1062, "uk_tm_ai_agent_event_seq", `INSERT INTO tm_ai_agent_events (run_id,seq,event,record_json) VALUES (?,1,'submit_reserved','{}')`, r.ID); err != nil {
			return err
		}
		if err := reject(1644, "Agent events are immutable", `UPDATE tm_ai_agent_events SET record_json='{}' WHERE run_id=?`, r.ID); err != nil {
			return err
		}
		if err := reject(1644, "Agent events are immutable", `DELETE FROM tm_ai_agent_events WHERE run_id=?`, r.ID); err != nil {
			return err
		}
		if err := reject(1644, "Agent runs cannot be deleted", `DELETE FROM tm_ai_agent_runs WHERE id=?`, r.ID); err != nil {
			return err
		}
		// 用独立哨兵确保外层始终回滚，哪怕全部反例通过。
		return errAgentProbeRollback
	})
	if errors.Is(err, errAgentProbeRollback) {
		return nil
	}
	return err
}

var errAgentProbeRollback = errors.New("Agent 自检完成，必须回滚")
