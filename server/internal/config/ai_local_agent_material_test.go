package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func agentMaterialFixture() (AILocalAgentMaterial, *AILocalRuntime) {
	r := &AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: strings.Repeat("a", 32), Database: "tm_ai_local_" + strings.Repeat("a", 32), Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
	now := time.Now().UnixMilli()
	return AILocalAgentMaterial{Version: 1, InstanceID: r.InstanceID, Revision: "v1", Scope: AgentMaterialScope, IssuedAtMS: now - 1000, ExpiresAtMS: now + 3600000,
		Members: []int64{1, 2},
		Use: AILocalAgentMaterialUse{DatasetDigest: strings.Repeat("a", 64), ApprovalRef: "local-eval:agent-development", Purpose: AgentMaterialPurpose,
			Bindings: []AILocalAgentMaterialBinding{
				{SubjectType: "theme", SubjectID: 1, ScopeApproval: "local-eval:theme-1", Evidence: []AILocalAgentMaterialEvidence{{SnapshotID: 3, Hash: strings.Repeat("b", 64)}, {SnapshotID: 7, Hash: strings.Repeat("c", 64)}}},
				{SubjectType: "topic", SubjectID: 4, ScopeApproval: "local-eval:topic-4", Evidence: []AILocalAgentMaterialEvidence{{SnapshotID: 9, Hash: strings.Repeat("d", 64)}}},
			}},
		Limits: AILocalAgentMaterialLimits{MaxSources: 4, MaxEvidenceBytes: 65536}}, r
}

func writeAgentMaterial(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "agent-material.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAILocalAgentMaterialConfiguration(t *testing.T) {
	cases := map[string]func(c *AILocalAgentMaterial){
		"valid":            func(c *AILocalAgentMaterial) {},
		"version":          func(c *AILocalAgentMaterial) { c.Version = 2 },
		"qa_scope":         func(c *AILocalAgentMaterial) { c.Scope = "official-materials-development-only" },
		"qa_purpose":       func(c *AILocalAgentMaterial) { c.Use.Purpose = "bounded-material-qa" },
		"report_purpose":   func(c *AILocalAgentMaterial) { c.Use.Purpose = "report-generation" },
		"instance":         func(c *AILocalAgentMaterial) { c.InstanceID = strings.Repeat("b", 32) },
		"expired":          func(c *AILocalAgentMaterial) { c.ExpiresAtMS = time.Now().UnixMilli() - 1 },
		"future_issue":     func(c *AILocalAgentMaterial) { c.IssuedAtMS = time.Now().UnixMilli() + 60000 },
		"no_members":       func(c *AILocalAgentMaterial) { c.Members = nil },
		"unsorted_members": func(c *AILocalAgentMaterial) { c.Members = []int64{2, 1} },
		"dataset":          func(c *AILocalAgentMaterial) { c.Use.DatasetDigest = "unknown" },
		"approval":         func(c *AILocalAgentMaterial) { c.Use.ApprovalRef = "production-approved" },
		"no_bindings":      func(c *AILocalAgentMaterial) { c.Use.Bindings = nil },
		"stock_subject":    func(c *AILocalAgentMaterial) { c.Use.Bindings[0].SubjectType = "stock" },
		"zero_subject":     func(c *AILocalAgentMaterial) { c.Use.Bindings[0].SubjectID = 0 },
		"unsafe_subject":   func(c *AILocalAgentMaterial) { c.Use.Bindings[0].SubjectID = 9007199254740992 },
		"duplicate_object": func(c *AILocalAgentMaterial) { c.Use.Bindings[1] = c.Use.Bindings[0] },
		"unsorted_objects": func(c *AILocalAgentMaterial) {
			c.Use.Bindings[0], c.Use.Bindings[1] = c.Use.Bindings[1], c.Use.Bindings[0]
		},
		"scope_approval": func(c *AILocalAgentMaterial) { c.Use.Bindings[0].ScopeApproval = "" },
		"no_evidence":    func(c *AILocalAgentMaterial) { c.Use.Bindings[0].Evidence = nil },
		"over_sources":   func(c *AILocalAgentMaterial) { c.Limits.MaxSources = 1 },
		"zero_snapshot":  func(c *AILocalAgentMaterial) { c.Use.Bindings[0].Evidence[0].SnapshotID = 0 },
		"upper_hash":     func(c *AILocalAgentMaterial) { c.Use.Bindings[0].Evidence[0].Hash = strings.Repeat("B", 64) },
		"dup_snapshot":   func(c *AILocalAgentMaterial) { c.Use.Bindings[0].Evidence[1].SnapshotID = 3 },
		"limits":         func(c *AILocalAgentMaterial) { c.Limits.MaxEvidenceBytes = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c, runtime := agentMaterialFixture()
			mutate(&c)
			_, err := NewAILocalAgentMaterialSource(writeAgentMaterial(t, c), runtime)
			if name == "valid" && err != nil {
				t.Fatalf("合法批准被拒绝：%v", err)
			}
			if name != "valid" && err == nil {
				t.Fatal("非法批准被接受")
			}
		})
	}
}

