package finalreport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type sevenPolicyFixture struct {
	dossierPolicyTest
	comparison *dossier.CardComparisonPlan
	compared   *dossier.CardComparisonReview
	response   provider.Response
}

// 只测试结构、捕获绑定和回放；六条不同合成原句不充当真实资料质量评测。
func sevenPolicy(t *testing.T) sevenPolicyFixture {
	t.Helper()
	f := dossierPolicyFixture(t, "supported", "direct_support")
	f.cfg.DossierSupportDigest = ""
	f.cfg.AnalysisType = task.DeepAnalysisType
	f.cfg.MinQuotes = task.DeepMinQuotes
	f.model.AnalysisType = task.DeepAnalysisType
	var err error
	f.cfg.EntailmentVersion, err = CheckPolicyVersion(CheckEntailment, f.model)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.SemanticVersion, err = CheckPolicyVersion(CheckSemantic, f.model)
	if err != nil {
		t.Fatal(err)
	}
	old := f.snapshots[0]
	s, err := sources.BuildSnapshot(sources.Source{ID: old.SourceID, Domain: f.cfg.AllowedDomains[0], Category: old.SourceCategory, LicenseTag: old.LicenseTag, LicenseRef: old.LicenseRef, Enabled: true}, old.URL, sources.MediaPlain, []byte(old.Body+"示例题材还包括另一条资料环节。示例题材另有一条公司信息。"), old.FetchedAt, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	s.ID = old.ID
	f.snapshots = []sources.Snapshot{s}
	f.admission.Materials[0].Digest = s.Hash
	sentence := s.Sentences[0]
	ref := dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}
	u := dossier.Field{State: "unknown"}
	known := dossier.Field{State: "known", Ref: ref}
	card := dossier.FactCard{Key: "card_a", Section: "definition", Statement: ref, Subject: known, Scope: known, PublishedDate: u, EffectiveDate: u, Period: u, Stage: u, Basis: u, Unit: u}
	second := card
	second.Key = "card_b"
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: f.proposal.ThemeID, Version: 1, CreatedAt: s.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{card, second}}
	bound, err := dossier.BindDraft(d, f.snapshots)
	if err != nil {
		t.Fatal(err)
	}
	v := task.Task{ID: 7, UserID: f.admission.Scope.UserID, No: f.proposal.TaskNo, SubjectType: task.Theme, SubjectID: f.proposal.ThemeID, AnalysisType: task.DeepAnalysisType}
	c := f.plan.Config()
	c.ReviewStep = 4
	f.plan, err = dossier.NewSupportPlan(context.Background(), c, v, f.proposal.ThemeName, bound, f.snapshots, []string{card.Key, second.Key})
	if err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{}
	for i := 1; i <= 6; i++ {
		rows = append(rows, map[string]any{"id": i, "verdict": "supported", "reason": "direct_support"})
	}
	raw, err := json.Marshal(map[string]any{"results": rows})
	if err != nil {
		t.Fatal(err)
	}
	reply := f.response
	reply.Text = string(raw)
	packet, err := f.plan.CapturePayload(reply)
	if err != nil {
		t.Fatal(err)
	}
	f.review, err = f.plan.Review(context.Background(), packet)
	if err != nil {
		t.Fatal(err)
	}
	c.ReviewStep = 0
	c.MaxOutputTokens = 1024
	prepared, err := dossier.PrepareCardComparison(context.Background(), dossier.CardComparisonConfig{Model: c, Focus: "definition", ReviewStep: 5}, f.proposal.ThemeName, bound, f.snapshots, []string{card.Key, second.Key})
	if err != nil {
		t.Fatal(err)
	}
	q, err := prepared.BindTask(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	axes := []map[string]any{}
	for i := 1; i <= 7; i++ {
		left, right, relation := "unknown", "unknown", "not_applicable"
		if i == 1 || i == 4 {
			left, right, relation = "supported", "supported", "aligned"
		}
		axes = append(axes, map[string]any{"id": i, "left": left, "right": right, "relation": relation})
	}
	raw, err = json.Marshal(map[string]any{"axes": axes, "wording": "same_meaning"})
	if err != nil {
		t.Fatal(err)
	}
	reply.Text = string(raw)
	packet, err = q.CapturePayload(reply)
	if err != nil {
		t.Fatal(err)
	}
	qr, err := q.Review(context.Background(), packet)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := NewPolicy(f.cfg, func(context.Context, EntailmentInput) (Verdict, error) { return Uncertain, nil }, func(context.Context, string) (Verdict, error) { return Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	stage, err := NewSelectionStage(plain, f.proposal.TaskNo, f.proposal.ThemeID, f.proposal.ThemeName, f.snapshots)
	if err != nil {
		t.Fatal(err)
	}
	var selections SelectionResponse
	if json.Unmarshal([]byte(selectionJSON(t, s.ID)), &selections) != nil {
		t.Fatal("合成选句解析失败")
	}
	selections.Selections = append(selections.Selections, Selection{Section: "segment", SnapshotID: s.ID, SentenceIDs: []int{5}}, Selection{Section: "companies", SnapshotID: s.ID, SentenceIDs: []int{6}})
	selectionRaw, err := json.Marshal(selections)
	if err != nil {
		t.Fatal(err)
	}
	proposalRaw, err := stage.Build(context.Background(), string(selectionRaw))
	if err != nil {
		t.Fatal("六句合成报告构造失败", err)
	}
	f.proposal, err = plain.Decode(proposalRaw)
	if err != nil {
		t.Fatal(err)
	}
	input, visible, err := plain.Inputs(encoded(t, f.proposal), f.snapshots)
	if err != nil {
		t.Fatal(err)
	}
	f.baseVisible = visible
	ep, err := NewEntailmentCheck(f.model, input)
	if err != nil {
		t.Fatal(err)
	}
	f.ent = captureEvidence(t, ep, checkReply(f.model, `{"verdict":"pass","reason":"supported"}`))
	visible, err = DossierVisible(visible, f.plan, f.review)
	if err != nil {
		t.Fatal(err)
	}
	visible, err = DossierComparisonVisible(visible, f.plan, f.review, q, qr)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := NewSemanticCheck(f.model, f.proposal.TaskNo, visible)
	if err != nil {
		t.Fatal(err)
	}
	f.sem = captureEvidence(t, sp, checkReply(f.model, `{"verdict":"pass","reason":"factual"}`))
	f.cfg.DossierSupportDigest = f.plan.ComponentDigest()
	f.cfg.DossierComparisonDigest = q.ComponentDigest()
	f.admission.Scope.AnalysisType = task.DeepAnalysisType
	f.admission.Price.Points = task.DeepPricePoints
	f.admission.Budget.MaxCalls = 7
	f.admission.Budget.MaxTokens = 100000
	f.admission.ReportProfile = task.DossierComparisonReportProfile
	f.admission.DossierSupportDigest = f.cfg.DossierSupportDigest
	f.admission.DossierComparisonDigest = f.cfg.DossierComparisonDigest
	return sevenPolicyFixture{f, q, qr, reply}
}

func TestDossierSevenStepCapturedPolicy(t *testing.T) {
	f := sevenPolicy(t)
	p, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, f.review, f.compared, f.ent, f.sem)
	if err != nil {
		t.Fatal(err)
	}
	e, s, reason, err := p.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots)
	if err != nil || e != Pass || s != Pass || reason != "" {
		t.Fatal("终检可见文字丢比较或语义回放不一致", e, s, reason, err)
	}
	if !p.MatchesAdmission(f.admission, f.proposal) {
		t.Fatal("两份准入组件未匹配")
	}
	q, r := p.ComparisonProof()
	if q != f.comparison || r != f.compared {
		t.Fatal("比较回执丢失")
	}
	for _, mode := range []string{"missing_entailment", "missing_fields", "missing_comparison", "missing_semantic", "wrong_component", "light", "old_constructor"} {
		t.Run(mode, func(t *testing.T) {
			c := f.cfg
			review, compared, ent, sem := f.review, f.compared, f.ent, f.sem
			switch mode {
			case "missing_entailment":
				ent = nil
			case "missing_fields":
				review = nil
			case "missing_comparison":
				compared = nil
			case "missing_semantic":
				sem = nil
			case "wrong_component":
				c.DossierComparisonDigest = strings.Repeat("f", 64)
			case "light":
				c.AnalysisType = 0
			}
			var got *Policy
			var err error
			if mode == "old_constructor" {
				got, err = NewDossierPolicy(c, f.plan, f.ent.Entailment, f.sem.Semantic)
			} else {
				got, err = NewDossierComparisonCapturedPolicy(c, f.plan, f.comparison, review, compared, ent, sem)
			}
			if err == nil || got != nil {
				t.Fatal("缺证据或旧入口批准了比较报告")
			}
		})
	}
	changed := f.response
	changed.RequestID += "-another"
	raw, err := f.comparison.CapturePayload(changed)
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.comparison.Review(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, f.review, other, f.ent, f.sem)
	if err != nil || p2.Digest() == p.Digest() {
		t.Fatal("相同判定新包复用了策略摘要", err)
	}
	var doc struct {
		Axes    []map[string]any `json:"axes"`
		Wording string           `json:"wording"`
	}
	if json.Unmarshal([]byte(f.response.Text), &doc) != nil {
		t.Fatal("响应解析失败")
	}
	for _, row := range doc.Axes {
		if row["relation"] == "aligned" {
			row["relation"] = "uncertain"
		}
	}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	changed.Text = string(raw)
	raw, err = f.comparison.CapturePayload(changed)
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := f.comparison.Review(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	p3, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, f.review, unknown, f.ent, nil)
	if err != nil {
		t.Fatal("全部口径不足仍应可沿终检失败释放", err)
	}
	e, s, _, err = p3.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots)
	if err != nil || e == Pass && s == Pass {
		t.Fatal("未捕获语义默认通过")
	}
}

