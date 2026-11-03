package dossier

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

const CorrectionProtocol = "dossier-explicit-correction-v1"
const MaxCorrections = 16
const MaxCorrectionBytes = 32768

type CorrectionPair struct {
	ThemeID       int64 `json:"themeId"`
	BeforeVersion int64 `json:"beforeVersion"`
	AfterVersion  int64 `json:"afterVersion"`
}

func (p CorrectionPair) Valid() bool {
	return conversation.ID(p.ThemeID) && conversation.ID(p.BeforeVersion) && conversation.ID(p.AfterVersion) && p.BeforeVersion < p.AfterVersion
}

type CorrectionLocator struct {
	Key   string `json:"key"`
	Field string `json:"field"`
}

func (l CorrectionLocator) Valid() bool {
	return cardKey.MatchString(l.Key) && slices.Contains([]string{"", "subject", "publishedDate", "effectiveDate", "period", "scope", "stage", "basis", "unit"}, l.Field)
}

type CorrectionDeclaration struct {
	Before       CorrectionLocator `json:"before"`
	After        CorrectionLocator `json:"after"`
	StatementKey string            `json:"statementKey"`
}
type CorrectionRequest struct {
	RequestID        string                  `json:"requestId"`
	Object           CorrectionPair          `json:"object"`
	BeforeDigest     string                  `json:"beforeDigest"`
	AfterDigest      string                  `json:"afterDigest"`
	ExpectedRevision int64                   `json:"expectedRevision"`
	Enabled          bool                    `json:"enabled"`
	ValidUntil       int64                   `json:"validUntil"`
	Declarations     []CorrectionDeclaration `json:"declarations"`
	ReviewReference  string                  `json:"reviewReference"`
}

func (CorrectionRequest) String() string {
	return "更正声明审核请求（内部引用已隐藏）"
}
func (r CorrectionRequest) GoString() string { return r.String() }
func (r CorrectionRequest) Validate() error {
	if !ValidManagementRequestID(r.RequestID) || !r.Object.Valid() || !managementDigest(r.BeforeDigest) || !managementDigest(r.AfterDigest) || r.ExpectedRevision < 0 || r.ExpectedRevision >= 9007199254740991 || !conversation.Text(r.ReviewReference, 128) || r.Declarations == nil || len(r.Declarations) > MaxCorrections {
		return ErrInvalidManagement
	}
	if r.Enabled {
		if len(r.Declarations) == 0 || r.ValidUntil <= 0 || r.ValidUntil > ManagementMaxTime {
			return ErrInvalidManagement
		}
	} else if len(r.Declarations) != 0 || r.ValidUntil != 0 {
		return ErrInvalidManagement
	}
	seen := map[string]bool{}
	for _, d := range r.Declarations {
		k := d.Before.Key + "/" + d.Before.Field + "/" + d.After.Key + "/" + d.After.Field
		if !d.Before.Valid() || !d.After.Valid() || !cardKey.MatchString(d.StatementKey) || seen[k] {
			return ErrInvalidManagement
		}
		seen[k] = true
	}
	return nil
}
func correctionShape(raw []byte, names ...string) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || len(m) != len(names) {
		return nil, false
	}
	for _, n := range names {
		v, ok := m[n]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, false
		}
	}
	return m, true
}
func ParseCorrectionRequest(raw []byte) (CorrectionRequest, error) {
	var r CorrectionRequest
	if len(raw) > MaxCorrectionBytes || task.CheckStageJSON(raw) != nil {
		return r, ErrInvalidManagement
	}
	m, ok := correctionShape(raw, "requestId", "object", "beforeDigest", "afterDigest", "expectedRevision", "enabled", "validUntil", "declarations", "reviewReference")
	if !ok {
		return r, ErrInvalidManagement
	}
	if _, ok = correctionShape(m["object"], "themeId", "beforeVersion", "afterVersion"); !ok {
		return r, ErrInvalidManagement
	}
	var ds []json.RawMessage
	if json.Unmarshal(m["declarations"], &ds) != nil || len(ds) > MaxCorrections {
		return r, ErrInvalidManagement
	}
	for _, v := range ds {
		d, valid := correctionShape(v, "before", "after", "statementKey")
		if !valid {
			return r, ErrInvalidManagement
		}
		for _, side := range []string{"before", "after"} {
			if _, valid = correctionShape(d[side], "key", "field"); !valid {
				return r, ErrInvalidManagement
			}
		}
	}
	if json.Unmarshal(raw, &r) != nil || r.Validate() != nil {
		return CorrectionRequest{}, ErrInvalidManagement
	}
	return r, nil
}

