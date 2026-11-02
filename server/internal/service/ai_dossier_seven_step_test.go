package service

import (
	"context"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func sevenStepReportPlan(t *testing.T) (AIL1ReportPlan, task.Task, *provider.OpenAICompat) {
	t.Helper()
	p, v, model := l1PlanFixture(t)
	v.AnalysisType = task.DeepAnalysisType
	v.Budget.MaxCalls = 7
	v.Budget.MaxTokens = 100000
	p.AnalysisType = task.DeepAnalysisType
	p.Model.AnalysisType = task.DeepAnalysisType
	p.Policy.AnalysisType = task.DeepAnalysisType
	p.Policy.MinQuotes = task.DeepMinQuotes
	var err error
	p.Policy.EntailmentVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckEntailment, p.Model)
	if err != nil {
		t.Fatal(err)
	}
	p.Policy.SemanticVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckSemantic, p.Model)
	if err != nil {
		t.Fatal(err)
	}
	s := p.Snapshots[0]
	sentence := s.Sentences[0]
	ref := dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}
	u := dossier.Field{State: "unknown"}
	card := dossier.FactCard{Key: "card_a", Section: "definition", Statement: ref, Subject: dossier.Field{State: "known", Ref: ref}, Scope: dossier.Field{State: "known", Ref: ref}, PublishedDate: u, EffectiveDate: u, Period: u, Stage: u, Basis: u, Unit: u}
	second := card
	second.Key = "card_b"
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: v.SubjectID, Version: 1, CreatedAt: s.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{card, second}}
	b, err := dossier.BindDraft(d, []sources.Snapshot{s})
	if err != nil {
		t.Fatal(err)
	}
	c := dossier.SupportConfig{Model: p.Model.Model, Version: p.Model.Version, FilingNo: p.Model.FilingNo, ApprovalRef: p.Model.ApprovalRef, QualityRef: p.Model.QualityRef, HardBudgetRef: p.Model.HardBudgetRef, ProfileDigest: p.Model.ProfileDigest, MaxInputTokens: 32, MaxOutputTokens: 128, MaxPromptBytes: 16000, ReviewStep: 4}
	p.Dossier, err = dossier.NewSupportPlan(context.Background(), c, v, p.ThemeName, b, []sources.Snapshot{s}, []string{card.Key, second.Key})
	if err != nil {
		t.Fatal(err)
	}
	c.ReviewStep = 0
	c.MaxOutputTokens = 1024
	prepared, err := dossier.PrepareCardComparison(context.Background(), dossier.CardComparisonConfig{Model: c, Focus: "definition", ReviewStep: 5}, p.ThemeName, b, []sources.Snapshot{s}, []string{card.Key, second.Key})
	if err != nil {
		t.Fatal(err)
	}
	p.Comparison, err = prepared.BindTask(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	p.ReportProfile = task.DossierComparisonReportProfile
	p.Policy.DossierSupportDigest = p.Dossier.ComponentDigest()
	p.Policy.DossierComparisonDigest = p.Comparison.ComponentDigest()
	units := p.Dossier.Units()
	p.SupportUnits = &units
	cu := p.Comparison.Units()
	p.ComparisonUnits = &cu
	return p, v, model
}

func TestDossierSevenStepPlanBinding(t *testing.T) {
	for _, mode := range []string{"same", "missing_comparison", "missing_units", "changed_units", "old_profile", "light", "cross_task", "budget", "component"} {
		t.Run(mode, func(t *testing.T) {
			p, v, model := sevenStepReportPlan(t)
			switch mode {
			case "missing_comparison":
				p.Comparison = nil
			case "missing_units":
				p.ComparisonUnits = nil
			case "changed_units":
				p.ComparisonUnits.Tokens++
			case "old_profile":
				p.ReportProfile = task.DossierReportProfile
			case "light":
				p.AnalysisType = 0
			case "cross_task":
				v.UserID++
			case "budget":
				v.Budget.MaxCalls = 6
			case "component":
				p.Policy.DossierComparisonDigest = p.Policy.DossierSupportDigest
			}
			policy, err := p.validate(v, model)
			if (err == nil) != (mode == "same") || mode != "same" && policy != nil {
				t.Fatal("七步计划身份、组件或容量未绑定", err)
			}
		})
	}
	p, _, _ := sevenStepReportPlan(t)
	copy := cloneL1Plan(p)
	copy.ComparisonUnits.Tokens++
	if *copy.ComparisonUnits == *p.ComparisonUnits {
		t.Fatal("单位副本仍共享")
	}
	if _, err := p.AdmissionDigest(); err != nil {
		t.Fatal("七步完整摘要失败", err)
	}
}
