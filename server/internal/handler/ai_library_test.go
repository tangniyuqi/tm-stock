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

	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type libraryHandlerFake struct {
	calls   int
	actor   int64
	body    []byte
	enabled bool
	err     error
}

func (f *libraryHandlerFake) RegisterSource(_ context.Context, actor int64, s sources.Source) (sources.Source, error) {
	f.calls++
	f.actor = actor
	s.ID = 1
	return s, f.err
}
func (f *libraryHandlerFake) SourceByID(context.Context, int64) (sources.Source, error) {
	f.calls++
	return sources.Source{ID: 1}, f.err
}
func (f *libraryHandlerFake) EnableSource(_ context.Context, actor, _ int64, enabled bool) error {
	f.calls++
	f.actor = actor
	f.enabled = enabled
	return f.err
}
func (f *libraryHandlerFake) ImportText(_ context.Context, actor, _ int64, _, _ string, body []byte) (sources.Snapshot, error) {
	f.calls++
	f.actor = actor
	f.body = body
	return sources.Snapshot{ID: 2, FetchedAt: time.UnixMilli(1700000000123)}, f.err
}
func (f *libraryHandlerFake) FetchSnapshot(_ context.Context, actor, _ int64, _ string) (sources.Snapshot, error) {
	f.calls++
	f.actor = actor
	return sources.Snapshot{ID: 2, FetchedAt: time.UnixMilli(1700000000123)}, f.err
}
func (f *libraryHandlerFake) SourceAudits(context.Context, int64, int64, int) ([]sources.SourceAudit, error) {
	f.calls++
	expires := time.UnixMilli(1700000000123).UTC()
	before := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport, LicenseExpires: &expires, MinInterval: time.Second, Enabled: true}
	after := before
	after.Enabled = false
	return []sources.SourceAudit{{ID: 5, ActorID: 7, Operation: "disable", Before: &before, After: after, CreatedAt: expires}}, f.err
}
func (f *libraryHandlerFake) SnapshotByID(context.Context, int64) (sources.Snapshot, error) {
	f.calls++
	return sources.Snapshot{ID: 2, FetchedAt: time.UnixMilli(1700000000123)}, f.err
}
func (f *libraryHandlerFake) Extract(_ context.Context, actor int64, r pipeline.RunRequest) (pipeline.RunRecord, error) {
	f.calls++
	f.actor = actor
	return pipeline.RunRecord{ID: 3, Request: r, State: "succeeded", CreatedAt: time.UnixMilli(1700000000123)}, f.err
}
func (f *libraryHandlerFake) RunByRequest(_ context.Context, actor int64, id string) (pipeline.RunRecord, error) {
	f.calls++
	f.actor = actor
	return pipeline.RunRecord{ID: 3, Request: pipeline.RunRequest{RequestID: id}, State: "running", CreatedAt: time.UnixMilli(1700000000123)}, f.err
}

