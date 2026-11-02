package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
	"strings"
	"testing"
)

func TestDossierReviewDTO(t *testing.T) {
	for _, verdict := range []string{"supported", "partial", "unsupported", "uncertain"} {
		t.Run(verdict, func(t *testing.T) {
			p, v, _ := dossierReportUnitPlan(t)
			snap := p.Snapshots[0]
			s := snap.Sentences[0]
			ref := dossier.SentenceRef{SnapshotID: snap.ID, Hash: snap.Hash, Number: s.Number, StartByte: s.StartByte, EndByte: s.EndByte}
			unknown := dossier.Field{State: "unknown"}
			card := dossier.FactCard{Key: "test_card", Section: "definition", Statement: ref, Subject: dossier.Field{State: "known", Ref: ref}, PublishedDate: unknown, EffectiveDate: unknown, Period: unknown, Scope: unknown, Stage: unknown, Basis: unknown, Unit: unknown}
			draft := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: v.SubjectID, Version: 1, CreatedAt: snap.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{card}}
			bound, err := dossier.BindDraft(draft, []sources.Snapshot{snap})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := dossier.NewSupportPlan(context.Background(), p.Dossier.Config(), v, p.ThemeName, bound, []sources.Snapshot{snap}, []string{"test_card"})
			if err != nil {
				t.Fatal(err)
			}
			reason := map[string]string{"supported": "direct_support", "partial": "partial_support", "unsupported": "not_supported", "uncertain": "insufficient_context"}[verdict]
			body := fmt.Sprintf(`{"results":[{"id":1,"verdict":%q,"reason":%q},{"id":2,"verdict":%q,"reason":%q}]}`, verdict, reason, verdict, reason)
			raw, err := plan.CapturePayload(provider.Response{Text: body, Model: plan.Config().Model, FinishReason: "stop", RequestID: "test-only-review", Usage: provider.Usage{Prompt: 8, Completion: 8, Total: 16}})
			if err != nil {
				t.Fatal(err)
			}
			review, err := plan.Review(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			out, err := dossierReviewDTO(plan, review, bound)
			if verdict == "uncertain" {
				if err == nil || out != nil {
					t.Fatal("全部不确定仍公开")
				}
				return
			}
			if err != nil || out == nil || len(out.Items) != 2 || out.Items[1].Field != "subject" || out.Items[0].Checked.Text != s.Text || out.Items[0].Verdict != verdict {
				t.Fatal("白名单投影不符", err)
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"InputDigest", "ApprovalRef", "requestId", "profileDigest", "component", "payloadDigest"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatal("泄漏内部字段", secret)
				}
			}
			if strings.Contains(fmt.Sprintf("%#v", *out), s.Text) {
				t.Fatal("格式化日志泄漏原文")
			}
			out.Items[0].Checked.Text = "外部篡改"
			again, err := dossierReviewDTO(plan, review, bound)
			if err != nil || again.Items[0].Checked.Text == out.Items[0].Checked.Text {
				t.Fatal("DTO别名影响原结果", err)
			}
			if bad, err := dossierReviewDTO(nil, review, bound); err == nil || bad != nil {
				t.Fatal("缺计划仍公开")
			}
			if bad, err := dossierReviewDTO(plan, nil, bound); err == nil || bad != nil {
				t.Fatal("缺回执仍公开")
			}
			draft.Version = 2
			changed, err := dossier.BindDraft(draft, []sources.Snapshot{snap})
			if err != nil {
				t.Fatal(err)
			}
			if bad, err := dossierReviewDTO(plan, review, changed); err == nil || bad != nil {
				t.Fatal("跨版本仍公开")
			}
			if bad, err := dossierReviewDTO(plan, review, nil); err == nil || bad != nil {
				t.Fatal("缺档案仍公开")
			}
		})
	}
}
