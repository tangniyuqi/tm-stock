package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func dossierReportUnitPlan(t *testing.T) (AIL1ReportPlan, task.Task, *provider.OpenAICompat) {
	t.Helper()
	p, v, model := l1PlanFixture(t)
	s := p.Snapshots[0]
	sentence := s.Sentences[0]
	u := dossier.Field{State: "unknown"}
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: v.SubjectID, Version: 1, CreatedAt: s.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{{Key: "test_card", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
	bound, err := dossier.BindDraft(d, []sources.Snapshot{s})
	if err != nil {
		t.Fatal(err)
	}
	cfg := dossier.SupportConfig{Model: p.Model.Model, FilingNo: p.Model.FilingNo, Version: p.Model.Version, ApprovalRef: p.Model.ApprovalRef, QualityRef: p.Model.QualityRef, HardBudgetRef: p.Model.HardBudgetRef, ProfileDigest: p.Model.ProfileDigest, MaxInputTokens: 32, MaxOutputTokens: 128, MaxPromptBytes: 16000, ReviewStep: 4}
	p.Dossier, err = dossier.NewSupportPlan(context.Background(), cfg, v, p.ThemeName, bound, []sources.Snapshot{s}, []string{"test_card"})
	if err != nil {
		t.Fatal(err)
	}
	p.ReportProfile = task.DossierReportProfile
	p.Policy.DossierSupportDigest = p.Dossier.ComponentDigest()
	units := p.Dossier.Units()
	p.SupportUnits = &units
	if _, err = p.validate(v, model); err != nil {
		t.Fatal("合成六步计划无效", err)
	}
	return p, v, model
}

func TestDossierReportPlanScopeAndBudget(t *testing.T) {
	for _, mode := range []string{"same", "unknown_profile", "missing_profile", "missing_plan", "missing_units", "changed_units", "missing_component", "changed_component", "cross_task", "calls", "tokens"} {
		t.Run(mode, func(t *testing.T) {
			p, v, model := dossierReportUnitPlan(t)
			switch mode {
			case "unknown_profile":
				p.ReportProfile = "unknown"
			case "missing_profile":
				p.ReportProfile = ""
			case "missing_plan":
				p.Dossier = nil
			case "missing_units":
				p.SupportUnits = nil
			case "changed_units":
				p.SupportUnits.Tokens++
			case "missing_component":
				p.Policy.DossierSupportDigest = ""
			case "changed_component":
				p.Policy.DossierSupportDigest = strings.Repeat("e", 64)
			case "cross_task":
				v.ID++
			case "calls":
				v.Budget.MaxCalls = 5
			case "tokens":
				v.Budget.MaxTokens = p.Agent.Units.Tokens + 3*p.Model.MaxInputTokens + int64(p.Model.SelectionTokens()) + 2*int64(p.Model.CheckTokens()) + p.SupportUnits.Tokens
			}
			policy, err := p.validate(v, model)
			if (err == nil) != (mode == "same") || mode != "same" && policy != nil {
				t.Fatal("新增核验没有完整容量或身份仍获准入", err)
			}
		})
	}
}

func TestDossierReportPlanDigestAndCopies(t *testing.T) {
	p, _, _ := dossierReportUnitPlan(t)
	digest, err := p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	q := cloneL1Plan(p)
	q.Agent.TaskID++
	q.Agent.UserID++
	q.Agent.TaskNo = strings.Repeat("e", 32)
	q.Agent.OperationKey = "changed"
	if got, err := q.AdmissionDigest(); err != nil || got != digest {
		t.Fatal("创建后身份改写首次准入摘要", err)
	}
	q.SupportUnits.Tokens++
	if q.SupportUnits.Tokens == p.SupportUnits.Tokens {
		t.Fatal("克隆共享核验预算")
	}
	if got, err := q.AdmissionDigest(); err != nil || got == digest {
		t.Fatal("完整执行摘要遗漏核验预算", err)
	}
	q = cloneL1Plan(p)
	q.ReportProfile = ""
	if got, err := q.AdmissionDigest(); err == nil && got == digest {
		t.Fatal("新旧执行摘要混用")
	}
	raw, err := json.Marshal(p)
	if err != nil || strings.Contains(string(raw), "test_card") || strings.Contains(string(raw), "inputDigest") {
		t.Fatal("准入摘要编码复制任务计划")
	}
	var empty *dossier.SupportPreparation
	if empty.Config() != (dossier.SupportConfig{}) || empty.Units() != (task.Units{}) {
		t.Fatal("空准备提供执行单位")
	}
}
