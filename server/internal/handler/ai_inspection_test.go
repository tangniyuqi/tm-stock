package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type inspectionHandlerFake struct {
	err     error
	calls   int
	actor   int64
	badItem bool
}

func (s *inspectionHandlerFake) Create(_ context.Context, actor int64, r candidate.InspectionBatchRequest) (candidate.InspectionBatch, error) {
	s.calls++
	s.actor = actor
	if err := r.Validate(); err != nil {
		return candidate.InspectionBatch{}, err
	}
	return candidate.InspectionBatch{ID: 1, Request: r, ActorID: actor, CreatedAt: time.UnixMilli(1700000000123)}, s.err
}
func (s *inspectionHandlerFake) Batch(_ context.Context, actor int64, id string) (candidate.InspectionBatch, error) {
	s.calls++
	s.actor = actor
	return candidate.InspectionBatch{ID: 1, ActorID: actor, Request: candidate.InspectionBatchRequest{RequestID: id}, CreatedAt: time.UnixMilli(1700000000123)}, s.err
}
func (s *inspectionHandlerFake) Item(ctx context.Context, actor, id int64) (candidate.InspectionItem, error) {
	s.calls++
	s.actor = actor
	records, _ := (&reviewHandlerFake{}).ListPending(ctx, 0, 1)
	c := records[0].Candidate
	digest, _ := candidate.EvidenceDigest(c)
	item := candidate.InspectionItem{Selection: candidate.InspectionSelection{ID: id, BatchID: 2, ReviewID: 3, CandidateID: c.ID}, ActorID: actor, Candidate: c, Original: candidate.AuditEvent{ID: 3, Request: candidate.ReviewRequest{CandidateID: c.ID, EvidenceDigest: digest, Decision: "approve", RequestID: "test_original_review_0001"}, ActorID: 8, MappingID: 100, CreatedAt: time.UnixMilli(1700000000123)}}
	if s.badItem {
		item.Candidate.SourceExcerpt = "变化"
	}
	return item, s.err
}
func (s *inspectionHandlerFake) Decide(_ context.Context, actor int64, r candidate.InspectionRequest) (candidate.InspectionEvent, error) {
	s.calls++
	s.actor = actor
	return candidate.InspectionEvent{ID: 1, Request: r, ActorID: actor, MappingID: 100, CreatedAt: time.UnixMilli(1700000000123)}, s.err
}
func (s *inspectionHandlerFake) Issues(_ context.Context, after int64, limit int) ([]candidate.InspectionEvent, error) {
	s.calls++
	if after < 0 || limit < 1 || limit > 200 {
		return nil, candidate.ErrInvalidInspection
	}
	return []candidate.InspectionEvent{{ID: 2, Request: candidate.InspectionRequest{Decision: "issue", Reason: "证据不足"}, ActorID: 7, MappingID: 100, CreatedAt: time.UnixMilli(1700000000123)}}, s.err
}

func inspectionHTTP(t *testing.T, s *inspectionHandlerFake) (http.Handler, func(string, string, string) *httptest.ResponseRecorder) {
	t.Helper()
	mux := http.NewServeMux()
	NewAIInspectionHandler(s).Register(mux)
	credential := strings.Repeat("test_", 8)
	auth, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: credential}})
	if err != nil {
		t.Fatal(err)
	}
	h := auth(mux)
	return mux, func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
}
func TestInspectionHTTPIdentityRoutesAndStrictInput(t *testing.T) {
	s := &inspectionHandlerFake{}
	mux, call := inspectionHTTP(t, s)
	valid := `{"item_id":1,"evidence_digest":"` + strings.Repeat("0", 64) + `","decision":"confirm","reason":"","request_id":"test_inspection_decision_0001"}`
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"POST", "/internal/v1/ai-inspection-batches", `{"request_id":"test_inspection_batch_0001","after_review_id":0,"window_size":100,"sample_size":10}`, 200},
		{"GET", "/internal/v1/ai-inspection-batches/test_inspection_batch_0001", "", 200},
		{"GET", "/internal/v1/ai-inspection-items/1", "", 200},
		{"POST", "/internal/v1/ai-inspection-decisions", valid, 200},
		{"GET", "/internal/v1/ai-inspection-issues?after_id=0&limit=20", "", 200},
		{"POST", "/internal/v1/ai-inspection-decisions", strings.TrimSuffix(valid, "}") + `,"actor_id":99}`, 400},
		{"POST", "/internal/v1/ai-inspection-decisions", valid + ` {}`, 400},
		{"POST", "/internal/v1/ai-inspection-decisions", strings.Repeat(" ", 8193) + valid, 400},
		{"GET", "/internal/v1/ai-inspection-items/0", "", 400},
		{"GET", "/internal/v1/ai-inspection-items/bad", "", 400},
		{"GET", "/internal/v1/ai-inspection-issues?after_id=bad", "", 400},
		{"GET", "/internal/v1/ai-inspection-issues?limit=bad", "", 400},
		{"GET", "/internal/v1/ai-inspection-issues?limit=1&limit=2", "", 400},
		{"GET", "/internal/v1/ai-inspection-issues?actor_id=99", "", 400},
	} {
		w := call(tc.method, tc.path, tc.body)
		if w.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
		if tc.code == 200 {
			var payload map[string]any
			if json.Unmarshal(w.Body.Bytes(), &payload) != nil || payload["code"] != float64(0) || strings.Contains(w.Body.String(), "2023-") {
				t.Fatal("响应时点/包装错误")
			}
		}
	}
	if s.calls != 5 || s.actor != 7 {
		t.Fatalf("错误路径调用下游或身份未绑定: %d %d", s.calls, s.actor)
	}
	for _, path := range []string{"/internal/v1/ai-inspection-batches", "/internal/v1/ai-inspection-items/1", "/internal/v1/ai-inspection-issues"} {
		method := "GET"
		if strings.HasSuffix(path, "batches") {
			method = "POST"
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(`{}`)))
		if w.Code != 401 {
			t.Fatal("缺中间件仍可访问")
		}
	}
}
func TestInspectionHTTPErrorMappingAndBrokenEvidence(t *testing.T) {
	s := &inspectionHandlerFake{}
	_, call := inspectionHTTP(t, s)
	for _, tc := range []struct {
		err  error
		code int
	}{{candidate.ErrInvalidInspection, 400}, {candidate.ErrInspectionNotFound, 404}, {candidate.ErrInspectionActor, 403}, {candidate.ErrConflict, 409}, {errors.New("sensitive-internal"), 500}} {
		s.err = tc.err
		w := call("GET", "/internal/v1/ai-inspection-batches/test_inspection_batch_0001", "")
		if w.Code != tc.code || strings.Contains(w.Body.String(), "sensitive") {
			t.Fatalf("错误映射: %d %s", w.Code, w.Body.String())
		}
	}
	s.err = nil
	s.badItem = true
	if w := call("GET", "/internal/v1/ai-inspection-items/1", ""); w.Code != 409 {
		t.Fatal(w.Code)
	}
}
