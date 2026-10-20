package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type submissionHandlerFake struct {
	calls int
	err   error
}

func (f *submissionHandlerFake) Quote(_ context.Context, uid int64, q dto.AITaskQuoteInput) (dto.AITaskQuote, error) {
	f.calls++
	if uid != 42 || q.SubjectID != 9 || q.AnalysisType != "light" || q.SubjectType != "theme" {
		return dto.AITaskQuote{}, points.ErrInvalid
	}
	return dto.AITaskQuote{SubjectID: 9, PricePoints: 7, MaterialScope: []dto.AITaskMaterialScope{}}, f.err
}
func (f *submissionHandlerFake) Create(_ context.Context, uid int64, key string, q dto.AITaskCreateInput) (dto.AITaskCreation, error) {
	f.calls++
	if uid != 42 || key != "test_request_key_01" || q.SubjectID != 9 || q.PriceVersion != "test-v1" {
		return dto.AITaskCreation{}, points.ErrInvalid
	}
	return dto.AITaskCreation{TaskID: 9, PricePoints: 7, PriceVersion: "test-v1"}, f.err
}
func (f *submissionHandlerFake) Receipt(_ context.Context, uid int64, key string) (dto.AITaskCreation, error) {
	f.calls++
	if uid != 42 || key != "test_request_key_01" {
		return dto.AITaskCreation{}, points.ErrInvalid
	}
	return dto.AITaskCreation{TaskID: 9, PricePoints: 7, PriceVersion: "test-v1"}, f.err
}
func submissionMux(f *submissionHandlerFake, writes bool) *http.ServeMux {
	mux := http.NewServeMux()
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	NewAITaskCreateHandler(f).Register(mux, auth, writes)
	return mux
}

const submissionQuoteBody = `{"analysis_type":"light","subject_type":"theme","subject_id":9}`
const submissionCreateBody = `{"analysis_type":"light","subject_type":"theme","subject_id":9,"price_version":"test-v1"}`

func submissionRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer token")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "test_request_key_01")
	return r
}

