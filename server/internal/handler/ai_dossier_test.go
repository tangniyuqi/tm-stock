package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type dossierReaderStub struct {
	calls int
	err   error
}

func (s *dossierReaderStub) ViewVersions(_ context.Context, token string, user, theme int64, versions []int64) (dto.AIDossierView, error) {
	s.calls++
	return dto.AIDossierView{ThemeID: theme, Notice: "仅测试私有原文", Versions: []dto.AIDossierVersion{}}, s.err
}
func TestDossierReadClosedProtocol(t *testing.T) {
	base := "/api/v1/ai/themes/7/dossiers/1"
	for _, c := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"单版", "GET", base, "", 200}, {"双版", "GET", base + "?compareVersion=2", "", 200},
		{"重复版本", "GET", base + "?compareVersion=2&compareVersion=3", "", 400},
		{"逆序", "GET", base + "?compareVersion=1", "", 400}, {"空值", "GET", base + "?compareVersion=", "", 400},
		{"未知参数", "GET", base + "?userId=2", "", 400}, {"前导零", "GET", "/api/v1/ai/themes/07/dossiers/1", "", 400},
		{"超界", "GET", base + "?compareVersion=9007199254740992", "", 400}, {"正文夹带", "GET", base, "{}", 400},
		{"HEAD", "HEAD", base, "", 400}, {"写入", "POST", base, "{}", 405},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &dossierReaderStub{}
			auth, err := middleware.NewMemberAuth(qaHandlerAuth{})
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			NewAIDossierHandler(s).Register(mux, auth)
			r := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			r.Header.Set("Authorization", "Bearer test-member")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != c.status || (s.calls == 1) != (c.status == 200) {
				t.Fatal("请求边界不符", w.Code, s.calls)
			}
			if c.method != "POST" && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("私有结果可缓存")
			}
		})
	}
}
func TestDossierReadErrorsHidePartialData(t *testing.T) {
	for _, err := range []error{dossier.ErrNotFound, errors.New("仅测试内部错误")} {
		s := &dossierReaderStub{err: err}
		auth, _ := middleware.NewMemberAuth(qaHandlerAuth{})
		mux := http.NewServeMux()
		NewAIDossierHandler(s).Register(mux, auth)
		r := httptest.NewRequest("GET", "/api/v1/ai/themes/7/dossiers/1?compareVersion=2", nil)
		r.Header.Set("Authorization", "Bearer test-member")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		want := 503
		if errors.Is(err, dossier.ErrNotFound) {
			want = 404
		}
		if w.Code != want || strings.Contains(w.Body.String(), "仅测试") || strings.Contains(w.Body.String(), "versions") {
			t.Fatal("拒读泄露部分对象")
		}
	}
}
