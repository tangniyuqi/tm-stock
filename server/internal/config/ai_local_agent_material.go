package config

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// AILocalAgentMaterial 是题材资料 Agent 的独立用途批准（ADR-0029）。
// 它与资料问答用途（bounded-material-qa）互不替代：问答批准不能开启 Agent，本批准也不能开启问答。
// 文件只声明会员、对象与快照身份，不承载正文、URL、凭据或生产批准；不调用模型、不提交远端任务。
type AILocalAgentMaterial struct {
	Version     int                        `json:"version"`
	InstanceID  string                     `json:"instance_id"`
	Revision    string                     `json:"revision"`
	Scope       string                     `json:"scope"`
	IssuedAtMS  int64                      `json:"issued_at_ms"`
	ExpiresAtMS int64                      `json:"expires_at_ms"`
	Members     []int64                    `json:"members"`
	Use         AILocalAgentMaterialUse    `json:"use"`
	Limits      AILocalAgentMaterialLimits `json:"limits"`
}

type AILocalAgentMaterialUse struct {
	DatasetDigest string                        `json:"dataset_digest"`
	ApprovalRef   string                        `json:"approval_ref"`
	Purpose       string                        `json:"purpose"`
	Bindings      []AILocalAgentMaterialBinding `json:"bindings"`
}

type AILocalAgentMaterialBinding struct {
	SubjectType   string                         `json:"subject_type"`
	SubjectID     int64                          `json:"subject_id"`
	ScopeApproval string                         `json:"scope_approval"`
	Evidence      []AILocalAgentMaterialEvidence `json:"evidence"`
}

type AILocalAgentMaterialEvidence struct {
	SnapshotID int64  `json:"snapshot_id"`
	Hash       string `json:"hash"`
}

type AILocalAgentMaterialLimits struct {
	MaxSources       int `json:"max_sources"`
	MaxEvidenceBytes int `json:"max_evidence_bytes"`
}

const (
	AgentMaterialScope   = "agent-materials-development-only"
	AgentMaterialPurpose = "bounded-material-agent"
	agentMaterialPackV1  = "tm-stock-agent-material-pack-v1"
	maxAgentMaterialSize = 32 << 10
)

func (AILocalAgentMaterial) String() string {
	return "本地Agent资料用途批准（内容已隐藏）"
}
func (c AILocalAgentMaterial) GoString() string { return c.String() }

func (c AILocalAgentMaterial) validate(runtime AILocalRuntime, now int64) bool {
	u := c.Use
	if c.Version != 1 || c.Scope != AgentMaterialScope || c.InstanceID != runtime.InstanceID || !localRevision.MatchString(c.Revision) ||
		c.IssuedAtMS <= 0 || c.IssuedAtMS > now || c.ExpiresAtMS <= now || c.ExpiresAtMS <= c.IssuedAtMS || !localQAIDs(c.Members) {
		return false
	}
	l := c.Limits
	if l.MaxSources < 1 || l.MaxSources > 8 || l.MaxEvidenceBytes < 1 || l.MaxEvidenceBytes > 1<<20 {
		return false
	}
	// 用途必须是 Agent 专用值；资料问答用途或其他用途一律拒绝，不做别名映射。
	if !localPlanDigest.MatchString(u.DatasetDigest) || !localRef.MatchString(u.ApprovalRef) || u.Purpose != AgentMaterialPurpose || len(u.Bindings) < 1 || len(u.Bindings) > 16 {
		return false
	}
	for n, b := range u.Bindings {
		if b.SubjectType != "theme" && b.SubjectType != "topic" || b.SubjectID <= 0 || b.SubjectID > 9007199254740991 || !localRef.MatchString(b.ScopeApproval) ||
			len(b.Evidence) < 1 || len(b.Evidence) > l.MaxSources {
			return false
		}
		// 对象身份按 (类型,编号) 严格递增，杜绝同一对象重复批准或多份口径。
		if n > 0 {
			p := u.Bindings[n-1]
			if b.SubjectType < p.SubjectType || b.SubjectType == p.SubjectType && b.SubjectID <= p.SubjectID {
				return false
			}
		}
		for i, e := range b.Evidence {
			if e.SnapshotID <= 0 || e.SnapshotID > 9007199254740991 || !localPlanDigest.MatchString(e.Hash) || i > 0 && e.SnapshotID <= b.Evidence[i-1].SnapshotID {
				return false
			}
		}
	}
	return true
}

// Binding 只返回批准中声明的对象；会员不在名单或对象未批准时返回 false，不推断、不回退。
func (c AILocalAgentMaterial) Binding(memberID int64, subjectType string, subjectID int64) (AILocalAgentMaterialBinding, bool) {
	member := false
	for _, m := range c.Members {
		if m == memberID {
			member = true
			break
		}
	}
	if !member {
		return AILocalAgentMaterialBinding{}, false
	}
	for _, b := range c.Use.Bindings {
		if b.SubjectType == subjectType && b.SubjectID == subjectID {
			out := b
			out.Evidence = append([]AILocalAgentMaterialEvidence(nil), b.Evidence...)
			return out, true
		}
	}
	return AILocalAgentMaterialBinding{}, false
}

// MaterialsDigest 是资料包身份摘要，绑定数据集、用途批准、对象及有序快照哈希，
// 可作为 agents.Request.MaterialsDigest。它只证明声明身份，不证明当前许可仍有效。
func (c AILocalAgentMaterial) MaterialsDigest(b AILocalAgentMaterialBinding) (string, error) {
	raw, err := json.Marshal([]any{agentMaterialPackV1, c.Use.DatasetDigest, c.Use.ApprovalRef, c.Use.Purpose, b.SubjectType, b.SubjectID, b.ScopeApproval, b.Evidence})
	if err != nil {
		return "", invalidLocalRuntime()
	}
	return task.StageDigest(raw), nil
}

type AILocalAgentMaterialSource struct {
	path    string
	runtime AILocalRuntime
	digest  string
	now     func() time.Time
}

func NewAILocalAgentMaterialSource(path string, runtime *AILocalRuntime) (*AILocalAgentMaterialSource, error) {
	if runtime == nil || runtime.validate() != nil {
		return nil, invalidLocalRuntime()
	}
	s := &AILocalAgentMaterialSource{path: path, runtime: *runtime, now: time.Now}
	c, err := s.read()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, invalidLocalRuntime()
	}
	s.digest = task.StageDigest(b)
	return s, nil
}

func (s *AILocalAgentMaterialSource) read() (*AILocalAgentMaterial, error) {
	if s == nil {
		return nil, invalidLocalRuntime()
	}
	b, err := readAILocalFile(s.path, maxAgentMaterialSize)
	if err != nil || task.CheckStageJSON(b) != nil || !exactLocalPlanJSON(b, reflect.TypeOf(AILocalAgentMaterial{})) {
		return nil, invalidLocalRuntime()
	}
	var c AILocalAgentMaterial
	if json.Unmarshal(b, &c) != nil || s.runtime.validate() != nil || !c.validate(s.runtime, s.now().UnixMilli()) {
		return nil, invalidLocalRuntime()
	}
	return &c, nil
}

// Current 每次重读文件并要求与启动时摘要一致；文件被改写、过期或撤回即失败，不沿用旧批准。
func (s *AILocalAgentMaterialSource) Current(ctx context.Context) (*AILocalAgentMaterial, error) {
	if ctx == nil {
		return nil, invalidLocalRuntime()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, err := s.read()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(c)
	if err != nil || task.StageDigest(b) != s.digest {
		return nil, invalidLocalRuntime()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c, nil
}
