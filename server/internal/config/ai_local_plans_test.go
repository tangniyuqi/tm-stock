package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func localPlansFixture() (*AILocalRuntime, AILocalPlans) {
	r := &AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: strings.Repeat("a", 32), Database: "tm_ai_local_" + strings.Repeat("a", 32), Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
	now := time.Now().UnixMilli()
	p := AILocalPlan{AnalysisType: 1, SubjectID: 1, SubjectName: "仅测试:题材", ActorID: 7, Symbol: "000001", AgentProfileDigest: strings.Repeat("b", 64), AgentCalls: 1, AgentTokens: 100, AgentPages: 0, ModelName: "仅测试:模型", ModelProfileDigest: strings.Repeat("c", 64), FilingNo: "仅测试:公示编号", ModelVersion: "local-eval:v1", MaxInputTokens: 32, SelectionMaxTokens: 2048, CheckMaxTokens: 64, PolicyRef: "local-eval:policy", AllowedDomains: []string{"evidence.example.test"}, ApprovalRef: "local-eval:approval", QualityRef: "local-eval:quality", HardBudgetRef: "local-eval:hard-budget", PriceRef: "local-eval:price", SourceRef: "local-eval:source", PriceVersion: "local-v1", PricePoints: 7, Budget: task.Budget{MaxAttempts: 1, MaxCalls: 6, MaxTokens: 5000, MaxPages: 10, MaxDurationMS: 60000}, Materials: []AILocalMaterial{{SnapshotID: 1, Digest: strings.Repeat("d", 64), LicenseRef: "local-eval:license"}}}
	return r, AILocalPlans{Version: 1, InstanceID: r.InstanceID, Revision: "v1", Scope: "synthetic-local-only", IssuedAtMS: now - 1000, ExpiresAtMS: now + 60000, Members: []int64{1, 2}, Plans: []AILocalPlan{p}}
}

func TestAILocalPlansExactAndIsolated(t *testing.T) {
	r, p := localPlansFixture()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	file := filepath.Join(t.TempDir(), "plans.json")
	for name, value := range map[string]string{
		"正常": raw, "未知密钥": strings.Replace(raw, `"version":1`, `"version":1,"api_key":"forbidden_value"`, 1),
		"嵌套大小写": strings.Replace(raw, `"model_name"`, `"Model_Name"`, 1),
		"嵌套重复":  strings.Replace(raw, `"agent_calls":1`, `"agent_calls":1,"agent_calls":1`, 1),
		"空零值":   strings.Replace(raw, `"agent_pages":0`, `"agent_pages": null `, 1),
		"缺零值":   strings.Replace(raw, `"agent_pages":0,`, "", 1),
		"尾随":    raw + `{}`, "跨实例": strings.Replace(raw, r.InstanceID, strings.Repeat("f", 32), 1),
		"生产范围":  strings.Replace(raw, "synthetic-local-only", "production", 1),
		"非合成域名": strings.Replace(raw, "evidence.example.test", "example.com", 1),
		"重复计划":  strings.Replace(raw, `"plans":[`, `"plans":[`+string(mustLocalJSON(t, p.Plans[0]))+`,`, 1),
		"超大":    strings.Repeat(" ", 128<<10+1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(file, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := NewAILocalPlanSource(file, r)
			if (err == nil) != (name == "正常") {
				t.Fatal("本地计划解析边界错误", err)
			}
			if err != nil && strings.Contains(err.Error(), "forbidden_value") {
				t.Fatal("泄漏配置正文")
			}
		})
	}
	for name, mutate := range map[string]func(*AILocalPlans){
		"过期":     func(p *AILocalPlans) { p.ExpiresAtMS = p.IssuedAtMS },
		"未来签发":   func(p *AILocalPlans) { p.IssuedAtMS = time.Now().Add(time.Hour).UnixMilli() },
		"过长有效期":  func(p *AILocalPlans) { p.ExpiresAtMS = p.IssuedAtMS + 86400001 },
		"会员重复":   func(p *AILocalPlans) { p.Members = []int64{1, 1} },
		"会员空":    func(p *AILocalPlans) { p.Members = []int64{} },
		"输出预算溢出": func(p *AILocalPlans) { p.Plans[0].CheckMaxTokens = 1 << 30 },
	} {
		t.Run(name, func(t *testing.T) {
			_, q := localPlansFixture()
			mutate(&q)
			if err := os.WriteFile(file, mustLocalJSON(t, q), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewAILocalPlanSource(file, r); err == nil {
				t.Fatal("时间或主体范围未拒绝")
			}
		})
	}
}

func mustLocalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAILocalPlansNoHotReplacementOrSharedSlices(t *testing.T) {
	r, p := localPlansFixture()
	file := filepath.Join(t.TempDir(), "plans.json")
	write := func(v any) {
		t.Helper()
		if err := os.WriteFile(file, mustLocalJSON(t, v), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(p)
	s, err := NewAILocalPlanSource(file, r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	got, err := s.Current(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got.Members[0] = 99
	got.Plans[0].Materials[0].SnapshotID = 99
	if next, err := s.Current(ctx); err != nil || next.Members[0] != 1 || next.Plans[0].Materials[0].SnapshotID != 1 {
		t.Fatal("返回值改写了可信计划")
	}
	pretty, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, pretty, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Current(ctx); err != nil {
		t.Fatal("仅JSON排版变化改变了计划身份")
	}
	p.Revision = "v2"
	write(p)
	if _, err := s.Current(ctx); err == nil {
		t.Fatal("运行中接受新版本")
	}
	p.Revision = "v1"
	p.Plans[0].PricePoints++
	write(p)
	if _, err := s.Current(ctx); err == nil {
		t.Fatal("原版本下修改价格未拒绝")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Current(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal("取消未保留")
	}
	if _, err := NewAILocalPlanSource("relative.json", r); err == nil {
		t.Fatal("相对路径未拒绝")
	}
	if _, err := NewAILocalPlanSource(file, nil); err == nil {
		t.Fatal("没有隔离启动配置")
	}
}
