package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type balanceStub struct {
	uid int64
	err error
}

func (s *balanceStub) Balance(_ context.Context, uid int64) (service.PointsBalance, error) {
	s.uid = uid
	return service.PointsBalance{Available: 13, Frozen: 7, CheckedAt: 1792281600000}, s.err
}
func TestPointsBalanceHTTP(t *testing.T) {
	for _, mode := range []string{"ok", "anonymous", "query", "empty_query", "body", "encoding", "error", "post", "head"} {
		t.Run(mode, func(t *testing.T) {
			s := &balanceStub{}
			auth, err := middleware.NewMemberAuth(taskMemberStub{})
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			NewPointsBalanceHandler(s).Register(mux, auth)
			url := "/api/v1/ai/points/balance"
			method := "GET"
			body := ""
			want := 200
			switch mode {
			case "anonymous":
				want = 401
			case "query":
				url += "?user_id=9"
				want = 400
			case "empty_query":
				url += "?"
				want = 400
			case "body":
				body = "{}"
				want = 400
			case "encoding":
				want = 400
			case "error":
				s.err = errors.New("内部数据库秘密")
				want = 503
			case "post":
				method = "POST"
				want = 405
			case "head":
				method = "HEAD"
				want = 400
			}
			r := httptest.NewRequest(method, url, strings.NewReader(body))
			if mode != "anonymous" {
				r.Header.Set("Authorization", "Bearer test-token")
			}
			if mode == "encoding" {
				r.Header.Set("Content-Encoding", "gzip")
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatal(w.Code, w.Body.String())
			}
			if mode == "ok" || mode == "error" {
				if s.uid != 42 {
					t.Fatal("未绑定鉴权身份", s.uid)
				}
			} else if s.uid != 0 {
				t.Fatal("无效请求进入余额服务")
			}
			if strings.Contains(w.Body.String(), "秘密") {
				t.Fatal("泄露内部错误")
			}
			if mode == "ok" && (!strings.Contains(w.Body.String(), `"availablePoints":13`) || !strings.Contains(w.Body.String(), `"frozenPoints":7`) || w.Header().Get("Cache-Control") != "no-store") {
				t.Fatal(w.Body.String())
			}
		})
	}
}
