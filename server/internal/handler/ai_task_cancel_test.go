package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type taskCancelStub struct {
	calls int
	err   error
}

func (s *taskCancelStub) Cancel(_ context.Context, uid, id int64) (dto.AITaskCancellation, error) {
	s.calls++
	if uid != 42 || id != 9 {
		panic("取消身份/任务不匹配")
	}
	return dto.AITaskCancellation{ID: id, Status: 2, EventSeq: 3, CancelRequested: true}, s.err
}

func TestAITaskCancellationHTTPContract(t *testing.T) {
	stub := &taskCancelStub{}
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	mux := http.NewServeMux()
	NewAITaskCancelHandler(stub).Register(mux, auth)
	for _, tc := range []struct {
		method, path, body, media string
		want                      int
		err                       error
	}{
		{"POST", "/api/v1/ai/tasks/9/cancel", "", "", 200, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}", "application/json", 200, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", " { } \n", "application/json; charset=UTF-8", 200, nil},
		{"POST", "/api/v1/ai/tasks/09/cancel", "", "", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel?", "", "", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel?user_id=42", "", "", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}", "", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}", "text/plain", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}", "application/json; charset=gbk", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}", "application/json; other=x", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}", "application/json; charset=utf-8; other=x", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "null", "application/json", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "[]", "application/json", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{}{}", "application/json", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "{\"user_id\":42}", "application/json", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", strings.Repeat(" ", 65), "application/json", 400, nil},
		{"POST", "/api/v1/ai/tasks/9/cancel", "", "", 404, points.ErrNotFound},
		{"POST", "/api/v1/ai/tasks/9/cancel", "", "", 503, errors.New("fake-sensitive-marker")},
		{"GET", "/api/v1/ai/tasks/9/cancel", "", "", 405, nil},
	} {
		stub.err = tc.err
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer token")
		if tc.media != "" {
			r.Header.Set("Content-Type", tc.media)
		}
		w := httptest.NewRecorder()
		before := stub.calls
		mux.ServeHTTP(w, r)
		if w.Code != tc.want || strings.Contains(w.Body.String(), "fake-sensitive-marker") {
			t.Fatal(tc.path, w.Code, w.Body.String())
		}
		if tc.want == 400 && before != stub.calls {
			t.Fatal("非法载荷触达取消")
		}
		if w.Header().Get("Cache-Control") != "no-store" && tc.want != 405 {
			t.Fatal("取消响应可缓存")
		}
	}
	for _, headers := range []http.Header{{"Content-Encoding": {"gzip"}}, {"Content-Type": {"application/json", "application/json"}}} {
		r := httptest.NewRequest("POST", "/api/v1/ai/tasks/9/cancel", strings.NewReader("{}"))
		r.Header = headers
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/api/v1/ai/tasks/9/cancel", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("未认证取消")
	}
	w = httptest.NewRecorder()
	NewAITaskCancelHandler(stub).cancel(w, r)
	if w.Code != 401 {
		t.Fatal("绕过中间件")
	}
	mux = http.NewServeMux()
	NewAITaskCancelHandler(nil).Register(mux, auth)
	r.Header.Set("Authorization", "Bearer token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("空依赖取消")
	}
}
