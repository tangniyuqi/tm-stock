package main

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIFeedbackRegistrationFailsClosed(t *testing.T) {
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, enabled := range []bool{false, true} {
		mux := http.NewServeMux()
		err := registerAIFeedback(context.Background(), mux, nil, read, enabled)
		if (err != nil) != enabled {
			t.Fatal(err)
		}
		for _, method := range []string{"GET", "POST"} {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/ai/tasks/9/feedback", nil))
			if w.Code != 404 {
				t.Fatal("自检前注册反馈入口")
			}
		}
	}
}
