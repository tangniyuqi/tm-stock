package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

func TestAITaskReadRegistrationIsExplicitAndReadOnly(t *testing.T) {
	mux := http.NewServeMux()
	if err := registerAITaskReads(mux, nil, config.AITaskReadConfig{}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/tasks", nil))
	if w.Code != 404 {
		t.Fatal("默认开放路由")
	}
	if err := registerAITaskReads(mux, nil, config.AITaskReadConfig{Enabled: true}); err == nil {
		t.Fatal("弱配置开放")
	}
	c := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	if err := registerAITaskReads(mux, nil, c); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/api/v1/ai/tasks", "/api/v1/ai/tasks/1", "/api/v1/ai/tasks/1/events"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		if w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/ai/tasks", nil))
	if w.Code != 405 {
		t.Fatal("出现提交入口")
	}
}
