// Package finalreport 定义内部报告终检；没有生产默认通过策略或外部执行器。
package finalreport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// Version 绑定结构、最小形态与证据校验语义；语义变更必须递增，配置/词表另参与摘要。
const Version = "closed-quote-report-v1"
const MaxSnapshots = 32
const MaxEvidenceBodyBytes = 1 << 20
const MaxTotalEvidenceBytes = 4 << 20

var ErrRejected = fmt.Errorf("报告未通过终检")

type Verdict string

const (
	Pass      Verdict = "pass"
	Block     Verdict = "block"
	Uncertain Verdict = "uncertain"
)

func (v Verdict) Valid() bool { return v == Pass || v == Block || v == Uncertain }

// Proposal 只由可信内部渲染步骤生成，不是客户端输入或上游研究原文。
type Proposal struct {
	TaskNo    string        `json:"task_no"`
	ThemeID   int64         `json:"theme_id"`
	ThemeName string        `json:"theme_name"`
	Report    render.Report `json:"report"`
}

type Config struct {
	Ref, ModelName, FilingNo, EntailmentVersion, SemanticVersion string
	AllowedDomains                                               []string
	CompanyDigest                                                string `json:",omitempty"`
}

// EntailmentInput 的字符串是独立不可变副本；不含账户、许可归档或其他任务正文。
type EntailmentInput struct{ ProposalJSON, EvidenceJSON string }
type EntailmentCheck func(context.Context, EntailmentInput) (Verdict, error)
type SemanticCheck func(context.Context, string) (Verdict, error)

// Policy 的回调必须是可信、受限的纯校验器；没有模型调用、工具或默认放行实现。
type Policy struct {
	cfg        Config
	digest     string
	entailment EntailmentCheck
	semantic   SemanticCheck
	companies  *CompanyDraft
}

func NewPolicy(c Config, entailment EntailmentCheck, semantic SemanticCheck) (*Policy, error) {
	if c.CompanyDigest != "" {
		return nil, points.ErrInvalid
	}
	for _, s := range []string{c.Ref, c.ModelName, c.FilingNo, c.EntailmentVersion, c.SemanticVersion} {
		if s == "" || len(s) > 128 || guard.Normalize(s) != s || guard.Blocked(s) || strings.ContainsAny(s, "\r\n") {
			return nil, points.ErrInvalid
		}
		for _, r := range s {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				return nil, points.ErrInvalid
			}
		}
	}
	if entailment == nil || semantic == nil || utf8.RuneCountInString(c.ModelName) > 64 || len(c.AllowedDomains) < 1 || len(c.AllowedDomains) > MaxSnapshots {
		return nil, points.ErrInvalid
	}
	c.AllowedDomains = slices.Clone(c.AllowedDomains)
	slices.Sort(c.AllowedDomains)
	for i, d := range c.AllowedDomains {
		if i > 0 && d == c.AllowedDomains[i-1] {
			return nil, points.ErrInvalid
		}
		u, err := sources.ValidateURL(d, "https://"+d+"/")
		if err != nil || u != "https://"+d+"/" {
			return nil, points.ErrInvalid
		}
	}
	b, err := json.Marshal([]any{Version, guard.PolicyDigest(), c})
	if err != nil {
		return nil, points.ErrInvalid
	}
	return &Policy{cfg: c, digest: task.StageDigest(b), entailment: entailment, semantic: semantic}, nil
}

func (p *Policy) Valid() bool {
	return p != nil && p.entailment != nil && p.semantic != nil && ValidDigest(p.digest)
}
func (p *Policy) Digest() string {
	if p == nil {
		return ""
	}
	return p.digest
}
func (p *Policy) StagePolicy() task.StagePolicy {
	return task.StagePolicy{Digest: p.Digest(), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if in.Stage != "judge" {
			return ErrRejected
		}
		_, err := p.Decode(in.Payload)
		return err
	}}
}

var taskNoPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidDigest(v string) bool { return digestPattern.MatchString(v) }

