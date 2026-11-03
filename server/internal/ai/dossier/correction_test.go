package dossier

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func correctionTestRequest() CorrectionRequest {
	return CorrectionRequest{RequestID: "test_correction_protocol_001", Object: CorrectionPair{ThemeID: 7, BeforeVersion: 1, AfterVersion: 2}, BeforeDigest: strings.Repeat("a", 64), AfterDigest: strings.Repeat("b", 64), Enabled: true, ValidUntil: 1793703600000, Declarations: []CorrectionDeclaration{{Before: CorrectionLocator{Key: "sample"}, After: CorrectionLocator{Key: "sample", Field: "stage"}, StatementKey: "declaration"}}, ReviewReference: "test-fixture:private-correction"}
}
func TestDossierCorrectionRequestProtocol(t *testing.T) {
	r := correctionTestRequest()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCorrectionRequest(raw)
	if err != nil || !strings.Contains(parsed.String(), "隐藏") {
		t.Fatal("合法声明请求被拒绝")
	}
	for name, edit := range map[string]func(map[string]any){
		"missing_enabled": func(v map[string]any) { delete(v, "enabled") }, "null_enabled": func(v map[string]any) { v["enabled"] = nil }, "string_enabled": func(v map[string]any) { v["enabled"] = "true" }, "actor_spoof": func(v map[string]any) { v["actorId"] = 8 }, "null_declarations": func(v map[string]any) { v["declarations"] = nil }, "empty_enabled": func(v map[string]any) { v["declarations"] = []any{} }, "object_extra": func(v map[string]any) { v["object"].(map[string]any)["userId"] = 1 }, "string_version": func(v map[string]any) { v["object"].(map[string]any)["beforeVersion"] = "1" }, "reverse_version": func(v map[string]any) { v["object"].(map[string]any)["afterVersion"] = 1 }, "bad_digest": func(v map[string]any) { v["beforeDigest"] = "bad" }, "overflow_revision": func(v map[string]any) { v["expectedRevision"] = float64(9007199254740991) }, "overflow_time": func(v map[string]any) { v["validUntil"] = float64(ManagementMaxTime + 1) }, "private_field": func(v map[string]any) {
			v["declarations"].([]any)[0].(map[string]any)["after"].(map[string]any)["value"] = "free"
		}, "unknown_field": func(v map[string]any) {
			v["declarations"].([]any)[0].(map[string]any)["after"].(map[string]any)["field"] = "value"
		}, "null_statement": func(v map[string]any) { v["declarations"].([]any)[0].(map[string]any)["statementKey"] = nil }, "duplicate_relation": func(v map[string]any) { ds := v["declarations"].([]any); v["declarations"] = append(ds, ds[0]) }, "bad_reference": func(v map[string]any) { v["reviewReference"] = " " },
	} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			if json.Unmarshal(raw, &v) != nil {
				t.Fatal("测试输入解析失败")
			}
			edit(v)
			b, er := json.Marshal(v)
			if er != nil {
				t.Fatal(er)
			}
			if _, er = ParseCorrectionRequest(b); er == nil {
				t.Fatal("非法更正请求被接受")
			}
		})
	}
	for name, b := range map[string][]byte{"duplicate": []byte(strings.TrimSuffix(string(raw), "}") + `,"enabled":false}`), "alias": []byte(strings.TrimSuffix(string(raw), "}") + `,"Enabled":true}`), "array": []byte("[]"), "trailing": append(append([]byte{}, raw...), []byte("{}")...), "oversize": []byte(strings.Repeat(" ", MaxCorrectionBytes+1)), "invalid_utf8": {'{', '"', 255}} {
		t.Run(name, func(t *testing.T) {
			if _, e := ParseCorrectionRequest(b); e == nil {
				t.Fatal("畸形更正JSON通过")
			}
		})
	}
	r.Enabled = false
	r.ValidUntil = 0
	r.Declarations = []CorrectionDeclaration{}
	raw, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseCorrectionRequest(raw); err != nil {
		t.Fatal("明确撤回请求被拒绝")
	}
}
func TestDossierCorrectionEventProtocol(t *testing.T) {
	r := correctionTestRequest()
	e := CorrectionEvent{Protocol: CorrectionProtocol, ActorID: 7, Request: r, Revision: 1, CreatedAt: r.ValidUntil - 60000}
	raw, digest, err := e.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreCorrectionEvent(raw, digest)
	if err != nil || !restored.Matches(7, r) || restored.Matches(8, r) {
		t.Fatal("原事件/身份绑定失败")
	}
	if strings.Contains(fmt.Sprintf("%v %#v %v %#v", e, e, r, r), r.ReviewReference) {
		t.Fatal("格式化暴露审核引用")
	}
	for name, edit := range map[string]func(*CorrectionEvent){"protocol": func(e *CorrectionEvent) { e.Protocol = "other" }, "actor": func(e *CorrectionEvent) { e.ActorID = 0 }, "revision": func(e *CorrectionEvent) { e.Revision++ }, "time": func(e *CorrectionEvent) { e.CreatedAt = 0 }, "time_overflow": func(e *CorrectionEvent) { e.CreatedAt = ManagementMaxTime + 1 }, "expired": func(e *CorrectionEvent) { e.CreatedAt = e.Request.ValidUntil }, "first_previous": func(e *CorrectionEvent) { e.PreviousDigest = strings.Repeat("c", 64) }, "missing_previous": func(e *CorrectionEvent) { e.Request.ExpectedRevision = 1; e.Revision = 2 }, "first_withdraw": func(e *CorrectionEvent) {
		e.Request.Enabled = false
		e.Request.ValidUntil = 0
		e.Request.Declarations = []CorrectionDeclaration{}
	}} {
		t.Run(name, func(t *testing.T) {
			v := e
			edit(&v)
			if _, _, er := v.Canonical(); er == nil {
				t.Fatal("矛盾事件可归档")
			}
		})
	}
	for name, b := range map[string][]byte{"unknown": []byte(strings.TrimSuffix(string(raw), "}") + `,"private":1}`), "null_previous": []byte(strings.Replace(string(raw), `"previousDigest":""`, `"previousDigest":null`, 1)), "reorder": []byte(" " + string(raw)), "bad_bytes": append([]byte{}, raw...)} {
		t.Run(name, func(t *testing.T) {
			hash := task.StageDigest(b)
			if name == "bad_bytes" {
				hash = strings.Repeat("0", 64)
			}
			if _, er := RestoreCorrectionEvent(b, hash); er == nil {
				t.Fatal("非规范或损坏存证通过")
			}
		})
	}
	e.Request.Enabled = false
	e.Request.ValidUntil = 0
	e.Request.Declarations = []CorrectionDeclaration{}
	e.Request.ExpectedRevision = 1
	e.Revision = 2
	e.PreviousDigest = digest
	if _, _, err = e.Canonical(); err != nil {
		t.Fatal("保留前事件摘要的撤回被拒绝")
	}
}
