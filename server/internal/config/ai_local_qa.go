package config

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// 本地开发问答装配；正式资料须显式绑定快照和用途，不承载正文、凭据或生产批准。
type AILocalQA struct {
	Version        int                   `json:"version"`
	InstanceID     string                `json:"instance_id"`
	Revision       string                `json:"revision"`
	Scope          string                `json:"scope"`
	IssuedAtMS     int64                 `json:"issued_at_ms"`
	ExpiresAtMS    int64                 `json:"expires_at_ms"`
	Members        []int64               `json:"members"`
	Subjects       []int64               `json:"subjects"`
	Models         []AILocalQAModel      `json:"models"`
	AllowedDomains []string              `json:"allowed_domains"`
	Limits         AILocalQALimits       `json:"limits"`
	MaterialUse    *AILocalQAMaterialUse `json:"material_use,omitempty"`
}

// 用途声明不替代原数据库的范围、来源许可、会员和预算复核。
type AILocalQAMaterialUse struct {
	DatasetDigest string                     `json:"dataset_digest"`
	ApprovalRef   string                     `json:"approval_ref"`
	Purpose       string                     `json:"purpose"`
	Bindings      []AILocalQAMaterialBinding `json:"bindings"`
}
type AILocalQAMaterialBinding struct {
	SubjectID     int64              `json:"subject_id"`
	ScopeApproval string             `json:"scope_approval"`
	Evidence      []qa.EvidenceProof `json:"evidence"`
}

type AILocalQAModel struct {
	Endpoint           AILocalExecutionModel `json:"endpoint"`
	FilingNo           string                `json:"filing_no"`
	ApprovalRef        string                `json:"approval_ref"`
	QualityRef         string                `json:"quality_ref"`
	BudgetRef          string                `json:"budget_ref"`
	ProfileDigest      string                `json:"profile_digest"`
	JSONSchemaFallback bool                  `json:"json_schema_fallback,omitempty"`
	ReasoningEffort    string                `json:"reasoning_effort,omitempty"`
}

type AILocalQALimits struct {
	MaxSources       int `json:"max_sources"`
	MaxSentences     int `json:"max_sentences"`
	MaxEvidenceBytes int `json:"max_evidence_bytes"`
	MaxPromptBytes   int `json:"max_prompt_bytes"`
	MaxInputTokens   int `json:"max_input_tokens"`
	MaxOutputTokens  int `json:"max_output_tokens"`
	MaxTotalTokens   int `json:"max_total_tokens"`
	TimeoutMS        int `json:"timeout_ms"`
}

func (AILocalQA) String() string     { return "本地问答配置（内容已隐藏）" }
func (c AILocalQA) GoString() string { return c.String() }
func (m AILocalQAModel) ProviderConfig() provider.Config {
	return provider.Config{Name: m.Endpoint.Name, BaseURL: m.Endpoint.BaseURL, Model: m.Endpoint.Model, TimeoutMS: m.Endpoint.TimeoutMS, JSONSchemaFallback: m.JSONSchemaFallback, ReasoningEffort: m.ReasoningEffort}
}
func (m AILocalQAModel) Metadata() qa.Model {
	return qa.Model{Name: m.Endpoint.Model, FilingNo: m.FilingNo, ApprovalRef: m.ApprovalRef, QualityRef: m.QualityRef, BudgetRef: m.BudgetRef, ProfileDigest: m.ProfileDigest}
}
func (c AILocalQA) QAConfig() (qa.Config, error) {
	if len(c.Models) != 3 {
		return qa.Config{}, invalidLocalRuntime()
	}
	l := c.Limits
	if l.TimeoutMS < 1 || l.TimeoutMS > 20000 {
		return qa.Config{}, invalidLocalRuntime()
	}
	v := qa.Config{Selector: c.Models[0].Metadata(), Entailment: c.Models[1].Metadata(), Semantic: c.Models[2].Metadata(), AllowedDomains: append([]string(nil), c.AllowedDomains...), Limits: qa.Limits{MaxSources: l.MaxSources, MaxSentences: l.MaxSentences, MaxEvidenceBytes: l.MaxEvidenceBytes, MaxPromptBytes: l.MaxPromptBytes, MaxInputTokens: l.MaxInputTokens, MaxOutputTokens: l.MaxOutputTokens, MaxTotalTokens: l.MaxTotalTokens, Timeout: time.Duration(l.TimeoutMS) * time.Millisecond}}
	if !v.Limits.Valid() {
		return qa.Config{}, invalidLocalRuntime()
	}
	if _, err := v.PolicyDigest(); err != nil {
		return qa.Config{}, invalidLocalRuntime()
	}
	return v, nil
}

