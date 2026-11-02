package dossier

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func sevenStepSupport(t *testing.T) (*SupportPlan, *SupportReview, *CardComparisonPlan, *CardComparisonReview) {
	t.Helper()
	d, snaps := comparisonDraft(t, nil)
	cfg := comparisonConfig()
	prepared := comparisonPreparation(t, cfg, d, snaps)
	q, err := prepared.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	scfg := cfg.Model
	scfg.ReviewStep = 4
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := NewSupportPlan(context.Background(), scfg, supportTaskFixture(), "仅测试专题", bound, snaps, q.CardKeys())
	if err != nil {
		t.Fatal(err)
	}
	response := supportResponse(t, fields)
	rows := supportRows(t, response)
	for _, r := range rows {
		r["verdict"], r["reason"] = "supported", "direct_support"
	}
	response = replaceSupportRows(t, response, rows)
	raw, err := fields.CapturePayload(response)
	if err != nil {
		t.Fatal(err)
	}
	review, err := fields.Review(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	compared, _ := comparisonReview(t, q, nil)
	return fields, review, q, compared
}

func TestDossierSevenStepSupportConsistency(t *testing.T) {
	for _, mode := range []string{"consistent", "field_partial", "field_unknown_meaning", "section_partial", "cross_account", "cross_draft", "missing_review", "missing_comparison"} {
		t.Run(mode, func(t *testing.T) {
			fields, review, q, compared := sevenStepSupport(t)
			switch mode {
			case "field_partial":
				review.results[1].Verdict, review.results[1].Reason = "partial", "partial_support"
			case "field_unknown_meaning":
				review.results[1].Verdict, review.results[1].Reason = "uncertain", "insufficient_context"
			case "section_partial":
				review.results[0].Verdict, review.results[0].Reason = "partial", "partial_support"
			case "cross_account":
				fields.identity.UserID++
			case "cross_draft":
				fields.draftDigest = strings.Repeat("a", 64)
			case "missing_review":
				review = nil
			case "missing_comparison":
				compared = nil
			}
			err := q.CheckSupport(fields, review, compared)
			if (err == nil) != (mode == "consistent") {
				t.Fatal("矛盾或错绑定回执被当一致", err)
			}
		})
	}
	fields, review, q, _ := sevenStepSupport(t)
	review.results[1].Verdict, review.results[1].Reason = "partial", "partial_support"
	compared, _ := comparisonReview(t, q, func(d *comparisonTestDocument) { d.Axes[0]["left"], d.Axes[0]["relation"] = "partial", "uncertain" })
	if err := q.CheckSupport(fields, review, compared); err != nil || compared.Outcome() != "insufficient_context" {
		t.Fatal("一致的部分支持未保留不足", err)
	}
	keys := q.CardKeys()
	keys[0] = "changed"
	if q.CardKeys()[0] == "changed" || q.DraftDigest() != fields.DraftDigest() {
		t.Fatal("返回卡键可改内部或档案不一致")
	}
	if (*SupportPlan)(nil).DraftDigest() != "" || (*CardComparisonPlan)(nil).DraftDigest() != "" || (*CardComparisonPlan)(nil).CardKeys() != nil {
		t.Fatal("空值输出定位")
	}
}

func TestDossierSevenStepComparisonAdmission(t *testing.T) {
	fields, _, q, _ := sevenStepSupport(t)
	a := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: supportTaskFixture().UserID, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: supportTaskFixture().SubjectID}, Price: points.Price{Version: "test-only-price", Points: 30}, Budget: task.Budget{MaxAttempts: 2, MaxCalls: 7, MaxTokens: 100000, MaxPages: 3, MaxDurationMS: 60000}, SubjectName: q.themeName, ModelName: q.cfg.Model.Model, FilingNo: q.cfg.Model.FilingNo, ProfileDigest: q.cfg.Model.ProfileDigest, ExecutionDigest: strings.Repeat("e", 64), ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", PriceRef: "test-only-price", SourceRef: "test-only-source", Materials: q.materials, ReportProfile: task.DossierComparisonReportProfile, DossierSupportDigest: fields.ComponentDigest(), DossierComparisonDigest: q.ComponentDigest()}
	if err := a.Validate(a.Scope); err != nil || !q.MatchesAdmission(a) || !fields.MatchesAdmission(a) {
		t.Fatal("合成准入未绑定两个组件", err)
	}
	for _, mode := range []string{"account", "theme", "component", "model", "profile", "material", "budget", "old_profile"} {
		t.Run(mode, func(t *testing.T) {
			b := a.Clone()
			switch mode {
			case "account":
				b.Scope.UserID++
			case "theme":
				b.Scope.SubjectID++
			case "component":
				b.DossierComparisonDigest = strings.Repeat("f", 64)
			case "model":
				b.ModelName = "other"
			case "profile":
				b.ProfileDigest = strings.Repeat("f", 64)
			case "material":
				b.Materials[0].Digest = strings.Repeat("f", 64)
			case "budget":
				b.Budget.MaxCalls = 6
			case "old_profile":
				b.ReportProfile = task.DossierReportProfile
			}
			if q.MatchesAdmission(b) {
				t.Fatal("错误准入被比较计划复用")
			}
		})
	}
	var zero *CardComparisonPlan
	if zero.MatchesAdmission(a) || zero.MatchesFields(fields) {
		t.Fatal("空比较计划匹配准入")
	}
	if (*CardComparisonPreparation)(nil).Config() != (CardComparisonConfig{}) || (*CardComparisonPreparation)(nil).MatchesFields(nil) {
		t.Fatal("空准备返回配置")
	}
	d, snaps := comparisonDraft(t, nil)
	prep := comparisonPreparation(t, comparisonConfig(), d, snaps)
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	fieldPrep, err := PrepareSupport(context.Background(), comparisonConfig().Model, "仅测试专题", bound, snaps, []string{"left_card"})
	if err != nil {
		t.Fatal(err)
	}
	if prep.MatchesFields(fieldPrep) {
		t.Fatal("字段准备未覆盖比较两卡")
	}
	one, err := fieldPrep.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	if q.MatchesFields(one) {
		t.Fatal("单卡字段计划覆盖两卡")
	}
	if err := q.CheckSupport(nil, nil, nil); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
}
