package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type memberAuthFunc func(context.Context, string) (int64, error)

func (f memberAuthFunc) Authenticate(ctx context.Context, s string) (int64, error) { return f(ctx, s) }
func TestMemberAuthTransportAndIdentity(t *testing.T) {
	if _, err := NewMemberAuth(nil); err == nil {
		t.Fatal("空服务")
	}
	for _, tc := range []struct {
		name    string
		headers []string
		want    int
		fault   error
		uid     int64
	}{
		{"正常", []string{"Bearer token"}, 200, nil, 42}, {"大小写方案", []string{"bearer token"}, 200, nil, 42},
		{"缺失", nil, 401, nil, 42}, {"重复", []string{"Bearer token", "Bearer token"}, 401, nil, 42}, {"逗号", []string{"Bearer token,Bearer token"}, 401, nil, 42},
		{"空令牌", []string{"Bearer "}, 401, nil, 42}, {"多空格", []string{"Bearer  token"}, 401, nil, 42}, {"其他方案", []string{"Basic token"}, 401, nil, 42},
		{"无效身份", []string{"Bearer token"}, 401, memberauth.ErrUnauthorized, 0}, {"零身份", []string{"Bearer token"}, 401, nil, 0},
		{"数据库故障", []string{"Bearer token"}, 503, memberauth.ErrUnavailable, 0}, {"未知异常", []string{"Bearer token"}, 503, errors.New("模拟敏感异常"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth, err := NewMemberAuth(memberAuthFunc(func(_ context.Context, token string) (int64, error) {
				if token != "token" {
					t.Fatal("传输令牌被改写")
				}
				return tc.uid, tc.fault
			}))
			if err != nil {
				t.Fatal(err)
			}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if MemberID(r.Context()) != 42 || AIAdminActor(r.Context()) != 0 {
					t.Fatal("上下文身份混用")
				}
				w.WriteHeader(200)
			})
			r := httptest.NewRequest("GET", "https://example.invalid/?token=token&user_id=999", nil)
			r.Header.Set("x-token", "token")
			r.AddCookie(&http.Cookie{Name: "token", Value: "token"})
			for _, h := range tc.headers {
				r.Header.Add("Authorization", h)
			}
			w := httptest.NewRecorder()
			auth(next).ServeHTTP(w, r)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	if MemberID(context.Background()) != 0 {
		t.Fatal("空上下文身份")
	}
}
