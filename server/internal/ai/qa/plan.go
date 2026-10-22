// Package qa 定义只选编号、逐字回取及独立判定的受限资料问答核；不接积分或公共路由。
package qa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const Version = "scoped-citation-qa-v1"

var (
	ErrInvalid     = errors.New("问答计划无效")
	ErrUnavailable = errors.New("问答范围或执行条件不可用")
	ErrProtocol    = errors.New("问答模型结果未通过受限协议")
	ErrNotFound    = errors.New("问答记录不存在")
	ErrConflict    = errors.New("问答原键或执行状态冲突")
	ErrQuota       = errors.New("问答安全配额不足")
)

type Identity struct{ UserID, ConversationID, MessageSeq int64 }

func (v Identity) Valid() bool {
	return conversation.ID(v.UserID) && conversation.ID(v.ConversationID) && conversation.ID(v.MessageSeq)
}

// Scope必须由当前授权仓储构造；字段完整不代表授权引用已被真实核验。
type Scope struct {
	SubjectType              string
	SubjectID                int64
	SubjectName, ApprovalRef string
	ValidUntil               time.Time
	Snapshots                []sources.Snapshot
}

func (Scope) String() string     { return "问答授权范围（资料已隐藏）" }
func (s Scope) GoString() string { return s.String() }

type Model struct{ Name, FilingNo, ApprovalRef, QualityRef, BudgetRef, ProfileDigest string }
type Limits struct {
	MaxSources, MaxSentences, MaxEvidenceBytes, MaxPromptBytes int
	MaxInputTokens, MaxOutputTokens, MaxTotalTokens            int
	Timeout                                                    time.Duration
}

func (v Limits) validMax(maxSources int) bool {
	return v.MaxSources >= 1 && v.MaxSources <= maxSources && v.MaxSentences >= 1 && v.MaxSentences <= 512 && v.MaxEvidenceBytes >= 1 && v.MaxEvidenceBytes <= 64<<10 && v.MaxPromptBytes >= 1024 && v.MaxPromptBytes <= 128<<10 && v.MaxInputTokens >= 1 && v.MaxInputTokens <= 1<<18 && v.MaxOutputTokens >= 128 && v.MaxOutputTokens <= 2048 && v.MaxTotalTokens >= 3*(v.MaxInputTokens+v.MaxOutputTokens) && v.MaxTotalTokens <= 1<<20 && v.Timeout >= time.Millisecond && v.Timeout <= 20*time.Second
}

func (v Limits) Valid() bool { return v.validMax(8) }

type Config struct {
	Selector, Entailment, Semantic Model
	AllowedDomains                 []string
	Limits                         Limits
}
type EvidenceProof struct {
	ID   int64
	Hash string
}
type Proof struct {
	Identity                                 Identity
	InputDigest, PolicyDigest, ScopeApproval string
	SubjectType                              string
	SubjectID                                int64
	SubjectName                              string
	ValidUntil                               time.Time
	Evidence                                 []EvidenceProof
	Models                                   []Model
	MaxCalls, MaxTokens                      int
}

func (Proof) String() string     { return "问答执行证明（身份和摘要已隐藏）" }
func (p Proof) GoString() string { return p.String() }

type Plan struct {
	identity       Identity
	input          conversation.MessageInput
	scope          Scope
	cfg            Config
	digest, policy string
}

func (Plan) String() string     { return "受限问答计划（问题和资料已隐藏）" }
func (p Plan) GoString() string { return p.String() }