func TestDossierSevenStepSealedModelStep(t *testing.T) {
	f := sevenPolicy(t)
	step, err := NewDossierComparisonStep(f.comparison)
	if err != nil || !step.Valid() || step.Config().AnalysisType != task.DeepAnalysisType || step.Config().CheckTokens() != 1024 || step.InputDigest() != f.comparison.InputDigest() || step.Request().Step != 5 || !step.StagePolicy().Valid() {
		t.Fatal("封闭适配丢类型、步位或容量", err)
	}
	v := task.Task{ID: 7, UserID: f.admission.Scope.UserID, No: f.proposal.TaskNo, SubjectType: task.Theme, SubjectID: f.proposal.ThemeID, AnalysisType: task.DeepAnalysisType}
	if !step.MatchesTask(v) || step.OperationKey(v, 4) != "" || step.OperationKey(v, 5) != f.comparison.OperationKey(v, 5) || step.Units() != f.comparison.Units() {
		t.Fatal("原任务或固定步位未绑定")
	}
	if _, err := step.CapturePayload(f.response); err != nil {
		t.Fatal(err)
	}
	var zero *DossierComparisonStep
	if zero.Valid() || zero.InputDigest() != "" || zero.Config() != (CheckConfig{}) || zero.MatchesTask(v) || zero.OperationKey(v, 5) != "" || zero.Units() != (task.Units{}) || zero.Request().Prompt != "" || zero.StagePolicy().Valid() {
		t.Fatal("空适配提供执行能力")
	}
	if _, err := zero.CapturePayload(provider.Response{}); err == nil {
		t.Fatal("空适配捕获")
	}
	if _, err := NewDossierComparisonStep(nil); err == nil {
		t.Fatal("空计划适配")
	}
	if step.StageName() != "judge" || strings.Contains(fmt.Sprintf("%+v %#v", *step, *step), f.comparison.InputDigest()) {
		t.Fatal("适配阶段或调试隐私不符")
	}
}

