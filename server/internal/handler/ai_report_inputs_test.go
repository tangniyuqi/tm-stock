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

type reportInputsReaderStub struct {
	reportReaderStub
	inputCalls int
}

func (s *reportInputsReaderStub) ReadInputs(_ context.Context, uid, id int64) (dto.AIReportInputs, error) {
	if uid != 42 || id != 9 {
		panic("输入记录身份错误")
	}
	s.inputCalls++
	return dto.AIReportInputs{TaskID: id, Scope: "synthetic-fixture-only"}, s.err
}
func TestAIReportInputsHTTPBoundary(t *testing.T) {
	s := &reportInputsReaderStub{}
	auth, err := middleware.NewMemberAuth(taskMemberStub{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewAIReportReadHandler(s).Register(mux, auth)
	for _, tc := range []struct {
		method, path, body, token string
		err                       error
		status                    int
	}{{"GET", "/api/v1/ai/tasks/9/report/inputs", "", "token", nil, 200}, {"GET", "/api/v1/ai/tasks/9/report/inputs", "", "", nil, 401}, {"GET", "/api/v1/ai/tasks/09/report/inputs", "", "token", nil, 400}, {"GET", "/api/v1/ai/tasks/9/report/inputs?", "", "token", nil, 400}, {"GET", "/api/v1/ai/tasks/9/report/inputs?user_id=42", "", "token", nil, 400}, {"GET", "/api/v1/ai/tasks/9/report/inputs", "{}", "token", nil, 400}, {"POST", "/api/v1/ai/tasks/9/report/inputs", "", "token", nil, 405}, {"GET", "/api/v1/ai/tasks/9/report/inputs", "", "token", points.ErrNotFound, 404}, {"GET", "/api/v1/ai/tasks/9/report/inputs", "", "token", finalreport.ErrUnavailable, 422}, {"GET", "/api/v1/ai/tasks/9/report/inputs", "", "token", errors.New("仅测试私有输入错误"), 503}} {
		s.err = tc.err
		before := s.inputCalls
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "私有输入") || tc.status == 400 && s.inputCalls != before {
			t.Fatal("报告输入HTTP边界不符", w.Code, tc.status)
		}
		if tc.status != 405 && w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("私有记录允许缓存")
		}
	}
}