func meta(s string) bool {
	if !conversation.Text(s, 128) || guard.Normalize(s) != s || guard.Blocked(s) {
		return false
	}
	for _, r := range s {
		if unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
func digest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (m Model) valid() bool {
	return meta(m.Name) && meta(m.FilingNo) && meta(m.ApprovalRef) && meta(m.QualityRef) && meta(m.BudgetRef) && digest(m.ProfileDigest)
}

func NewPlan(id Identity, in conversation.MessageInput, scope Scope, c Config, now time.Time) (*Plan, error) {
	return newPlan(id, in, scope, c, now, "material_qa", 8)
}

// newPlan 是资料问答与报告追问共用的构造器；技能和快照上限由调用方固定，不能来自请求。
func newPlan(id Identity, in conversation.MessageInput, scope Scope, c Config, now time.Time, skill string, maxSources int) (*Plan, error) {
	if !id.Valid() || !in.Valid() || in.Kind != "question" || in.Skill != skill || in.SubjectID != scope.SubjectID || in.SubjectType != scope.SubjectType || !meta(scope.SubjectName) || !meta(scope.ApprovalRef) || !conversation.Time(now) || !conversation.Time(scope.ValidUntil) || !scope.ValidUntil.After(now) || !c.Limits.validMax(maxSources) || !c.Selector.valid() || !c.Entailment.valid() || !c.Semantic.valid() || len(c.AllowedDomains) < 1 || len(c.AllowedDomains) > 16 || len(scope.Snapshots) > maxSources {
		return nil, ErrInvalid
	}
	c.AllowedDomains = slices.Clone(c.AllowedDomains)
	slices.Sort(c.AllowedDomains)
	for n, d := range c.AllowedDomains {
		canonical, err := sources.ValidateURL(d, "https://"+d+"/")
		if err != nil || canonical != "https://"+d+"/" || n > 0 && d == c.AllowedDomains[n-1] {
			return nil, ErrInvalid
		}
	}
	scope.Snapshots = slices.Clone(scope.Snapshots)
	slices.SortFunc(scope.Snapshots, func(a, b sources.Snapshot) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	bytes, sentences := 0, 0
	for n := range scope.Snapshots {
		s := &scope.Snapshots[n]
		bytes += len(s.Body)
		sentences += len(s.Sentences)
		if !conversation.ID(s.ID) || !conversation.ID(s.SourceID) || n > 0 && s.ID == scope.Snapshots[n-1].ID || bytes > c.Limits.MaxEvidenceBytes || sentences > c.Limits.MaxSentences || s.Validate() != nil || !conversation.Time(s.FetchedAt) || s.FetchedAt.After(now) || s.LicenseExpires != nil && (!s.LicenseExpires.After(now) || s.LicenseExpires.Before(scope.ValidUntil)) {
			return nil, ErrInvalid
		}
		u, err := url.Parse(s.URL)
		if err != nil || !slices.Contains(c.AllowedDomains, u.Hostname()) {
			return nil, ErrInvalid
		}
		canonical, err := sources.ValidateURL(u.Hostname(), s.URL)
		if err != nil || canonical != s.URL {
			return nil, ErrInvalid
		}
		s.Sentences = slices.Clone(s.Sentences)
		if s.LicenseExpires != nil {
			v := *s.LicenseExpires
			s.LicenseExpires = &v
		}
	}
	policy, err := json.Marshal([]any{Version, guard.PolicyDigest(), c, selectSystem, selectSchema, entailSystem, semanticSystem, checkSchema})
	if err != nil {
		return nil, ErrInvalid
	}
	p := &Plan{identity: id, input: in, scope: scope, cfg: c, policy: task.StageDigest(policy)}
	raw, err := json.Marshal([]any{Version, id, in, scope, p.policy})
	if err != nil {
		return nil, ErrInvalid
	}
	p.digest = task.StageDigest(raw)
	return p, nil
}
func (p *Plan) Valid() bool {
	return p != nil && p.identity.Valid() && digest(p.digest) && digest(p.policy)
}
func (p *Plan) Proof() Proof {
	if !p.Valid() {
		return Proof{}
	}
	v := Proof{Identity: p.identity, InputDigest: p.digest, PolicyDigest: p.policy, ScopeApproval: p.scope.ApprovalRef, SubjectType: p.scope.SubjectType, SubjectID: p.scope.SubjectID, SubjectName: p.scope.SubjectName, ValidUntil: p.scope.ValidUntil, MaxCalls: 3, MaxTokens: p.cfg.Limits.MaxTotalTokens, Models: []Model{p.cfg.Selector, p.cfg.Entailment, p.cfg.Semantic}, Evidence: []EvidenceProof{}}
	for _, s := range p.scope.Snapshots {
		v.Evidence = append(v.Evidence, EvidenceProof{s.ID, s.Hash})
	}
	return v
}

type promptSentence struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}
type promptSource struct {
	ID        int64            `json:"snapshot_id"`
	Hash      string           `json:"hash"`
	Sentences []promptSentence `json:"sentences"`
}
type promptData struct {
	SubjectType string         `json:"subject_type"`
	SubjectID   int64          `json:"subject_id"`
	SubjectName string         `json:"subject_name"`
	Question    string         `json:"question"`
	Sources     []promptSource `json:"sources"`
	Picks       []Citation     `json:"picks"`
}

func (p *Plan) request(stage string, picks []Citation) (provider.Request, Model, error) {
	if !p.Valid() {
		return provider.Request{}, Model{}, ErrInvalid
	}
	d := promptData{p.scope.SubjectType, p.scope.SubjectID, p.scope.SubjectName, p.input.Question, []promptSource{}, slices.Clone(picks)}
	for _, s := range p.scope.Snapshots {
		row := promptSource{ID: s.ID, Hash: s.Hash, Sentences: []promptSentence{}}
		for _, sent := range s.Sentences {
			row.Sentences = append(row.Sentences, promptSentence{sent.Number, sent.Text})
		}
		d.Sources = append(d.Sources, row)
	}
	raw, err := json.Marshal(struct {
		Data promptData `json:"data"`
	}{d})
	if err != nil {
		return provider.Request{}, Model{}, ErrInvalid
	}
	system, schema, m := selectSystem, selectSchema, p.cfg.Selector
	switch stage {
	case "select":
	case "entailment":
		system, schema, m = entailSystem, checkSchema, p.cfg.Entailment
	case "semantic":
		system, schema, m = semanticSystem, checkSchema, p.cfg.Semantic
	default:
		return provider.Request{}, Model{}, ErrInvalid
	}
	// 每个UTF-8字节计一个输入token并额外预留协议开销；不是供应商实际用量。
	if len(raw)+len(system)+len(schema) > p.cfg.Limits.MaxPromptBytes || len(raw)+len(system)+len(schema)+2048 > p.cfg.Limits.MaxInputTokens {
		return provider.Request{}, Model{}, ErrInvalid
	}
	zero := 0.0
	return provider.Request{System: system, Prompt: string(raw), Model: m.Name, Format: provider.FormatJSONSchema, Schema: json.RawMessage(schema), Strict: true, Temperature: &zero, MaxTokens: p.cfg.Limits.MaxOutputTokens, Attempt: 1, AttemptTimeout: p.cfg.Limits.Timeout}, m, nil
}

// 所有引用都由当前快照回取；本方法不认定提问相关性或模型判定已通过。
func (p *Plan) quotes(picks []Citation) ([]Quote, error) {
	if !p.Valid() || len(picks) < 1 || len(picks) > 4 {
		return nil, ErrProtocol
	}
	seen := map[string]bool{}
	out := make([]Quote, 0, len(picks))
	for _, pick := range picks {
		n := slices.IndexFunc(p.scope.Snapshots, func(s sources.Snapshot) bool { return s.ID == pick.SnapshotID })
		if n < 0 || len(pick.SentenceIDs) < 1 || len(pick.SentenceIDs) > 8 {
			return nil, ErrProtocol
		}
		s := p.scope.Snapshots[n]
		for i, id := range pick.SentenceIDs {
			key := fmt.Sprintf("%d:%d", pick.SnapshotID, id)
			if id < 1 || id > len(s.Sentences) || i > 0 && id != pick.SentenceIDs[i-1]+1 || seen[key] {
				return nil, ErrProtocol
			}
			seen[key] = true
		}
		start, end := s.Sentences[pick.SentenceIDs[0]-1].StartByte, s.Sentences[pick.SentenceIDs[len(pick.SentenceIDs)-1]-1].EndByte
		text := s.Body[start:end]
		if utf8.RuneCountInString(text) > 1000 || guard.Blocked(text) {
			return nil, ErrProtocol
		}
		out = append(out, Quote{SnapshotID: s.ID, Start: start, End: end, Text: text, SourceType: s.SourceCategory, SourceURL: s.URL, CollectedAt: s.FetchedAt.UnixMilli()})
	}
	slices.SortFunc(out, func(a, b Quote) int {
		if a.SnapshotID < b.SnapshotID {
			return -1
		}
		if a.SnapshotID > b.SnapshotID {
			return 1
		}
		return a.Start - b.Start
	})
	return out, nil
}

func clonePicks(in []Citation) []Citation {
	out := slices.Clone(in)
	for n := range out {
		out[n].SentenceIDs = slices.Clone(out[n].SentenceIDs)
	}
	return out
}
func safeQuestion(q string) bool { return !guard.Blocked(q) && !strings.Contains(q, "\x00") }
