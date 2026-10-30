package dossier

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const DraftVersion = "dossier-facts-v1"
const MaxCards = 64
const MaxDraftSnapshots = 32
const MaxDraftBodyBytes = 4 << 20
const MaxDraftRecordBytes = 256 << 10

var ErrInvalidDraft = errors.New("专题档案字段或证据绑定无效")
var cardKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// Field只携带原文位置，不接收自由生成值。未知必须显式标记且引用为零。
type Field struct {
	State string      `json:"state"`
	Ref   SentenceRef `json:"ref"`
}

// FactCard的正文为完整原句；各字段保持原文，不进行日期、阶段或主体推断。
type FactCard struct {
	Key           string      `json:"key"`
	Section       string      `json:"section"`
	Statement     SentenceRef `json:"statement"`
	Subject       Field       `json:"subject"`
	PublishedDate Field       `json:"publishedDate"`
	EffectiveDate Field       `json:"effectiveDate"`
	Period        Field       `json:"period"`
	Scope         Field       `json:"scope"`
	Stage         Field       `json:"stage"`
	Basis         Field       `json:"basis"`
	Unit          Field       `json:"unit"`
}

type Gap struct {
	Key     string `json:"key"`
	Section string `json:"section"`
	Reason  string `json:"reason"`
}

// Draft仍是未获发布批准的内部候选；字段正确不等于事实或权限正确。
type Draft struct {
	Protocol  string     `json:"protocol"`
	ThemeID   int64      `json:"themeId"`
	Version   int64      `json:"version"`
	CreatedAt int64      `json:"createdAt"`
	Cards     []FactCard `json:"cards"`
	Gaps      []Gap      `json:"gaps"`
}

type BoundField struct {
	State, Text, Context string
	Ref                  SentenceRef
}

type BoundCard struct {
	Key, Section, Statement                                                  string
	StatementRef                                                             SentenceRef
	Subject, PublishedDate, EffectiveDate, Period, Scope, Stage, Basis, Unit BoundField
}

// BoundDraft只供后续审核与存储装配；不含许可通过标记，不是公开DTO。
type BoundDraft struct {
	canonical []byte
	digest    string
	cards     []BoundCard
	gaps      []Gap
	identity  DraftIdentity
	evidence  []EvidenceRef
	origins   []Origin
}

// Origin仅保存可回溯出处，不含来源许可和内部批准引用；本身不代表读取授权。
type Origin struct {
	SnapshotID          int64
	Hash, Category, URL string
	FetchedAt           int64
}

func (Origin) String() string     { return "专题出处（内部定位已隐藏）" }
func (o Origin) GoString() string { return o.String() }

func (b *BoundDraft) Origins() []Origin {
	if b == nil {
		return nil
	}
	return slices.Clone(b.origins)
}

type DraftIdentity struct{ ThemeID, Version, CreatedAt int64 }

func (Draft) String() string          { return "专题档案候选（内部引用已隐藏）" }
func (d Draft) GoString() string      { return d.String() }
func (BoundCard) String() string      { return "专题事实卡（原文已隐藏）" }
func (c BoundCard) GoString() string  { return c.String() }
func (BoundField) String() string     { return "专题字段（原文已隐藏）" }
func (f BoundField) GoString() string { return f.String() }
func (BoundDraft) String() string     { return "专题绑定结果（原文已隐藏）" }
func (b BoundDraft) GoString() string { return b.String() }

func (b *BoundDraft) Canonical() []byte {
	if b == nil {
		return nil
	}
	return slices.Clone(b.canonical)
}
func (b *BoundDraft) Digest() string {
	if b == nil {
		return ""
	}
	return b.digest
}
func (b *BoundDraft) Cards() []BoundCard {
	if b == nil {
		return nil
	}
	return slices.Clone(b.cards)
}

func (b *BoundDraft) Gaps() []Gap {
	if b == nil {
		return nil
	}
	return slices.Clone(b.gaps)
}

