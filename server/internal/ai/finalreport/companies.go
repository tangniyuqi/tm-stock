package finalreport

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const MaxCompanyMappings = 32
const CompanyVersion = "reviewed-company-candidates-v1"

// CompanyRecord 只能来自事务内的已审核资料库读取，不接受模型或会员输入。
type CompanyRecord struct {
	ID, NodeID, ParentID, StockID, ReviewerID   int64
	NodeName, Name, Symbol, TsCode, MappingCode string
	SourceType, AuditStatus, Status             int
	Excerpt, URL                                string
	Trimmed                                     bool
	CollectedAt, ReviewedAt                     time.Time
}

func (CompanyRecord) String() string     { return "已审核公司依据（主数据与原文已隐藏）" }
func (c CompanyRecord) GoString() string { return c.String() }

// CompanyReader 是报告事务的可选只读能力，不扩张所有终检事务的写权限。
type CompanyReader interface {
	ReportCompanies(context.Context, int64) ([]CompanyRecord, error)
}

// CompanyDraft 不是报告或结算收据；私有字段阻止它被会员接口意外序列化。
type CompanyDraft struct {
	rows          []render.Node
	digest        string
	contentDigest string
	scope         task.AdmissionScope
	name          string
	materials     []task.AdmissionMaterial
}

func (*CompanyDraft) String() string     { return "内部公司候选（尚未终检，不可发布）" }
func (d *CompanyDraft) GoString() string { return d.String() }
func (d *CompanyDraft) Digest() string {
	if d == nil {
		return ""
	}
	return d.digest
}

// ContentDigest 不含整份执行/准入摘要，避免被绑定回首次准入时形成循环。
func (d *CompanyDraft) ContentDigest() string {
	if d == nil {
		return ""
	}
	return d.contentDigest
}
func (d *CompanyDraft) Rows() []render.Node {
	if d == nil {
		return nil
	}
	out := slices.Clone(d.rows)
	for i := range out {
		row := *out[i].Row
		row.Evidence = slices.Clone(row.Evidence)
		out[i].Row = &row
	}
	return out
}

