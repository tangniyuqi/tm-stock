package candidate

import (
	"strings"
	"testing"
	"time"
)

func TestInspectionRequestsAndEvidenceBinding(t *testing.T) {
	r := InspectionBatchRequest{RequestID: "test_inspection_batch_0001", WindowSize: 2000, SampleSize: 200}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*InspectionBatchRequest){func(r *InspectionBatchRequest) { r.WindowSize = 2001 }, func(r *InspectionBatchRequest) { r.SampleSize = 0 }, func(r *InspectionBatchRequest) { r.SampleSize = 201 }, func(r *InspectionBatchRequest) { r.WindowSize = 1 }, func(r *InspectionBatchRequest) { r.AfterReviewID = -1 }, func(r *InspectionBatchRequest) { r.RequestID = "bad" }} {
		bad := r
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatal("坏批次参数未拒绝")
		}
	}
	c := validCandidate()
	c.ID = 1
	digest, err := EvidenceDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	item := InspectionItem{Selection: InspectionSelection{ID: 1, BatchID: 2, ReviewID: 3, CandidateID: 1}, ActorID: 8, Candidate: c, Original: AuditEvent{ID: 3, Request: ReviewRequest{CandidateID: 1, EvidenceDigest: digest, Decision: "approve", RequestID: "test_original_review_0001"}, ActorID: 7, MappingID: 10, CreatedAt: time.UnixMilli(1700000000123)}}
	first, err := InspectionDigest(item)
	if err != nil {
		t.Fatal(err)
	}
	item.Result = &InspectionEvent{ID: 1}
	item.Original.CreatedAt = item.Original.CreatedAt.In(time.FixedZone("测试时区", 8*3600))
	if next, err := InspectionDigest(item); err != nil || next != first {
		t.Fatal("终态或时区改变了原摘要")
	}
	for _, mutate := range []func(*InspectionItem){func(i *InspectionItem) { i.Selection.ID++ }, func(i *InspectionItem) { i.Selection.BatchID++ }, func(i *InspectionItem) { i.Original.MappingID++ }, func(i *InspectionItem) { i.ActorID++ }} {
		v := item
		mutate(&v)
		if next, err := InspectionDigest(v); err != nil || next == first {
			t.Fatal("摘要未绑定完整身份/分派/归属")
		}
	}
	for _, mutate := range []func(*InspectionItem){func(i *InspectionItem) { i.Selection.ReviewID = 0; i.Original.ID = 0 }, func(i *InspectionItem) { i.Candidate.SourceExcerpt = "不匹配" }, func(i *InspectionItem) { i.Original.Request.EvidenceDigest = strings.Repeat("0", 64) }} {
		v := item
		mutate(&v)
		if _, err := InspectionDigest(v); err == nil {
			t.Fatal("坏依据未拒绝")
		}
	}
	for _, decision := range []string{"confirm", "issue"} {
		request := InspectionRequest{ItemID: 1, EvidenceDigest: first, Decision: decision, RequestID: "test_inspection_decision_0001"}
		if decision == "issue" {
			request.Reason = "原文不支持"
		}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		request.Reason = " "
		if request.Validate() == nil {
			t.Fatal("坏原因未拒绝")
		}
	}
	if (InspectionRequest{Decision: "unknown"}).Validate() == nil {
		t.Fatal("未知复核决定未拒绝")
	}
}

func TestInspectionMappingStrictEvidenceMatch(t *testing.T) {
	c := validCandidate()
	m := InspectionMapping{ID: 10, ThemeID: c.ThemeID, StockID: c.StockID, TsCode: c.TsCode, SourceType: c.SourceType, SourceExcerpt: c.SourceExcerpt, SourceURL: c.SourceURL, CollectedAt: c.CollectedAt, AuditStatus: 2, Status: 1}
	if !m.Matches(c) {
		t.Fatal("有效归属未匹配")
	}
	m.ExcerptTrimmed = !c.ExcerptTrimmed
	if m.Matches(c) {
		t.Fatal("节选标记变化仍匹配")
	}
	m.ExcerptTrimmed = c.ExcerptTrimmed
	m.Status = 0
	if !m.Matches(c) {
		t.Fatal("已暂停的同证据仍应可记录问题")
	}
	for _, mutate := range []func(*InspectionMapping){func(m *InspectionMapping) { m.Deleted = true }, func(m *InspectionMapping) { m.ThemeID++ }, func(m *InspectionMapping) { m.StockID++ }, func(m *InspectionMapping) { m.TsCode = "other" }, func(m *InspectionMapping) { m.AuditStatus = 0 }, func(m *InspectionMapping) { m.Status = 2 }, func(m *InspectionMapping) { m.CollectedAt = m.CollectedAt.Add(time.Second) }, func(m *InspectionMapping) { m.SourceURL += "?other=1" }, func(m *InspectionMapping) { m.SourceType++ }, func(m *InspectionMapping) { m.SourceExcerpt += "其他" }} {
		v := m
		mutate(&v)
		if v.Matches(c) {
			t.Fatal("变更后的归属仍被匹配")
		}
	}
}