func TestDossierSevenStepAuditShape(t *testing.T) {
	for _, tc := range []struct {
		step  int
		stage string
		valid bool
	}{{5, "judge", true}, {6, "judge", true}, {6, "extract", false}, {7, "judge", false}} {
		a := auditRecordFixture()
		a.Request.StepNo, a.Request.Stage = tc.step, tc.stage
		if (a.Validate() == nil) != tc.valid {
			t.Fatal("审计联合格式步位不封闭", tc.step, tc.stage)
		}
	}
	// 联合格式接受第六步不表示任何旧准入获权；权限由原服务和SQL原键探针另核。
}

func TestDossierSevenStepPolicyFailures(t *testing.T) {
	f := sevenPolicy(t)
	for _, mode := range []string{"same", "missing_field_digest", "minimum_quotes", "nil_fields", "nil_comparison", "model"} {
		t.Run(mode, func(t *testing.T) {
			c, p, q := f.cfg, f.plan, f.comparison
			switch mode {
			case "missing_field_digest":
				c.DossierSupportDigest = ""
			case "minimum_quotes":
				c.MinQuotes = 5
			case "nil_fields":
				p = nil
			case "nil_comparison":
				q = nil
			case "model":
				c.ModelName = "other"
			}
			policy, err := NewDossierComparisonPolicy(c, p, q, f.ent.Entailment, f.sem.Semantic)
			if (err == nil) != (mode == "same") || mode != "same" && policy != nil {
				t.Fatal("待捕获策略放宽了原组件或深度约束", err)
			}
		})
	}
	t.Run("entailment_block", func(t *testing.T) {
		blocked := captureEvidence(t, f.ent.plan, checkReply(f.model, `{"verdict":"block","reason":"unsupported"}`))
		policy, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, nil, nil, blocked, nil)
		if err != nil {
			t.Fatal("蕴含拒绝不能沿原终检结束", err)
		}
		e, s, _, err := policy.Review(context.Background(), encoded(t, f.proposal), f.proposal, f.snapshots)
		if err != nil || e == Pass || s == Pass {
			t.Fatal("拒绝时补造通过回执", e, s, err)
		}
	})
	t.Run("bad_field_support", func(t *testing.T) {
		rows := []map[string]any{}
		for i := 1; i <= 6; i++ {
			v, r := "supported", "direct_support"
			if i == 2 {
				v, r = "partial", "partial_support"
			}
			rows = append(rows, map[string]any{"id": i, "verdict": v, "reason": r})
		}
		raw, err := json.Marshal(map[string]any{"results": rows})
		if err != nil {
			t.Fatal(err)
		}
		response := provider.Response{Text: string(raw), Model: f.model.Model, FinishReason: "stop", RequestID: "test-only-partial-fields", Usage: provider.Usage{Prompt: 2, Completion: 1, Total: 3}}
		raw, err = f.plan.CapturePayload(response)
		if err != nil {
			t.Fatal(err)
		}
		review, err := f.plan.Review(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		if policy, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, review, f.compared, f.ent, f.sem); err == nil || policy != nil {
			t.Fatal("矛盾回执形成终检策略")
		}
		if text, err := DossierComparisonVisible("", f.plan, review, f.comparison, f.compared); err == nil || text != "" {
			t.Fatal("矛盾回执提供可见文字")
		}
	})
	t.Run("wrong_semantic", func(t *testing.T) {
		if p, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, f.review, f.compared, f.ent, f.ent); err == nil || p != nil {
			t.Fatal("蕴含回执冒充语义")
		}
	})
	t.Run("wrong_entailment", func(t *testing.T) {
		if p, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, f.review, f.compared, f.sem, f.sem); err == nil || p != nil {
			t.Fatal("语义回执冒充蕴含")
		}
	})
	var none *Policy
	if p, r := none.ComparisonProof(); p != nil || r != nil {
		t.Fatal("空策略输出比较证据")
	}
	if text, err := DossierComparisonVisible("", nil, nil, nil, nil); err == nil || text != "" {
		t.Fatal("缺计划输出比较正文")
	}
}

