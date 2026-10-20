package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestLibraryCLICommandsStrictInputAndAuthenticatedRoutes(t *testing.T) {
	credential := strings.Repeat("test_", 8)
	var mu sync.Mutex
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+credential {
			t.Error("缺凭据")
		}
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if err := json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"id": 1}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("TM_AI_ADMIN_URL", server.URL)
	t.Setenv("TM_AI_ADMIN_TOKEN", credential)
	for _, tc := range []struct{ command, input, path string }{
		{"source-register", `{"domain":"example.test","category":"annual_report"}`, "POST /internal/v1/ai-sources"},
		{"source-state", `{"source_id":1,"enabled":false}`, "POST /internal/v1/ai-sources/1/state"},
		{"source-show", `{"id":1}`, "GET /internal/v1/ai-sources/1"},
		{"source-audits", `{"source_id":1,"after_id":0,"limit":20}`, "GET /internal/v1/ai-sources/1/audits"},
		{"snapshot-import", `{"source_id":1,"body":"5rWL6K+V6LWE5paZ"}`, "POST /internal/v1/ai-snapshot-imports"},
		{"snapshot-fetch", `{"source_id":1,"url":"https://example.test/report"}`, "POST /internal/v1/ai-snapshot-fetches"},
		{"snapshot-show", `{"id":2}`, "GET /internal/v1/ai-snapshots/2"},
		{"extract", `{"snapshot_id":2,"request_id":"test_request_00001"}`, "POST /internal/v1/ai-extractions"},
		{"extract-show", `{"request_id":"test_request_00001"}`, "GET /internal/v1/ai-extractions/test_request_00001"},
		{"inspection-create", `{"request_id":"test_inspection_batch_0001","window_size":100,"sample_size":10}`, "POST /internal/v1/ai-inspection-batches"},
		{"inspection-show", `{"request_id":"test_inspection_batch_0001"}`, "GET /internal/v1/ai-inspection-batches/test_inspection_batch_0001"},
		{"inspection-item", `{"id":1}`, "GET /internal/v1/ai-inspection-items/1"},
		{"inspection-decide", `{"item_id":1,"evidence_digest":"` + strings.Repeat("0", 64) + `","decision":"issue","reason":"原文不足","request_id":"test_inspection_decision_0001"}`, "POST /internal/v1/ai-inspection-decisions"},
		{"inspection-issues", `{"after_id":0,"limit":20}`, "GET /internal/v1/ai-inspection-issues"},
	} {
		if err := run([]string{tc.command}, strings.NewReader(tc.input), io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		last := paths[len(paths)-1]
		mu.Unlock()
		if last != tc.path {
			t.Fatalf("路由=%s 期待=%s", last, tc.path)
		}
	}
	mu.Lock()
	before := len(paths)
	mu.Unlock()
	for _, tc := range []struct{ command, input string }{
		{"source-state", `{"source_id":1}`}, {"source-show", `{"id":0}`}, {"extract-show", `{"request_id":"../../bad"}`},
		{"source-audits", `{"source_id":0,"limit":20}`}, {"source-audits", `{"source_id":1,"after_id":-1,"limit":20}`},
		{"source-audits", `{"source_id":1,"limit":0}`}, {"source-audits", `{"source_id":1,"limit":201}`},
		{"source-audits", `{"source_id":1,"limit":20,"actor_id":99}`},
		{"extract", `{"snapshot_id":2,"request_id":"test_request_00001","model":"other"}`}, {"source-show", `{"id":1} {}`}, {"extract", `null`},
		{"inspection-create", `{"request_id":"test_inspection_batch_0001","window_size":10,"sample_size":11}`},
		{"inspection-decide", `{"item_id":1,"decision":"issue"}`}, {"inspection-issues", `{"limit":201}`},
	} {
		if err := run([]string{tc.command}, strings.NewReader(tc.input), io.Discard, io.Discard); err == nil {
			t.Fatal("错误输入仍发送请求")
		}
	}
	mu.Lock()
	after := len(paths)
	mu.Unlock()
	if after != before {
		t.Fatal("错误输入发送了请求")
	}
}
