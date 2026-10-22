package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type feedbackHTTPStub struct {
	calls int
	err   error
}

func (s *feedbackHTTPStub) Submit(_ context.Context, u, id int64, c string) (dto.AIFeedback, error) {
	s.calls++
	if u != 42 || id != 9 || c != "fact" {
		panic("受理参数不匹配")
	}
	return dto.AIFeedback{ID: 1, TaskID: id, Category: c, Status: "received", Version: 1}, s.err
}
func (s *feedbackHTTPStub) ForReport(_ context.Context, u, id int64) (dto.AIFeedbackList, error) {
	s.calls++
	if u != 42 || id != 9 {
		panic("查询归属不匹配")
	}
	return dto.AIFeedbackList{TaskID: id, Items: []dto.AIFeedback{}}, s.err
}
func (s *feedbackHTTPStub) List(_ context.Context, a, after int64, l int) (dto.AIFeedbackPage, error) {
	s.calls++
	if a != 7 || after != 0 || l != 20 {
		panic("内部身份或分页不匹配")
	}
	return dto.AIFeedbackPage{Items: []dto.AIFeedback{}}, s.err
}
func (s *feedbackHTTPStub) Detail(_ context.Context, a, id int64) (dto.AIInternalFeedback, error) {
	s.calls++
	if a != 7 || id != 1 {
		panic("内部详情参数不匹配")
	}
	return dto.AIInternalFeedback{}, s.err
}
func (s *feedbackHTTPStub) Decide(_ context.Context, a, id int64, d feedback.Decision) (dto.AIFeedback, error) {
	s.calls++
	if a != 7 || id != 1 || d.ExpectedVersion != 1 || d.Status != "processing" {
		panic("内部处理参数不匹配")
	}
	return dto.AIFeedback{ID: id, Status: d.Status, Version: 2}, s.err
}

func TestAIFeedbackHTTPBoundaries(t *testing.T) {
	s := &feedbackHTTPStub{}
	h := NewAIFeedbackHandler(s)
	ma, err := middleware.NewMemberAuth(taskMemberStub{})
	if err != nil {
		t.Fatal(err)
	}
	member := http.NewServeMux()
	h.RegisterMember(member, ma)
	credential := strings.Repeat("f", 40)
	aa, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: credential}})
	if err != nil {
		t.Fatal(err)
	}
	internal := http.NewServeMux()
	h.RegisterInternal(internal)
	admin := aa(internal)
	for _, tc := range []struct {
		method, path, body string
		want               int
		internal           bool
	}{
		{"GET", "/api/v1/ai/tasks/9/feedback", "", 200, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `{"category":"fact"}`, 200, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `{"category":"fact","category":"source"}`, 400, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `{"Category":"fact"}`, 400, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `{"category":"fact","userId":42}`, 400, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `{"category":null}`, 400, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `{"category":" advice"}`, 400, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", `[]`, 400, false},
		{"POST", "/api/v1/ai/tasks/9/feedback", strings.Repeat(" ", 2050) + `{}`, 400, false},
		{"GET", "/api/v1/ai/tasks/09/feedback", "", 400, false},
		{"GET", "/api/v1/ai/tasks/9007199254740992/feedback", "", 400, false},
		{"GET", "/api/v1/ai/tasks/9/feedback?", "", 400, false},
		{"GET", "/api/v1/ai/tasks/9/feedback?userId=42", "", 400, false},
		{"GET", "/api/v1/ai/tasks/9/feedback", "x", 400, false},
		{"GET", "/internal/v1/ai-feedback", "", 404, false},
		{"POST", "/internal/v1/ai-feedback/1/decisions", `{}`, 404, false},
		{"GET", "/internal/v1/ai-feedback", "", 200, true},
		{"GET", "/internal/v1/ai-feedback/1", "", 200, true},
		{"POST", "/internal/v1/ai-feedback/1/decisions", `{"expectedVersion":1,"status":"processing","resolution":""}`, 200, true},
		{"POST", "/internal/v1/ai-feedback/1/decisions", `{"expectedVersion":1,"status":"resolved","resolution":"reviewed"}`, 400, true},
		{"POST", "/internal/v1/ai-feedback/1/decisions", `{"expectedVersion":1,"status":"processing","resolution":"","actorId":7}`, 400, true},
		{"GET", "/internal/v1/ai-feedback?limit=101", "", 400, true},
		{"GET", "/internal/v1/ai-feedback?after_id=0", "", 400, true},
		{"GET", "/internal/v1/ai-feedback?limit=20&limit=20", "", 400, true},
		{"GET", "/internal/v1/ai-feedback/1?limit=20", "", 400, true},
		{"GET", "/api/v1/ai/tasks/9/feedback", "", 404, true},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.RemoteAddr = "127.0.0.1:2345"
		r.Header.Set("Content-Type", "application/json")
		target := http.Handler(member)
		token := "token"
		if tc.internal {
			target = admin
			token = credential
		}
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		before := s.calls
		target.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.want != 200 && before != s.calls {
			t.Fatal("拒绝前触达服务")
		}
		if tc.want == 200 && w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("可缓存反馈")
		}
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{feedback.ErrInvalid, 400}, {feedback.ErrNotFound, 404}, {feedback.ErrNoReport, 422}, {feedback.ErrConflict, 409}, {feedback.ErrInvariant, 503}, {errors.New("敏感正文与密钥"), 503}} {
		s.err = tc.err
		r := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/feedback", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		member.ServeHTTP(w, r)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "敏感") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	s.err = nil
	for _, fn := range []http.HandlerFunc{h.member, h.internal} {
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 401 {
			t.Fatal("裸处理器越权")
		}
	}
	for _, remote := range []string{"192.0.2.1:1234", ""} {
		r := httptest.NewRequest("GET", "/internal/v1/ai-feedback", nil)
		r.RemoteAddr = remote
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		admin.ServeHTTP(w, r)
		if w.Code == 200 {
			t.Fatal("非回环处理放行")
		}
	}
	for _, encoding := range []string{"gzip", "identity"} {
		r := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/feedback", nil)
		r.Header.Set("Authorization", "Bearer token")
		r.Header.Set("Content-Encoding", encoding)
		w := httptest.NewRecorder()
		member.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("编码请求未拒绝")
		}
	}
}
