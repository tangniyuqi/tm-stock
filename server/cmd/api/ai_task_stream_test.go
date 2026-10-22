package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestAITaskStreamRegistrationClosed(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		mux := http.NewServeMux()
		err := registerAITaskStreams(context.Background(), mux, nil, config.AITaskReadConfig{}, enabled)
		if (err != nil) != enabled {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/tasks/9/stream", nil))
		if w.Code != 404 {
			t.Fatal("自检前注册入口")
		}
	}
}
