//go:build integration && liveagents

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 迁移原真实/预完成装配，不让普通回归因继承凭据产生费用；每次只运行一个隔离级别。
func TestIntegrationAIReportDeepLiveAgents(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LIVE") != "local-fixture-only" || os.Getenv("TM_TEST_DSN") == "" || flag.Lookup("test.count").Value.String() != "1" || flag.Lookup("test.run").Value.String() != "^TestIntegrationAIReportDeepLiveAgents$" {
		t.Fatal("真实深度联调须显式隔离开关、专用数据库、精确单用例且 count=1")
	}
	if os.Getenv("TM_REAL_MODEL_KEY") == "" || (os.Getenv("TM_TA_TOKEN") == "" && os.Getenv("TM_TA_PRECOMPLETED") != "1") {
		t.Fatal("缺少安全注入的本地测试配置")
	}
	runDeepAdmissionExecution(t, []sql.IsolationLevel{sql.LevelReadCommitted}, func(t *testing.T, e *workflowEnv, cfg *deepWorkflowConfig) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
		t.Cleanup(cancel)
		e.pointsTestEnv.ctx = ctx
		name := os.Getenv("TM_REAL_MODEL_NAME")
		if name == "" {
			name = "deepseek-flash"
		}
		baseURL := os.Getenv("TM_REAL_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.deepseek.com"
		}
		model, err := provider.NewDirect(provider.Config{Enabled: true, Name: "real-model", BaseURL: baseURL, Model: name, APIKey: os.Getenv("TM_REAL_MODEL_KEY"), TimeoutMS: 240000, JSONSchemaFallback: os.Getenv("TM_REAL_NATIVE_SCHEMA") != "1", ReasoningEffort: os.Getenv("TM_REAL_REASONING_EFFORT")}, true)
		if err != nil {
			t.Fatal(err)
		}
		cfg.model, cfg.modelName = model, name
		cfg.inputTokens, cfg.selectionTokens, cfg.checkTokens, cfg.maxTokens = 24000, 8192, 1024, 200000
		cfg.maxDuration, cfg.leaseTimeout = 1200000, 20*time.Minute
		cc := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: "http://127.0.0.1:8000", ApprovedHost: "127.0.0.1", Bearer: os.Getenv("TM_TA_TOKEN"), ApprovalRef: "local:test-only", LicenseRef: "local:self-use", QuickModel: "qwen3.7-plus", DeepModel: "qwen3.7-plus", Timeout: 30 * time.Second, MaxSubmissions: 1, SelectedAnalysts: []string{"news"}, AnalysisDate: "2024-06-28"}
		if os.Getenv("TM_TA_PRECOMPLETED") == "1" {
			payload, err := os.ReadFile(os.Getenv("TM_TA_PAYLOAD"))
			if err != nil {
				t.Fatal("读取预完成报告失败")
			}
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal(payload, &envelope) == nil && len(envelope.Data) > 0 {
				payload = envelope.Data
			}
			remote := os.Getenv("TM_TA_RESULT_ID")
			if remote == "" {
				t.Fatal("预完成装配必须指定原任务编号，不冒充新 Agent 执行")
			}
			e.agent, err = service.NewAIAgentService(e.payloads, precompletedExecutor{remote: remote, payload: payload}, cc.Profile())
		} else {
			client, cerr := tradingcn.New(cc)
			if cerr != nil {
				t.Fatal(cerr)
			}
			t.Cleanup(client.Close)
			e.agent, err = service.NewAIAgentService(e.payloads, client, cc.Profile())
		}
		if err != nil {
			t.Fatal(err)
		}
		cfg.profileDigest, cfg.symbol = cc.Profile().Digest, "000001"
	})
}
