package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func TestReviewCLIExportApplyAndTamperRejection(t *testing.T) {
	credential := strings.Repeat("test_", 8)
	item := candidate.Candidate{ID: 1, SnapshotID: 2, ThemeID: 3, ThemeKey: "算力", StockID: 4, EntityKey: "星河科技", TsCode: "688001.SH", Predicate: "DISCLOSES_BUSINESS",
		SourceType: 2, SourceExcerpt: "星河科技主营算力服务。", SentenceIDs: []int{1}, QuoteSegments: []string{"星河科技主营算力服务。"}, SourceURL: "https://example.test/report",
		CollectedAt: time.UnixMilli(1700000000123), Provider: "test-provider", Model: "test-model", PromptVersion: "test-v1"}
	d, err := dto.NewAIReviewItem(item)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+credential {
			w.WriteHeader(401)
			return
		}
		var data any
		if r.Method == http.MethodGet {
			data = dto.AIReviewQueue{Items: []dto.AIReviewItem{d}, NextAfterID: 1}
		} else {
			var request candidate.ReviewRequest
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if err := request.Validate(); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			mu.Lock()
			postCount++
			mu.Unlock()
			data = dto.AIReviewEvent{ID: 1, RequestID: request.RequestID, CandidateID: request.CandidateID, Decision: request.Decision, ActorID: 7, MappingID: 100, CreatedAt: time.Now().UnixMilli()}
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("TM_AI_ADMIN_URL", server.URL)
	t.Setenv("TM_AI_ADMIN_TOKEN", credential)
	t.Setenv("TM_AI_REVIEW_AFTER_ID", "")
	var exported, status bytes.Buffer
	if err := run([]string{"export"}, nil, &exported, &status); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&exported).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows[1][2] = "approve"
	encode := func() string {
		var b bytes.Buffer
		writer := csv.NewWriter(&b)
		if err := writer.WriteAll(rows); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	input := encode()
	for i := 0; i < 2; i++ {
		if err := run([]string{"apply"}, strings.NewReader(input), io.Discard, &status); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	got := postCount
	mu.Unlock()
	if got != 2 {
		t.Fatalf("回传次数=%d", got)
	}
	rows[1][14] = "星河科技主营其他业务。"
	if err := run([]string{"apply"}, strings.NewReader(encode()), io.Discard, io.Discard); err == nil {
		t.Fatal("篡改 CSV 未被拒绝")
	}
	mu.Lock()
	got = postCount
	mu.Unlock()
	if got != 2 {
		t.Fatal("篡改 CSV 仍发送了审核请求")
	}
	if strings.Contains(status.String(), credential) {
		t.Fatal("状态输出泄露凭据")
	}
}

func TestReviewCLIRejectsRemoteURLAndRedirects(t *testing.T) {
	credential := strings.Repeat("test_", 8)
	for _, raw := range []string{"http://example.test:8081", "http://192.168.1.2:8081", "http://127.0.0.1:8081/path", "http://test_user:test_password@127.0.0.1:8081", "http://127.0.0.1:8081?token=test_value", "http://127.0.0.1:8081#fragment", "http://127.0.0.1:0"} {
		if _, err := newReviewClient(raw, credential); err == nil {
			t.Fatalf("不安全 URL 被接受: %q", raw)
		}
	}
	if _, err := newReviewClient("", "test_short"); err == nil {
		t.Fatal("短凭据未拒绝")
	}
	var mu sync.Mutex
	redirected := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); redirected++; mu.Unlock() }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer server.Close()
	client, err := newReviewClient(server.URL, credential)
	if err != nil {
		t.Fatal(err)
	}
	defer client.http.CloseIdleConnections()
	var out any
	if err := client.call(t.Context(), http.MethodGet, "/", nil, &out); err == nil {
		t.Fatal("重定向未拒绝")
	}
	mu.Lock()
	got := redirected
	mu.Unlock()
	if got != 0 {
		t.Fatal("凭据跟随重定向外送")
	}
}

func TestReviewCLIDoesNotEchoServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "test_sensitive_upstream_details")
	}))
	defer server.Close()
	client, err := newReviewClient(server.URL, strings.Repeat("test_", 8))
	if err != nil {
		t.Fatal(err)
	}
	defer client.http.CloseIdleConnections()
	var out any
	err = client.call(t.Context(), http.MethodPost, "/", map[string]string{"decision": "approve"}, &out)
	if err == nil || strings.Contains(err.Error(), "test_sensitive") {
		t.Fatalf("错误详情泄露: %v", err)
	}
	if err := run([]string{"unknown"}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("未知操作被接受")
	}
}
