package repository

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AgentMaterialRequest 只接受上层独立用途批准的身份；不是会员可上传的资料或许可。
// ScopeApproval 与用途文件的 Current/Binding 由上层核验，本仓储不借用 QA 政策授予 Agent 用途。
type AgentMaterialRequest struct {
	MemberID                     int64
	SubjectType                  string
	SubjectID                    int64
	ValidUntil                   time.Time
	Evidence                     []AgentMaterialEvidence
	MaxSources, MaxEvidenceBytes int
}
type AgentMaterialEvidence struct {
	SnapshotID int64
	Hash       string
}
type AgentMaterialScope struct {
	SubjectName string
	Now         time.Time
	Sources     []sources.Source
	Snapshots   []sources.Snapshot
}

func (AgentMaterialRequest) String() string     { return "Agent资料读取身份（内容已隐藏）" }
func (v AgentMaterialRequest) GoString() string { return v.String() }
func (AgentMaterialScope) String() string       { return "Agent当前资料范围（正文已隐藏）" }
func (v AgentMaterialScope) GoString() string   { return v.String() }

type AIAgentMaterialRepo struct{ db *sql.DB }

func NewAIAgentMaterialRepo(db *sql.DB) *AIAgentMaterialRepo { return &AIAgentMaterialRepo{db: db} }

func validAgentMaterialRequest(v AgentMaterialRequest) bool {
	if !conversation.ID(v.MemberID) || v.SubjectType != "theme" || !conversation.ID(v.SubjectID) || !conversation.Time(v.ValidUntil) || v.MaxSources < 1 || v.MaxSources > 8 || v.MaxEvidenceBytes < 1 || v.MaxEvidenceBytes > 1<<20 || len(v.Evidence) < 1 || len(v.Evidence) > v.MaxSources {
		return false
	}
	for i, e := range v.Evidence {
		if !conversation.ID(e.SnapshotID) || len(e.Hash) != 64 || i > 0 && e.SnapshotID <= v.Evidence[i-1].SnapshotID {
			return false
		}
		for _, c := range e.Hash {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}

// Load 只读短事务复核当前会员、题材、来源和快照；专题目录尚未接线，topic 失败关闭。
// 不持锁跨模型请求，不返回执行许可；提交、每次外发及读取前须由上层重读用途并再次 Load。
func (r *AIAgentMaterialRepo) Load(parent context.Context, v AgentMaterialRequest) (AgentMaterialScope, error) {
	if parent == nil || !validAgentMaterialRequest(v) {
		return AgentMaterialScope{}, agents.ErrInvalid
	}
	if err := parent.Err(); err != nil {
		return AgentMaterialScope{}, err
	}
	if r == nil || r.db == nil {
		return AgentMaterialScope{}, agents.ErrInvalid
	}
	v.Evidence = slices.Clone(v.Evidence)
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return AgentMaterialScope{}, fmt.Errorf("开启Agent资料只读事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后 ErrTxDone 可忽略，其余出口统一回滚。
	out, err := loadAgentMaterial(ctx, tx, v)
	if err != nil {
		return AgentMaterialScope{}, err
	}
	if err := tx.Commit(); err != nil {
		return AgentMaterialScope{}, fmt.Errorf("结束Agent资料只读事务失败: %w", err)
	}
	return out, nil
}

func loadAgentMaterial(ctx context.Context, tx *sql.Tx, v AgentMaterialRequest) (AgentMaterialScope, error) {
	var out AgentMaterialScope
	var member int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM addon_member WHERE id=? AND status=1 AND deleted_at IS NULL", v.MemberID).Scan(&member); err != nil {
		return out, fmt.Errorf("复核Agent资料会员失败: %w", err)
	}
	// 先读长度与来源身份，再读正文，避免超限正文或索引先进入内存。后续使用同一一致性快照。
	ids := make([]int64, 0, len(v.Evidence))
	total, count := int64(0), int64(0)
	for _, e := range v.Evidence {
		var id, size, indexSize, sentences int64
		err := tx.QueryRowContext(ctx, "SELECT s.source_id,OCTET_LENGTH(b.body_text),OCTET_LENGTH(b.sentences_json),JSON_LENGTH(b.sentences_json) FROM tm_ai_source_snapshots s JOIN tm_ai_source_bodies b ON b.content_hash=s.body_ref WHERE s.id=? AND s.content_hash=? AND s.body_ref=s.content_hash", e.SnapshotID, e.Hash).Scan(&id, &size, &indexSize, &sentences)
		if err != nil {
			return out, fmt.Errorf("预检Agent资料快照失败: %w", err)
		}
		total += size
		count += sentences
		// 句子索引包含正文副本和偏移；限定其序列化大小，不能用微小正文夹带无界 JSON。
		if !conversation.ID(id) || size < 1 || total > int64(v.MaxEvidenceBytes) || sentences < 1 || count > 512 || indexSize < 1 || indexSize > 8*int64(v.MaxEvidenceBytes)+65536 {
			return out, agents.ErrInvalid
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	current := map[int64]sources.Source{}
	for _, id := range ids {
		source, err := sourceByIDWithMode(ctx, tx, id, sourceUnlocked)
		if err != nil {
			return out, err
		}
		current[id] = source
		out.Sources = append(out.Sources, source)
	}
	now, err := (&pointsTx{tx}).Now(ctx)
	if err != nil {
		return out, err
	}
	if !conversation.Time(now) || !now.Before(v.ValidUntil) {
		return out, agents.ErrInvalid
	}
	out.Now = now
	total, count = 0, 0
	for _, e := range v.Evidence {
		s, err := sourceSnapshotByID(ctx, tx, e.SnapshotID, false)
		if err != nil {
			return AgentMaterialScope{}, err
		}
		total += int64(len(s.Body))
		count += int64(len(s.Sentences))
		source, ok := current[s.SourceID]
		if !ok || total > int64(v.MaxEvidenceBytes) || count > 512 || !agentMaterialSnapshotCurrent(s, source, e, now, v.ValidUntil) {
			return AgentMaterialScope{}, agents.ErrInvalid
		}
		out.Snapshots = append(out.Snapshots, s)
	}
	var name string
	if err := tx.QueryRowContext(ctx, "SELECT name FROM addon_quant_theme WHERE id=? AND deleted_at IS NULL AND IFNULL(status,1)=1", v.SubjectID).Scan(&name); err != nil {
		return AgentMaterialScope{}, fmt.Errorf("复核Agent资料对象失败: %w", err)
	}
	out.SubjectName = guard.Normalize(name)
	if !conversation.Text(out.SubjectName, 128) || guard.Blocked(out.SubjectName) {
		return AgentMaterialScope{}, agents.ErrInvalid
	}
	return out, nil
}

func agentMaterialSnapshotCurrent(s sources.Snapshot, source sources.Source, e AgentMaterialEvidence, now, until time.Time) bool {
	if s.ID != e.SnapshotID || s.Hash != e.Hash || s.SourceID != source.ID || s.Validate() != nil || !conversation.Time(s.FetchedAt) || s.FetchedAt.After(now) || !source.HasValidAuthorization(now) || source.Category != s.SourceCategory || source.LicenseTag != s.LicenseTag || source.LicenseRef != s.LicenseRef || !sameOptionalTime(source.LicenseExpires, s.LicenseExpires) || source.LicenseExpires != nil && source.LicenseExpires.Before(until) {
		return false
	}
	canonical, err := sources.ValidateURL(source.Domain, s.URL)
	return err == nil && canonical == s.URL
}
