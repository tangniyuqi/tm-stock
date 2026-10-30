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

func TestLocalConversationDossierClosed(t *testing.T) {
	for _, c := range []struct {
		name, read, saved, conversations string
		bad                              bool
	}{
		{"默认关闭", "", "", "", false}, {"显式关闭", "false", "false", "", false},
		{"缺档案读取", "false", "true", "true", true}, {"缺会话", "true", "true", "false", true},
		{"缺身份配置", "true", "true", "true", true}, {"非法开关", "", "bad", "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TM_AI_LOCAL_DOSSIER_READ_ENABLED", c.read)
			t.Setenv("TM_AI_LOCAL_CONVERSATION_DOSSIER_ENABLED", c.saved)
			t.Setenv("TM_AI_CONVERSATIONS_ENABLED", c.conversations)
			mux := http.NewServeMux()
			err := registerAILocalDossiers(context.Background(), mux, nil, config.AITaskReadConfig{}, nil, "")
			if (err != nil) != c.bad {
				t.Fatal("引用关闭/装配边界不符", err)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/conversations/3/dossiers", nil))
			if w.Code != 404 {
				t.Fatal("未就绪仍注册引用路由")
			}
		})
	}
}
