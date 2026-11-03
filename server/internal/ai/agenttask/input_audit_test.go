package agenttask

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestHintExcludesCompletePrivateInputs(t *testing.T) {
	p, v, r := fixture()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "eval", "agents", "inputs-synthetic-protocol.json"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"analysis_id": r.RemoteID, "reports": map[string]string{"news_report": "仅测试的研究线索"}, "evaluation_scope": "synthetic_fixture_only", "input_audit": json.RawMessage(raw)})
	if err != nil {
		t.Fatal(err)
	}
	result := agents.Result{RemoteID: r.RemoteID, Payload: payload}
	r.ResultDigest = result.Digest()
	h, err := NewHint(v, p, r, result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(h.Payload()), "隔离提示私有标记") || strings.Contains(string(h.Payload()), "input_audit") || !strings.Contains(string(h.Payload()), "仅测试的研究线索") || h.Record().ResultDigest != result.Digest() {
		t.Fatal("私有上下文混入后续模型或丢失原结果绑定")
	}
	// 大于旧线索上限的私有审计可以隔离保存，后续模型仍只接原256KiB以内的线索。
	var audit agents.InputAudit
	if json.Unmarshal(raw, &audit) != nil {
		t.Fatal("合成审计夹具错误")
	}
	var request struct {
		Model     string          `json:"model"`
		MaxTokens int             `json:"max_tokens"`
		Messages  json.RawMessage `json:"messages"`
	}
	request.Model = "deepseek-flash"
	request.MaxTokens = 2048
	messages, err := json.Marshal([]map[string]string{{"role": "system", "content": strings.Repeat("x", MaxHintBytes)}})
	if err != nil {
		t.Fatal(err)
	}
	request.Messages = messages
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	audit.Calls[0].RequestJSON = string(wire)
	audit.Calls[0].RequestDigest = task.StageDigest(wire)
	audit.Calls[0].PromptDigest = task.StageDigest(messages)
	large, err := json.Marshal(map[string]any{"analysis_id": r.RemoteID, "reports": map[string]string{"news_report": "仅测试的研究线索"}, "evaluation_scope": "synthetic_fixture_only", "input_audit": audit})
	if err != nil || len(large) <= MaxHintBytes {
		t.Fatal("大审计边界夹具错误")
	}
	p.InputAudit = agents.InputAuditVersion
	result.Payload = large
	r.ResultDigest = result.Digest()
	if h, err := NewHint(v, p, r, result); err != nil || len(h.Payload()) >= MaxHintBytes {
		t.Fatal("私有审计大小污染线索限制", err)
	}
	bad := strings.Replace(string(payload), "synthetic-fixture-only", "real-materials", 1)
	result.Payload = []byte(bad)
	r.ResultDigest = result.Digest()
	if _, err := NewHint(v, p, r, result); err == nil {
		t.Fatal("未知审计协议回退成普通线索")
	}
}

func TestRequiredAgentInputsCannotDowngrade(t *testing.T) {
	p, v, r := fixture()
	legacyDigest, key := p.InputDigest(v), p.RequestID()
	b, err := json.Marshal(p)
	if err != nil || strings.Contains(string(b), "InputAudit") {
		t.Fatal("旧计划编码被改变")
	}
	p.InputAudit = agents.InputAuditVersion
	if p.InputDigest(v) == legacyDigest || p.RequestID() != key || p.ValidateScope(v) != nil {
		t.Fatal("审计要求未绑定原输入或擅自换原请求键")
	}
	result := agents.Result{RemoteID: r.RemoteID, Payload: []byte(`{"analysis_id":"test_remote_id","reports":{"news":"legacy"}}`)}
	r.ResultDigest = result.Digest()
	if _, err := NewHint(v, p, r, result); err == nil {
		t.Fatal("删除已声明审计后回退旧协议")
	}
	p.InputAudit = "unknown"
	if p.ValidateScope(v) == nil {
		t.Fatal("未知审计版本放行")
	}
	for _, alias := range []string{"Input_Audit", "inputAudit"} {
		result.Payload = []byte(`{"analysis_id":"test_remote_id","reports":{"news":"legacy"},"` + alias + `":{}}`)
		p.InputAudit = ""
		r.ResultDigest = result.Digest()
		if _, err := NewHint(v, p, r, result); err == nil {
			t.Fatal("保留键别名混入模型线索")
		}
	}
}
