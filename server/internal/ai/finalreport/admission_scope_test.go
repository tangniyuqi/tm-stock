package finalreport

import (
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 这里只验证首次范围匹配；示例批准不授予任何生产执行或发布权限。
func admissionScopeFixture(t *testing.T) (*Policy, Proposal, *SelectionStage, task.AdmissionPlan) {
	t.Helper()
	p, v, snap := fixture(t)
	s, err := NewSelectionStage(p, v.TaskNo, v.ThemeID, v.ThemeName, []sources.Snapshot{snap})
	if err != nil {
		t.Fatal(err)
	}
	a := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: 1, AnalysisType: 1, SubjectType: task.Theme, SubjectID: v.ThemeID}, Price: points.Price{Version: "test-v1", Points: 7}, Budget: task.Budget{MaxAttempts: 2, MaxCalls: 6, MaxTokens: 5000, MaxPages: 3, MaxDurationMS: 300000}, SubjectName: v.ThemeName, ModelName: p.cfg.ModelName, FilingNo: p.cfg.FilingNo, ProfileDigest: strings.Repeat("a", 64), ExecutionDigest: strings.Repeat("b", 64), ApprovalRef: "test-only-model", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", PriceRef: "test-only-price", SourceRef: "test-only-source", Materials: []task.AdmissionMaterial{{SnapshotID: snap.ID, Digest: snap.Hash, Domain: "evidence.example.test", Category: snap.SourceCategory, URL: snap.URL, CollectedAt: snap.FetchedAt}}}
	if err := a.Validate(a.Scope); err != nil {
		t.Fatal(err)
	}
	return p, v, s, a
}

func TestReportAdmissionScope(t *testing.T) {
	for _, mode := range []string{"same", "subset", "subject", "name", "model", "filing", "scope", "budget", "snapshot", "hash", "url", "category", "collected"} {
		t.Run(mode, func(t *testing.T) {
			p, v, s, a := admissionScopeFixture(t)
			switch mode {
			case "subset":
				extra := a.Materials[0]
				extra.SnapshotID++
				a.Materials = append(a.Materials, extra)
			case "subject":
				a.Scope.SubjectID++
			case "name":
				a.SubjectName = "另一个示例题材"
			case "model":
				a.ModelName = "另一个示例模型"
			case "filing":
				a.FilingNo = "other-test-only-filing"
			case "scope":
				a.Scope.SubjectType = task.Topic
			case "budget":
				a.Budget.MaxCalls = 1
			case "snapshot":
				a.Materials[0].SnapshotID++
			case "hash":
				a.Materials[0].Digest = strings.Repeat("f", 64)
			case "url":
				a.Materials[0].URL = "https://evidence.example.test/other"
			case "category":
				a.Materials[0].Category = sources.CategoryAnnouncement
			case "collected":
				a.Materials[0].CollectedAt = a.Materials[0].CollectedAt.Add(-time.Millisecond)
			}
			want := mode == "same" || mode == "subset"
			if p.MatchesAdmission(a, v) != want || s.MatchesAdmission(a) != want {
				t.Fatal("报告或转换越过首次范围", mode)
			}
		})
	}
}

func TestReportAdmissionMalformedProposal(t *testing.T) {
	for _, mode := range []string{"task", "sections", "section_key", "nodes", "quote", "model", "filing"} {
		p, v, _, a := admissionScopeFixture(t)
		switch mode {
		case "task":
			v.TaskNo = "bad"
		case "sections":
			v.Report.Sections = nil
		case "section_key":
			v.Report.Sections[0].Key = v.Report.Sections[1].Key
		case "nodes":
			v.Report.Sections[0].Nodes = nil
		case "quote":
			v.Report.Sections[0].Nodes[0].Quote = nil
		case "model":
			v.Report.Disclosure.ModelName = "other"
		case "filing":
			v.Report.Disclosure.FilingNo = "other"
		}
		if p.MatchesAdmission(a, v) {
			t.Fatal("坏候选仍匹配首次范围", mode)
		}
	}
	p, v, s, a := admissionScopeFixture(t)
	var missingPolicy *Policy
	var missingSelection *SelectionStage
	if missingPolicy.MatchesAdmission(a, v) || missingSelection.MatchesAdmission(a) {
		t.Fatal("空策略放行")
	}
	s.snapshots[0].Hash = strings.Repeat("f", 64)
	if s.MatchesAdmission(a) || !p.MatchesAdmission(a, v) {
		t.Fatal("快照变更未隔离")
	}
}