func validProposalIdentity(taskNo string, themeID int64, themeName string) bool {
	return taskNoPattern.MatchString(taskNo) && themeID > 0 && themeName != "" && utf8.ValidString(themeName) && utf8.RuneCountInString(themeName) <= 32 && guard.Normalize(themeName) == themeName && !guard.Blocked(themeName)
}

var sectionOrder = []render.SectionKey{render.SecDefinition, render.SecTimeline, render.SecSegment, render.SecCompanies}

// Decode 拒绝非规范编码与未知/重复字段；原规则只接引文，显式公司规则仅接等同原全集的数据行。
func (p *Policy) Decode(raw string) (Proposal, error) {
	var v Proposal
	if !p.Valid() || task.CheckStageJSON([]byte(raw)) != nil {
		return v, ErrRejected
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return Proposal{}, ErrRejected
	}
	canonical, err := json.Marshal(v)
	if err != nil || !bytes.Equal(canonical, []byte(raw)) || !validProposalIdentity(v.TaskNo, v.ThemeID, v.ThemeName) {
		return Proposal{}, ErrRejected
	}
	if render.Validate(&v.Report, render.Options{AllowedLinkDomains: p.cfg.AllowedDomains, RequireFiling: true}) != nil || v.Report.Disclosure != (render.Disclosure{AIGenerated: true, ModelName: p.cfg.ModelName, FilingNo: p.cfg.FilingNo}) || len(v.Report.Sections) != len(sectionOrder) {
		return Proposal{}, ErrRejected
	}
	seen := map[string]bool{}
	total := 0
	for i, s := range v.Report.Sections {
		if p.companies != nil && s.Key == render.SecCompanies {
			if i != 3 || v.ThemeID != p.companies.scope.SubjectID || v.ThemeName != p.companies.name || !p.companySection(s) {
				return Proposal{}, ErrRejected
			}
			continue
		}
		if s.Key != sectionOrder[i] || len(s.Nodes) < 1 || len(s.Nodes) > 16 {
			return Proposal{}, ErrRejected
		}
		for _, n := range s.Nodes {
			total++
			q := n.Quote
			if n.Kind != render.KindQuote || n.Provenance != render.ProvAIExtractedQuote || q == nil || q.Trimmed || q.Reprint != nil || q.SourceTitle != "" || q.URL == "" || !strings.Contains(q.Text, v.ThemeName) {
				return Proposal{}, ErrRejected
			}
			key := fmt.Sprintf("%d:%d:%d", q.SnapshotID, q.Start, q.End)
			if seen[key] {
				return Proposal{}, ErrRejected
			}
			seen[key] = true
		}
	}
	if total > 64 || len(v.SnapshotIDs()) > MaxSnapshots {
		return Proposal{}, ErrRejected
	}
	return v, nil
}

