//go:build integration

package agents_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
)

func TestIntegrationTradingAgentsReal(t *testing.T) {
	base, token := os.Getenv("TM_TA_URL"), os.Getenv("TM_TA_TOKEN")
	if base == "" || token == "" {
		t.Skip("缺少 TM_TA_URL/TM_TA_TOKEN")
	}
	cfg := tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: base, ApprovedHost: "127.0.0.1", Bearer: token,
		ApprovalRef: "local:test-only", LicenseRef: "local:self-use", QuickModel: "qwen3.7-plus", DeepModel: "qwen3.7-plus", Timeout: 30 * time.Second, MaxSubmissions: 2}
	client, err := tradingcn.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if id := os.Getenv("TM_TA_RESULT_ID"); id != "" {
		res, err := client.Result(ctx, id)
		if err != nil {
			t.Fatal("取已有结果失败", err)
		}
		var v struct {
			ID      string                     `json:"analysis_id"`
			Reports map[string]json.RawMessage `json:"reports"`
		}
		if json.Unmarshal(res.Payload, &v) != nil || v.ID == "" || len(v.Reports) == 0 {
			t.Fatal("已有结果结构不符", len(res.Payload))
		}
		t.Logf("real agent result reports=%d bytes=%d", len(v.Reports), len(res.Payload))
		return
	}
	acc, err := client.Submit(ctx, "000001")
	if err != nil {
		t.Fatal("提交失败", err)
	}
	t.Log("remote=", acc.RemoteID)
	deadline := time.Now().Add(14 * time.Minute)
	for {
		obs, err := client.Status(ctx, acc.RemoteID)
		if err != nil {
			t.Fatal("状态失败", err)
		}
		t.Log("state=", obs.State)
		if agents.Terminal(obs.State) {
			if obs.State != agents.Completed {
				t.Fatalf("远端终态=%s", obs.State)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("等待超时")
		}
		time.Sleep(10 * time.Second)
	}
	res, err := client.Result(ctx, acc.RemoteID)
	if err != nil {
		t.Fatal("取结果失败", err)
	}
	var v struct {
		ID      string                     `json:"analysis_id"`
		Reports map[string]json.RawMessage `json:"reports"`
	}
	if json.Unmarshal(res.Payload, &v) != nil || v.ID == "" || len(v.Reports) == 0 {
		t.Fatal("结果结构不符", len(res.Payload))
	}
	t.Logf("real agent result reports=%d bytes=%d", len(v.Reports), len(res.Payload))
}