func TestLibraryHandlerRoutesAndIdentity(t *testing.T) {
	fake := &libraryHandlerFake{}
	mux := http.NewServeMux()
	h := NewAILibraryHandler(fake, fake)
	h.Register(mux)
	credential := strings.Repeat("test_", 8)
	auth, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: credential}})
	if err != nil {
		t.Fatal(err)
	}
	server := auth(mux)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/internal/v1/ai-sources", `{"domain":"example.test","category":"annual_report","license_expires_at":1700000000123}`, 200},
		{"GET", "/internal/v1/ai-sources/1", "", 200}, {"POST", "/internal/v1/ai-sources/1/state", `{"enabled":true}`, 200},
		{"GET", "/internal/v1/ai-sources/1/audits?after_id=0&limit=10", "", 200},
		{"GET", "/internal/v1/ai-sources/1/audits?after_id=-1", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?after_id=abc", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?after_id=", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?limit=201", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?limit=abc", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?limit=10&limit=20", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?actor_id=9", "", 400},
		{"GET", "/internal/v1/ai-sources/1/audits?limit=1;ignored=2", "", 400},
		{"POST", "/internal/v1/ai-snapshot-imports", `{"source_id":1,"url":"https://example.test/report","media_type":"text/plain","body":"5rWL6K+V6LWE5paZ"}`, 200},
		{"POST", "/internal/v1/ai-snapshot-fetches", `{"source_id":1,"url":"https://example.test/report"}`, 200},
		{"GET", "/internal/v1/ai-snapshots/2", "", 200},
		{"POST", "/internal/v1/ai-extractions", `{"snapshot_id":2,"request_id":"test_request_00001"}`, 200},
		{"GET", "/internal/v1/ai-extractions/test_request_00001", "", 200},
		{"POST", "/internal/v1/ai-sources", `{"domain":"example.test","enabled":true}`, 400},
		{"POST", "/internal/v1/ai-sources", `{"domain":"example.test","actor_id":99}`, 400},
		{"POST", "/internal/v1/ai-snapshot-fetches", `{"source_id":1,"actor_id":99}`, 400},
		{"POST", "/internal/v1/ai-snapshot-imports", `{"source_id":1,"actor_id":99}`, 400},
		{"POST", "/internal/v1/ai-sources/1/state", `{"enabled":true,"actor_id":99}`, 400},
		{"POST", "/internal/v1/ai-sources", `{"license_expires_at":9223372036854775807}`, 400},
		{"POST", "/internal/v1/ai-sources/1/state", `{}`, 400}, {"POST", "/internal/v1/ai-sources/1/state", `{"enabled":true} {}`, 400},
		{"GET", "/internal/v1/ai-snapshots/abc", "", 400}, {"GET", "/internal/v1/ai-sources/0", "", 400},
		{"POST", "/internal/v1/ai-extractions", `{"snapshot_id":2,"request_id":"test_request_00001","actor_id":99}`, 400},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.RemoteAddr = "127.0.0.1:3000"
			r.Header.Set("Authorization", "Bearer "+credential)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("状态=%d 内容=%s", w.Code, w.Body.String())
			}
			if tc.want == 200 && tc.method == "POST" && fake.actor != 7 {
				t.Fatal("写操作身份未来自中间件")
			}
			if tc.want == 200 && strings.Contains(tc.path, "/audits") {
				var result struct {
					Data struct {
						Items []struct {
							CreatedAt int64 `json:"created_at"`
							Before    struct {
								LicenseExpiresAt int64 `json:"license_expires_at"`
							} `json:"before"`
						} `json:"items"`
						NextAfterID int64 `json:"next_after_id"`
					} `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || len(result.Data.Items) != 1 || result.Data.Items[0].CreatedAt != 1700000000123 || result.Data.Items[0].Before.LicenseExpiresAt != 1700000000123 || result.Data.NextAfterID != 5 {
					t.Fatal("审计时间/副本/游标错误")
				}
			}
			if tc.path == "/internal/v1/ai-snapshots/2" {
				var result struct {
					Data struct {
						FetchedAt int64 `json:"fetched_at"`
					} `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Data.FetchedAt != 1700000000123 {
					t.Fatal("时点不是毫秒")
				}
			}
		})
	}
	if fake.calls != 9 || fake.actor != 7 || string(fake.body) != "测试资料" || !fake.enabled {
		t.Fatalf("身份/拒绝路径/正文解码错误: 次数=%d 身份=%d 正文=%q", fake.calls, fake.actor, fake.body)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/internal/v1/ai-extractions", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatal("缺中间件仍可进入")
	}
	fake.err = pipeline.ErrDisabled
	r := httptest.NewRequest("POST", "/internal/v1/ai-extractions", strings.NewReader(`{}`))
	r.RemoteAddr = "127.0.0.1:3000"
	r.Header.Set("Authorization", "Bearer "+credential)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("模型关闭未拒绝")
	}
	for _, err := range []error{pipeline.ErrInvalidRun, pipeline.ErrRunNotFound, pipeline.ErrRunConflict, sources.ErrUnauthorized, sources.ErrBodyTooLarge, errors.New("test_sensitive_database_detail")} {
		w := httptest.NewRecorder()
		h.failure(w, err)
		if w.Code < 400 || strings.Contains(w.Body.String(), "test_sensitive") {
			t.Fatal("内部错误泄露")
		}
	}
}
