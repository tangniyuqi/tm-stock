package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type qaHandlerAuth struct{}

func (qaHandlerAuth) Authenticate(context.Context, string) (int64, error) { return 1, nil }

type qaHandlerSvc struct {
	calls int
	err   error
	last  dto.AIQAInput
}

func (s *qaHandlerSvc) Receipt(context.Context, string, int64, int64, string) (dto.AIQAStatus, error) {
	s.calls++
	return dto.AIQAStatus{Seq: 1, State: "queued"}, s.err
}
func (s *qaHandlerSvc) Status(context.Context, string, int64, int64, int64) (dto.AIQAStatus, error) {
	s.calls++
	return dto.AIQAStatus{Seq: 1, State: "queued"}, s.err
}
func (s *qaHandlerSvc) Answer(context.Context, string, int64, int64, int64) (dto.AIQAAnswer, error) {
	s.calls++
	return dto.AIQAAnswer{Seq: 1, State: "refused", Quotes: []dto.AIQAQuote{}}, s.err
}
func (s *qaHandlerSvc) Submit(_ context.Context, _ string, _, _ int64, _ string, in dto.AIQAInput) (dto.AIQAStatus, error) {
	s.calls++
	s.last = in
	return dto.AIQAStatus{Seq: 1, State: "queued"}, s.err
}
func qaHandlerFixture(t *testing.T, writes bool) (*http.ServeMux, *qaHandlerSvc) {
	t.Helper()
	auth, err := middleware.NewMemberAuth(qaHandlerAuth{})
	if err != nil {
		t.Fatal(err)
	}
	svc := &qaHandlerSvc{}
	mux := http.NewServeMux()
	NewAIQAHandler(svc).Register(mux, auth, writes)
	return mux, svc
}
func qaHandlerRequest(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer fake-member")
	r.Header.Set("Idempotency-Key", "test_question_key_01")
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}
func TestQAHandlerClosedProtocol(t *testing.T) {
	body := `{"version":1,"subjectType":"theme","subjectId":1,"skill":"material_qa","question":"测试问题"}`
	reportBody := `{"version":1,"subjectType":"theme","subjectId":1,"skill":"report_qa","taskId":9,"question":"测试报告问题"}`
	for _, c := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/v1/ai/conversations/1/questions", body, 200}, {"POST", "/api/v1/ai/conversations/1/questions", reportBody, 200}, {"GET", "/api/v1/ai/conversations/1/question-receipt", "", 200}, {"GET", "/api/v1/ai/conversations/1/questions/1", "", 200}, {"GET", "/api/v1/ai/conversations/1/questions/1/answer", "", 200},
		{"POST", "/api/v1/ai/conversations/1/questions", strings.Replace(body, `"version":1`, `"version":1,"Version":1`, 1), 400},
		{"POST", "/api/v1/ai/conversations/1/questions", strings.Replace(body, `"version":1`, `"version":null`, 1), 400},
		{"POST", "/api/v1/ai/conversations/1/questions", strings.Replace(body, `"version":1`, `"version":1,"user_id":1`, 1), 400},
		{"POST", "/api/v1/ai/conversations/1/questions", strings.Replace(reportBody, `"taskId":9`, `"taskId":null`, 1), 400},
		{"POST", "/api/v1/ai/conversations/1/questions", body + `{}`, 400}, {"POST", "/api/v1/ai/conversations/1/questions", strings.Repeat(" ", 8193) + body, 400},
		{"GET", "/api/v1/ai/conversations/01/questions/1", "", 400}, {"GET", "/api/v1/ai/conversations/1/questions/0", "", 400}, {"GET", "/api/v1/ai/conversations/1/questions/1?model=bad", "", 400}, {"HEAD", "/api/v1/ai/conversations/1/questions/1", "", 400},
	} {
		t.Run(c.method+c.path+c.body, func(t *testing.T) {
			mux, svc := qaHandlerFixture(t, true)
			w := qaHandlerRequest(mux, c.method, c.path, c.body)
			if w.Code != c.status || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Vary") != "Authorization" {
				t.Fatal("问答路由结果错误", w.Code)
			}
			if (svc.calls > 0) != (c.status == 200) {
				t.Fatal("非法请求到达服务")
			}
		})
	}
	mux, svc := qaHandlerFixture(t, false)
	if w := qaHandlerRequest(mux, "POST", "/api/v1/ai/conversations/1/questions", body); w.Code != 404 && w.Code != 405 || svc.calls != 0 {
		t.Fatal("关闭提交仍调用")
	}
	mux, svc = qaHandlerFixture(t, true)
	w := qaHandlerRequest(mux, "POST", "/api/v1/ai/conversations/1/questions", strings.Replace(body, "测试问题", strings.Repeat("示", 1000), 1))
	if w.Code != 200 || len(svc.last.Question) <= 2048 {
		t.Fatal("问答被旧收费正文限额截断")
	}
	for _, err := range []error{qa.ErrInvalid, qa.ErrNotFound, qa.ErrConflict, qa.ErrQuota, errors.New("test-only-private-diagnostic")} {
		svc.err = err
		w := qaHandlerRequest(mux, "GET", "/api/v1/ai/conversations/1/questions/1", "")
		if strings.Contains(w.Body.String(), "test-only-private-diagnostic") || w.Code == 200 {
			t.Fatal("内部错误泄漏/误成功")
		}
	}
}