func TestAITaskSubmissionHTTPStrictBoundaryAndErrors(t *testing.T) {
	f := &submissionHandlerFake{}
	mux := submissionMux(f, true)
	for _, tc := range []struct {
		method, path, body string
		want               int
		err                error
	}{
		{"POST", "/api/v1/ai/tasks/quote", submissionQuoteBody, 200, nil},
		{"POST", "/api/v1/ai/tasks", submissionCreateBody, 200, nil},
		{"GET", "/api/v1/ai/task-receipt", "", 200, nil},
		{"POST", "/api/v1/ai/tasks", submissionCreateBody, 409, points.ErrIdempotency},
		{"POST", "/api/v1/ai/tasks", submissionCreateBody, 409, points.ErrPriceChanged},
		{"POST", "/api/v1/ai/tasks", submissionCreateBody, 422, points.ErrInsufficient},
		{"POST", "/api/v1/ai/tasks", submissionCreateBody, 422, task.ErrAdmissionUnavailable},
		{"POST", "/api/v1/ai/tasks", submissionCreateBody, 503, errors.New("test-only-private-marker")},
		{"GET", "/api/v1/ai/task-receipt", "", 404, points.ErrNotFound},
		{"GET", "/api/v1/ai/task-receipt", "", 503, task.ErrCreationDisabled},
		{"POST", "/api/v1/ai/tasks?", submissionCreateBody, 400, nil},
		{"POST", "/api/v1/ai/tasks?user_id=42", submissionCreateBody, 400, nil},
		{"GET", "/api/v1/ai/task-receipt?", "", 400, nil},
		{"GET", "/api/v1/ai/task-receipt?key=anything", "", 400, nil},
		{"GET", "/api/v1/ai/task-receipt", "{}", 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", "[]", 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", "null", 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", "", 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", submissionQuoteBody + "{}", 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", strings.Repeat(" ", 2049), 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", `{"analysis_type":"light","subject_type":"theme","subject_id":9,"subject_id":8}`, 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", `{"analysis_type":"light","subject_type":"theme","subject_id":9,"SUBJECT_ID":9}`, 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", `{"analysis_type":"light","subject_type":"theme","subject_id":null}`, 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", `{"analysis_type":"light","subject_type":"theme","subject_id":1.5}`, 400, nil},
		{"POST", "/api/v1/ai/tasks/quote", `{"analysis_type":"light","subject_type":"theme","subject_id":9,"price_points":0}`, 400, nil},
		{"POST", "/api/v1/ai/tasks", `{"analysis_type":"light","subject_type":"theme","subject_id":9,"price_version":"test-v1","model":"other"}`, 400, nil},
		{"POST", "/api/v1/ai/tasks", submissionQuoteBody, 400, nil},
		{"GET", "/api/v1/ai/tasks/quote", "", 405, nil},
		{"PUT", "/api/v1/ai/task-receipt", "", 405, nil},
	} {
		f.err = tc.err
		before := f.calls
		r := submissionRequest(tc.method, tc.path, tc.body)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want || strings.Contains(w.Body.String(), "private-marker") {
			t.Fatal(tc.path, w.Code, w.Body.String())
		}
		if tc.want == 400 && f.calls != before {
			t.Fatal("坏JSON/参数触达服务")
		}
		if tc.want != 405 && (w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Vary") != "Authorization") {
			t.Fatal("响应可缓存")
		}
		if tc.want != 200 && strings.Contains(w.Body.String(), "taskId") {
			t.Fatal("失败返回候选收据")
		}
	}
}

func TestAITaskSubmissionHeadersAndOffRoutes(t *testing.T) {
	f := &submissionHandlerFake{}
	mux := submissionMux(f, true)
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.Header.Del("Idempotency-Key") },
		func(r *http.Request) { r.Header.Set("Idempotency-Key", "bad") },
		func(r *http.Request) { r.Header.Add("Idempotency-Key", "test_request_key_02") },
		func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=gbk") },
		func(r *http.Request) { r.Header.Set("Content-Type", "application/json; other=bad") },
		func(r *http.Request) { r.Header.Add("Content-Type", "application/json") },
		func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") },
		func(r *http.Request) { r.Body = nil },
	} {
		r := submissionRequest("POST", "/api/v1/ai/tasks", submissionCreateBody)
		mutate(r)
		before := f.calls
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 400 || f.calls != before {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	r := submissionRequest("POST", "/api/v1/ai/tasks", submissionCreateBody)
	r.Header.Set("Content-Type", "application/json; charset=UTF-8")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, methodPath := range [][2]string{{"POST", "/api/v1/ai/tasks"}, {"POST", "/api/v1/ai/tasks/quote"}, {"GET", "/api/v1/ai/task-receipt"}} {
		r := submissionRequest(methodPath[0], methodPath[1], submissionCreateBody)
		r.Header.Del("Authorization")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 401 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("未登录或缓存边界失效", w.Code)
		}
	}
	mux = submissionMux(f, false)
	for _, path := range []string{"/api/v1/ai/tasks", "/api/v1/ai/tasks/quote"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, submissionRequest("POST", path, submissionCreateBody))
		if w.Code != 404 {
			t.Fatal("只读开放了写入")
		}
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, submissionRequest("GET", "/api/v1/ai/task-receipt", ""))
	if w.Code != 200 {
		t.Fatal("停止新建影响了只读核查")
	}
	for _, call := range []func(http.ResponseWriter, *http.Request){NewAITaskCreateHandler(f).quote, NewAITaskCreateHandler(f).create, NewAITaskCreateHandler(f).receipt} {
		w := httptest.NewRecorder()
		call(w, httptest.NewRequest("POST", "/", nil))
		if w.Code != 401 {
			t.Fatal("绕过中间件")
		}
	}
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	mux = http.NewServeMux()
	NewAITaskCreateHandler(nil).Register(mux, auth, true)
	for _, request := range []*http.Request{submissionRequest("POST", "/api/v1/ai/tasks", submissionCreateBody), submissionRequest("POST", "/api/v1/ai/tasks/quote", submissionQuoteBody), submissionRequest("GET", "/api/v1/ai/task-receipt", "")} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, request)
		if w.Code != 503 {
			t.Fatal("缺服务仍成功")
		}
	}
}
