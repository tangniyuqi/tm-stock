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

const SupportManifestVersion = "dossier-support-manifest-v1"
const MaxSupportManifestBytes = 16384

// 清单只保存重建定位与策略，不复制正文、请求、回执或会员身份。
type supportManifestDocument struct {
	Protocol        string        `json:"protocol"`
	AnalysisType    int           `json:"analysisType"`
	Draft           DraftIdentity `json:"draft"`
	DraftDigest     string        `json:"draftDigest"`
	ThemeName       string        `json:"themeName"`
	Keys            []string      `json:"keys"`
	Config          SupportConfig `json:"config"`
	ComponentDigest string        `json:"componentDigest"`
}

type SupportManifest struct {
	doc       supportManifestDocument
	canonical []byte
}

func (SupportManifest) String() string     { return "专题核验清单（定位与策略已隐藏）" }
func (m SupportManifest) GoString() string { return m.String() }

func (p *SupportPreparation) Manifest(kind int) (*SupportManifest, error) {
	component, err := p.ComponentDigest(kind)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for _, check := range p.checks {
		keys = append(keys, check.Card)
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	doc := supportManifestDocument{Protocol: SupportManifestVersion, AnalysisType: kind, Draft: p.identity, DraftDigest: p.draftDigest, ThemeName: p.themeName, Keys: keys, Config: p.cfg, ComponentDigest: component}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, ErrSupportReview
	}
	return DecodeSupportManifest(raw, component)
}

func DecodeSupportManifest(raw []byte, component string) (*SupportManifest, error) {
	if len(raw) > MaxSupportManifestBytes || !supportDigest(component) || task.CheckStageJSON(raw) != nil {
		return nil, ErrSupportReview
	}
	var doc supportManifestDocument
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&doc) != nil || doc.Protocol != SupportManifestVersion || doc.ComponentDigest != component ||
		(doc.AnalysisType != 1 && doc.AnalysisType != 2) || !conversation.ID(doc.Draft.ThemeID) || !conversation.ID(doc.Draft.Version) ||
		doc.Draft.CreatedAt <= 0 || doc.Draft.CreatedAt > 253402300799999 || !supportDigest(doc.DraftDigest) || !supportMeta(doc.ThemeName, 128) ||
		len(doc.Keys) < 1 || len(doc.Keys) > MaxSupportCards {
		return nil, ErrSupportReview
	}
	if _, err := doc.Config.PolicyDigest(); err != nil {
		return nil, err
	}
	for i, key := range doc.Keys {
		if !cardKey.MatchString(key) || i > 0 && key <= doc.Keys[i-1] {
			return nil, ErrSupportReview
		}
	}
	canonical, err := json.Marshal(doc)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, ErrSupportReview
	}
	return &SupportManifest{doc: doc, canonical: bytes.Clone(raw)}, nil
}

func (m *SupportManifest) Valid() bool { return m != nil && len(m.canonical) > 0 }
func (m *SupportManifest) Canonical() []byte {
	if !m.Valid() {
		return nil
	}
	return bytes.Clone(m.canonical)
}
func (m *SupportManifest) Identity() DraftIdentity {
	if !m.Valid() {
		return DraftIdentity{}
	}
	return m.doc.Draft
}
func (m *SupportManifest) DraftDigest() string {
	if !m.Valid() {
		return ""
	}
	return m.doc.DraftDigest
}
func (m *SupportManifest) ComponentDigest() string {
	if !m.Valid() {
		return ""
	}
	return m.doc.ComponentDigest
}
func (m *SupportManifest) AnalysisType() int {
	if !m.Valid() {
		return 0
	}
	return m.doc.AnalysisType
}

// Rebuild必须重新取得精确原文；清单自身既不能证明语义，也不能恢复读取/AI处理许可。
func (m *SupportManifest) Rebuild(ctx context.Context, bound *BoundDraft, snapshots []sources.Snapshot) (*SupportPreparation, error) {
	if !m.Valid() || bound == nil || bound.Identity() != m.doc.Draft || bound.Digest() != m.doc.DraftDigest {
		return nil, ErrSupportReview
	}
	p, err := PrepareSupport(ctx, m.doc.Config, m.doc.ThemeName, bound, snapshots, m.doc.Keys)
	if err != nil {
		return nil, err
	}
	digest, err := p.ComponentDigest(m.doc.AnalysisType)
	if err != nil || digest != m.doc.ComponentDigest {
		return nil, ErrSupportReview
	}
	return p, nil
}

type SupportRegistration struct {
	Identity                        DraftIdentity
	AnalysisType                    int
	ComponentDigest, ManifestDigest string
	Canonical                       []byte
	CreatedAt                       time.Time
}

func (SupportRegistration) String() string     { return "专题核验登记（清单已隐藏）" }
func (r SupportRegistration) GoString() string { return r.String() }
func (m *SupportManifest) Registration(at time.Time) (SupportRegistration, error) {
	if !m.Valid() {
		return SupportRegistration{}, ErrSupportReview
	}
	r := SupportRegistration{Identity: m.Identity(), AnalysisType: m.AnalysisType(), ComponentDigest: m.ComponentDigest(), ManifestDigest: task.StageDigest(m.canonical), Canonical: m.Canonical(), CreatedAt: at}
	if _, err := r.Manifest(); err != nil {
		return SupportRegistration{}, err
	}
	return r, nil
}
func (r SupportRegistration) Manifest() (*SupportManifest, error) {
	if r.CreatedAt.UnixMilli() < r.Identity.CreatedAt || r.CreatedAt.Year() > 9999 || r.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 || r.ManifestDigest != task.StageDigest(r.Canonical) {
		return nil, ErrSupportReview
	}
	m, err := DecodeSupportManifest(r.Canonical, r.ComponentDigest)
	if err != nil || m.Identity() != r.Identity || m.AnalysisType() != r.AnalysisType {
		return nil, ErrSupportReview
	}
	return m, nil
}

type SupportRegistry interface {
	WithinSupport(context.Context, bool, func(SupportRegistryTx) error) error
}
type SupportRegistryTx interface {
	Dossiers() Tx
	LockSupportTheme(context.Context, int64) error
	SupportManifest(context.Context, string) (SupportRegistration, error)
	InsertSupportManifest(context.Context, SupportRegistration) error
	SupportUse(context.Context, string) (Approval, error)
}
