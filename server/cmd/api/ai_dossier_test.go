package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestLocalDossierDefaultClosed(t *testing.T) {
	for _, raw := range []string{"", "false", "true", "bad"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("TM_AI_LOCAL_DOSSIER_READ_ENABLED", raw)
			mux := http.NewServeMux()
			err := registerAILocalDossiers(context.Background(), mux, nil, config.AITaskReadConfig{}, nil, "")
			if (err == nil) != (raw == "" || raw == "false") {
				t.Fatal("默认关闭或非法配置行为错误")
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/themes/7/dossiers/1", nil))
			if w.Code != 404 {
				t.Fatal("未就绪仍注册路由")
			}
		})
	}
}
