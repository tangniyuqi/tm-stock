//go:build integration

package quant

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
)

// GVA的Casbin与安全配置是进程级单例；每次用独立测试子进程验证真实MySQL，
// 不重置产品单例，不让SQLite策略/连接污染MySQL验证，支持父测试-count重复。
func TestFeedbackConsoleMySQLLogin(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("未设置TM_TEST_DSN，反馈MySQL登录专项未执行")
	}
	if os.Getenv("TM_TEST_FEEDBACK_MYSQL_LOGIN_CHILD") == "1" {
		runFeedbackConsoleLogin(t, testutil.NewFeedbackMySQLDB(t))
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal("无法取得反馈MySQL测试程序路径")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestFeedbackConsoleMySQLLogin$", "-test.count=1", "-test.v")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "TM_TEST_FEEDBACK_MYSQL_LOGIN_CHILD=") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "TM_TEST_FEEDBACK_MYSQL_LOGIN_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 只输出已知测试文件的断言位置，不转发登录回包、SQL或凭据。
		positions := regexp.MustCompile(`(?m)^\s*((?:ai_feedback_console_login_test|feedback_mysql_integration)\.go:[0-9]+):`)
		for _, position := range positions.FindAllSubmatch(output, -1) {
			t.Logf("子进程失败断言：%s", position[1])
		}
		t.Fatal("反馈MySQL真实登录/授权子进程失败；未转发敏感输出")
	}
	passed := regexp.MustCompile(`(?m)^--- PASS[:][ ]TestFeedbackConsoleMySQLLogin `)
	if !passed.Match(output) || strings.Contains(string(output), "--- SKIP:") {
		t.Fatal("反馈MySQL登录子进程未实际通过")
	}
}
