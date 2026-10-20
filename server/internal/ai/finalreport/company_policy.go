package finalreport

import (
	"encoding/json"
	"reflect"
	"slices"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const CompanyReportVersion = "reviewed-company-report-v1"

// NewCompanyPolicy 必须显式给出已校验公司全集，没有默认公司或生产判定器。
func NewCompanyPolicy(c Config, d *CompanyDraft, e EntailmentCheck, s SemanticCheck) (*Policy, error) {
	if d == nil || !ValidDigest(d.ContentDigest()) || c.CompanyDigest != d.ContentDigest() || len(d.rows) < 1 || len(d.rows) > MaxCompanyMappings {
		return nil, points.ErrInvalid
	}
	base := c
	base.CompanyDigest = ""
	p, err := NewPolicy(base, e, s)
	if err != nil {
		return nil, err
	}
	p.cfg.CompanyDigest = c.CompanyDigest
	cp := *d
	cp.rows = d.Rows()
	cp.materials = slices.Clone(d.materials)
	p.companies = &cp
	b, err := json.Marshal([]any{CompanyReportVersion, guard.PolicyDigest(), p.cfg})
	if err != nil {
		return nil, points.ErrInvalid
	}
	p.digest = task.StageDigest(b)
	return p, nil
}
func (p *Policy) CompanyDigest() string {
	if p == nil {
		return ""
	}
	return p.cfg.CompanyDigest
}
func (p *Policy) companySection(s render.Section) bool {
	return p.companies != nil && s.Key == render.SecCompanies && !s.AILabel && reflect.DeepEqual(s.Nodes, p.companies.rows)
}
func (p *Policy) EvidenceIDs(v Proposal) []int64 {
	ids := v.SnapshotIDs()
	if p != nil && p.companies != nil {
		for _, m := range p.companies.materials {
			ids = append(ids, m.SnapshotID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
func (p *Policy) companyAdmission(a task.AdmissionPlan) bool {
	if p.companies == nil {
		return a.CompanyDigest == ""
	}
	if a.CompanyDigest != p.cfg.CompanyDigest || a.Scope != p.companies.scope || a.SubjectName != p.companies.name || len(a.Materials) != len(p.companies.materials) {
		return false
	}
	for i, m := range a.Materials {
		n := p.companies.materials[i]
		if m.SnapshotID != n.SnapshotID || m.Digest != n.Digest || m.Domain != n.Domain || m.Category != n.Category || m.URL != n.URL || !m.CollectedAt.Equal(n.CollectedAt) {
			return false
		}
	}
	return true
}
