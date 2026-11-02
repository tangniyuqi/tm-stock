package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func reportComparisonFixture(t *testing.T, relation, wording string) (*dossier.CardComparisonPlan, *dossier.CardComparisonReview, *dossier.BoundDraft) {
	t.Helper()
	p, v, _ := sevenStepReportPlan(t)
	s := p.Snapshots[0]
	sentence := s.Sentences[0]
	ref := dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}
	u := dossier.Field{State: "unknown"}
	card := dossier.FactCard{Key: "card_a", Section: "definition", Statement: ref, Subject: dossier.Field{State: "known", Ref: ref}, Scope: dossier.Field{State: "known", Ref: ref}, PublishedDate: u, EffectiveDate: u, Period: u, Stage: u, Basis: u, Unit: u}
	right := card
	right.Key = "card_b"
	bound, err := dossier.BindDraft(dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: v.SubjectID, Version: 1, CreatedAt: s.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{card, right}}, []sources.Snapshot{s})
	if err != nil || bound.Digest() != p.Comparison.DraftDigest() {
		t.Fatal("展示夹具与原计划不一致", err)
	}
	rows := make([]map[string]any, 0, 7)
	for i := 1; i <= 7; i++ {
		left, right, rel := "unknown", "unknown", "not_applicable"
		if i == 1 || i == 4 {
			left, right, rel = "supported", "supported", relation
		}
		rows = append(rows, map[string]any{"id": i, "left": left, "right": right, "relation": rel})
	}
	raw, err := json.Marshal(map[string]any{"axes": rows, "wording": wording})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := p.Comparison.CapturePayload(provider.Response{Text: string(raw), Model: p.Comparison.Config().Model.Model, FinishReason: "stop", RequestID: "test-only-public-comparison", Usage: provider.Usage{Prompt: 8, Completion: 8, Total: 16}})
	if err != nil {
		t.Fatal(err)
	}
	review, err := p.Comparison.Review(context.Background(), packet)
	if err != nil {
		t.Fatal(err)
	}
	return p.Comparison, review, bound
}

func TestReportComparisonDTOProjection(t *testing.T) {
	for _, sample := range []struct{ relation, wording, outcome string }{{"aligned", "same_meaning", "scope_aligned"}, {"aligned", "different_wording", "wording_differs"}, {"different", "different_wording", "not_comparable"}, {"aligned", "insufficient_context", "insufficient_context"}} {
		t.Run(sample.outcome, func(t *testing.T) {
			plan, review, bound := reportComparisonFixture(t, sample.relation, sample.wording)
			out, err := dossierComparisonDTO(plan, review, bound)
			if err != nil || out == nil || out.Outcome != sample.outcome || len(out.Axes) != 7 || out.Left.Key != "card_a" || out.Right.Key != "card_b" || out.StatementOrigins != "same_document" || out.Notice != reportComparisonNotice {
				t.Fatal("比较投影不符", err)
			}
			for i, axis := range out.Axes {
				known := i == 0 || i == 3
				if axis.Required != known || (axis.Left.Quote != nil) != known || (axis.Right.Quote != nil) != known {
					t.Fatal("未知补值或必核项不符")
				}
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"requestId", "componentDigest", "inputDigest", "payloadDigest", "profileDigest", "approval", "test-only-public-comparison"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatal("泄漏内部定位", secret)
				}
			}
			if strings.Contains(fmt.Sprintf("%#v", *out), out.Left.Statement.Text) {
				t.Fatal("格式化日志泄漏原文")
			}
			out.Axes[0].Left.Quote.Text = "外部改写"
			again, err := dossierComparisonDTO(plan, review, bound)
			if err != nil || again.Axes[0].Left.Quote.Text == "外部改写" {
				t.Fatal("外部投影修改影响原证据", err)
			}
		})
	}
}

func TestReportComparisonDTORefusals(t *testing.T) {
	plan, review, bound := reportComparisonFixture(t, "aligned", "same_meaning")
	for _, mode := range []string{"plan", "review", "bound", "foreign_review", "foreign_bound", "unresolved"} {
		t.Run(mode, func(t *testing.T) {
			p, r, b := plan, review, bound
			switch mode {
			case "plan":
				p = nil
			case "review":
				r = nil
			case "bound":
				b = nil
			case "foreign_review":
				other, task, _ := sevenStepReportPlan(t)
				cfg := p.Config()
				cfg.Model.MaxPromptBytes++
				prepared, err := dossier.PrepareCardComparison(context.Background(), cfg, other.ThemeName, bound, other.Snapshots, p.CardKeys())
				if err != nil {
					t.Fatal(err)
				}
				p, err = prepared.BindTask(context.Background(), task)
				if err != nil {
					t.Fatal(err)
				}
			case "foreign_bound":
				var stored struct{ Draft dossier.Draft }
				if err := json.Unmarshal(bound.Canonical(), &stored); err != nil {
					t.Fatal(err)
				}
				stored.Draft.Version++
				other, _, _ := sevenStepReportPlan(t)
				var err error
				b, err = dossier.BindDraft(stored.Draft, other.Snapshots)
				if err != nil {
					t.Fatal(err)
				}
			case "unresolved":
				p, r, b = reportComparisonFixture(t, "uncertain", "insufficient_context")
			}
			if out, err := dossierComparisonDTO(p, r, b); err == nil || out != nil {
				t.Fatal("坏绑定仍公开")
			}
		})
	}
}

func TestReportComparisonSideRefusals(t *testing.T) {
	q := dto.AIDossierQuote{Text: "仅测试原文"}
	for _, field := range []dto.AIDossierField{{Key: "subject", State: "known"}, {Key: "subject", State: "unknown", Quote: &q}, {Key: "scope", State: "known", Quote: &q}} {
		if _, err := dossierComparisonSide(dto.AIDossierCard{Fields: []dto.AIDossierField{field}}, "subject", "supported"); err == nil {
			t.Fatal("缺定位或字段不符仍公开")
		}
	}
	if _, err := dossierComparisonSide(dto.AIDossierCard{Fields: []dto.AIDossierField{{Key: "subject", State: "known", Quote: &q}}}, "subject", "unknown"); err == nil {
		t.Fatal("已知字段按未知输出")
	}
}

func TestReportComparisonLegacyEncoding(t *testing.T) {
	raw, err := json.Marshal(dto.AIReportDossierReview{Version: 1, Items: []dto.AIReportDossierField{}})
	if err != nil || strings.Contains(string(raw), "comparison") {
		t.Fatal("旧报告编码新增空比较", err)
	}
}
