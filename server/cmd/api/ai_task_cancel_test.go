package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

func TestAITaskCancelRegistrationFailsClosed(t *testing.T) {
	good := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, tc := range []struct {
		enabled bool
		read    config.AITaskReadConfig
		invalid bool
	}{{false, good, false}, {true, config.AITaskReadConfig{}, true}, {true, config.AITaskReadConfig{Enabled: true}, true}, {true, good, true}} {
		mux := http.NewServeMux()
		err := registerAITaskCancels(context.Background(), mux, nil, tc.read, tc.enabled)
		if (err != nil) != tc.invalid {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/ai/tasks/1/cancel", nil))
		if w.Code != 404 {
			t.Fatal("自检前注册写入口")
		}
	}
}
