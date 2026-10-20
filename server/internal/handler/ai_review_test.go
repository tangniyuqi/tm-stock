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
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type reviewHandlerFake struct {
	err     error
	calls   int
	actor   int64
	request candidate.ReviewRequest
}

func (s *reviewHandlerFake) ListPending(context.Context, int64, int) ([]candidate.ReviewRecord, error) {
	return []candidate.ReviewRecord{{Candidate: candidate.Candidate{ID: 1, SnapshotID: 1, ThemeID: 2, ThemeKey: "算力", StockID: 3, EntityKey: "星河科技", TsCode: "688001.SH",
		Predicate: "DISCLOSES_BUSINESS", SourceType: 2, SourceExcerpt: "星河科技主营算力服务。", SentenceIDs: []int{1}, QuoteSegments: []string{"星河科技主营算力服务。"},
		SourceURL: "https://example.test/report", CollectedAt: time.UnixMilli(1700000000123), Provider: "test-provider", Model: "test-model", PromptVersion: "test-v1"}}}, s.err
}
func (s *reviewHandlerFake) Review(_ context.Context, actor int64, r candidate.ReviewRequest) (candidate.AuditEvent, error) {
	s.calls++
	s.actor, s.request = actor, r
	return candidate.AuditEvent{ID: 1, Request: r, ActorID: actor, MappingID: 100, CreatedAt: time.UnixMilli(1700000000123)}, s.err
}

func TestAIReviewHandlerIdentityAndStrictJSON(t *testing.T) {
	svc := &reviewHandlerFake{}
	mux := http.NewServeMux()
	NewAIReviewHandler(svc).Register(mux)
	credential := strings.Repeat("test_", 8)
	auth, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: credential}})
	if err != nil {
		t.Fatal(err)
	}
	h := auth(mux)
	valid := `{"candidate_id":1,"evidence_digest":"` + strings.Repeat("0", 64) + `","decision":"approve","reason":"","request_id":"test_request_00001"}`
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"合法请求", valid, 200}, {"伪造身份", strings.TrimSuffix(valid, "}") + `,"actor_id":999}`, 400},
		{"尾随JSON", valid + ` {}`, 400}, {"畸形JSON", `{`, 400}, {"过大请求", strings.Repeat(" ", 8193) + valid, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/internal/v1/ai-review-decisions", strings.NewReader(tc.body))
			r.RemoteAddr = "127.0.0.1:3000"
			r.Header.Set("Authorization", "Bearer "+credential)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatalf("状态=%d 内容=%s", w.Code, w.Body.String())
			}
		})
	}
	if svc.calls != 1 || svc.actor != 7 {
		t.Fatalf("身份或拒绝路径有误: 次数=%d 身份=%d", svc.calls, svc.actor)
	}
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/ai-review-candidates", nil)
	r.RemoteAddr = "127.0.0.1:3000"
	r.Header.Set("Authorization", "Bearer "+credential)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var body struct {
		Data struct {
			Items []struct {
				CollectedAt    int64  `json:"collected_at"`
				EvidenceDigest string `json:"evidence_digest"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Data.Items) != 1 || body.Data.Items[0].CollectedAt != 1700000000123 || len(body.Data.Items[0].EvidenceDigest) != 64 {
		t.Fatalf("时间戳或摘要契约失败: %s %v", w.Body.String(), err)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/v1/ai-review-candidates", nil))
	if w.Code != 401 {
		t.Fatal("漏接中间件时未拒绝")
	}
}

func TestAIReviewErrorMappingNeverLeaksDetails(t *testing.T) {
	h := NewAIReviewHandler(&reviewHandlerFake{})
	for _, tc := range []struct {
		err  error
		code int
	}{
		{candidate.ErrInvalidReview, 400}, {candidate.ErrNotFound, 404}, {candidate.ErrConflict, 409}, {candidate.ErrEvidenceChanged, 409},
		{sources.ErrUnauthorized, 422}, {errors.New("test_sensitive_database_details"), 500},
	} {
		w := httptest.NewRecorder()
		h.reviewError(w, tc.err)
		if w.Code != tc.code || strings.Contains(w.Body.String(), "test_sensitive") {
			t.Fatalf("错误映射有误: %d %s", w.Code, w.Body.String())
		}
	}
}
