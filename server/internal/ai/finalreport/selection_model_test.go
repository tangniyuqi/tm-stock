package finalreport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func selectionModelFixture(t *testing.T) (*SelectionModelPlan, task.Task, *SelectionStage, *agenttask.Hint) {
	t.Helper()
	sel, _, _ := selectionFixture(t)
	v := task.Task{ID: 9, UserID: 2, No: sel.taskNo, SubjectID: sel.themeID, SubjectType: task.Theme, AnalysisType: 1, CurrentStep: 1, Budget: task.Budget{MaxAttempts: 3, MaxCalls: 5, MaxTokens: 10000, MaxPages: 3, MaxDurationMS: 30000}}
	p := agenttask.Plan{UserID: v.UserID, TaskID: v.ID, TaskNo: v.No, SubjectID: v.SubjectID, SubjectType: v.SubjectType, ActorID: 7, Symbol: "600001", ProfileDigest: strings.Repeat("b", 64), Units: task.Units{Calls: 1, Tokens: 100}}
	p.OperationKey = p.RequestID()
	result := agents.Result{RemoteID: "test_remote", Payload: json.RawMessage(`{"analysis_id":"test_remote","reports":{"news":"test-only-private-hint"}}`)}
	r := agents.Record{ID: 1, ActorID: p.ActorID, Request: agents.Request{RequestID: p.RequestID(), Symbol: p.Symbol}, ProfileDigest: p.ProfileDigest, RemoteID: result.RemoteID, State: agents.Quarantined, ResultDigest: result.Digest()}
	h, err := agenttask.NewHint(v, p, r, result)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewSelectionModelPlan(checkConfig(), sel, v, h)
	if err != nil {
		t.Fatal(err)
	}
	return plan, v, sel, h
}
func TestReportSelectionModelBoundedRequestAndReplay(t *testing.T) {
	p, v, s, h := selectionModelFixture(t)
	if !p.Valid() || p.StageName() != "extract" || p.Config() != checkConfig() || p.Units() != (task.Units{Calls: 1, Tokens: 3048}) || !p.MatchesTask(v) || !ValidDigest(p.OperationKey(v, 2)) || p.OperationKey(v, 1) != "" || !ValidDigest(p.InputDigest()) || p.Selection() != s {
		t.Fatal("选择计划绑定失败")
	}
	req := p.Request()
	if !strings.Contains(req.System, "同一章节允许返回多条selection") || !strings.Contains(req.System, "[1,5]") {
		t.Fatal("选句提示未说明非连续编号必须分条，不能靠修补模型回包通过")
	}
	if !strings.Contains(req.Prompt, "test-only-private-hint") || strings.Contains(req.Prompt, s.snapshots[0].LicenseRef) || strings.Contains(req.Prompt, s.snapshots[0].URL) || req.Attempt != 1 || req.MaxTokens != 2048 || !req.Strict || req.Format != provider.FormatJSONSchema {
		t.Fatal("提示/预算/信息边界错误")
	}
	req.Schema[0] = 'x'
	*req.Temperature = 1
	if !json.Valid(p.Request().Schema) || *p.Request().Temperature != 0 {
		t.Fatal("请求可变")
	}
	raw, err := p.CapturePayload(checkReply(p.cfg, selectionJSON(t, 1)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Build(context.Background(), string(raw))
	if err != nil || strings.Contains(got, "private-hint") {
		t.Fatal("回取泄漏线索", err)
	}
	want, err := s.Build(context.Background(), selectionJSON(t, 1))
	if err != nil || got != want {
		t.Fatal("正文不是授权回取", err)
	}
	if err := p.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "extract", InputDigest: p.digest, Payload: string(raw)}); err != nil {
		t.Fatal(err)
	}
	proof, policy, err := p.AgentBinding(v, "worker_selection_test")
	if err != nil || proof.StepNo != 1 || proof.OperationKey != h.Plan().OperationKey || !policy.Valid() {
		t.Fatal(proof, err)
	}
	v.UserID++
	if p.MatchesTask(v) || p.OperationKey(v, 2) != "" {
		t.Fatal("跨会员")
	}
	if _, _, err := p.AgentBinding(v, "worker_selection_test"); err == nil {
		t.Fatal("证明跨会员")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", p, p), "private-hint") {
		t.Fatal("格式化泄漏")
	}
}
func TestReportSelectionModelRejectsInvalidResponse(t *testing.T) {
	p, _, _, _ := selectionModelFixture(t)
	for _, name := range []string{"estimated", "usage", "model", "finish", "receipt", "free_text", "empty", "overlap", "extra", "duplicate", "too_large", "bad_digest", "noncanonical", "bad_envelope", "raw"} {
		t.Run(name, func(t *testing.T) {
			r := checkReply(p.cfg, selectionJSON(t, 1))
			switch name {
			case "estimated":
				r.UsageEstimated = true
			case "usage":
				r.Usage.Prompt = 1001
			case "model":
				r.Model = "其他模型"
			case "finish":
				r.FinishReason = "length"
			case "receipt":
				r.RequestID = ""
			case "free_text":
				r.Text = "测试自由正文"
			case "empty":
				r.Text = `{"selections":[]}`
			case "overlap":
				r.Text = strings.Replace(r.Text, "[2]", "[1]", 1)
			case "extra":
				r.Text = strings.Replace(r.Text, `"section":`, `"text":"自写测试正文","section":`, 1)
			case "duplicate":
				r.Text = strings.Replace(r.Text, `"section":`, `"section":"companies","section":`, 1)
			case "too_large":
				r.Text = strings.Repeat("x", selectionOutputBytes+1)
			}
			raw, err := p.CapturePayload(r)
			if name == "too_large" {
				if err == nil {
					t.Fatal("超大未拒")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "bad_digest":
				raw = []byte(strings.Replace(string(raw), p.digest, strings.Repeat("f", 64), 1))
			case "noncanonical":
				raw = append([]byte(" "), raw...)
			case "bad_envelope":
				raw = []byte(`{"unknown":1}`)
			case "raw":
				raw = []byte(strings.Replace(string(raw), `"response":{`, `"response":{"Raw":"eA==",`, 1))
			}
			if out, err := p.Build(context.Background(), string(raw)); err == nil || out != "" {
				t.Fatal("坏响应产生报告", err)
			}
		})
	}
	raw, _ := p.CapturePayload(checkReply(p.cfg, selectionJSON(t, 1)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Build(ctx, string(raw)); err != context.Canceled {
		t.Fatal(err)
	}
	for _, in := range []task.StageValidationInput{{Stage: "judge", InputDigest: p.digest, Payload: string(raw)}, {Stage: "extract", InputDigest: strings.Repeat("e", 64), Payload: string(raw)}} {
		if p.StagePolicy().Check(context.Background(), in) == nil {
			t.Fatal("错阶段/输入")
		}
	}
	if p.StagePolicy().Check(ctx, task.StageValidationInput{}) != context.Canceled {
		t.Fatal("忽略取消")
	}
}
func TestReportSelectionModelNilAndConstructionGates(t *testing.T) {
	p, v, s, h := selectionModelFixture(t)
	var missing *SelectionModelPlan
	if missing.Valid() || missing.InputDigest() != "" || missing.Config() != (CheckConfig{}) || missing.Selection() != nil || missing.Request().Prompt != "" || missing.Units() != (task.Units{}) || missing.MatchesTask(v) || missing.StagePolicy().Valid() || missing.OperationKey(v, 2) != "" {
		t.Fatal("空计划放行")
	}
	if _, err := missing.CapturePayload(provider.Response{}); err == nil {
		t.Fatal("空捕获")
	}
	if _, _, err := missing.AgentBinding(v, "bad"); err == nil {
		t.Fatal("空证明")
	}
	if _, err := missing.Build(context.Background(), `{}`); err == nil {
		t.Fatal("空构建")
	}
	for _, name := range []string{"meta", "selector", "hint", "user", "scope"} {
		t.Run(name, func(t *testing.T) {
			cfg := p.cfg
			sel := s
			hint := h
			vv := v
			switch name {
			case "meta":
				cfg.HardBudgetRef = ""
			case "selector":
				sel = nil
			case "hint":
				hint = nil
			case "user":
				vv.UserID = 0
			case "scope":
				vv.SubjectID++
			}
			if _, err := NewSelectionModelPlan(cfg, sel, vv, hint); err == nil {
				t.Fatal("未关闭")
			}
		})
	}
	if _, _, err := p.AgentBinding(v, "bad"); err == nil {
		t.Fatal("坏owner")
	}
	for _, r := range []provider.Response{{Text: strings.Repeat("x", selectionOutputBytes+1)}, {Model: strings.Repeat("x", 129)}, {RequestID: strings.Repeat("x", 257)}} {
		if _, err := p.CapturePayload(r); err == nil {
			t.Fatal("捕获上界")
		}
	}
}
