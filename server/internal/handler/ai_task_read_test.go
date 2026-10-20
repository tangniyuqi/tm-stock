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

type taskReaderStub struct {
	calls int
	err   error
}

func (s *taskReaderStub) History(_ context.Context, uid, before int64, limit int) (dto.AITaskHistory, error) {
	s.calls++
	if uid != 42 || before != 0 || limit != 20 {
		panic("身份或分页不匹配")
	}
	return dto.AITaskHistory{Items: []dto.AITaskSummary{}}, s.err
}
func (s *taskReaderStub) Progress(_ context.Context, uid, id int64) (dto.AITaskProgress, error) {
	s.calls++
	if uid != 42 || id != 9 {
		panic("身份或任务不匹配")
	}
	return dto.AITaskProgress{ID: id, Status: 1}, s.err
}

type taskMemberStub struct{}

func (s *taskReaderStub) Events(_ context.Context, uid, id, after int64, limit int) (dto.AITaskEventPage, error) {
	s.calls++
	if uid != 42 || id != 9 || after != 0 || limit != 20 {
		panic("事件身份或参数不匹配")
	}
	return dto.AITaskEventPage{Items: []dto.AITaskEvent{}, UpperSeq: 1}, s.err
}

func (taskMemberStub) Authenticate(context.Context, string) (int64, error) { return 42, nil }

func TestAITaskReadHTTPContract(t *testing.T) {
	st := &taskReaderStub{}
	auth, err := middleware.NewMemberAuth(taskMemberStub{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewAITaskReadHandler(st).Register(mux, auth)
	for _, tc := range []struct {
		method, url string
		want        int
		err         error
	}{
		{"GET", "/api/v1/ai/tasks", 200, nil}, {"GET", "/api/v1/ai/tasks/9", 200, nil},
		{"GET", "/api/v1/ai/tasks?user_id=5", 400, nil}, {"GET", "/api/v1/ai/tasks?limit=2&limit=3", 400, nil}, {"GET", "/api/v1/ai/tasks?limit=", 400, nil}, {"GET", "/api/v1/ai/tasks?limit=101", 400, nil}, {"GET", "/api/v1/ai/tasks?limit=-1", 400, nil}, {"GET", "/api/v1/ai/tasks?limit=x", 400, nil}, {"GET", "/api/v1/ai/tasks?before_id=0", 400, nil}, {"GET", "/api/v1/ai/tasks?before_id=x", 400, nil}, {"GET", "/api/v1/ai/tasks?x=%zz", 400, nil},
		{"GET", "/api/v1/ai/tasks/09", 400, nil}, {"GET", "/api/v1/ai/tasks/-1", 400, nil}, {"GET", "/api/v1/ai/tasks/9?user_id=42", 400, nil},
		{"GET", "/api/v1/ai/tasks/9", 404, points.ErrNotFound}, {"GET", "/api/v1/ai/tasks/9", 503, errors.New("模拟供应商敏感正文")}, {"GET", "/api/v1/ai/tasks", 503, points.ErrInvariant},
		{"POST", "/api/v1/ai/tasks", 405, nil}, {"POST", "/api/v1/ai/tasks/9/cancel", 404, nil}, {"GET", "/api/v1/ai/tasks/9/report", 404, nil},
	} {
		st.err = tc.err
		r := httptest.NewRequest(tc.method, tc.url, nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		before := st.calls
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(tc.url, w.Code, w.Body.String())
		}
		if tc.want == 400 && st.calls != before {
			t.Fatal("无效参数触达服务")
		}
		if strings.Contains(w.Body.String(), "敏感") {
			t.Fatal("泄漏异常")
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/ai/tasks", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	h := NewAITaskReadHandler(st)
	for _, f := range []http.HandlerFunc{h.history, h.progress} {
		w := httptest.NewRecorder()
		f(w, r)
		if w.Code != 401 {
			t.Fatal("绕过中间件放行")
		}
	}
}
