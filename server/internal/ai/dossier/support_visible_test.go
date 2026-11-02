package dossier

import (
	"context"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDossierReportVisibleBinding(t *testing.T) {
	d, snapshots := testDraft(t)
	p := supportPlan(t, d, snapshots, "progress_a")
	response := supportResponse(t, p)
	rows := supportRows(t, response)
	for _, row := range rows {
		row["verdict"] = "supported"
		row["reason"] = "direct_support"
	}
	response = replaceSupportRows(t, response, rows)
	raw := supportPayload(t, p, response)
	r, err := p.Review(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	visible, err := p.VisibleReview(r)
	if err != nil || !strings.Contains(visible, "原文完整支持") || !strings.Contains(visible, p.checks[0].Value) || r.ResolvedCount() != len(r.Results()) || r.PayloadDigest() != task.StageDigest(raw) {
		t.Fatal("固定呈现与捕获不符", err)
	}
	response.RequestID = "test-only-another-receipt"
	other, err := p.Review(context.Background(), supportPayload(t, p, response))
	if err != nil || other.PayloadDigest() == r.PayloadDigest() {
		t.Fatal("同状态回执被当同一捕获", err)
	}
	for _, mode := range []string{"input", "draft", "payload", "missing", "id", "card", "field", "free_reason"} {
		t.Run(mode, func(t *testing.T) {
			bad := *r
			bad.results = r.Results()
			switch mode {
			case "input":
				bad.inputDigest = strings.Repeat("e", 64)
			case "draft":
				bad.draftDigest = strings.Repeat("e", 64)
			case "payload":
				bad.payloadDigest = ""
			case "missing":
				bad.results = bad.results[1:]
			case "id":
				bad.results[0].ID++
			case "card":
				bad.results[0].Card = "other"
			case "field":
				bad.results[0].Field = "other"
			case "free_reason":
				bad.results[0].Reason = "模型自由解释"
			}
			if got, err := p.VisibleReview(&bad); got != "" || err == nil {
				t.Fatal("错定位或自由文本进入呈现")
			}
		})
	}
	var empty *SupportReview
	var missing *SupportPlan
	if empty.PayloadDigest() != "" || empty.ResolvedCount() != 0 {
		t.Fatal("空核验有结果")
	}
	if got, err := missing.VisibleReview(r); got != "" || err == nil {
		t.Fatal("空计划有呈现")
	}
}

func TestDossierReportVisibleVerdicts(t *testing.T) {
	d, s := testDraft(t)
	p := supportPlan(t, d, s, "progress_a")
	for _, tc := range []struct {
		verdict, reason, label string
		resolved               bool
	}{{"supported", "direct_support", "原文完整支持", true}, {"partial", "partial_support", "原文部分支持", true}, {"unsupported", "not_supported", "原文不支持", true}, {"uncertain", "insufficient_context", "资料不足", false}} {
		t.Run(tc.verdict, func(t *testing.T) {
			response := supportResponse(t, p)
			rows := supportRows(t, response)
			for _, row := range rows {
				row["verdict"] = tc.verdict
				row["reason"] = tc.reason
			}
			response = replaceSupportRows(t, response, rows)
			r, err := p.Review(context.Background(), supportPayload(t, p, response))
			if err != nil {
				t.Fatal(err)
			}
			visible, err := p.VisibleReview(r)
			if err != nil || !strings.Contains(visible, tc.label) || (r.ResolvedCount() > 0) != tc.resolved {
				t.Fatal("否定或未解决状态失真", err)
			}
		})
	}
}
