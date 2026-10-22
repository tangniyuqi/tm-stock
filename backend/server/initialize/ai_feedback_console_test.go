package initialize

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestFeedbackConsoleRegistration(t *testing.T) {
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_ENABLED", "")
	g := gin.New()
	registerFeedbackConsole(g.Group(""))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest("GET", "/quant/ai-feedback", nil))
	if w.Code != 404 {
		t.Fatal("默认注册了反馈后台")
	}
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_ENABLED", "TRUE")
	defer func() {
		if recover() == nil {
			t.Fatal("坏开关未拒绝启动")
		}
	}()
	registerFeedbackConsole(g.Group(""))
}
