package main

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIConversationStartupClosed(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		mux := http.NewServeMux()
		err := registerAIConversations(context.Background(), mux, nil, config.AITaskReadConfig{}, enabled)
		if (err != nil) != enabled {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/conversations", nil))
		if w.Code != 404 {
			t.Fatal("自检前注册")
		}
	}
}
