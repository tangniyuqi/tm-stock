package handler

import (
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

type correctionHTTPStub struct {
	calls int
	err   error
}

func (s *correctionHTTPStub) Inspect(context.Context, int64, dossier.CorrectionPair) (dto.AIDossierCorrectionInspect, error) {
	s.calls++
	return dto.AIDossierCorrectionInspect{}, s.err
}
func (s *correctionHTTPStub) Apply(context.Context, int64, dossier.CorrectionRequest) (dto.AIDossierCorrectionReceipt, error) {
	s.calls++
	return dto.AIDossierCorrectionReceipt{}, s.err
}
func (s *correctionHTTPStub) Request(context.Context, int64, string) (dto.AIDossierCorrectionReceipt, error) {
	s.calls++
	return dto.AIDossierCorrectionReceipt{}, s.err
}
func (s *correctionHTTPStub) Read(context.Context, string, int64, dossier.CorrectionPair) (dto.AIDossierCorrections, error) {
	s.calls++
	return dto.AIDossierCorrections{}, s.err
}

type correctionHTTPMember struct{ token string }

func (a correctionHTTPMember) Authenticate(_ context.Context, token string) (int64, error) {
	if token != a.token {
		return 0, memberauth.ErrUnauthorized
	}
	return 1, nil
}
func TestDossierCorrectionHTTPBoundary(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(seed[:])
	adminAuth, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: key}})
	if err != nil {
		t.Fatal(err)
	}
	r := dossier.CorrectionRequest{RequestID: "test_correction_http_001", Object: dossier.CorrectionPair{ThemeID: 7, BeforeVersion: 1, AfterVersion: 2}, BeforeDigest: strings.Repeat("a", 64), AfterDigest: strings.Repeat("b", 64), Enabled: true, ValidUntil: 1793703600000, Declarations: []dossier.CorrectionDeclaration{{Before: dossier.CorrectionLocator{Key: "sample"}, After: dossier.CorrectionLocator{Key: "sample"}, StatementKey: "sample"}}, ReviewReference: "test-fixture:http"}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	const base = "/internal/v1/ai-dossier-corrections"
	const query = base + "?themeId=7&beforeVersion=1&afterVersion=2"
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(http.Header)
		want, calls              int
	}{
		{"inspect", "GET", query, "", nil, 200, 1}, {"apply", "POST", base, string(raw), nil, 200, 1}, {"request", "GET", base + "/requests/" + r.RequestID, "", nil, 200, 1}, {"anonymous", "GET", query, "", func(h http.Header) { h.Del("Authorization") }, 401, 0}, {"origin", "GET", query, "", func(h http.Header) { h.Set("Origin", "https://example.test") }, 401, 0}, {"empty_origin", "GET", query, "", func(h http.Header) { h["Origin"] = []string{""} }, 403, 0}, {"head", "HEAD", query, "", nil, 400, 0}, {"body", "GET", query, "{}", nil, 400, 0}, {"query_extra", "GET", query + "&actorId=8", "", nil, 400, 0}, {"query_duplicate", "GET", query + "&afterVersion=3", "", nil, 400, 0}, {"missing_version", "GET", base + "?themeId=7&beforeVersion=1", "", nil, 400, 0}, {"request_query", "GET", base + "/requests/" + r.RequestID + "?x=1", "", nil, 400, 0}, {"wrong_media", "POST", base, string(raw), func(h http.Header) { h.Set("Content-Type", "text/plain") }, 400, 0}, {"duplicate_media", "POST", base, string(raw), func(h http.Header) { h.Add("Content-Type", "application/json") }, 400, 0}, {"second_key", "POST", base, string(raw), func(h http.Header) { h.Set("Idempotency-Key", "another_request") }, 400, 0}, {"encoding", "POST", base, string(raw), func(h http.Header) { h.Set("Content-Encoding", "gzip") }, 400, 0}, {"oversize", "POST", base, strings.Repeat(" ", dossier.MaxCorrectionBytes+1), nil, 400, 0}, {"null_enabled", "POST", base, strings.Replace(string(raw), `"enabled":true`, `"enabled":null`, 1), nil, 400, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &correctionHTTPStub{}
			mux := http.NewServeMux()
			NewAIDossierCorrectionHandler(svc).RegisterInternal(mux)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.RemoteAddr = "127.0.0.1:12345"
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Content-Type", "application/json")
			if tc.edit != nil {
				tc.edit(req.Header)
			}
			w := httptest.NewRecorder()
			adminAuth(mux).ServeHTTP(w, req)
			if w.Code != tc.want || svc.calls != tc.calls || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("更正内部边界不符", w.Code, svc.calls)
			}
		})
	}
	for _, err := range []error{dossier.ErrInvalidManagement, dossier.ErrNotFound, dossier.ErrManagementConflict, dossier.ErrManagementEvidence, errors.New("private-diagnostic")} {
		t.Run("error_"+err.Error(), func(t *testing.T) {
			svc := &correctionHTTPStub{err: err}
			mux := http.NewServeMux()
			NewAIDossierCorrectionHandler(svc).RegisterInternal(mux)
			req := httptest.NewRequest("GET", query, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			req.Header.Set("Authorization", "Bearer "+key)
			w := httptest.NewRecorder()
			adminAuth(mux).ServeHTTP(w, req)
			if w.Code < 400 || strings.Contains(w.Body.String(), "private-diagnostic") {
				t.Fatal("错误被放行或透出诊断")
			}
		})
	}
	memberAuth, err := middleware.NewMemberAuth(correctionHTTPMember{key})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, token string
		want, calls       int
	}{{"member", "/api/v1/ai/themes/7/dossier-corrections?beforeVersion=1&afterVersion=2", key, 200, 1}, {"member_anonymous", "/api/v1/ai/themes/7/dossier-corrections?beforeVersion=1&afterVersion=2", "", 401, 0}, {"member_reverse", "/api/v1/ai/themes/7/dossier-corrections?beforeVersion=2&afterVersion=1", key, 400, 0}, {"member_extra", "/api/v1/ai/themes/7/dossier-corrections?beforeVersion=1&afterVersion=2&x=1", key, 400, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &correctionHTTPStub{}
			mux := http.NewServeMux()
			NewAIDossierCorrectionHandler(svc).RegisterPublic(mux, memberAuth)
			req := httptest.NewRequest("GET", tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.want || svc.calls != tc.calls {
				t.Fatal("更正会员边界不符", w.Code, svc.calls)
			}
		})
	}
}