func (v Proposal) SnapshotIDs() []int64 {
	m := map[int64]bool{}
	for _, s := range v.Report.Sections {
		for _, n := range s.Nodes {
			if n.Quote != nil {
				m[n.Quote.SnapshotID] = true
			}
		}
	}
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (p *Policy) Bind(v Proposal, snaps []sources.Snapshot, theme string) error {
	if !p.Valid() || theme != v.ThemeName || len(snaps) < 1 || len(snaps) > MaxSnapshots || len(snaps) != len(p.EvidenceIDs(v)) {
		return ErrRejected
	}
	m := map[int64]sources.Snapshot{}
	total := 0
	for _, s := range snaps {
		total += len(s.Body)
		if len(s.Body) > MaxEvidenceBodyBytes || total > MaxTotalEvidenceBytes || len(s.Sentences) > 10000 || s.Validate() != nil || m[s.ID].ID != 0 || s.ID <= 0 {
			return ErrRejected
		}
		m[s.ID] = s
	}
	if p.companies != nil {
		for _, ref := range p.companies.materials {
			s, ok := m[ref.SnapshotID]
			if !ok || s.Hash != ref.Digest || s.URL != ref.URL || s.SourceCategory != ref.Category || !s.FetchedAt.Equal(ref.CollectedAt) {
				return ErrRejected
			}
		}
	}
	for _, sec := range v.Report.Sections {
		if p.companies != nil && sec.Key == render.SecCompanies {
			if !p.companySection(sec) {
				return ErrRejected
			}
			continue
		}
		for _, n := range sec.Nodes {
			q := n.Quote
			if q == nil {
				return ErrRejected
			}
			s, ok := m[q.SnapshotID]
			if !ok || q.ContentHash != s.Hash || q.URL != s.URL || q.FetchedAt != s.FetchedAt.UnixMilli() || q.SourceType != sourceType(s.SourceCategory) || q.Start < 0 || q.End > len(s.Body) || q.End <= q.Start || !utf8.ValidString(s.Body[q.Start:q.End]) || q.Text != s.Body[q.Start:q.End] {
				return ErrRejected
			}
		}
	}
	return nil
}

func sourceType(c string) string {
	switch c {
	case sources.CategoryAnnouncement:
		return "公告"
	case sources.CategoryAnnualReport:
		return "年报"
	case sources.CategoryProspectus:
		return "招股书"
	case sources.CategoryOfficialList:
		return "官方产业目录"
	case sources.CategoryInteractive:
		return "互动易问答"
	}
	return ""
}

// Review 只把最终可见文字交给语义检查，授权引用等内部元数据不进入判定输入。
func (p *Policy) Review(ctx context.Context, raw string, v Proposal, snaps []sources.Snapshot) (Verdict, Verdict, string, error) {
	if err := ctx.Err(); err != nil {
		return Uncertain, Uncertain, "", err
	}
	if !p.Valid() {
		return Uncertain, Uncertain, "entailment", nil
	}
	entailmentInput, visible, err := p.reviewInputs(raw, v, snaps)
	if err != nil {
		return Uncertain, Uncertain, "entailment", nil
	}
	e, err := p.entailment(ctx, entailmentInput)
	if ctx.Err() != nil {
		return Uncertain, Uncertain, "", ctx.Err()
	}
	if err != nil || !e.Valid() {
		e = Uncertain
	}
	if e != Pass {
		return e, Uncertain, "entailment", nil
	}
	s, err := p.semantic(ctx, visible)
	if ctx.Err() != nil {
		return e, Uncertain, "", ctx.Err()
	}
	if err != nil || !s.Valid() {
		s = Uncertain
	}
	if s != Pass {
		return e, s, "semantic", nil
	}
	return e, s, "", nil
}

// Inputs 与终检使用同一输入构造函数；调用模型前先过结构/出处绑定，不把许可归档发给模型。
func (p *Policy) Inputs(raw string, snaps []sources.Snapshot) (EntailmentInput, string, error) {
	v, err := p.Decode(raw)
	if err != nil {
		return EntailmentInput{}, "", err
	}
	if p.Bind(v, snaps, v.ThemeName) != nil {
		return EntailmentInput{}, "", ErrRejected
	}
	return p.reviewInputs(raw, v, snaps)
}
func (p *Policy) reviewInputs(raw string, v Proposal, snaps []sources.Snapshot) (EntailmentInput, string, error) {
	type evidence struct {
		ID         int64
		Hash, Body string
	}
	rows := make([]evidence, 0, len(snaps))
	for _, s := range snaps {
		rows = append(rows, evidence{s.ID, s.Hash, s.Body})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return EntailmentInput{}, "", points.ErrInvalid
	}
	text := []string{v.ThemeName, p.cfg.ModelName, p.cfg.FilingNo}
	for _, s := range v.Report.Sections {
		text = append(text, s.Title)
		for _, n := range s.Nodes {
			text = append(text, n.Text)
			if n.Row != nil {
				text = append(text, n.Row.Name, n.Row.Code, n.Row.Node)
				for _, e := range n.Row.Evidence {
					text = append(text, e.Type, e.Excerpt, e.URL)
					if e.Trimmed {
						text = append(text, "节选")
					}
				}
			}
		}
	}
	return EntailmentInput{raw, string(b)}, strings.Join(text, "\n"), nil
}
