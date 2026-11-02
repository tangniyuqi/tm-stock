package dossier

import (
	"context"
	"testing"
)

func TestDossierReviewPresentation(t *testing.T) {
	d, snaps := testDraft(t)
	p := supportPlan(t, d, snaps, "progress_a")
	r, err := p.Review(context.Background(), supportPayload(t, p, supportResponse(t, p)))
	if err != nil {
		t.Fatal(err)
	}
	fields, err := p.ReviewedFields(r)
	if err != nil || len(fields) != len(p.checks) {
		t.Fatal("未逐项投影", err)
	}
	for i, f := range fields {
		c := p.checks[i]
		if f.Value != c.Value || f.Statement != c.Statement || f.Reference != c.Reference || f.StatementRef != c.StatementRef || f.Verdict != "uncertain" {
			t.Fatal("投影失去原绑定")
		}
	}
	fields[0].Value = "外部篡改"
	again, err := p.ReviewedFields(r)
	if err != nil || again[0].Value == fields[0].Value {
		t.Fatal("投影暴露可写别名", err)
	}
	for _, mode := range []string{"plan", "review", "input", "payload", "rows", "reason"} {
		t.Run(mode, func(t *testing.T) {
			q, rr := *p, *r
			rr.results = r.Results()
			var plan *SupportPlan = &q
			var review *SupportReview = &rr
			switch mode {
			case "plan":
				plan = nil
			case "review":
				review = nil
			case "input":
				rr.inputDigest = ""
			case "payload":
				rr.payloadDigest = ""
			case "rows":
				rr.results = nil
			case "reason":
				rr.results[0].Reason = "自由解释"
			}
			if out, e := plan.ReviewedFields(review); e == nil || out != nil {
				t.Fatal("非法绑定产生公开投影")
			}
		})
	}
}
