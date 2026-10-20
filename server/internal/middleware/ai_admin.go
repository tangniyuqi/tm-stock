package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

type aiActorKey struct{}
type aiAdminIdentity struct {
	digest  [32]byte
	actorID int64
}

func AIAdminActor(ctx context.Context) int64 {
	actor, _ := ctx.Value(aiActorKey{}).(int64)
	return actor
}

// NewAIAdminAuth 只接受真实 TCP 对端的回环 IP，不信任转发头或会员 JWT。
func NewAIAdminAuth(reviewers []config.AIAdminReviewer) (func(http.Handler) http.Handler, error) {
	if err := config.ValidateAIAdminReviewers(reviewers); err != nil {
		return nil, err
	}
	identities := make([]aiAdminIdentity, 0, len(reviewers))
	for _, reviewer := range reviewers {
		identities = append(identities, aiAdminIdentity{sha256.Sum256([]byte(reviewer.Token)), reviewer.ActorID})
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			deny := func() {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write([]byte(`{"code":40100,"msg":"内部审核凭据无效"}`))
				if err != nil {
					return
				}
			}
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			ip := net.ParseIP(host)
			if err != nil || ip == nil || !ip.IsLoopback() || r.Header.Get("Origin") != "" || len(r.Header.Values("Authorization")) != 1 {
				deny()
				return
			}
			scheme, bearer, found := strings.Cut(r.Header.Get("Authorization"), " ")
			if !found || scheme != "Bearer" || len(bearer) < 32 || len(bearer) > 256 {
				deny()
				return
			}
			digest := sha256.Sum256([]byte(bearer))
			actorID := int64(0)
			for _, identity := range identities {
				if subtle.ConstantTimeCompare(digest[:], identity.digest[:]) == 1 {
					actorID = identity.actorID
				}
			}
			if actorID <= 0 {
				deny()
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), aiActorKey{}, actorID)))
		})
	}, nil
}
