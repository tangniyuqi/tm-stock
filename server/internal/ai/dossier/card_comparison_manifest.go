package dossier

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const CardComparisonManifestVersion = "dossier-card-comparison-manifest-v1"

type cardComparisonManifestDocument struct {
	Protocol        string               `json:"protocol"`
	AnalysisType    int                  `json:"analysisType"`
	Draft           DraftIdentity        `json:"draft"`
	DraftDigest     string               `json:"draftDigest"`
	ThemeName       string               `json:"themeName"`
	Keys            []string             `json:"keys"`
	Config          CardComparisonConfig `json:"config"`
	ComponentDigest string               `json:"componentDigest"`
}

type CardComparisonManifest struct {
	doc       cardComparisonManifestDocument
	canonical []byte
}

func (CardComparisonManifest) String() string {
	return "两卡口径清单（定位与策略已隐藏）"
}
func (m CardComparisonManifest) GoString() string { return m.String() }

func (p *CardComparisonPreparation) Manifest() (*CardComparisonManifest, error) {
	component, err := p.ComponentDigest(task.DeepAnalysisType)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for _, check := range p.base.checks {
		if check.Field == "section" {
			keys = append(keys, check.Card)
		}
	}
	slices.Sort(keys)
	doc := cardComparisonManifestDocument{CardComparisonManifestVersion, task.DeepAnalysisType, p.base.identity, p.base.draftDigest, p.base.themeName, keys, p.cfg, component}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, ErrCardComparison
	}
	return DecodeCardComparisonManifest(raw, component)
}

func DecodeCardComparisonManifest(raw []byte, component string) (*CardComparisonManifest, error) {
	if len(raw) > MaxSupportManifestBytes || !supportDigest(component) || task.CheckStageJSON(raw) != nil {
		return nil, ErrCardComparison
	}
	var doc cardComparisonManifestDocument
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&doc) != nil || doc.Protocol != CardComparisonManifestVersion || doc.ComponentDigest != component || doc.AnalysisType != task.DeepAnalysisType ||
		!conversation.ID(doc.Draft.ThemeID) || !conversation.ID(doc.Draft.Version) || doc.Draft.CreatedAt <= 0 || doc.Draft.CreatedAt > 253402300799999 ||
		!supportDigest(doc.DraftDigest) || !supportMeta(doc.ThemeName, 128) || len(doc.Keys) != 2 || !cardKey.MatchString(doc.Keys[0]) || !cardKey.MatchString(doc.Keys[1]) || doc.Keys[0] >= doc.Keys[1] {
		return nil, ErrCardComparison
	}
	if _, err := doc.Config.PolicyDigest(); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(doc)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, ErrCardComparison
	}
	return &CardComparisonManifest{doc, bytes.Clone(raw)}, nil
}

func (m *CardComparisonManifest) Valid() bool { return m != nil && len(m.canonical) > 0 }
func (m *CardComparisonManifest) Canonical() []byte {
	if !m.Valid() {
		return nil
	}
	return bytes.Clone(m.canonical)
}
func (m *CardComparisonManifest) Identity() DraftIdentity {
	if !m.Valid() {
		return DraftIdentity{}
	}
	return m.doc.Draft
}
func (m *CardComparisonManifest) DraftDigest() string {
	if !m.Valid() {
		return ""
	}
	return m.doc.DraftDigest
}
func (m *CardComparisonManifest) ComponentDigest() string {
	if !m.Valid() {
		return ""
	}
	return m.doc.ComponentDigest
}
func (m *CardComparisonManifest) AnalysisType() int {
	if !m.Valid() {
		return 0
	}
	return m.doc.AnalysisType
}

// Rebuild重取完整已绑定资料；不从清单摘要反推原文，不授予当前读取或AI用途。
func (m *CardComparisonManifest) Rebuild(ctx context.Context, bound *BoundDraft, snapshots []sources.Snapshot) (*CardComparisonPreparation, error) {
	if !m.Valid() || bound == nil || bound.Identity() != m.Identity() || bound.Digest() != m.DraftDigest() {
		return nil, ErrCardComparison
	}
	p, err := PrepareCardComparison(ctx, m.doc.Config, m.doc.ThemeName, bound, snapshots, m.doc.Keys)
	if err != nil {
		return nil, err
	}
	component, err := p.ComponentDigest(m.AnalysisType())
	if err != nil || component != m.ComponentDigest() {
		return nil, ErrCardComparison
	}
	return p, nil
}

func (m *CardComparisonManifest) Registration(at time.Time) (SupportRegistration, error) {
	if !m.Valid() {
		return SupportRegistration{}, ErrCardComparison
	}
	r := SupportRegistration{Identity: m.Identity(), AnalysisType: m.AnalysisType(), ComponentDigest: m.ComponentDigest(), ManifestDigest: task.StageDigest(m.canonical), Canonical: m.Canonical(), CreatedAt: at}
	if _, err := r.CardComparisonManifest(); err != nil {
		return SupportRegistration{}, err
	}
	return r, nil
}

func (r SupportRegistration) CardComparisonManifest() (*CardComparisonManifest, error) {
	if r.CreatedAt.UnixMilli() < r.Identity.CreatedAt || r.CreatedAt.Year() > 9999 || r.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 || r.ManifestDigest != task.StageDigest(r.Canonical) {
		return nil, ErrCardComparison
	}
	m, err := DecodeCardComparisonManifest(r.Canonical, r.ComponentDigest)
	if err != nil || m.Identity() != r.Identity || m.AnalysisType() != r.AnalysisType {
		return nil, ErrCardComparison
	}
	return m, nil
}

// ValidatedDraftDigest仅供原通用仓储核记录元数据；专用执行入口仍各自闭集解码。
func (r SupportRegistration) ValidatedDraftDigest() (string, error) {
	var header struct {
		Protocol string `json:"protocol"`
	}
	if len(r.Canonical) > MaxSupportManifestBytes || json.Unmarshal(r.Canonical, &header) != nil {
		return "", ErrSupportReview
	}
	switch header.Protocol {
	case SupportManifestVersion:
		m, err := r.Manifest()
		if err != nil {
			return "", err
		}
		return m.DraftDigest(), nil
	case CardComparisonManifestVersion:
		m, err := r.CardComparisonManifest()
		if err != nil {
			return "", err
		}
		return m.DraftDigest(), nil
	default:
		return "", ErrSupportReview
	}
}