// NewCompanyDraft 复核全批记录与首次材料；不做语义放行，不允许模型选择公司子集。
func NewCompanyDraft(p *Policy, a task.AdmissionPlan, snaps []sources.Snapshot, records []CompanyRecord, now time.Time) (*CompanyDraft, error) {
	if !p.admittedIdentity(a, a.Scope.SubjectID, a.SubjectName) || now.IsZero() || len(records) > MaxCompanyMappings || len(snaps) != len(a.Materials) {
		return nil, points.ErrInvalid
	}
	byID := make(map[int64]sources.Snapshot, len(snaps))
	total := 0
	for _, s := range snaps {
		total += len(s.Body)
		if _, duplicate := byID[s.ID]; duplicate || s.Validate() != nil || len(s.Body) > MaxEvidenceBodyBytes || total > MaxTotalEvidenceBytes || len(s.Sentences) > 10000 || s.FetchedAt.After(now) || s.LicenseExpires != nil && !s.LicenseExpires.After(now) {
			return nil, ErrRejected
		}
		byID[s.ID] = s
	}
	for _, m := range a.Materials {
		s, found := byID[m.SnapshotID]
		url, err := sources.ValidateURL(m.Domain, s.URL)
		if !found || err != nil || url != m.URL || !slices.Contains(p.cfg.AllowedDomains, m.Domain) || s.Hash != m.Digest || s.SourceCategory != m.Category || !s.FetchedAt.Equal(m.CollectedAt) {
			return nil, ErrRejected
		}
	}
	records = slices.Clone(records)
	slices.SortFunc(records, func(a, b CompanyRecord) int {
		if c := strings.Compare(a.TsCode, b.TsCode); c != 0 {
			return c
		}
		if a.NodeID < b.NodeID {
			return -1
		}
		if a.NodeID > b.NodeID {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	ids, pairs, stocks := map[int64]bool{}, map[string]bool{}, map[string]int64{}
	stockIdentity, nodeIdentity := map[int64]string{}, map[int64]string{}
	d := &CompanyDraft{rows: make([]render.Node, 0, len(records)), scope: a.Scope, name: a.SubjectName, materials: slices.Clone(a.Materials)}
	for i := range records {
		r := &records[i]
		pair := fmt.Sprintf("%d:%d", r.NodeID, r.StockID)
		stockKey := r.TsCode + "\x00" + r.Name + "\x00" + r.Symbol
		nodeKey := fmt.Sprintf("%d:%s", r.ParentID, r.NodeName)
		if old, ok := stockIdentity[r.StockID]; ok && old != stockKey {
			return nil, ErrRejected
		}
		if old, ok := nodeIdentity[r.NodeID]; ok && old != nodeKey {
			return nil, ErrRejected
		}
		if r.ID <= 0 || r.NodeID <= 0 || r.StockID <= 0 || r.ReviewerID <= 0 || r.AuditStatus != 2 || r.Status != 1 || ids[r.ID] || pairs[pair] || stocks[r.TsCode] != 0 && stocks[r.TsCode] != r.StockID || r.NodeID != a.Scope.SubjectID && r.ParentID != a.Scope.SubjectID || r.NodeID == a.Scope.SubjectID && r.NodeName != a.SubjectName || r.MappingCode != r.TsCode || len(r.Symbol) != 6 || len(r.TsCode) != 9 || !strings.HasPrefix(r.TsCode, r.Symbol+".") || !slices.Contains([]string{"SH", "SZ", "BJ"}, r.TsCode[7:]) || !companyTime(r.CollectedAt, now) || !companyTime(r.ReviewedAt, now) || r.ReviewedAt.Before(r.CollectedAt) || r.SourceType < 1 || r.SourceType > len(render.SourceTypes) || r.Excerpt == "" || !utf8.ValidString(r.Excerpt) || guard.Normalize(r.Excerpt) != r.Excerpt || !strings.Contains(r.Excerpt, r.Name) {
			return nil, ErrRejected
		}
		ids[r.ID], pairs[pair], stocks[r.TsCode] = true, true, r.StockID
		stockIdentity[r.StockID], nodeIdentity[r.NodeID] = stockKey, nodeKey
		matched := false
		for _, s := range snaps {
			if r.URL == s.URL && r.CollectedAt.Equal(s.FetchedAt) && sourceType(s.SourceCategory) == render.SourceTypes[r.SourceType-1] && strings.Contains(s.Body, r.Excerpt) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, ErrRejected
		}
		n, err := render.RowNode(render.StockRow{Name: r.Name, Code: r.Symbol, Node: r.NodeName, Evidence: []render.EvidenceItem{{Type: render.SourceTypes[r.SourceType-1], Excerpt: r.Excerpt, URL: r.URL, FetchedAt: r.CollectedAt.UnixMilli(), Trimmed: r.Trimmed}}})
		if err != nil {
			return nil, ErrRejected
		}
		d.rows = append(d.rows, n)
		r.CollectedAt, r.ReviewedAt = r.CollectedAt.UTC(), r.ReviewedAt.UTC()
	}
	_, admissionDigest, err := a.Encode()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal([]any{CompanyVersion, admissionDigest, records})
	if err != nil {
		return nil, points.ErrInvalid
	}
	d.digest = task.StageDigest(b)
	for i := range d.materials {
		d.materials[i].CollectedAt = d.materials[i].CollectedAt.UTC()
	}
	b, err = json.Marshal([]any{CompanyVersion, "content", a.Scope, a.SubjectName, d.materials, records})
	if err != nil {
		return nil, points.ErrInvalid
	}
	d.contentDigest = task.StageDigest(b)
	if a.CompanyDigest != "" && a.CompanyDigest != d.contentDigest {
		return nil, ErrRejected
	}
	return d, nil
}

func companyTime(at, now time.Time) bool {
	return at.UnixMilli() > 0 && at.Year() <= 9999 && at.Nanosecond()%int(time.Millisecond) == 0 && !at.After(now)
}
