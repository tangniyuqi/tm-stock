package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type dossierManagementHTTPStub struct {
	calls int
	actor int64
	err   error
}

func (s *dossierManagementHTTPStub) Inspect(_ context.Context, a int64, k dossier.ManagementObject) (dto.AIDossierManagementInspect, error) {
	s.calls++
	s.actor = a
	return dto.AIDossierManagementInspect{Object: k}, s.err
}
func (s *dossierManagementHTTPStub) Apply(_ context.Context, a int64, r dossier.ManagementRequest) (dto.AIDossierManagementReceipt, error) {
	s.calls++
	s.actor = a
	return dto.AIDossierManagementReceipt{RequestID: r.RequestID, Object: r.Object, ActorID: a, Revision: r.ExpectedRevision + 1, Enabled: r.Enabled, RecordedAt: 1793700000000}, s.err
}
func (s *dossierManagementHTTPStub) Request(_ context.Context, a int64, id string) (dto.AIDossierManagementReceipt, error) {
	s.calls++
	s.actor = a
	return dto.AIDossierManagementReceipt{RequestID: id}, s.err
}
func TestDossierManagementHTTPBoundary(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(seed[:])
	request := dossier.ManagementRequest{RequestID: "test_management_http_001", Object: dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: 1}, VersionDigest: strings.Repeat("a", 64), Enabled: true, ValidUntil: 1793703600000, ReviewReference: "test-fixture:http"}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	query := "/internal/v1/ai-dossier-management?kind=publication&userId=0&themeId=7&version=1"
	for _, tc := range []struct {
		name, method, path, body string
		header                   func(http.Header)
		want, calls              int
	}{
		{"inspect", "GET", query, "", nil, 200, 1},
		{"apply", "POST", "/internal/v1/ai-dossier-management", string(raw), nil, 200, 1},
		{"request", "GET", "/internal/v1/ai-dossier-management/requests/test_management_http_001", "", nil, 200, 1},
		{"anonymous", "GET", query, "", func(h http.Header) { h.Del("Authorization") }, 401, 0},
		{"member", "GET", query, "", func(h http.Header) { h.Set("Authorization", "Bearer member-jwt") }, 401, 0},
		{"origin", "GET", query, "", func(h http.Header) { h.Set("Origin", "https://example.test") }, 401, 0},
		{"empty_origin", "GET", query, "", func(h http.Header) { h["Origin"] = []string{""} }, 403, 0},
		{"head", "HEAD", query, "", nil, 400, 0},
		{"get_body", "GET", query, "{}", nil, 400, 0},
		{"query_extra", "GET", query + "&actorId=8", "", nil, 400, 0},
		{"query_duplicate", "GET", query + "&version=1", "", nil, 400, 0},
		{"query_missing", "GET", "/internal/v1/ai-dossier-management?kind=publication&themeId=7&version=1", "", nil, 400, 0},
		{"request_query", "GET", "/internal/v1/ai-dossier-management/requests/test_management_http_001?x=1", "", nil, 400, 0},
		{"content_type", "POST", "/internal/v1/ai-dossier-management", string(raw), func(h http.Header) { h.Set("Content-Type", "text/plain") }, 400, 0},
		{"duplicate_type", "POST", "/internal/v1/ai-dossier-management", string(raw), func(h http.Header) { h.Add("Content-Type", "application/json") }, 400, 0},
		{"encoding", "POST", "/internal/v1/ai-dossier-management", string(raw), func(h http.Header) { h.Set("Content-Encoding", "gzip") }, 400, 0},
		{"second_key", "POST", "/internal/v1/ai-dossier-management", string(raw), func(h http.Header) { h.Set("Idempotency-Key", "another_request") }, 400, 0},
		{"actor_spoof", "POST", "/internal/v1/ai-dossier-management", strings.TrimSuffix(string(raw), "}") + ",\"actorId\":8}", nil, 400, 0},
		{"null_enabled", "POST", "/internal/v1/ai-dossier-management", strings.Replace(string(raw), "\"enabled\":true", "\"enabled\":null", 1), nil, 400, 0},
		{"oversized", "POST", "/internal/v1/ai-dossier-management", strings.Repeat(" ", 8193), nil, 400, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &dossierManagementHTTPStub{}
			mux := http.NewServeMux()
			NewAIDossierManagementHandler(stub).Register(mux)
			auth, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: key}})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Content-Type", "application/json")
			if tc.header != nil {
				tc.header(req.Header)
			}
			recorder := httptest.NewRecorder()
			auth(mux).ServeHTTP(recorder, req)
			if recorder.Code != tc.want || stub.calls != tc.calls || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("管理HTTP边界不符", recorder.Code, tc.want, stub.calls, tc.calls)
			}
			if stub.calls > 0 && stub.actor != 7 {
				t.Fatal("内部身份未由凭据绑定")
			}
			if strings.Contains(recorder.Body.String(), key) {
				t.Fatal("响应回显凭据")
			}
		})
	}
	t.Run("closed_failures", func(t *testing.T) {
		for _, tc := range []struct {
			err    error
			status int
		}{{memberauth.ErrUnauthorized, 403}, {dossier.ErrInvalidManagement, 400}, {dossier.ErrNotFound, 404}, {dossier.ErrManagementConflict, 409}, {dossier.ErrConflict, 409}, {errors.New("仅测试私有诊断"), 503}} {
			w := httptest.NewRecorder()
			NewAIDossierManagementHandler(nil).failure(w, tc.err)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "私有") {
				t.Fatal("内部失败泄漏或状态错误")
			}
		}
	})
}
