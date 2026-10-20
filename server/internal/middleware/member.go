package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type memberKey struct{}

func MemberID(ctx context.Context) int64 { id, _ := ctx.Value(memberKey{}).(int64); return id }

type memberAuthenticator interface {
	Authenticate(context.Context, string) (int64, error)
}

// NewMemberAuth 只接受单个 Authorization Bearer，不兜底 Cookie、查询参数或 x-token。
func NewMemberAuth(svc memberAuthenticator) (func(http.Handler) http.Handler, error) {
	if svc == nil {
		return nil, memberauth.ErrInvalidConfig
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			deny := func(unavailable bool) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				status, body := http.StatusUnauthorized, `{"code":40100,"msg":"会员身份无效，请重新登录"}`
				if unavailable {
					status, body = http.StatusServiceUnavailable, `{"code":50301,"msg":"会员身份校验暂不可用"}`
				} else {
					w.Header().Set("WWW-Authenticate", "Bearer")
				}
				w.WriteHeader(status)
				// 客户端断连后没有可发送的第二条响应，不记录令牌或请求 URL。
				if _, err := w.Write([]byte(body)); err != nil {
					return
				}
			}
			values := r.Header.Values("Authorization")
			if len(values) != 1 {
				deny(false)
				return
			}
			scheme, token, ok := strings.Cut(values[0], " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || len(token) == 0 || len(token) > 8192 || strings.ContainsAny(token, " \t\r\n,") {
				deny(false)
				return
			}
			uid, err := svc.Authenticate(r.Context(), token)
			if err != nil || uid <= 0 {
				deny(err != nil && !errors.Is(err, memberauth.ErrUnauthorized))
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberKey{}, uid)))
		})
	}, nil
}
