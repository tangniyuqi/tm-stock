package main

import (
	"context"
	"database/sql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQAStartupClosedAndNoImplicitFactories(t *testing.T) {
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	db, err := sql.Open("mysql", "test@tcp(127.0.0.1:1)/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, c := range []config.AIQAConfig{{}, {Read: true}, {Read: true, Submit: true}, {Submit: true}, {ReportSubmit: true}, {Read: true, ReportSubmit: true}, {Read: true, Submit: true, ReportSubmit: true}} {
		mux := http.NewServeMux()
		err := registerAIQA(context.Background(), mux, db, read, true, c)
		if (err != nil) != (c.Read || c.Submit || c.ReportSubmit) {
			t.Fatal("问答启动未拒绝缺失依赖", err)
		}
		for _, path := range []string{"/api/v1/ai/conversations/1/question-receipt", "/api/v1/ai/conversations/1/questions/1/answer"} {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 404 {
				t.Fatal("自检前注册问答")
			}
		}
	}
}
