//go:build integration && browser

package points_test

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// 网站API/worker先交付合成报告，完整后台原登录/菜单/Casbin再处理同库反馈。
func TestIntegrationAIReportLocalFeedbackConsoleH5(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LOCAL_FEEDBACK_CONSOLE") != "local-fixture-only" || os.Getenv("TM_TEST_AI_LIVE") != "" || os.Getenv("TM_TEST_DSN") == "" || flag.Lookup("test.count").Value.String() != "1" || flag.Lookup("test.run").Value.String() != "^TestIntegrationAIReportLocalFeedbackConsoleH5$" {
		t.Fatal("仅允许显式单次本地反馈后台浏览器专项")
	}
	u, err := h5TestOrigin(os.Getenv("TM_TEST_FEEDBACK_UI_ORIGIN"))
	if err != nil || u.Hostname() != "127.0.0.1" || u.Path != "" {
		t.Fatal("需要完整后台的字面回环源")
	}
	api, worker, login := buildLocalReportBinaries(t)
	admin := buildLocalFeedbackAdmin(t)
	runner := filepath.Join(t.TempDir(), "feedback-console.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "test", "-race", "-c", "-tags=integration", "-o", runner, "./api/v1/quant")
	build.Dir = filepath.Join("..", "..", "..", "backend", "server")
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("构建原后台登录/Casbin测试进程失败")
	}
	verifyLocalAPIWorker(t, sql.LevelReadCommitted, 1, api, worker, login, localProcessCase{feedbackAdmin: admin, feedbackBrowserRunner: runner})
}
