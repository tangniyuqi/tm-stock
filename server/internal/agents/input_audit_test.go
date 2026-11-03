package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "eval", "agents", "inputs-synthetic-protocol.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestAgentInputAuditClosedEvidence(t *testing.T) {
	for _, mode := range []string{"valid", "scope", "commit", "provider", "empty", "missing_node", "unknown_node", "sequence", "call_id", "duplicate_call", "model", "response_model", "unknown_response", "http", "input_usage", "output_usage", "wire", "prompt", "wire_model", "messages", "fixture", "fixture_digest"} {
		t.Run(mode, func(t *testing.T) {
			a, err := ParseInputAudit(auditFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "scope":
				a.Scope = "real-materials"
			case "commit":
				a.UpstreamCommit = strings.Repeat("a", 40)
			case "provider":
				a.Provider = "other"
			case "empty":
				a.Calls = nil
			case "missing_node":
				a.Calls = a.Calls[1:]
			case "unknown_node":
				a.Calls[0].Node = "unknown"
			case "sequence":
				a.Calls[0].Seq = 2
			case "call_id":
				a.Calls[0].CallID = "bad"
			case "duplicate_call":
				a.Calls[1].CallID = a.Calls[0].CallID
			case "model":
				a.Calls[0].Model = "other"
			case "response_model":
				a.Calls[0].ResponseModel = "other"
			case "unknown_response":
				a.Calls[0].State = "response_unknown"
			case "http":
				a.Calls[0].Status = 500
			case "input_usage":
				a.Calls[0].InputTokens = -1
			case "output_usage":
				a.Calls[0].OutputTokens = -1
			case "wire":
				a.Calls[0].RequestJSON += " "
			case "prompt":
				a.Calls[0].PromptDigest = strings.Repeat("a", 64)
			case "wire_model":
				a.Calls[0].RequestJSON = strings.Replace(a.Calls[0].RequestJSON, "deepseek-flash", "other", 1)
				a.Calls[0].RequestDigest = inputDigest([]byte(a.Calls[0].RequestJSON))
			case "messages":
				a.Calls[0].RequestJSON = `{"model":"deepseek-flash","max_tokens":2048,"messages":[]}`
				a.Calls[0].RequestDigest = inputDigest([]byte(a.Calls[0].RequestJSON))
				a.Calls[0].PromptDigest = inputDigest([]byte(`[]`))
			case "fixture":
				a.FixtureJSON = strings.Replace(a.FixtureJSON, "测试设备", "真实设备", 1)
				a.FixtureDigest = inputDigest([]byte(a.FixtureJSON))
			case "fixture_digest":
				a.FixtureDigest = strings.Repeat("a", 64)
			}
			raw, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseInputAudit(raw)
			if (err == nil) != (mode == "valid") {
				t.Fatal("上游输入存证边界不符")
			}
			if got != nil && strings.Contains(fmt.Sprintf("%+v %#v %+v", got, got, got.Calls[0]), "隔离提示私有标记") {
				t.Fatal("格式化泄漏内部输入")
			}
		})
	}
}
func TestAgentInputAuditStrictJSON(t *testing.T) {
	for _, mode := range []string{"unknown", "alias", "duplicate", "null", "nested_unknown", "nested_alias", "nested_null", "trailing", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			raw := auditFixture(t)
			switch mode {
			case "unknown":
				raw = bytes.Replace(raw, []byte(`"schema":`), []byte(`"extra":false,"schema":`), 1)
			case "alias":
				raw = bytes.Replace(raw, []byte(`"schema":`), []byte(`"Schema":`), 1)
			case "duplicate":
				raw = bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"duplicate","schema":`), 1)
			case "null":
				raw = []byte(`null`)
			case "nested_unknown":
				raw = bytes.Replace(raw, []byte(`"seq": 1,`), []byte(`"seq":1,"extra":false,`), 1)
			case "nested_alias":
				raw = bytes.Replace(raw, []byte(`"seq": 1,`), []byte(`"Seq":1,`), 1)
			case "nested_null":
				raw = bytes.Replace(raw, []byte(`"input_tokens": 1`), []byte(`"input_tokens":null`), 1)
			case "trailing":
				raw = append(raw, []byte(`{}`)...)
			case "oversize":
				raw = bytes.Repeat([]byte(" "), MaxInputAuditBytes+1)
			}
			if _, err := ParseInputAudit(raw); err == nil {
				t.Fatal("闭集JSON未拒绝")
			}
		})
	}
}

func TestAgentInputAuditResultProjection(t *testing.T) {
	for _, mode := range []string{"valid", "legacy", "wrong_remote", "unknown_top", "missing_reports", "scope", "alias", "audit_null", "audit_version", "reports_array", "empty_reports", "reports_null", "duplicate", "invalid", "private_format"} {
		t.Run(mode, func(t *testing.T) {
			obj := map[string]any{"analysis_id": "audit_remote_id", "reports": map[string]string{"news_report": "仅测试研究线索"}, "evaluation_scope": "synthetic_fixture_only", "input_audit": json.RawMessage(auditFixture(t))}
			switch mode {
			case "wrong_remote":
				obj["analysis_id"] = "different_remote"
			case "unknown_top":
				obj["extra"] = "私有字段"
			case "missing_reports":
				delete(obj, "reports")
			case "scope":
				obj["evaluation_scope"] = "real-materials"
			case "alias":
				obj["InputAudit"] = obj["input_audit"]
				delete(obj, "input_audit")
			case "audit_null":
				obj["input_audit"] = nil
			case "audit_version":
				a := string(auditFixture(t))
				obj["input_audit"] = json.RawMessage(strings.Replace(a, InputAuditVersion, "unknown", 1))
			case "reports_array":
				obj["reports"] = []string{"bad"}
			case "empty_reports":
				obj["reports"] = map[string]string{}
			case "reports_null":
				obj["reports"] = nil
			}
			raw, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "legacy" {
				raw = []byte(" {\"analysis_id\":\"audit_remote_id\",\"reports\":{\"news\":\"legacy\"}} ")
			}
			if mode == "duplicate" {
				raw = bytes.Replace(raw, []byte(`"analysis_id":`), []byte(`"analysis_id":"duplicate","analysis_id":`), 1)
			}
			if mode == "invalid" {
				raw = []byte(`not-json`)
			}
			result := Result{RemoteID: "audit_remote_id", Payload: raw}
			payload, audit, err := InputAuditPayload(result)
			want := mode == "valid" || mode == "legacy" || mode == "private_format"
			if (err == nil) != want {
				t.Fatal("隔离结果闭集/原身份/线索投影不符")
			}
			if err != nil && (payload != nil || audit != nil) {
				t.Fatal("损坏结果返回部分私有内容")
			}
			if mode == "legacy" {
				if !bytes.Equal(payload, raw) || audit != nil {
					t.Fatal("旧结果字节被重新解释")
				}
				payload[0] = 'x'
				if raw[0] == 'x' {
					t.Fatal("旧线索未复制")
				}
			}
			if mode == "valid" || mode == "private_format" {
				if audit == nil || bytes.Contains(payload, []byte("input_audit")) || bytes.Contains(payload, []byte("隔离提示私有标记")) || !bytes.Contains(payload, []byte("仅测试研究线索")) {
					t.Fatal("内部输入混入后续模型")
				}
				if strings.Contains(fmt.Sprintf("%#v", audit.Calls[0]), "隔离提示私有标记") {
					t.Fatal("GoString泄漏内部输入")
				}
			}
		})
	}
}