// 精确JSON：未知字段、null、缺字段及别名一律拒绝，防止用问答配置文件冒充 Agent 批准。
func TestAILocalAgentMaterialRejectsLooseJSON(t *testing.T) {
	c, runtime := agentMaterialFixture()
	base, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(base, &fields); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(m map[string]json.RawMessage){
		"unknown":   func(m map[string]json.RawMessage) { m["material_use"] = json.RawMessage("{}") },
		"null_use":  func(m map[string]json.RawMessage) { m["use"] = json.RawMessage("null") },
		"missing":   func(m map[string]json.RawMessage) { delete(m, "limits") },
		"alias":     func(m map[string]json.RawMessage) { m["Use"] = m["use"]; delete(m, "use") },
		"qa_models": func(m map[string]json.RawMessage) { m["models"] = json.RawMessage("[]") },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			m := map[string]json.RawMessage{}
			for k, v := range fields {
				m[k] = v
			}
			mutate(m)
			if _, err := NewAILocalAgentMaterialSource(writeAgentMaterial(t, m), runtime); err == nil {
				t.Fatal("宽松JSON被接受")
			}
		})
	}
	if _, err := NewAILocalAgentMaterialSource("relative/path.json", runtime); err == nil {
		t.Fatal("相对路径被接受")
	}
}

func TestAILocalAgentMaterialBindingAndDigest(t *testing.T) {
	c, runtime := agentMaterialFixture()
	s, err := NewAILocalAgentMaterialSource(writeAgentMaterial(t, c), runtime)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := s.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, ok := cur.Binding(1, "theme", 1)
	if !ok || len(b.Evidence) != 2 {
		t.Fatal("已批准对象未返回")
	}
	for _, miss := range []struct {
		member int64
		kind   string
		id     int64
	}{{3, "theme", 1}, {1, "topic", 1}, {1, "theme", 4}, {1, "stock", 1}} {
		if _, ok := cur.Binding(miss.member, miss.kind, miss.id); ok {
			t.Fatalf("未批准组合被返回：%+v", miss)
		}
	}
	b.Evidence[0].Hash = "tampered"
	again, _ := cur.Binding(1, "theme", 1)
	if again.Evidence[0].Hash == "tampered" {
		t.Fatal("返回绑定与批准共享底层切片")
	}
	d1, err := cur.MaterialsDigest(again)
	if err != nil || !localPlanDigest.MatchString(d1) {
		t.Fatal("资料包摘要格式错误")
	}
	d2, _ := cur.MaterialsDigest(again)
	if d1 != d2 {
		t.Fatal("资料包摘要不确定")
	}
	other, _ := cur.Binding(2, "topic", 4)
	if d3, _ := cur.MaterialsDigest(other); d3 == d1 {
		t.Fatal("不同对象摘要相同")
	}
	changed := again
	changed.Evidence = append([]AILocalAgentMaterialEvidence(nil), again.Evidence...)
	changed.Evidence[1].Hash = strings.Repeat("e", 64)
	if d4, _ := cur.MaterialsDigest(changed); d4 == d1 {
		t.Fatal("快照哈希变化未改变摘要")
	}
}

// 启动后批准文件被改写、过期或删除，Current 必须失败，不能沿用启动时的旧批准。
func TestAILocalAgentMaterialCurrentRevocation(t *testing.T) {
	for _, mode := range []string{"rewritten", "deleted", "expired", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			c, runtime := agentMaterialFixture()
			p := writeAgentMaterial(t, c)
			s, err := NewAILocalAgentMaterialSource(p, runtime)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			switch mode {
			case "rewritten":
				c.Use.Bindings[0].Evidence[0].Hash = strings.Repeat("f", 64)
				b, _ := json.Marshal(c)
				if err := os.WriteFile(p, b, 0o600); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			case "expired":
				s.now = func() time.Time { return time.UnixMilli(c.ExpiresAtMS + 1) }
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := s.Current(ctx); err == nil {
				t.Fatal("撤回后的批准仍被沿用")
			}
		})
	}
	var nilSource *AILocalAgentMaterialSource
	if _, err := nilSource.Current(context.Background()); err == nil {
		t.Fatal("空来源被接受")
	}
	if _, err := NewAILocalAgentMaterialSource("C:\\x.json", nil); err == nil {
		t.Fatal("空运行时被接受")
	}
}

// 问答批准与 Agent 批准互不替代：合法的资料问答文件不能作为 Agent 批准加载。
func TestAILocalAgentMaterialRejectsQAApproval(t *testing.T) {
	qaCfg, runtime := localQAMaterialsFixture(t)
	if _, err := NewAILocalAgentMaterialSource(writeAgentMaterial(t, qaCfg), runtime); err == nil {
		t.Fatal("资料问答批准被当作 Agent 批准加载")
	}
	c, _ := agentMaterialFixture()
	c.InstanceID = runtime.InstanceID
	if _, err := NewAILocalQASource(writeAgentMaterial(t, c), runtime); err == nil {
		t.Fatal("Agent 批准被当作资料问答批准加载")
	}
}
