//go:build integration && liveagents

package points_test

import (
	"database/sql"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// 专用HTTP进程实跑；浏览器由独立根组验收。
func TestIntegrationAIReportLocalAPIWorkerLive(t *testing.T) {
	depth, scenario := liveLocalProcessCase(t, "^TestIntegrationAIReportLocalAPIWorkerLive$")
	if os.Getenv("TM_TEST_AI_LIVE_PROCESS_H5") != "" {
		t.Fatal("HTTP根组不接受浏览器范围标记")
	}
	api, worker, login := buildLocalReportBinaries(t)
	kind := 1
	if depth == "deep" {
		kind = 2
	}
	verifyLocalAPIWorker(t, sql.LevelReadCommitted, kind, api, worker, login, scenario)
	t.Log("TM_STOCK_LIVE_CONVERSATION_VERIFIED")
	if depth == "deep" {
		t.Log("TM_STOCK_LIVE_DEEP_VERIFIED")
	}
}
func TestIntegrationAIReportLocalAPIWorkerLiveH5(t *testing.T) {
	depth, scenario := liveLocalProcessCase(t, "^TestIntegrationAIReportLocalAPIWorkerLiveH5$")
	if os.Getenv("TM_TEST_AI_LIVE_PROCESS_H5") != "local-fixture-only" {
		t.Fatal("须显式真实进程浏览器模式")
	}
	origin := os.Getenv("TM_TEST_AI_H5_ORIGIN")
	if _, err := h5TestOrigin(origin); err != nil {
		t.Fatal("缺少完整H5回环源")
	}
	runLocalProcessH5(t, depth, origin, &scenario)
	t.Log("TM_STOCK_LIVE_CONVERSATION_VERIFIED")
	if depth == "deep" {
		t.Log("TM_STOCK_LIVE_DEEP_VERIFIED")
	}
}
func liveLocalProcessCase(t *testing.T, expectedRun string) (string, localProcessCase) {
	t.Helper()
	if os.Getenv("TM_TEST_AI_LIVE") != "local-fixture-only" || os.Getenv("TM_TEST_AI_LIVE_PROCESS") != "local-fixture-only" || os.Getenv("TM_TEST_DSN") == "" || flag.Lookup("test.count").Value.String() != "1" || flag.Lookup("test.run").Value.String() != expectedRun {
		t.Fatal("真实进程仅允许显式隔离单任务精确入口")
	}
	depth := os.Getenv("TM_TEST_AI_H5_LOCAL_DEPTH")
	if depth != "light" && depth != "deep" {
		t.Fatal("必须明确轻深类型")
	}
	name := os.Getenv("TM_TEST_AI_REPORT_MODEL")
	if name != "deepseek-flash" && name != "qwen3.7-plus" {
		t.Fatal("模型不在本地评估白名单")
	}
	if _, err := h5TestOrigin(os.Getenv("TM_TEST_AI_AGENT_URL")); err != nil {
		t.Fatal("异步桥不是字面回环地址")
	}
	modelURL := os.Getenv("TM_TEST_AI_MODEL_URL")
	if !strings.HasSuffix(modelURL, "/v1") {
		t.Fatal("模型桥路径不符")
	}
	if _, err := h5TestOrigin(strings.TrimSuffix(modelURL, "/v1")); err != nil {
		t.Fatal("模型桥不是字面回环地址")
	}
	session, err := config.ParseAILocalModelSession(os.Getenv("TM_TEST_AI_MODEL_TOKEN"))
	if err != nil || session == "" {
		t.Fatal("缺少本轮临时模型会话")
	}
	for _, key := range []string{"TM_TEST_AI_H5_LOCAL_API_WORKER", "TM_TEST_AI_H5_DEEP", "TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_ASSISTANT", "TM_TEST_AI_H5_TIMEOUT_RECOVERY", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_REPORT_QA"} {
		if os.Getenv(key) != "" {
			t.Fatal("不混用浏览器替身专项")
		}
	}
	return depth, localProcessCase{external: true, configure: func(t *testing.T, f *localPublicationFixture, c *config.AILocalExecution) {
		c.Model = config.AILocalExecutionModel{Name: "local-eval-real-json-bridge-v1", BaseURL: modelURL, Model: name, TimeoutMS: 30000}
		c.Agent = config.AILocalExecutionAgent{BaseURL: os.Getenv("TM_TEST_AI_AGENT_URL"), ApprovalRef: "local-eval:user-approved", LicenseRef: "local-eval:community-51060a7", QuickModel: "deepseek-flash", DeepModel: "deepseek-flash", TimeoutMS: 3000, MaxSubmissions: 1}
		f.env.agentConfig = c.AgentConfig(os.Getenv("TM_TEST_AI_AGENT_TOKEN"))
		if f.env.agentConfig.Validate() != nil {
			t.Fatal("临时异步桥配置不符")
		}
		f.env.modelSession = session
		model, err := provider.NewDirect(c.ModelConfig(), true)
		if err != nil {
			t.Fatal("本地模型档案构造失败")
		}
		p := &f.spec
		p.AgentProfileDigest = f.env.agentConfig.Profile().Digest
		p.AgentTokens = 131072
		p.ModelName = name
		p.ModelProfileDigest = model.DirectProfileDigest()
		p.MaxInputTokens = 32768
		p.SelectionMaxTokens = 2048
		p.CheckMaxTokens = 512
		p.ModelVersion = "local-eval:real-v1"
		p.Budget.MaxTokens = 262144
		p.Budget.MaxDurationMS = 900000
		f.file.ExpiresAtMS = time.Now().Add(25 * time.Minute).UnixMilli()
		c.ExpiresAtMS = f.file.ExpiresAtMS
		c.WorkerTimeoutMS = 900000
		c.AgentPollMS = 1000
	}}
}
