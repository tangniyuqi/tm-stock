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

func TestAIReportReadStartupIsExplicitAndClosed(t *testing.T) {
	good := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	if _, err := prepareAIReportReads(context.Background(), nil, good, true, nil, nil); err == nil {
		t.Fatal("仅身份配置放行报告")
	}
	r, err := prepareAIReportReads(context.Background(), nil, config.AITaskReadConfig{}, false, nil, nil)
	if err != nil || r != nil {
		t.Fatal(r, err)
	}
	if _, err := prepareAIReportReads(context.Background(), nil, config.AITaskReadConfig{Enabled: true}, true, nil, nil); err == nil {
		t.Fatal("缺真实发布证明仍装配")
	}
	mux := http.NewServeMux()
	if err := registerAIReportReads(mux, nil, config.AITaskReadConfig{}, nil); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/tasks/1/report", nil))
	if w.Code != 404 {
		t.Fatal("默认开放报告")
	}
}
