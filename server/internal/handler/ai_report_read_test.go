package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type reportReaderStub struct {
	calls int
	err   error
}

func (s *reportReaderStub) Read(_ context.Context, uid, id int64) (dto.AIReport, error) {
	s.calls++
	if uid != 42 || id != 9 {
		panic("身份错配")
	}
	return dto.AIReport{TaskID: id, SubjectName: "示例题材"}, s.err
}
func TestAIReportReadHTTPGatesAndContract(t *testing.T) {
	st := &reportReaderStub{}
	auth, err := middleware.NewMemberAuth(taskMemberStub{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h := NewAIReportReadHandler(st)
	h.Register(mux, auth)
	for _, tc := range []struct {
		method, path, body string
		err                error
		status             int
	}{
		{"GET", "/api/v1/ai/tasks/9/report", "", nil, 200},
		{"GET", "/api/v1/ai/tasks/09/report", "", nil, 400},
		{"GET", "/api/v1/ai/tasks/9/report?user_id=42", "", nil, 400},
		{"GET", "/api/v1/ai/tasks/9/report?", "", nil, 400},
		{"GET", "/api/v1/ai/tasks/9/report", "{}", nil, 400},
		{"POST", "/api/v1/ai/tasks/9/report", "", nil, 405},
		{"GET", "/api/v1/ai/tasks/9/report", "", points.ErrNotFound, 404},
		{"GET", "/api/v1/ai/tasks/9/report", "", finalreport.ErrUnavailable, 422},
		{"GET", "/api/v1/ai/tasks/9/report", "", finalreport.ErrDisabled, 503},
		{"GET", "/api/v1/ai/tasks/9/report", "", errors.New("test-only:敏感供应商正文"), 503},
	} {
		st.err = tc.err
		before := st.calls
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(tc.path, w.Code, w.Body.String())
		}
		if tc.status == 400 && st.calls != before {
			t.Fatal("非法输入访问服务")
		}
		if strings.Contains(w.Body.String(), "敏感") {
			t.Fatal("异常泄漏")
		}
		if tc.status != 405 && w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("报告响应可缓存")
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/report", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("无身份读取")
	}
	w = httptest.NewRecorder()
	h.read(w, r)
	if w.Code != 401 {
		t.Fatal("绕过中间件")
	}
	h.svc = nil
	r.Header.Set("Authorization", "Bearer token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("空依赖")
	}
}
