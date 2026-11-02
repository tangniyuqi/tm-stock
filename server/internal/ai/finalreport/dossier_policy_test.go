package finalreport

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

type dossierPolicyTest struct {
	cfg         Config
	model       CheckConfig
	plan        *dossier.SupportPlan
	review      *dossier.SupportReview
	response    provider.Response
	ent, sem    *CheckEvidence
	proposal    Proposal
	snapshots   []sources.Snapshot
	admission   task.AdmissionPlan
	baseVisible string
}

func dossierPolicyFixture(t *testing.T, verdict, reason string) dossierPolicyTest {
	t.Helper()
	base, v, selection, a := admissionScopeFixture(t)
	snapshot := selection.snapshots[0]
	sentence := snapshot.Sentences[0]
	u := dossier.Field{State: "unknown"}
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: v.ThemeID, Version: 1, CreatedAt: snapshot.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{{Key: "test_card", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: snapshot.ID, Hash: snapshot.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
	bound, err := dossier.BindDraft(d, []sources.Snapshot{snapshot})
	if err != nil {
		t.Fatal(err)
	}
	model := checkConfig()
	model.Model = base.cfg.ModelName
	model.FilingNo = base.cfg.FilingNo
	model.ProfileDigest = a.ProfileDigest
	c := base.cfg
	c.EntailmentVersion, err = CheckPolicyVersion(CheckEntailment, model)
	if err != nil {
		t.Fatal(err)
	}
	c.SemanticVersion, err = CheckPolicyVersion(CheckSemantic, model)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := NewPolicy(c, func(context.Context, EntailmentInput) (Verdict, error) { return Uncertain, nil }, func(context.Context, string) (Verdict, error) { return Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	in, visible, err := plain.Inputs(encoded(t, v), []sources.Snapshot{snapshot})
	if err != nil {
		t.Fatal(err)
	}
	scfg := dossier.SupportConfig{Model: model.Model, FilingNo: model.FilingNo, Version: "test-only-support", ApprovalRef: model.ApprovalRef, QualityRef: model.QualityRef, HardBudgetRef: model.HardBudgetRef, ProfileDigest: model.ProfileDigest, MaxInputTokens: 1000, MaxOutputTokens: 128, MaxPromptBytes: 16000, ReviewStep: 4}
	taskValue := task.Task{ID: 7, UserID: a.Scope.UserID, No: v.TaskNo, SubjectType: task.Theme, SubjectID: v.ThemeID, AnalysisType: 1}
	p, err := dossier.NewSupportPlan(context.Background(), scfg, taskValue, v.ThemeName, bound, []sources.Snapshot{snapshot}, []string{"test_card"})
	if err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(map[string]any{"results": []map[string]any{{"id": 1, "verdict": verdict, "reason": reason}}})
	if err != nil {
		t.Fatal(err)
	}
	response := provider.Response{Model: model.Model, Text: string(output), FinishReason: "stop", RequestID: "test-only-dossier-receipt", Usage: provider.Usage{Prompt: 2, Completion: 1, Total: 3}}
	raw, err := p.CapturePayload(response)
	if err != nil {
		t.Fatal(err)
	}
	review, err := p.Review(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	ep, err := NewEntailmentCheck(model, in)
	if err != nil {
		t.Fatal(err)
	}
	evidence := captureEvidence(t, ep, checkReply(model, `{"verdict":"pass","reason":"supported"}`))
	full, err := DossierVisible(visible, p, review)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := NewSemanticCheck(model, v.TaskNo, full)
	if err != nil {
		t.Fatal(err)
	}
	semantic := captureEvidence(t, sp, checkReply(model, `{"verdict":"pass","reason":"factual"}`))
	c.DossierSupportDigest = p.ComponentDigest()
	a.DossierSupportDigest = p.ComponentDigest()
	a.ReportProfile = task.DossierReportProfile
	a.Budget.MaxCalls = 6
	return dossierPolicyTest{cfg: c, model: model, plan: p, review: review, response: response, ent: evidence, sem: semantic, proposal: v, snapshots: []sources.Snapshot{snapshot}, admission: a, baseVisible: visible}
}

func TestDossierReportPolicyDelivery(t *testing.T) {
	for _, state := range []struct {
		verdict, reason string
		pass            bool
	}{{"supported", "direct_support", true}, {"partial", "partial_support", true}, {"unsupported", "not_supported", true}, {"uncertain", "insufficient_context", false}} {
		t.Run(state.verdict, func(t *testing.T) {
			f := dossierPolicyFixture(t, state.verdict, state.reason)
			policy, err := NewDossierCapturedPolicy(f.cfg, f.plan, f.review, f.ent, f.sem)
			if err != nil {
				t.Fatal(err)
			}
			e, s, reason, err := policy.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots)
			if err != nil || (e == Pass && s == Pass && reason == "") != state.pass || !state.pass && reason != "evidence" {
				t.Fatal("未解决与合法否定交付语义错误", e, s, reason, err)
			}
			if !policy.MatchesAdmission(f.admission, f.proposal) {
				t.Fatal("新完整准入不匹配")
			}
			pending, err := NewDossierPolicy(f.cfg, f.plan, func(context.Context, EntailmentInput) (Verdict, error) { return Pass, nil }, func(context.Context, string) (Verdict, error) { return Pass, nil })
			if err != nil {
				t.Fatal(err)
			}
			if pending.Digest() == policy.Digest() {
				t.Fatal("最终策略未绑定实际捕获")
			}
			if e, s, _, err := pending.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots); err != nil || e == Pass && s == Pass {
				t.Fatal("缺字段捕获得到成功")
			}
			if state.verdict == "uncertain" {
				if _, err := NewDossierCapturedPolicy(f.cfg, f.plan, f.review, f.ent, nil); err != nil {
					t.Fatal("全部未解决不能提前失败释放", err)
				}
			}
		})
	}
}

func TestDossierReportPolicyCaptureAndScope(t *testing.T) {
	f := dossierPolicyFixture(t, "supported", "direct_support")
	policy, err := NewDossierCapturedPolicy(f.cfg, f.plan, f.review, f.ent, f.sem)
	if err != nil {
		t.Fatal(err)
	}
	response := f.response
	response.RequestID = "test-only-different-capture"
	raw, err := f.plan.CapturePayload(response)
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.plan.Review(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewDossierCapturedPolicy(f.cfg, f.plan, other, f.ent, f.sem)
	if err != nil || second.Digest() == policy.Digest() {
		t.Fatal("同状态不同捕获未改变最终策略", err)
	}
	if p, r := policy.DossierProof(); p != f.plan || r != f.review {
		t.Fatal("内部证明不符")
	}
	for _, mode := range []string{"legacy", "single_step", "other_member", "other_task"} {
		t.Run(mode, func(t *testing.T) {
			a, v := f.admission.Clone(), f.proposal
			switch mode {
			case "legacy":
				a.ReportProfile = ""
				a.DossierSupportDigest = ""
			case "single_step":
				a.ReportProfile = ""
			case "other_member":
				a.Scope.UserID++
			case "other_task":
				v.TaskNo = strings.Repeat("e", 32)
			}
			if policy.MatchesAdmission(a, v) {
				t.Fatal("新策略越过完整准入或实际任务")
			}
		})
	}
	sel, err := NewSelectionStage(policy, f.proposal.TaskNo, f.proposal.ThemeID, f.proposal.ThemeName, f.snapshots)
	if err != nil || !sel.HasDossierSupport() || !sel.MatchesAdmission(f.admission) {
		t.Fatal("新版选择前置范围不符", err)
	}
	otherTask, err := NewSelectionStage(policy, strings.Repeat("e", 32), f.proposal.ThemeID, f.proposal.ThemeName, f.snapshots)
	if err != nil {
		t.Fatal(err)
	}
	if otherTask.MatchesAdmission(f.admission) {
		t.Fatal("选择跨任务复用字段计划")
	}
	old := f.cfg
	old.DossierSupportDigest = ""
	legacy, err := NewCapturedPolicy(old, f.ent, f.sem)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.MatchesAdmission(f.admission, f.proposal) {
		t.Fatal("旧策略解释新准入")
	}
	if _, err := NewCapturedPolicy(f.cfg, f.ent, f.sem); err == nil {
		t.Fatal("旧构造器接受新组件")
	}
	sp, err := NewSemanticCheck(f.model, f.proposal.TaskNo, f.baseVisible)
	if err != nil {
		t.Fatal(err)
	}
	missingVisible := captureEvidence(t, sp, checkReply(f.model, `{"verdict":"pass","reason":"factual"}`))
	missing, err := NewDossierCapturedPolicy(f.cfg, f.plan, f.review, f.ent, missingVisible)
	if err != nil {
		t.Fatal(err)
	}
	if e, s, reason, err := missing.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots); err != nil || e != Pass || s == Pass || reason != "semantic" {
		t.Fatal("未判定核验呈现的语义回执仍通过", err)
	}
}

func TestDossierReportPolicyRefusal(t *testing.T) {
	for _, mode := range []string{"plan", "component", "model", "filing", "analysis", "entailment", "ent_version", "missing_review", "missing_semantic", "sem_version", "company_count"} {
		t.Run(mode, func(t *testing.T) {
			f := dossierPolicyFixture(t, "partial", "partial_support")
			switch mode {
			case "plan":
				f.plan = nil
			case "component":
				f.cfg.DossierSupportDigest = strings.Repeat("e", 64)
			case "model":
				f.cfg.ModelName = "other-model"
			case "filing":
				f.cfg.FilingNo = "other-filing"
			case "analysis":
				f.cfg.AnalysisType = 2
			case "entailment":
				f.ent = nil
			case "ent_version":
				f.cfg.EntailmentVersion = "other"
			case "missing_review":
				f.review = nil
			case "missing_semantic":
				f.sem = nil
			case "sem_version":
				f.cfg.SemanticVersion = "other"
			}
			var companies []*CompanyDraft
			if mode == "company_count" {
				companies = []*CompanyDraft{nil, nil}
			}
			if p, err := NewDossierCapturedPolicy(f.cfg, f.plan, f.review, f.ent, f.sem, companies...); err == nil || p != nil {
				t.Fatal("不完整的最终核验策略被接受")
			}
		})
	}
	f := dossierPolicyFixture(t, "supported", "direct_support")
	blocked := captureEvidence(t, f.ent.plan, checkReply(f.model, `{"verdict":"block","reason":"unsupported"}`))
	p, err := NewDossierCapturedPolicy(f.cfg, f.plan, nil, blocked, nil)
	if err != nil {
		t.Fatal("蕴含失败不能沿原路径释放", err)
	}
	if e, s, reason, err := p.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots); err != nil || e != Block || s != Uncertain || reason != "entailment" {
		t.Fatal("提前失败语义失真", err)
	}
	var missing *Policy
	if p, r := missing.DossierProof(); p != nil || r != nil {
		t.Fatal("空策略有证明")
	}
	var selection *SelectionStage
	if selection.HasDossierSupport() {
		t.Fatal("空选择策略有新能力")
	}
}