type CorrectionEvent struct {
	Protocol       string            `json:"protocol"`
	ActorID        int64             `json:"actorId"`
	Request        CorrectionRequest `json:"request"`
	Revision       int64             `json:"revision"`
	PreviousDigest string            `json:"previousDigest"`
	CreatedAt      int64             `json:"createdAt"`
}

func (CorrectionEvent) String() string     { return "更正声明审核事件（内部记录已隐藏）" }
func (e CorrectionEvent) GoString() string { return e.String() }
func (e CorrectionEvent) Canonical() ([]byte, string, error) {
	if e.Protocol != CorrectionProtocol || !conversation.ID(e.ActorID) || e.Request.Validate() != nil || e.Revision != e.Request.ExpectedRevision+1 || e.CreatedAt <= 0 || e.CreatedAt > ManagementMaxTime || e.Revision == 1 && e.PreviousDigest != "" || e.Revision > 1 && !managementDigest(e.PreviousDigest) || e.Request.Enabled && e.Request.ValidUntil <= e.CreatedAt || !e.Request.Enabled && e.Revision == 1 {
		return nil, "", ErrInvalidManagement
	}
	b, err := json.Marshal(e)
	if err != nil || len(b) > MaxCorrectionBytes {
		return nil, "", ErrInvalidManagement
	}
	return b, task.StageDigest(b), nil
}
func RestoreCorrectionEvent(raw []byte, digest string) (CorrectionEvent, error) {
	var e CorrectionEvent
	if len(raw) > MaxCorrectionBytes || task.CheckStageJSON(raw) != nil || task.StageDigest(raw) != digest || json.Unmarshal(raw, &e) != nil {
		return CorrectionEvent{}, ErrManagementEvidence
	}
	b, _, err := e.Canonical()
	if err != nil || !bytes.Equal(b, raw) {
		return CorrectionEvent{}, ErrManagementEvidence
	}
	return e, nil
}
func (e CorrectionEvent) Matches(actor int64, r CorrectionRequest) bool {
	a, _ := json.Marshal(e.Request)
	b, _ := json.Marshal(r)
	return e.ActorID == actor && bytes.Equal(a, b)
}

// 只核引文定位、版本身份和显式审核范围，不以关键词或模型投票判断语义。
func BindCorrections(r CorrectionRequest, before, after *BoundDraft) error {
	if r.Validate() != nil || before == nil || after == nil || r.Object != (CorrectionPair{before.Identity().ThemeID, before.Identity().Version, after.Identity().Version}) || before.Identity().ThemeID != after.Identity().ThemeID || before.Digest() != r.BeforeDigest || after.Digest() != r.AfterDigest {
		return ErrManagementEvidence
	}
	for _, d := range r.Declarations {
		if !correctionQuoteExists(before, d.Before) || !correctionQuoteExists(after, d.After) || !correctionQuoteExists(after, CorrectionLocator{Key: d.StatementKey}) {
			return ErrManagementEvidence
		}
	}
	return nil
}
func correctionQuoteExists(b *BoundDraft, l CorrectionLocator) bool {
	for _, c := range b.Cards() {
		if c.Key != l.Key {
			continue
		}
		if l.Field == "" {
			return true
		}
		fs := map[string]BoundField{"subject": c.Subject, "publishedDate": c.PublishedDate, "effectiveDate": c.EffectiveDate, "period": c.Period, "scope": c.Scope, "stage": c.Stage, "basis": c.Basis, "unit": c.Unit}
		f, ok := fs[l.Field]
		return ok && f.State == "known"
	}
	return false
}

type CorrectionTx interface {
	ManagementTx
	CorrectionReadLock(context.Context, CorrectionPair) (bool, error)
	LatestCorrection(context.Context, CorrectionPair) (CorrectionEvent, string, error)
	CorrectionEvent(context.Context, string) (CorrectionEvent, error)
	AppendCorrection(context.Context, CorrectionEvent) error
}
type CorrectionStore interface {
	WithinCorrection(context.Context, bool, func(CorrectionTx) error) error
}
