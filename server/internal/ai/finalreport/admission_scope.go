package finalreport

import "github.com/tangniyuqi/tm-stock/server/internal/ai/task"

// 只比对首次已批准范围，不授予当前来源、质量、模型或发布许可。
func (p *Policy) admittedIdentity(a task.AdmissionPlan, id int64, name string) bool {
	if !p.Valid() {
		return false
	}
	wantType := 1
	if p.cfg.AnalysisType == 2 {
		wantType = 2
	}
	return a.Validate(a.Scope) == nil && a.Scope.AnalysisType == wantType && a.Scope.SubjectType == task.Theme && a.Scope.SubjectID == id && a.SubjectName == name && a.ModelName == p.cfg.ModelName && a.FilingNo == p.cfg.FilingNo && p.companyAdmission(a)
}
func admittedMaterial(a task.AdmissionPlan, id int64, hash, url, category string, at int64) bool {
	for _, m := range a.Materials {
		if m.SnapshotID == id {
			return m.Digest == hash && m.URL == url && sourceType(m.Category) == category && m.CollectedAt.UnixMilli() == at
		}
	}
	return false
}

// MatchesAdmission 允许引用原批准范围的子集，但任何引文都不能来自其外部。
func (p *Policy) MatchesAdmission(a task.AdmissionPlan, v Proposal) bool {
	if !p.admittedIdentity(a, v.ThemeID, v.ThemeName) || !validProposalIdentity(v.TaskNo, v.ThemeID, v.ThemeName) || len(v.Report.Sections) != len(sectionOrder) || v.Report.Disclosure.ModelName != a.ModelName || v.Report.Disclosure.FilingNo != a.FilingNo {
		return false
	}
	quotes := 0
	for i, s := range v.Report.Sections {
		if p.companies != nil && s.Key == "companies" {
			if i != 3 || !p.companySection(s) {
				return false
			}
			continue
		}
		if s.Key != sectionOrder[i] || len(s.Nodes) < 1 || len(s.Nodes) > 16 {
			return false
		}
		for _, n := range s.Nodes {
			q := n.Quote
			if q == nil || !admittedMaterial(a, q.SnapshotID, q.ContentHash, q.URL, q.SourceType, q.FetchedAt) {
				return false
			}
			quotes++
		}
	}
	if quotes < p.cfg.MinQuotes {
		return false
	}
	return true
}
func (s *SelectionStage) MatchesAdmission(a task.AdmissionPlan) bool {
	if !s.Valid() || !s.policy.admittedIdentity(a, s.themeID, s.themeName) {
		return false
	}
	for _, snap := range s.snapshots {
		if !admittedMaterial(a, snap.ID, snap.Hash, snap.URL, sourceType(snap.SourceCategory), snap.FetchedAt.UnixMilli()) {
			return false
		}
	}
	return true
}