func TestDossierSevenStepForeignReviewRefusal(t *testing.T) {
	f := sevenPolicy(t)
	foreign := dossierPolicyFixture(t, "supported", "direct_support").review
	if p, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, foreign, f.compared, f.ent, f.sem); err == nil || p != nil {
		t.Fatal("其他字段计划的回执获准交付")
	}
	c := f.cfg
	c.DossierSupportDigest = "bad"
	if p, err := NewDossierComparisonCapturedPolicy(c, f.plan, f.comparison, f.review, f.compared, f.ent, f.sem); err == nil || p != nil {
		t.Fatal("比较回执遮蔽了字段组件失效")
	}
	p, err := NewDossierComparisonCapturedPolicy(f.cfg, f.plan, f.comparison, f.review, f.compared, f.ent, f.sem)
	if err != nil {
		t.Fatal(err)
	}
	// 防御性检查也不能在内部装配漏失或拿错字段回执后返回可见正文。
	p.dossierReview = foreign
	if _, text, err := p.Inputs(encoded(t, f.proposal), f.snapshots); err == nil || text != "" {
		t.Fatal("错字段回执仍输出可见文字")
	}
	p.dossierReview = nil
	if _, text, err := p.Inputs(encoded(t, f.proposal), f.snapshots); err == nil || text != "" {
		t.Fatal("缺字段回执仍输出比较文字")
	}
}
