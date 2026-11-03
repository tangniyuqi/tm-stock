package finalreport

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// 先在旧实现上取证并固定策略摘要，防止新增协议改写旧任务的重读语义。
func TestReportCheckProtocolLegacySnapshot(t *testing.T) {
	want := map[CheckKind]string{CheckEntailment: "d7f33142d79f9edc50958caca4f63c214fdd079eb3e2ba8bde0ca5b2f30b8490", CheckSemantic: "8fef449241f1103cf1238ce2092d94a6e1d90a7d5401f39dce1f4deddd9b9488"}
	for _, kind := range []CheckKind{CheckEntailment, CheckSemantic} {
		got, err := CheckPolicyVersion(kind, checkConfig())
		if err != nil {
			t.Fatal(err)
		}
		if got != want[kind] {
			t.Fatal("旧判定请求或配置字节发生变化，不能改变历史重读")
		}
	}
}

func TestReportCheckProtocolPairs(t *testing.T) {
	for _, kind := range []CheckKind{CheckEntailment, CheckSemantic} {
		t.Run(string(kind), func(t *testing.T) {
			_, v, _, in, visible, _, _ := checkFixture(t)
			cfg := checkConfig()
			cfg.CheckProtocol = PairedCheckProtocol
			plan, err := NewEntailmentCheck(cfg, in)
			want := []string{"supported", "unsupported", "insufficient"}
			if kind == CheckSemantic {
				plan, err = NewSemanticCheck(cfg, v.TaskNo, visible)
				want = []string{"factual", "prohibited", "uncertain"}
			}
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Type                 string
				AdditionalProperties bool
				Properties           map[string]struct {
					Type string
					Enum []string
				}
				Required []string
				AnyOf    []struct {
					Properties map[string]struct{ Enum []string }
				}
			}
			var fields map[string]json.RawMessage
			r := plan.Request()
			if json.Unmarshal(r.Schema, &schema) != nil || json.Unmarshal(r.Schema, &fields) != nil || len(fields) != 5 || schema.Type != "object" || schema.AdditionalProperties || len(schema.Properties) != 2 || !slices.Equal(schema.Required, []string{"verdict", "reason"}) || len(schema.AnyOf) != 3 {
				t.Fatal("配对Schema结构不符")
			}
			verdicts := []string{"pass", "block", "uncertain"}
			if !slices.Equal(schema.Properties["verdict"].Enum, verdicts) || !slices.Equal(schema.Properties["reason"].Enum, want) {
				t.Fatal("Schema泄漏另一判定类型的原因值")
			}
			for i, branch := range schema.AnyOf {
				if len(branch.Properties) != 2 || !slices.Equal(branch.Properties["verdict"].Enum, []string{verdicts[i]}) || !slices.Equal(branch.Properties["reason"].Enum, []string{want[i]}) || !strings.Contains(r.System, verdicts[i]+"/"+want[i]) {
					t.Fatal("Schema与系统规则未声明同一合法配对")
				}
			}
			for i, verdict := range verdicts {
				for _, reason := range []string{"supported", "unsupported", "insufficient", "factual", "prohibited", "uncertain"} {
					t.Run(verdict+"_"+reason, func(t *testing.T) {
						evidence := captureEvidence(t, plan, checkReply(cfg, `{"verdict":"`+verdict+`","reason":"`+reason+`"}`))
						got := evidence.Decision()
						if reason == want[i] {
							if got.Verdict != Verdict(verdict) || got.Reason != reason {
								t.Fatal("合法配对被拒绝")
							}
						} else if got.Verdict != Uncertain || got.Reason != "invalid_output" {
							t.Fatal("错配原因被修补或放行")
						}
					})
				}
			}
		})
	}
}

func TestReportCheckProtocolBindings(t *testing.T) {
	_, v, _, in, visible, old, _ := checkFixture(t)
	cfg := checkConfig()
	cfg.CheckProtocol = PairedCheckProtocol
	paired, err := NewEntailmentCheck(cfg, in)
	if err != nil {
		t.Fatal(err)
	}
	current := task.Task{ID: 9, UserID: 2, No: v.TaskNo, SubjectID: v.ThemeID, SubjectType: task.Theme, AnalysisType: 1}
	if old.PolicyVersion() == paired.PolicyVersion() || old.InputDigest() == paired.InputDigest() || old.OperationKey(current, 3) == paired.OperationKey(current, 3) || old.StagePolicy().Digest == paired.StagePolicy().Digest || old.Request().Prompt != paired.Request().Prompt || old.Units() != paired.Units() {
		t.Fatal("协议未隔离身份或扩大原资料/预算范围")
	}
	for _, plans := range [][2]*CheckPlan{{old, paired}, {paired, old}} {
		raw, err := plans[0].CapturePayload(checkReply(plans[0].Config(), `{"verdict":"pass","reason":"supported"}`))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plans[1].Evidence(string(raw)); err == nil {
			t.Fatal("不同协议借用旧捕获")
		}
	}
	cfg.CheckProtocol = "unknown"
	if _, err := NewEntailmentCheck(cfg, in); err == nil {
		t.Fatal("未知蕴含协议未拒绝")
	}
	if _, err := NewSemanticCheck(cfg, v.TaskNo, visible); err == nil {
		t.Fatal("未知语义协议未拒绝")
	}
	if _, err := CheckPolicyVersion(CheckSemantic, cfg); err == nil {
		t.Fatal("未知策略协议未拒绝")
	}
	paired.cfg.CheckProtocol = "unknown"
	if paired.Valid() {
		t.Fatal("被破坏的内部协议仍有效")
	}
}