func (b *BoundDraft) Identity() DraftIdentity {
	if b == nil {
		return DraftIdentity{}
	}
	return b.identity
}

func validSection(s string) bool {
	return slices.Contains([]string{"definition", "chain", "policy", "progress", "membership"}, s)
}

type snapshotBinding struct {
	ID, SourceID                                int64
	Category, URL, Hash, LicenseTag, LicenseRef string
	FetchedAt                                   int64
	LicenseExpires                              int64
}

type storedDraft struct {
	Draft    Draft
	Evidence []snapshotBinding
	Guard    string
}

// RestoreDraft从规范存储记录和重新读取的快照重建绑定；不信任已存字段/摘要。
// 当前许可与发布批准仍由调用者校验，本函数不能用于绕过读取授权。
func RestoreDraft(raw []byte, expectedDigest string, snapshots []sources.Snapshot) (*BoundDraft, error) {
	if len(raw) > MaxDraftRecordBytes || task.StageDigest(raw) != expectedDigest || task.CheckStageJSON(raw) != nil {
		return nil, ErrInvalidDraft
	}
	var stored storedDraft
	if json.Unmarshal(raw, &stored) != nil {
		return nil, ErrInvalidDraft
	}
	bound, err := BindDraft(stored.Draft, snapshots)
	if err != nil || !bytes.Equal(raw, bound.canonical) {
		return nil, ErrInvalidDraft
	}
	return bound, nil
}

