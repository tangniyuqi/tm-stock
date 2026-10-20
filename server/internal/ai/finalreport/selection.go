package finalreport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// SelectionVersion 绑定编号回取与排序规则，不表示蕴含或语义质量已经验收。
const SelectionVersion = "closed-report-selection-v1"

// Selection 只允许模型选择出处与连续句子编号；正文、偏移、链接、公示均由代码生成。
type Selection struct {
	Section     render.SectionKey `json:"section"`
	SnapshotID  int64             `json:"snapshot_id"`
	SentenceIDs []int             `json:"sentence_ids"`
}

type SelectionResponse struct {
	Selections []Selection `json:"selections"`
}

// SelectionStage 固定可信任务及快照副本；调用方须从当前授权仓储读取，不能接受客户端快照。
// 本类型不调用模型、不发布报告，终检服务仍须重新核验当前授权与两道判定。
type SelectionStage struct {
	policy      *Policy
	taskNo      string
	themeID     int64
	themeName   string
	snapshots   []sources.Snapshot
	inputDigest string
	digest      string
}

func NewSelectionStage(p *Policy, taskNo string, themeID int64, themeName string, snapshots []sources.Snapshot) (*SelectionStage, error) {
	if !p.Valid() || !validProposalIdentity(taskNo, themeID, themeName) || len(snapshots) < 1 || len(snapshots) > MaxSnapshots {
		return nil, points.ErrInvalid
	}
	s := &SelectionStage{policy: p, taskNo: taskNo, themeID: themeID, themeName: themeName}
	s.snapshots = slices.Clone(snapshots)
	slices.SortFunc(s.snapshots, func(a, b sources.Snapshot) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	total := 0
	for i := range s.snapshots {
		v := &s.snapshots[i]
		total += len(v.Body)
		if v.ID <= 0 || i > 0 && v.ID == s.snapshots[i-1].ID || len(v.Body) > MaxEvidenceBodyBytes || total > MaxTotalEvidenceBytes || len(v.Sentences) > 10000 || v.Validate() != nil {
			return nil, points.ErrInvalid
		}
		u, err := url.Parse(v.URL)
		if err != nil {
			return nil, points.ErrInvalid
		}
		canonical, err := sources.ValidateURL(u.Hostname(), v.URL)
		if err != nil || canonical != v.URL || !slices.Contains(p.cfg.AllowedDomains, u.Hostname()) {
			return nil, points.ErrInvalid
		}
		v.Sentences = slices.Clone(v.Sentences)
		if v.LicenseExpires != nil {
			cp := *v.LicenseExpires
			v.LicenseExpires = &cp
		}
	}
	b, err := json.Marshal([]any{SelectionVersion, p.Digest(), taskNo, themeID, themeName, s.snapshots})
	if err != nil {
		return nil, points.ErrInvalid
	}
	s.inputDigest = task.StageDigest(b)
	s.digest = task.StageDigest([]byte(SelectionVersion + ":" + s.inputDigest))
	return s, nil
}

func (s *SelectionStage) Valid() bool {
	return s != nil && s.policy.Valid() && ValidDigest(s.inputDigest) && ValidDigest(s.digest)
}
func (s *SelectionStage) InputDigest() string {
	if !s.Valid() {
		return ""
	}
	return s.inputDigest
}
func (s *SelectionStage) Matches(v task.Task, p *Policy) bool {
	return s.Valid() && p.Valid() && p.Digest() == s.policy.Digest() && v.No == s.taskNo && v.SubjectType == task.Theme && v.SubjectID == s.themeID && v.AnalysisType == 1
}
func (s *SelectionStage) SnapshotIDs() []int64 {
	if !s.Valid() {
		return nil
	}
	out := make([]int64, len(s.snapshots))
	for i, v := range s.snapshots {
		out[i] = v.ID
	}
	return out
}
func (s *SelectionStage) MatchesEvidence(snapshots []sources.Snapshot, theme string) bool {
	if !s.Valid() {
		return false
	}
	other, err := NewSelectionStage(s.policy, s.taskNo, s.themeID, theme, snapshots)
	return err == nil && other.inputDigest == s.inputDigest
}
func (s *SelectionStage) Policy() task.StagePolicy {
	if !s.Valid() {
		return task.StagePolicy{}
	}
	return task.StagePolicy{Digest: s.digest, Check: func(ctx context.Context, in task.StageValidationInput) error {
		if in.Stage != "extract" || in.InputDigest != s.inputDigest {
			return task.ErrStagePolicy
		}
		_, err := s.Build(ctx, in.Payload)
		return err
	}}
}
func (s *SelectionStage) proposal() Proposal {
	return Proposal{TaskNo: s.taskNo, ThemeID: s.themeID, ThemeName: s.themeName, Report: render.Report{SchemaVer: render.CurrentSchemaVer,
		Disclosure: render.Disclosure{AIGenerated: true, ModelName: s.policy.cfg.ModelName, FilingNo: s.policy.cfg.FilingNo}}}
}

func decodeSelection(raw string) (SelectionResponse, error) {
	var out SelectionResponse
	if task.CheckStageJSON([]byte(raw)) != nil {
		return out, ErrRejected
	}
	var top map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &top) != nil || len(top) != 1 || top["selections"] == nil {
		return out, ErrRejected
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(top["selections"], &items) != nil || len(items) < 3 || len(items) > 64 {
		return out, ErrRejected
	}
	for _, fields := range items {
		if len(fields) != 3 || fields["section"] == nil || fields["snapshot_id"] == nil || fields["sentence_ids"] == nil {
			return out, ErrRejected
		}
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return SelectionResponse{}, ErrRejected
	}
	return out, nil
}

// Build 整份成功或整份拒绝；不截取/修补/去掉坏条目，不把章节选择当作事实蕴含证明。
func (s *SelectionStage) Build(ctx context.Context, raw string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !s.Valid() {
		return "", points.ErrInvalid
	}
	in, err := decodeSelection(raw)
	if err != nil {
		return "", err
	}
	sections := map[render.SectionKey][]render.Node{}
	used := map[string]bool{}
	for _, pick := range in.Selections {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if !slices.Contains(sectionOrder, pick.Section) || len(pick.SentenceIDs) < 1 || len(pick.SentenceIDs) > 8 {
			return "", ErrRejected
		}
		if s.policy.companies != nil && pick.Section == render.SecCompanies {
			return "", ErrRejected
		}
		idx := slices.IndexFunc(s.snapshots, func(v sources.Snapshot) bool { return v.ID == pick.SnapshotID })
		if idx < 0 {
			return "", ErrRejected
		}
		v := s.snapshots[idx]
		for i, id := range pick.SentenceIDs {
			key := fmt.Sprintf("%d:%d", v.ID, id)
			if id < 1 || id > len(v.Sentences) || i > 0 && id != pick.SentenceIDs[i-1]+1 || used[key] {
				return "", ErrRejected
			}
			used[key] = true
		}
		start, end := v.Sentences[pick.SentenceIDs[0]-1].StartByte, v.Sentences[pick.SentenceIDs[len(pick.SentenceIDs)-1]-1].EndByte
		n, err := render.QuoteNode(render.Quote{SnapshotID: v.ID, Start: start, End: end, Text: v.Body[start:end], SourceType: sourceType(v.SourceCategory), URL: v.URL,
			FetchedAt: v.FetchedAt.UnixMilli(), ContentHash: v.Hash}, render.ProvAIExtractedQuote)
		if err != nil {
			return "", ErrRejected
		}
		sections[pick.Section] = append(sections[pick.Section], n)
	}
	v := s.proposal()
	for _, key := range sectionOrder {
		nodes := sections[key]
		if s.policy.companies != nil && key == render.SecCompanies {
			title, _ := render.SectionTitle(key)
			v.Report.Sections = append(v.Report.Sections, render.Section{Key: key, Title: title, Nodes: s.policy.companies.Rows()})
			continue
		}
		// 只按客观出处编号/字节位置排序，模型返回顺序不成为显示权重。
		slices.SortFunc(nodes, func(a, b render.Node) int {
			if a.Quote.SnapshotID < b.Quote.SnapshotID {
				return -1
			}
			if a.Quote.SnapshotID > b.Quote.SnapshotID {
				return 1
			}
			return a.Quote.Start - b.Quote.Start
		})
		// key 来自本包封闭章节表；仍让最终 Decode 检查标题是否一致。
		title, ok := render.SectionTitle(key)
		if !ok {
			return "", points.ErrInvariant
		}
		v.Report.Sections = append(v.Report.Sections, render.Section{Key: key, Title: title, AILabel: true, Nodes: nodes})
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", points.ErrInvalid
	}
	out := string(b)
	decoded, err := s.policy.Decode(out)
	if err != nil {
		return "", ErrRejected
	}
	refs := make([]sources.Snapshot, 0)
	for _, v := range s.snapshots {
		if slices.Contains(s.policy.EvidenceIDs(decoded), v.ID) {
			refs = append(refs, v)
		}
	}
	if s.policy.Bind(decoded, refs, s.themeName) != nil {
		return "", ErrRejected
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return out, nil
}

func (s *SelectionStage) String() string   { return "编号证据转换策略(正文与授权已隐藏)" }
func (s *SelectionStage) GoString() string { return s.String() }