type AILocalQASource struct {
	path    string
	runtime AILocalRuntime
	digest  string
}

func NewAILocalQASource(path string, runtime *AILocalRuntime) (*AILocalQASource, error) {
	if runtime == nil || runtime.validate() != nil {
		return nil, invalidLocalRuntime()
	}
	s := &AILocalQASource{path: path, runtime: *runtime}
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
func localQAIDs(ids []int64) bool {
	if len(ids) < 1 || len(ids) > 16 {
		return false
	}
	for i, id := range ids {
		if id <= 0 || id > 9007199254740991 || i > 0 && id <= ids[i-1] {
			return false
		}
	}
	return true
}
func (s *AILocalQASource) read() (*AILocalQA, error) {
	if s == nil {
		return nil, invalidLocalRuntime()
	}
	b, err := readAILocalFile(s.path, 32<<10)
	if err != nil || task.CheckStageJSON(b) != nil || !exactLocalPlanJSON(b, reflect.TypeOf(AILocalQA{})) {
		return nil, invalidLocalRuntime()
	}
	var c AILocalQA
	if json.Unmarshal(b, &c) != nil {
		return nil, invalidLocalRuntime()
	}
	// v1保持原闭集契约，即使新增开关显式传false/空串也不能借旧版本启用新配置。
	if c.Version == 1 {
		var fields map[string]json.RawMessage
		var models []map[string]json.RawMessage
		if json.Unmarshal(b, &fields) != nil || json.Unmarshal(fields["models"], &models) != nil {
			return nil, invalidLocalRuntime()
		}
		for _, m := range models {
			if m["json_schema_fallback"] != nil || m["reasoning_effort"] != nil {
				return nil, invalidLocalRuntime()
			}
		}
	}
	now := time.Now().UnixMilli()
	if s.runtime.validate() != nil || !c.validMaterialUse() || c.InstanceID != s.runtime.InstanceID || !localRevision.MatchString(c.Revision) || c.IssuedAtMS <= 0 || c.IssuedAtMS > now || c.ExpiresAtMS <= now || c.ExpiresAtMS <= c.IssuedAtMS || c.ExpiresAtMS-c.IssuedAtMS > 86400000 || !localQAIDs(c.Members) || !localQAIDs(c.Subjects) || len(c.Models) != 3 {
		return nil, invalidLocalRuntime()
	}
	for _, m := range c.Models {
		if c.Version == 1 && (m.JSONSchemaFallback || m.ReasoningEffort != "") || m.ReasoningEffort != "" && m.ReasoningEffort != "none" {
			return nil, invalidLocalRuntime()
		}
		if !localExecutionURL(m.Endpoint.BaseURL, true) || m.Endpoint.TimeoutMS < 1 || m.Endpoint.TimeoutMS > 20000 || !localRef.MatchString(m.FilingNo) || !localRef.MatchString(m.ApprovalRef) || !localRef.MatchString(m.QualityRef) || !localRef.MatchString(m.BudgetRef) {
			return nil, invalidLocalRuntime()
		}
		client, err := provider.NewDirect(m.ProviderConfig(), true)
		if err != nil || !client.MatchesDirect(m.ProfileDigest, m.Endpoint.Model) {
			return nil, invalidLocalRuntime()
		}
	}
	if _, err := c.QAConfig(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c AILocalQA) validMaterialUse() bool {
	if c.Version == 1 {
		return c.Scope == "synthetic-loopback-only" && c.MaterialUse == nil
	}
	u := c.MaterialUse
	if c.Version != 2 || c.Scope != "official-materials-development-only" || u == nil || !localPlanDigest.MatchString(u.DatasetDigest) || !localRef.MatchString(u.ApprovalRef) || u.Purpose != "bounded-material-qa" || len(u.Bindings) != len(c.Subjects) || len(u.Bindings) == 0 {
		return false
	}
	for n, b := range u.Bindings {
		if b.SubjectID != c.Subjects[n] || !localRef.MatchString(b.ScopeApproval) || len(b.Evidence) < 1 || len(b.Evidence) > c.Limits.MaxSources || len(b.Evidence) > 8 {
			return false
		}
		for i, e := range b.Evidence {
			if e.ID <= 0 || e.ID > 9007199254740991 || !localPlanDigest.MatchString(e.Hash) || i > 0 && e.ID <= b.Evidence[i-1].ID {
				return false
			}
		}
	}
	return true
}
func (s *AILocalQASource) Current(ctx context.Context) (*AILocalQA, error) {
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
