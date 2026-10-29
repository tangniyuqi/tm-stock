package main

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPointsBalanceRegistrationClosed(t *testing.T) {
	mux := http.NewServeMux()
	if err := registerPointsBalance(context.Background(), mux, nil, config.AITaskReadConfig{}, false); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/points/balance", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	if err := registerPointsBalance(context.Background(), mux, nil, config.AITaskReadConfig{}, true); err == nil {
		t.Fatal("无数据库或鉴权配置仍开放")
	}
}
