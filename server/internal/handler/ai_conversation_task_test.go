package handler

import (
	"context"
	"errors"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type intentHTTPStub struct {
	calls int
	err   error
}

func (s *intentHTTPStub) Prepare(context.Context, int64, int64, string, conversation.IntentInput) (dto.AIConversationTask, error) {
	s.calls++
	return dto.AIConversationTask{ID: 1, State: conversation.Prepared}, s.err
}
func (s *intentHTTPStub) Confirm(context.Context, int64, int64, int64) (dto.AIConversationTask, error) {
	s.calls++
	return dto.AIConversationTask{ID: 1, State: conversation.Linked}, s.err
}
func (s *intentHTTPStub) Reconcile(context.Context, int64, int64, int64) (dto.AIConversationTask, error) {
	s.calls++
	return dto.AIConversationTask{ID: 1, State: conversation.Unconfirmed}, s.err
}
func (s *intentHTTPStub) Detail(context.Context, int64, int64, int64) (dto.AIConversationTask, error) {
	s.calls++
	return dto.AIConversationTask{ID: 1}, s.err
}
func (s *intentHTTPStub) Receipt(context.Context, int64, int64, string) (dto.AIConversationTask, error) {
	s.calls++
	return dto.AIConversationTask{ID: 1}, s.err
}
func (s *intentHTTPStub) List(context.Context, int64, int64, int64, int) (dto.AIConversationTasks, error) {
	s.calls++
	return dto.AIConversationTasks{Items: []dto.AIConversationTaskSummary{}}, s.err
}
func TestAIConversationTaskHTTPStrictInputs(t *testing.T) {
	st := &intentHTTPStub{}
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	mux := http.NewServeMux()
	NewAIConversationTaskHandler(st).Register(mux, auth, true)
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"POST", "/task-intents", `{"analysis_type":"light","subject_type":"theme","subject_id":1,"price_version":"v1"}`, 200},
		{"GET", "/task-intents", "", 200}, {"GET", "/task-intents?before_id=9&limit=1", "", 200}, {"GET", "/task-intent-receipt", "", 200}, {"GET", "/task-intents/1", "", 200}, {"POST", "/task-intents/1/confirm", `{"confirmed":true}`, 200}, {"POST", "/task-intents/1/reconcile", `{}`, 200},
		{"POST", "/task-intents/1/confirm", `{"confirmed":false}`, 400}, {"POST", "/task-intents/1/confirm", `{}`, 400}, {"POST", "/task-intents/1/confirm", `{"confirmed":true,"confirmed":true}`, 400}, {"POST", "/task-intents/1/confirm", `{"confirmed":null}`, 400}, {"POST", "/task-intents/1/reconcile", `null`, 400}, {"POST", "/task-intents/1/reconcile", `{"task_id":1}`, 400}, {"POST", "/task-intents/1/reconcile", `{} {}`, 400},
		{"GET", "/task-intents?limit=101", "", 400}, {"GET", "/task-intents?limit=0", "", 400}, {"GET", "/task-intents?before_id=0", "", 400}, {"GET", "/task-intents?limit=1&limit=2", "", 400}, {"GET", "/task-intents?user_id=42", "", 400}, {"GET", "/task-intents/01", "", 400}, {"GET", "/task-intents/1", `{}`, 400}, {"HEAD", "/task-intents/1", "", 400},
	} {
		r := httptest.NewRequest(tc.method, "/api/v1/ai/conversations/9"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer test-only")
		r.Header.Set("Idempotency-Key", "conversation_task_http")
		if tc.body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		before := st.calls
		mux.ServeHTTP(w, r)
		if w.Code != tc.code || tc.code == 400 && st.calls != before || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		err  error
		code int
	}{{conversation.ErrNotFound, 404}, {conversation.ErrConflict, 409}, {conversation.ErrUnconfirmed, 503}, {errors.New("测试私密键与SQL"), 503}} {
		st.err = tc.err
		r := httptest.NewRequest("GET", "/api/v1/ai/conversations/9/task-intents/1", nil)
		r.Header.Set("Authorization", "Bearer test-only")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.code || strings.Contains(w.Body.String(), "私密") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	st.err = nil
	closed := http.NewServeMux()
	NewAIConversationTaskHandler(st).Register(closed, auth, false)
	for _, tc := range []struct {
		path, body string
		code       int
	}{{"/task-intents", `{}`, 405}, {"/task-intents/1/confirm", `{"confirmed":true}`, 404}, {"/task-intents/1/reconcile", `{}`, 200}} {
		r := httptest.NewRequest("POST", "/api/v1/ai/conversations/9"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer test-only")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		closed.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
