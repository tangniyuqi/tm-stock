package finalreport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func checkConfig() CheckConfig {
	return CheckConfig{Model: "测试判定模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", MaxInputTokens: 1000, ProfileDigest: task.StageDigest([]byte("test-only-profile"))}
}
func checkReply(c CheckConfig, text string) provider.Response {
	return provider.Response{Model: c.Model, Text: text, RequestID: "test-only-request", FinishReason: "stop", Usage: provider.Usage{Prompt: 2, Completion: 1, Total: 3}}
}
func checkFixture(t *testing.T) (*Policy, Proposal, []sources.Snapshot, EntailmentInput, string, *CheckPlan, *CheckPlan) {
	t.Helper()
	p, v, s := fixture(t)
	in, text, err := p.Inputs(encoded(t, v), []sources.Snapshot{s})
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEntailmentCheck(checkConfig(), in)
	if err != nil {
		t.Fatal(err)
	}
	sem, err := NewSemanticCheck(checkConfig(), v.TaskNo, text)
	if err != nil {
		t.Fatal(err)
	}
	return p, v, []sources.Snapshot{s}, in, text, e, sem
}
func captureEvidence(t *testing.T, p *CheckPlan, r provider.Response) *CheckEvidence {
	t.Helper()
	raw, err := p.CapturePayload(r)
	if err != nil {
		t.Fatal(err)
	}
	e, err := p.Evidence(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestReportModelCheckRequestsAndCapturedPolicy(t *testing.T) {
	p, v, snaps, in, text, ent, sem := checkFixture(t)
	if strings.Contains(ent.Request().Prompt, snaps[0].LicenseRef) || strings.Contains(sem.Request().Prompt, snaps[0].URL) || strings.Contains(sem.Request().Prompt, snaps[0].LicenseRef) {
		t.Fatal("内部授权/来源全文混入语义输入")
	}
	if !strings.Contains(ent.Request().Prompt, "sentence") && !strings.Contains(ent.Request().Prompt, "ProposalJSON") {
		t.Fatal("蕴含输入丢失")
	}
	r := ent.Request()
	r.Schema[0] = 'x'
	*r.Temperature = 1
	if !json.Valid(ent.Request().Schema) || *ent.Request().Temperature != 0 || ent.Request().MaxTokens != 64 || !ent.Request().Strict || ent.Request().Format != provider.FormatJSONSchema || ent.Request().Attempt != 1 {
		t.Fatal("请求副本或受限输出被篡改")
	}
	if ent.Units().Calls != 1 || ent.Units().Tokens != 1064 || ent.Kind() != CheckEntailment || ent.Config() != checkConfig() || !ValidDigest(ent.PolicyVersion()) {
		t.Fatal("资源/配置绑定错误")
	}
	if ent.PolicyVersion() == sem.PolicyVersion() {
		t.Fatal("两个判定策略未区分")
	}
	ev := captureEvidence(t, ent, checkReply(checkConfig(), `{"verdict":"pass","reason":"supported"}`))
	sv := captureEvidence(t, sem, checkReply(checkConfig(), `{"verdict":"pass","reason":"factual"}`))
	cfg := p.cfg
	cfg.EntailmentVersion = ent.PolicyVersion()
	cfg.SemanticVersion = sem.PolicyVersion()
	checked, err := NewCapturedPolicy(cfg, ev, sv)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		e, s, reason, err := checked.Review(context.Background(), encoded(t, v), v, snaps)
		if err != nil || e != Pass || s != Pass || reason != "" {
			t.Fatal("纯回放判定失败", e, s, reason, err)
		}
	}
	if got, err := ev.Entailment(context.Background(), in); err != nil || got != Pass {
		t.Fatal(err)
	}
	if got, err := sv.Semantic(context.Background(), text); err != nil || got != Pass {
		t.Fatal(err)
	}
	if _, err := ev.Semantic(context.Background(), text); err == nil {
		t.Fatal("蕴含充当语义")
	}
	if _, err := sv.Entailment(context.Background(), in); err == nil {
		t.Fatal("语义充当蕴含")
	}
	in.ProposalJSON += " "
	if got, err := ev.Entailment(context.Background(), in); err == nil || got != Uncertain {
		t.Fatal("异输入借用通过判定")
	}
	if got, err := sv.Semantic(context.Background(), text+"改动"); err == nil || got != Uncertain {
		t.Fatal("展示文本变化未失效")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v %+v %#v", ent, ent, ev, ev), snaps[0].Body) {
		t.Fatal("提示/正文泄漏")
	}
	if _, err := NewCapturedPolicy(cfg, ev, nil); err == nil {
		t.Fatal("通过蕴含但缺语义仍可放行")
	}
	blocked := captureEvidence(t, ent, checkReply(checkConfig(), `{"verdict":"block","reason":"unsupported"}`))
	deny, err := NewCapturedPolicy(cfg, blocked, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e, s, reason, err := deny.Review(context.Background(), encoded(t, v), v, snaps); err != nil || e != Block || s != Uncertain || reason != "entailment" {
		t.Fatal("未短路语义", err)
	}
	if _, err := NewCapturedPolicy(cfg, nil, sv); err == nil {
		t.Fatal("空蕴含未拒绝")
	}
	bad := cfg
	bad.SemanticVersion = "changed"
	if _, err := NewCapturedPolicy(bad, ev, sv); err == nil {
		t.Fatal("错语义版本通过")
	}
	bad = cfg
	bad.EntailmentVersion = "changed"
	if _, err := NewCapturedPolicy(bad, ev, sv); err == nil {
		t.Fatal("错蕴含版本通过")
	}
	if _, err := NewCapturedPolicy(cfg, sv, sv); err == nil {
		t.Fatal("错判定类型通过")
	}
	if _, err := NewCapturedPolicy(cfg, ev, ev); err == nil {
		t.Fatal("错语义类型通过")
	}
}

func TestReportModelCheckResponseFailClosed(t *testing.T) {
	_, _, _, _, _, ent, sem := checkFixture(t)
	for _, p := range []*CheckPlan{ent, sem} {
		reason := "supported"
		if p.kind == CheckSemantic {
			reason = "factual"
		}
		good := checkReply(checkConfig(), `{"verdict":"pass","reason":"`+reason+`"}`)
		for name, mutate := range map[string]func(*provider.Response){
			"模型": func(r *provider.Response) { r.Model = "other" }, "截断": func(r *provider.Response) { r.FinishReason = "length" }, "估计用量": func(r *provider.Response) { r.UsageEstimated = true },
			"负用量": func(r *provider.Response) { r.Usage.Total = -1 }, "超输出": func(r *provider.Response) { r.Usage.Completion = 65; r.Usage.Total = 67 }, "超输入": func(r *provider.Response) { r.Usage.Prompt = 1001; r.Usage.Total = 1002 },
			"用量和": func(r *provider.Response) { r.Usage.Total++ }, "缓存": func(r *provider.Response) { r.Usage.Cached = 3 }, "推理": func(r *provider.Response) { r.Usage.Reasoning = 2 }, "缺回执": func(r *provider.Response) { r.RequestID = "" },
			"空白回执":  func(r *provider.Response) { r.RequestID = " " },
			"回执控制符": func(r *provider.Response) { r.RequestID = "opaque\u0001id" },
			"缺输入用量": func(r *provider.Response) { r.Usage.Prompt = 0; r.Usage.Total = 1 },
			"缺输出用量": func(r *provider.Response) { r.Usage.Completion = 0; r.Usage.Total = 2 },
			"空":     func(r *provider.Response) { r.Text = "" }, "多字段": func(r *provider.Response) {
				r.Text = `{"verdict":"pass","reason":"` + reason + `","explanation":"测试自由解释"}`
			},
			"大小写": func(r *provider.Response) { r.Text = strings.Replace(r.Text, "verdict", "Verdict", 1) }, "重复": func(r *provider.Response) { r.Text = `{"verdict":"block","verdict":"pass","reason":"` + reason + `"}` },
			"理由不一致": func(r *provider.Response) { r.Text = `{"verdict":"pass","reason":"uncertain"}` }, "无判定": func(r *provider.Response) { r.Text = `{"verdict":null,"reason":null}` }, "未知枚举": func(r *provider.Response) { r.Text = `{"verdict":"approved","reason":"` + reason + `"}` },
		} {
			t.Run(string(p.Kind())+"/"+name, func(t *testing.T) {
				r := good
				mutate(&r)
				e := captureEvidence(t, p, r)
				if e.Decision().Verdict != Uncertain {
					t.Fatal("无效判定被放行", e.Decision())
				}
			})
		}
		for _, v := range []Verdict{Block, Uncertain} {
			why := map[Verdict]string{Block: "unsupported", Uncertain: "insufficient"}[v]
			if p.kind == CheckSemantic {
				why = map[Verdict]string{Block: "prohibited", Uncertain: "uncertain"}[v]
			}
			e := captureEvidence(t, p, checkReply(checkConfig(), `{"verdict":"`+string(v)+`","reason":"`+why+`"}`))
			if e.Decision().Verdict != v {
				t.Fatal("合法拒绝枚举丢失")
			}
		}
	}
}

func TestReportModelCheckBindingsAndBoundaries(t *testing.T) {
	p, v, snaps, in, text, ent, sem := checkFixture(t)
	scoped := task.Task{ID: 1, UserID: 1, No: v.TaskNo, SubjectID: v.ThemeID, SubjectType: task.Theme, AnalysisType: 1}
	if !ent.MatchesTask(scoped) || !sem.MatchesTask(scoped) {
		t.Fatal("合法任务范围未匹配")
	}
	foreign := scoped
	foreign.No = strings.Repeat("f", 32)
	if ent.MatchesTask(foreign) || sem.MatchesTask(foreign) || ent.OperationKey(foreign, 1) != "" {
		t.Fatal("异任务借用判定计划")
	}
	foreign = scoped
	foreign.SubjectID++
	if ent.MatchesTask(foreign) {
		t.Fatal("异主题借用蕴含计划")
	}
	if _, err := NewSemanticCheck(checkConfig(), "bad", text); err == nil {
		t.Fatal("无任务范围仍能构造")
	}
	other, err := NewSemanticCheck(checkConfig(), strings.Repeat("f", 32), text)
	if err != nil {
		t.Fatal(err)
	}
	otherEvidence := captureEvidence(t, other, checkReply(checkConfig(), `{"verdict":"pass","reason":"factual"}`))
	evForTask := captureEvidence(t, ent, checkReply(checkConfig(), `{"verdict":"pass","reason":"supported"}`))
	boundCfg := p.cfg
	boundCfg.EntailmentVersion = ent.PolicyVersion()
	boundCfg.SemanticVersion = other.PolicyVersion()
	if _, err := NewCapturedPolicy(boundCfg, evForTask, otherEvidence); err == nil {
		t.Fatal("跨任务组合两道判定")
	}
	badProfile := checkConfig()
	badProfile.ProfileDigest = "bad"
	if _, err := NewSemanticCheck(badProfile, v.TaskNo, text); err == nil {
		t.Fatal("缺直连身份仍创建判定")
	}
	mutated := *ent
	mutated.cfg = badProfile
	if mutated.PolicyVersion() != "" {
		t.Fatal("坏直连身份仍有策略版本")
	}
	if _, _, err := p.Inputs("{}", snaps); err == nil {
		t.Fatal("结构错误仍生成模型输入")
	}
	if _, _, err := p.Inputs(encoded(t, v), nil); err == nil {
		t.Fatal("缺证据仍生成模型输入")
	}
	for name, mutate := range map[string]func(*CheckConfig){"模型": func(c *CheckConfig) { c.Model = "" }, "备案": func(c *CheckConfig) { c.FilingNo = "" }, "版本": func(c *CheckConfig) { c.Version = "" }, "批准": func(c *CheckConfig) { c.ApprovalRef = "" }, "质量": func(c *CheckConfig) { c.QualityRef = "" }, "硬预算": func(c *CheckConfig) { c.HardBudgetRef = "" }, "输入上界": func(c *CheckConfig) { c.MaxInputTokens = 0 }, "上界过大": func(c *CheckConfig) { c.MaxInputTokens = 1 << 23 }, "控制字符": func(c *CheckConfig) { c.Model = "a\nb" }, "长度": func(c *CheckConfig) { c.Model = strings.Repeat("x", 129) }, "无效编码": func(c *CheckConfig) { c.Model = string([]byte{0xff}) }} {
		t.Run(name, func(t *testing.T) {
			c := checkConfig()
			mutate(&c)
			if _, err := NewSemanticCheck(c, v.TaskNo, text); err == nil {
				t.Fatal("缺准入元数据通过")
			}
			if _, err := CheckPolicyVersion(CheckEntailment, c); err == nil {
				t.Fatal("无效策略版本通过")
			}
		})
	}
	for _, bad := range []EntailmentInput{{}, {ProposalJSON: "{}", EvidenceJSON: "bad"}} {
		if _, err := NewEntailmentCheck(checkConfig(), bad); err == nil {
			t.Fatal("坏蕴含输入通过")
		}
	}
	for _, bad := range []string{"", string([]byte{0xff}), strings.Repeat("x", task.MaxStageResultBytes+1)} {
		if _, err := NewSemanticCheck(checkConfig(), v.TaskNo, bad); err == nil {
			t.Fatal("坏语义输入通过")
		}
	}
	if _, err := CheckPolicyVersion("unknown", checkConfig()); err == nil {
		t.Fatal("未知类型通过")
	}
	keyTask := task.Task{ID: 1, UserID: 1, No: v.TaskNo}
	if !pointsKey(ent.OperationKey(keyTask, 1)) || ent.OperationKey(keyTask, 1) == sem.OperationKey(keyTask, 1) || ent.OperationKey(keyTask, 1) == ent.OperationKey(keyTask, 2) {
		t.Fatal("固定操作域未区分")
	}
	if ent.OperationKey(task.Task{}, 1) != "" || ent.OperationKey(keyTask, 0) != "" {
		t.Fatal("坏操作键通过")
	}
	raw, err := ent.CapturePayload(checkReply(checkConfig(), `{"verdict":"pass","reason":"supported"}`))
	if err != nil {
		t.Fatal(err)
	}
	if ent.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "judge", InputDigest: ent.InputDigest(), Payload: string(raw)}) != nil {
		t.Fatal("隔离响应不能完成")
	}
	for _, bad := range []string{"{}", string(raw) + " ", strings.Replace(string(raw), "input_digest", "Input_digest", 1), strings.Replace(string(raw), ent.InputDigest(), strings.Repeat("f", 64), 1)} {
		if _, err := ent.Evidence(bad); err == nil {
			t.Fatal("坏封装通过")
		}
	}
	if ent.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "extract", InputDigest: ent.InputDigest(), Payload: string(raw)}) == nil {
		t.Fatal("阶段错位通过")
	}
	if ent.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "judge", InputDigest: sem.InputDigest(), Payload: string(raw)}) == nil {
		t.Fatal("输入错位通过")
	}
	for _, r := range []provider.Response{{Text: strings.Repeat("x", 4097)}, {Model: strings.Repeat("x", 129)}, {RequestID: strings.Repeat("x", 257)}} {
		if _, err := ent.CapturePayload(r); err == nil {
			t.Fatal("超限响应通过")
		}
	}
	var absent *CheckPlan
	var ev *CheckEvidence
	if absent.Valid() || absent.InputDigest() != "" || absent.PolicyVersion() != "" || absent.Kind() != "" || absent.Config() != (CheckConfig{}) || absent.Units() != (task.Units{}) || absent.Request().Model != "" || absent.StagePolicy().Valid() || absent.OperationKey(keyTask, 1) != "" {
		t.Fatal("空计划未关闭")
	}
	if _, err := absent.CapturePayload(provider.Response{}); err == nil {
		t.Fatal("空计划可捕获")
	}
	if _, err := absent.Evidence("{}"); err == nil {
		t.Fatal("空计划可还原")
	}
	if ev.Decision().Verdict != Uncertain {
		t.Fatal("空判定默认通过")
	}
	if _, err := ev.Semantic(context.Background(), text); err == nil {
		t.Fatal("空判定可用")
	}
	if _, err := ev.Entailment(context.Background(), in); err == nil {
		t.Fatal("空判定可用")
	}
	ev = captureEvidence(t, ent, checkReply(checkConfig(), `{"verdict":"pass","reason":"supported"}`))
	sv := captureEvidence(t, sem, checkReply(checkConfig(), `{"verdict":"pass","reason":"factual"}`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ev.Entailment(ctx, in); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := sv.Semantic(ctx, text); err != context.Canceled {
		t.Fatal(err)
	}
	if ent.StagePolicy().Check(ctx, task.StageValidationInput{}) != context.Canceled {
		t.Fatal("取消未传播")
	}
}
func pointsKey(s string) bool { return len(s) == 64 && ValidDigest(s) }
