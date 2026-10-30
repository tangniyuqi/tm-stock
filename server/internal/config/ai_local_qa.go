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

// 只批准本机合成资料协议验收；不承载正文、凭据、生产许可或积分定价。
type AILocalQA struct {
	Version        int              `json:"version"`
	InstanceID     string           `json:"instance_id"`
	Revision       string           `json:"revision"`
	Scope          string           `json:"scope"`
	IssuedAtMS     int64            `json:"issued_at_ms"`
	ExpiresAtMS    int64            `json:"expires_at_ms"`
	Members        []int64          `json:"members"`
	Subjects       []int64          `json:"subjects"`
	Models         []AILocalQAModel `json:"models"`
	AllowedDomains []string         `json:"allowed_domains"`
	Limits         AILocalQALimits  `json:"limits"`
}

type AILocalQAModel struct {
	Endpoint      AILocalExecutionModel `json:"endpoint"`
	FilingNo      string                `json:"filing_no"`
	ApprovalRef   string                `json:"approval_ref"`
	QualityRef    string                `json:"quality_ref"`
	BudgetRef     string                `json:"budget_ref"`
	ProfileDigest string                `json:"profile_digest"`
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
	return provider.Config{Name: m.Endpoint.Name, BaseURL: m.Endpoint.BaseURL, Model: m.Endpoint.Model, TimeoutMS: m.Endpoint.TimeoutMS}
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
	now := time.Now().UnixMilli()
	if s.runtime.validate() != nil || c.Version != 1 || c.InstanceID != s.runtime.InstanceID || !localRevision.MatchString(c.Revision) || c.Scope != "synthetic-loopback-only" || c.IssuedAtMS <= 0 || c.IssuedAtMS > now || c.ExpiresAtMS <= now || c.ExpiresAtMS <= c.IssuedAtMS || c.ExpiresAtMS-c.IssuedAtMS > 86400000 || !localQAIDs(c.Members) || !localQAIDs(c.Subjects) || len(c.Models) != 3 {
		return nil, invalidLocalRuntime()
	}
	for _, m := range c.Models {
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