// BindDraft只验证候选结构、句子与原文位置，不访问数据库、网络或模型。
// 调用者必须另核当前范围、发布批准和语义；该函数不将命中原文判定为支持。
func BindDraft(d Draft, snapshots []sources.Snapshot) (*BoundDraft, error) {
	if d.Protocol != DraftVersion || !conversation.ID(d.ThemeID) || !conversation.ID(d.Version) ||
		d.CreatedAt <= 0 || d.CreatedAt > 8640000000000000 || len(d.Cards) > MaxCards || len(d.Gaps) > MaxCards ||
		len(d.Cards)+len(d.Gaps) == 0 || len(snapshots) > MaxDraftSnapshots {
		return nil, ErrInvalidDraft
	}
	d.Cards, d.Gaps = append([]FactCard{}, d.Cards...), append([]Gap{}, d.Gaps...)
	slices.SortFunc(d.Cards, func(a, b FactCard) int { return compareKey(a.Key, b.Key) })
	slices.SortFunc(d.Gaps, func(a, b Gap) int { return compareKey(a.Key, b.Key) })
	byID := make(map[int64]sources.Snapshot, len(snapshots))
	bindings := make([]snapshotBinding, 0, len(snapshots))
	total := 0
	for _, s := range snapshots {
		total += len(s.Body)
		if total > MaxDraftBodyBytes || len(s.LicenseTag) > 128 || len(s.LicenseRef) > 512 ||
			!utf8.ValidString(s.LicenseTag) || !utf8.ValidString(s.LicenseRef) || !conversation.ID(s.ID) || !conversation.ID(s.SourceID) || !boundedSnapshot(s) ||
			!conversation.Time(s.FetchedAt) || s.FetchedAt.UnixMilli() > d.CreatedAt || s.Validate() != nil {
			return nil, ErrInvalidDraft
		}
		if _, exists := byID[s.ID]; exists {
			return nil, ErrInvalidDraft
		}
		byID[s.ID] = s
		var expires int64
		if s.LicenseExpires != nil {
			if !conversation.Time(*s.LicenseExpires) {
				return nil, ErrInvalidDraft
			}
			expires = s.LicenseExpires.UnixMilli()
		}
		bindings = append(bindings, snapshotBinding{s.ID, s.SourceID, s.SourceCategory, s.URL, s.Hash, s.LicenseTag, s.LicenseRef, s.FetchedAt.UnixMilli(), expires})
	}
	used := make(map[int64]bool, len(snapshots))
	keys := make(map[string]bool, len(d.Cards)+len(d.Gaps))
	bound := make([]BoundCard, 0, len(d.Cards))
	for _, c := range d.Cards {
		if !cardKey.MatchString(c.Key) || keys[c.Key] || !validSection(c.Section) {
			return nil, ErrInvalidDraft
		}
		keys[c.Key] = true
		statement, _, ok := bindSpan(c.Statement, byID, true)
		if !ok {
			return nil, ErrInvalidDraft
		}
		used[c.Statement.SnapshotID] = true
		row := BoundCard{Key: c.Key, Section: c.Section, Statement: statement, StatementRef: c.Statement}
		inputs := []Field{c.Subject, c.PublishedDate, c.EffectiveDate, c.Period, c.Scope, c.Stage, c.Basis, c.Unit}
		outputs := []*BoundField{&row.Subject, &row.PublishedDate, &row.EffectiveDate, &row.Period, &row.Scope, &row.Stage, &row.Basis, &row.Unit}
		for i, f := range inputs {
			if f.State == "unknown" && f.Ref == (SentenceRef{}) {
				*outputs[i] = BoundField{State: "unknown"}
				continue
			}
			if f.State != "known" {
				return nil, ErrInvalidDraft
			}
			text, context, ok := bindSpan(f.Ref, byID, false)
			if !ok {
				return nil, ErrInvalidDraft
			}
			used[f.Ref.SnapshotID] = true
			*outputs[i] = BoundField{State: "known", Text: text, Context: context, Ref: f.Ref}
		}
		bound = append(bound, row)
	}
	for _, g := range d.Gaps {
		if !cardKey.MatchString(g.Key) || keys[g.Key] || !validSection(g.Section) ||
			!slices.Contains([]string{"source_missing", "subject_unverified", "time_unverified", "basis_unverified", "stage_unverified", "unit_unverified", "scope_unverified"}, g.Reason) {
			return nil, ErrInvalidDraft
		}
		keys[g.Key] = true
	}
	if len(used) != len(byID) {
		return nil, ErrInvalidDraft
	}
	slices.SortFunc(bindings, func(a, b snapshotBinding) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	// 规范编码只含引用及元数据，正文按快照哈希绑定；当前许可仍须重新查验。
	raw, err := json.Marshal(storedDraft{d, bindings, guard.PolicyDigest()})
	if err != nil || len(raw) > MaxDraftRecordBytes {
		return nil, ErrInvalidDraft
	}
	refs := make([]EvidenceRef, 0, len(bindings))
	origins := make([]Origin, 0, len(bindings))
	for _, s := range bindings {
		refs = append(refs, EvidenceRef{s.ID, s.SourceID, s.Hash})
		origins = append(origins, Origin{s.ID, s.Hash, s.Category, s.URL, s.FetchedAt})
	}
	return &BoundDraft{canonical: raw, digest: task.StageDigest(raw), cards: bound,
		gaps: slices.Clone(d.Gaps), identity: DraftIdentity{d.ThemeID, d.Version, d.CreatedAt}, evidence: refs, origins: origins}, nil
}

func compareKey(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func bindSpan(ref SentenceRef, snapshots map[int64]sources.Snapshot, whole bool) (string, string, bool) {
	s, ok := snapshots[ref.SnapshotID]
	if !ok || ref.Hash != s.Hash || ref.Number < 1 || ref.Number > len(s.Sentences) {
		return "", "", false
	}
	sentence := s.Sentences[ref.Number-1]
	if ref.StartByte < sentence.StartByte || ref.EndByte > sentence.EndByte || ref.StartByte >= ref.EndByte ||
		whole && (ref.StartByte != sentence.StartByte || ref.EndByte != sentence.EndByte) {
		return "", "", false
	}
	text := s.Body[ref.StartByte:ref.EndByte]
	limit := 200
	if whole {
		limit = 1000
	}
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > limit || guard.Normalize(text) != text ||
		utf8.RuneCountInString(sentence.Text) > 1000 || guard.Blocked(sentence.Text) {
		return "", "", false
	}
	return text, sentence.Text, true
}
