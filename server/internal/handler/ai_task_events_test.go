package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestAITaskEventsHTTPParametersAndErrors(t *testing.T) {
	st := &taskReaderStub{}
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	mux := http.NewServeMux()
	NewAITaskReadHandler(st).Register(mux, auth)
	for _, tc := range []struct {
		path    string
		want    int
		failure error
	}{
		{"/api/v1/ai/tasks/9/events", 200, nil}, {"/api/v1/ai/tasks/9/events?after_seq=0&limit=20", 200, nil},
		{"/api/v1/ai/tasks/09/events", 400, nil}, {"/api/v1/ai/tasks/9/events?after_seq=00", 400, nil}, {"/api/v1/ai/tasks/9/events?after_seq=-1", 400, nil}, {"/api/v1/ai/tasks/9/events?after_seq=", 400, nil}, {"/api/v1/ai/tasks/9/events?after_seq=0&after_seq=0", 400, nil}, {"/api/v1/ai/tasks/9/events?user_id=42", 400, nil}, {"/api/v1/ai/tasks/9/events?x=%zz", 400, nil}, {"/api/v1/ai/tasks/9/events?limit=0", 400, nil}, {"/api/v1/ai/tasks/9/events?limit=x", 400, nil}, {"/api/v1/ai/tasks/9/events?limit=101", 400, nil}, {"/api/v1/ai/tasks/9/events", 404, points.ErrNotFound}, {"/api/v1/ai/tasks/9/events", 503, points.ErrInvariant},
	} {
		st.err = tc.failure
		r := httptest.NewRequest("GET", tc.path, nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		before := st.calls
		mux.ServeHTTP(w, r)
		if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.want == 400 && st.calls != before {
			t.Fatal("非法查询触达服务")
		}
		if strings.Contains(w.Body.String(), points.ErrInvariant.Error()) {
			t.Fatal("原始错误泄漏")
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/events", nil)
	NewAITaskReadHandler(st).events(w, r)
	if w.Code != 401 {
		t.Fatal("绕过中间件")
	}
	mux = http.NewServeMux()
	NewAITaskReadHandler(nil).Register(mux, auth)
	r.Header.Set("Authorization", "Bearer token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("空服务放行")
	}
}
