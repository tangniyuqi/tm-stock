package task

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 仅为格式测试计划；这些引用不是任何真实来源、定价或质量批准。
func admissionTestPlan() AdmissionPlan {
	return AdmissionPlan{Scope: AdmissionScope{1, 1, Theme, 2}, Price: points.Price{Version: "test-v1", Points: 7}, Budget: Budget{MaxAttempts: 2, MaxCalls: 5, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 60000}, SubjectName: "示例题材", ModelName: "测试模型", FilingNo: "test-only-filing", ProfileDigest: strings.Repeat("a", 64), ExecutionDigest: StageDigest([]byte("test-only-execution")), ApprovalRef: "test-only-model", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", PriceRef: "test-only-price", SourceRef: "test-only-source", Materials: []AdmissionMaterial{{SnapshotID: 1, Digest: strings.Repeat("b", 64), Domain: "evidence.example.test", Category: sources.CategoryAnnouncement, URL: "https://evidence.example.test/test", CollectedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}}}
}

func TestAdmissionPlanRejectsMissingProofAndMalformedScope(t *testing.T) {
	base := admissionTestPlan()
	if err := base.Validate(base.Scope); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*AdmissionPlan){
		func(p *AdmissionPlan) { p.Scope.UserID = 0 }, func(p *AdmissionPlan) { p.Scope.SubjectID++ },
		func(p *AdmissionPlan) { p.Scope.AnalysisType = 2 }, func(p *AdmissionPlan) { p.Scope.SubjectType = Topic },
		func(p *AdmissionPlan) { p.Price.Points = 0 }, func(p *AdmissionPlan) { p.Budget.MaxCalls = 4 },
		func(p *AdmissionPlan) { p.Budget.MaxPages = 0 }, func(p *AdmissionPlan) { p.ProfileDigest = "wrong" },
		func(p *AdmissionPlan) { p.ApprovalRef = "" }, func(p *AdmissionPlan) { p.QualityRef = "" },
		func(p *AdmissionPlan) { p.HardBudgetRef = "" }, func(p *AdmissionPlan) { p.PriceRef = "" },
		func(p *AdmissionPlan) { p.SourceRef = "" }, func(p *AdmissionPlan) { p.SourceRef = "bad ref" },
		func(p *AdmissionPlan) { p.SubjectName = "" }, func(p *AdmissionPlan) { p.SubjectName = strings.Repeat("字", 33) },
		func(p *AdmissionPlan) { p.ModelName = "测试\n模型" }, func(p *AdmissionPlan) { p.FilingNo = "" },
		func(p *AdmissionPlan) { p.Materials = nil }, func(p *AdmissionPlan) { p.Materials = append(p.Materials, p.Materials[0]) },
		func(p *AdmissionPlan) { p.Materials[0].Digest = "wrong" }, func(p *AdmissionPlan) { p.Materials[0].URL = "https://evidence.example.test/test#fragment" },
		func(p *AdmissionPlan) { p.Materials[0].URL = "https://other.example.test/test" }, func(p *AdmissionPlan) { p.Materials[0].Category = "unknown" },
		func(p *AdmissionPlan) { p.Materials[0].CollectedAt = time.Time{} }, func(p *AdmissionPlan) { p.Materials[0].CollectedAt = p.Materials[0].CollectedAt.Add(time.Nanosecond) },
	} {
		p := base.Clone()
		mutate(&p)
		if err := p.Validate(base.Scope); err == nil {
			t.Fatal("坏计划被接受", fmt.Sprintf("%#v", p))
		}
	}
	for _, scope := range []AdmissionScope{{}, {1, 0, Theme, 1}, {1, 1, "unknown", 1}, {1, 1, Theme, 0}} {
		if scope.Validate() == nil {
			t.Fatal("坏主体被接受")
		}
	}
	for _, cat := range []string{sources.CategoryAnnouncement, sources.CategoryAnnualReport, sources.CategoryProspectus, sources.CategoryOfficialList, sources.CategoryInteractive} {
		p := base.Clone()
		p.Materials[0].Category = cat
		if p.Validate(base.Scope) != nil {
			t.Fatal("合法来源类别被拒绝")
		}
	}
	copy := base.Clone()
	copy.Materials[0].URL = "https://other.example.test/"
	if copy.Materials[0].URL == base.Materials[0].URL {
		t.Fatal("克隆仍共享可变数组")
	}
	for _, formatted := range []string{fmt.Sprint(base), fmt.Sprintf("%#v", base)} {
		if strings.Contains(formatted, base.SourceRef) || strings.Contains(formatted, base.Materials[0].URL) {
			t.Fatal("格式化泄漏出处或批准")
		}
	}
}
