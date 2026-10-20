package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestInternalAIAuthBindsActorAndIgnoresForwardedHeaders(t *testing.T) {
	credential := strings.Repeat("test_", 8)
	auth, err := NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: credential}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAIAdminAuth(nil); err == nil {
		t.Fatal("空凭据未拒绝")
	}
	cases := []struct {
		name, remote, authorization, origin, forwarded string
		allowed                                        bool
	}{
		{"回环正例", "127.0.0.1:3000", "Bearer " + credential, "", "", true},
		{"IPv6正例", "[::1]:3000", "Bearer " + credential, "", "", true},
		{"转发头无提权", "203.0.113.1:3000", "Bearer " + credential, "", "127.0.0.1", false},
		{"无凭据", "127.0.0.1:3000", "", "", "", false},
		{"错凭据", "127.0.0.1:3000", "Bearer " + strings.Repeat("test_other", 4), "", "", false},
		{"会员令牌", "127.0.0.1:3000", "Bearer test_member_jwt", "", "", false},
		{"浏览器来源", "127.0.0.1:3000", "Bearer " + credential, "https://example.test", "", false},
		{"非法对端", "localhost:3000", "Bearer " + credential, "", "", false},
		{"凭据带空白", "127.0.0.1:3000", "Bearer " + credential + " ", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if AIAdminActor(r.Context()) != 7 {
					t.Fatal("身份未绑定到凭据")
				}
				w.WriteHeader(http.StatusOK)
			}))
			r := httptest.NewRequest(http.MethodGet, "/internal/v1/ai-review-candidates", nil)
			r.RemoteAddr = tc.remote
			if tc.authorization != "" {
				r.Header.Set("Authorization", tc.authorization)
			}
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if called != tc.allowed || (w.Code == http.StatusOK) != tc.allowed || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("鉴权结果错误: %d %v", w.Code, called)
			}
			if strings.Contains(w.Body.String(), credential) {
				t.Fatal("响应泄露凭据")
			}
		})
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:3000"
	r.Header.Add("Authorization", "Bearer "+credential)
	r.Header.Add("Authorization", "Bearer "+credential)
	w := httptest.NewRecorder()
	auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("重复 Authorization 被接受") })).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal(w.Code)
	}
}
