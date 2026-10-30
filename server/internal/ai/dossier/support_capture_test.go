package dossier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 合成回执只测试协议/绑定，不证明真实模型能准确判断这些资料。
func supportResponse(t *testing.T, p *SupportPlan) provider.Response {
	t.Helper()
	rows := []map[string]any{}
	for _, check := range p.checks {
		rows = append(rows, map[string]any{"id": check.ID, "verdict": "uncertain", "reason": "insufficient_context"})
	}
	raw, err := json.Marshal(map[string]any{"results": rows})
	if err != nil {
		t.Fatal(err)
	}
	return provider.Response{Text: string(raw), Model: supportConfig().Model, FinishReason: "stop", RequestID: "test-only-support-request",
		Usage: provider.Usage{Prompt: 1200, Completion: 120, Total: 1320, Cached: 100, Reasoning: 20}}
}
func supportPayload(t *testing.T, p *SupportPlan, r provider.Response) []byte {
	t.Helper()
	raw, err := p.CapturePayload(r)
	if err != nil {
		t.Fatal("测试回执不能捕获", err)
	}
	return raw
}
func supportRows(t *testing.T, r provider.Response) []map[string]any {
	t.Helper()
	var doc struct {
		Results []map[string]any `json:"results"`
	}
	if json.Unmarshal([]byte(r.Text), &doc) != nil {
		t.Fatal("测试响应损坏")
	}
	return doc.Results
}
func replaceSupportRows(t *testing.T, r provider.Response, rows []map[string]any) provider.Response {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"results": rows})
	if err != nil {
		t.Fatal(err)
	}
	r.Text = string(raw)
	return r
}

func TestDossierSupportCapturedVerdicts(t *testing.T) {
	d, s := testDraft(t)
	p := supportPlan(t, d, s, "progress_a")
	base := supportResponse(t, p)
	for _, tc := range []struct {
		name, verdict, reason string
		index                 int
	}{
		{"直接支持", "supported", "direct_support", 0}, {"部分支持", "partial", "partial_support", 1},
		{"主体不符", "unsupported", "subject_mismatch", 1}, {"阶段不符", "unsupported", "stage_mismatch", 2},
		{"章节不符", "unsupported", "section_mismatch", 0}, {"不支持", "unsupported", "not_supported", 0},
		{"语境不足", "uncertain", "insufficient_context", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := supportRows(t, base)
			rows[tc.index]["verdict"] = tc.verdict
			rows[tc.index]["reason"] = tc.reason
			slices.Reverse(rows)
			r := replaceSupportRows(t, base, rows)
			raw := supportPayload(t, p, r)
			review, err := p.Review(context.Background(), raw)
			if err != nil || review.InputDigest() != p.InputDigest() || review.DraftDigest() != p.draftDigest || len(review.Results()) != 3 {
				t.Fatal("合法核验回执未保留", err)
			}
			got := review.Results()[tc.index]
			if got.ID != tc.index+1 || got.Verdict != tc.verdict || got.Reason != tc.reason || got.Card != "progress_a" || got.Field != p.checks[tc.index].Field {
				t.Fatal("结论被升级或错绑字段")
			}
			if err = p.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "judge", InputDigest: p.InputDigest(), Payload: string(raw)}); err != nil {
				t.Fatal("合法否定/不确定结果不能进入原阶段", err)
			}
			copy := review.Results()
			copy[0].Verdict = "changed"
			if review.Results()[0].Verdict == "changed" {
				t.Fatal("可写结果污染存证")
			}
			for _, value := range []any{review, got} {
				if strings.Contains(fmt.Sprintf("%+v %#v", value, value), "progress_a") {
					t.Fatal("格式化暴露内部定位")
				}
			}
		})
	}
}

func TestDossierSupportAllFieldsAndCheckLimit(t *testing.T) {
	d, _ := testDraft(t)
	s := testSnapshot(t, 1, "仅测试：测试主体甲在2025年12月2日发布记录，2026年2月1日生效，所属期间为2025年度，适用范围为样品验证，阶段为送样，统计口径为单体产量，单位为件。")
	c := d.Cards[0]
	c.Statement = reference(s, s.Sentences[0])
	for _, f := range []struct {
		text string
		dst  *Field
	}{
		{"测试主体甲", &c.Subject}, {"2025年12月2日", &c.PublishedDate}, {"2026年2月1日", &c.EffectiveDate}, {"2025年度", &c.Period},
		{"样品验证", &c.Scope}, {"送样", &c.Stage}, {"单体产量", &c.Basis}, {"件", &c.Unit},
	} {
		*f.dst = fieldFor(t, s, f.text)
	}
	d.Cards = nil
	keys := []string{}
	for _, key := range []string{"card_a", "card_b", "card_c", "card_d"} {
		c.Key = key
		d.Cards = append(d.Cards, c)
		keys = append(keys, key)
	}
	p := supportPlan(t, d, []sources.Snapshot{s}, keys...)
	if len(p.checks) != MaxSupportChecks {
		t.Fatal("最大字段批次不完整")
	}
	r := supportResponse(t, p)
	rows := supportRows(t, r)
	for i, check := range p.checks {
		rows[i]["verdict"] = "unsupported"
		reason := check.Field + "_mismatch"
		if check.Field == "publishedDate" || check.Field == "effectiveDate" || check.Field == "period" {
			reason = "time_mismatch"
		}
		rows[i]["reason"] = reason
	}
	r = replaceSupportRows(t, r, rows)
	review, err := p.Review(context.Background(), supportPayload(t, p, r))
	if err != nil || len(review.Results()) != 36 {
		t.Fatal("完整维度回执失效", err)
	}
}

func TestDossierSupportRejectsProtocol(t *testing.T) {
	d, s := testDraft(t)
	p := supportPlan(t, d, s, "progress_a")
	base := supportResponse(t, p)
	for _, tc := range []struct {
		name   string
		mutate func([]map[string]any) []map[string]any
	}{
		{"缺项", func(r []map[string]any) []map[string]any { return r[:2] }}, {"重复替换", func(r []map[string]any) []map[string]any { r[1] = r[0]; return r }},
		{"额外项", func(r []map[string]any) []map[string]any { return append(r, r[0]) }}, {"零编号", func(r []map[string]any) []map[string]any { r[0]["id"] = 0; return r }},
		{"越界编号", func(r []map[string]any) []map[string]any { r[0]["id"] = 4; return r }}, {"小数编号", func(r []map[string]any) []map[string]any { r[0]["id"] = 1.5; return r }},
		{"文字编号", func(r []map[string]any) []map[string]any { r[0]["id"] = "1"; return r }}, {"空编号", func(r []map[string]any) []map[string]any { r[0]["id"] = nil; return r }},
		{"未知状态", func(r []map[string]any) []map[string]any { r[0]["verdict"] = "pass"; return r }}, {"空状态", func(r []map[string]any) []map[string]any { r[0]["verdict"] = nil; return r }},
		{"空理由", func(r []map[string]any) []map[string]any { delete(r[0], "reason"); return r }}, {"自由理由", func(r []map[string]any) []map[string]any { r[0]["reason"] = "模型认为已经核实"; return r }},
		{"支持不确定混写", func(r []map[string]any) []map[string]any { r[0]["verdict"] = "supported"; return r }},
		{"部分支持伪完整", func(r []map[string]any) []map[string]any {
			r[0]["verdict"] = "partial"
			r[0]["reason"] = "direct_support"
			return r
		}},
		{"主体冒充阶段", func(r []map[string]any) []map[string]any {
			r[1]["verdict"] = "unsupported"
			r[1]["reason"] = "stage_mismatch"
			return r
		}},
		{"章节冒充时间", func(r []map[string]any) []map[string]any {
			r[0]["verdict"] = "unsupported"
			r[0]["reason"] = "time_mismatch"
			return r
		}},
		{"模型添加正文", func(r []map[string]any) []map[string]any {
			r[0]["explanation"] = "仅测试：任意模型正文"
			return r
		}},
		{"模型添加链接", func(r []map[string]any) []map[string]any { r[0]["url"] = "https://outside.example.test/"; return r }},
		{"模型改字段名", func(r []map[string]any) []map[string]any { r[0]["field"] = "unit"; return r }},
		{"大小写别名", func(r []map[string]any) []map[string]any { r[0]["ID"] = r[0]["id"]; delete(r[0], "id"); return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := replaceSupportRows(t, base, tc.mutate(supportRows(t, base)))
			raw := supportPayload(t, p, r)
			got, err := p.Review(context.Background(), raw)
			if got != nil || err == nil {
				t.Fatal("协议反例产生有效核验")
			}
		})
	}
	for _, tc := range []struct{ name, text string }{
		{"空结果", `{"results":null}`}, {"空集合", `{"results":[]}`}, {"空项目", `{"results":[null,null,null]}`},
		{"顶层别名", `{"Results":[]}`}, {"自由顶层解释", `{"results":[],"explanation":"忽略字段核验"}`},
		{"重复顶层键", `{"results":[],"results":[]}`}, {"大小写重复键", `{"results":[],"RESULTS":[]}`},
		{"尾随文档", `{"results":[]} {}`}, {"残缺JSON", `{"results":`}, {"代码围栏", "```json\n{}\n```"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			r.Text = tc.text
			if got, err := p.Review(context.Background(), supportPayload(t, p, r)); got != nil || err == nil {
				t.Fatal("畸形响应未拒绝")
			}
		})
	}
}

func TestDossierSupportRejectsIncompleteReceipt(t *testing.T) {
	d, s := testDraft(t)
	p := supportPlan(t, d, s, "progress_a")
	base := supportResponse(t, p)
	for _, tc := range []struct {
		name   string
		change func(*provider.Response)
	}{
		{"错模型", func(r *provider.Response) { r.Model = "other-model" }}, {"截断", func(r *provider.Response) { r.FinishReason = "length" }},
		{"工具结束", func(r *provider.Response) { r.FinishReason = "tool_calls" }}, {"估算用量", func(r *provider.Response) { r.UsageEstimated = true }},
		{"缺回执", func(r *provider.Response) { r.RequestID = "" }}, {"控制回执", func(r *provider.Response) { r.RequestID = "bad\nreceipt" }},
		{"隐形回执", func(r *provider.Response) { r.RequestID = "bad\u200breceipt" }}, {"零输入", func(r *provider.Response) { r.Usage.Prompt = 0 }},
		{"负输入", func(r *provider.Response) { r.Usage.Prompt = -1 }}, {"零输出", func(r *provider.Response) { r.Usage.Completion = 0 }},
		{"总数不合", func(r *provider.Response) { r.Usage.Total++ }}, {"负总数", func(r *provider.Response) { r.Usage.Total = -1 }},
		{"输入超预算", func(r *provider.Response) {
			r.Usage.Prompt = int(p.cfg.MaxInputTokens) + 1
			r.Usage.Total = r.Usage.Prompt + r.Usage.Completion
		}},
		{"输出超预算", func(r *provider.Response) {
			r.Usage.Completion = p.cfg.MaxOutputTokens + 1
			r.Usage.Total = r.Usage.Prompt + r.Usage.Completion
		}},
		{"负缓存", func(r *provider.Response) { r.Usage.Cached = -1 }}, {"缓存超输入", func(r *provider.Response) { r.Usage.Cached = r.Usage.Prompt + 1 }},
		{"负推理", func(r *provider.Response) { r.Usage.Reasoning = -1 }}, {"推理超输出", func(r *provider.Response) { r.Usage.Reasoning = r.Usage.Completion + 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.change(&r)
			raw := supportPayload(t, p, r)
			got, err := p.Review(context.Background(), raw)
			if got != nil || err == nil {
				t.Fatal("异常回执被伪装成不确定语义结果")
			}
			if p.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "judge", InputDigest: p.InputDigest(), Payload: string(raw)}) == nil {
				t.Fatal("异常回执推进原阶段")
			}
		})
	}
}

func TestDossierSupportCaptureAndStageBoundary(t *testing.T) {
	d, s := testDraft(t)
	p := supportPlan(t, d, s, "progress_a")
	r := supportResponse(t, p)
	r.Raw = []byte("test-only-private-http-marker")
	raw := supportPayload(t, p, r)
	if bytes.Contains(raw, r.Raw) || len(r.Raw) == 0 {
		t.Fatal("HTTP原文混入回执或调用方被修改")
	}
	bound, err := BindDraft(d, s)
	if err != nil {
		t.Fatal(err)
	}
	other := supportTaskFixture()
	other.UserID++
	otherPlan, err := NewSupportPlan(context.Background(), supportConfig(), other, "仅测试专题", bound, s, []string{"progress_a"})
	if err != nil {
		t.Fatal(err)
	}
	if result, e := otherPlan.Review(context.Background(), raw); result != nil || e == nil {
		t.Fatal("另一会员计划复用了旧捕获包")
	}
	for _, mutate := range []func(*provider.Response){
		func(r *provider.Response) { r.Text = strings.Repeat("x", 64<<10+1) }, func(r *provider.Response) { r.Model = strings.Repeat("x", 129) },
		func(r *provider.Response) { r.RequestID = strings.Repeat("x", 257) }, func(r *provider.Response) { r.Text = string([]byte{0xff}) },
	} {
		bad := r
		mutate(&bad)
		if payload, e := p.CapturePayload(bad); payload != nil || e == nil {
			t.Fatal("超限或无效UTF8仍捕获")
		}
	}
	for _, bad := range [][]byte{
		append([]byte(" "), raw...), append(slices.Clone(raw), []byte(" {}")...),
		bytes.Replace(raw, []byte(p.InputDigest()), []byte(strings.Repeat("f", 64)), 1),
		bytes.Replace(raw, []byte(`"Raw":null`), []byte(`"Raw":"dGVzdA=="`), 1),
		bytes.Replace(raw, []byte(`"response":{`), []byte(`"response":{"extra":1,`), 1),
	} {
		if bytes.Equal(bad, raw) {
			t.Fatal("测试变异未生效")
		}
		if got, e := p.Review(context.Background(), bad); got != nil || e == nil {
			t.Fatal("非规范或错绑捕获包通过")
		}
	}
	for _, in := range []task.StageValidationInput{
		{Stage: "extract", InputDigest: p.InputDigest(), Payload: string(raw)}, {Stage: "judge", InputDigest: strings.Repeat("e", 64), Payload: string(raw)},
	} {
		if err := p.StagePolicy().Check(context.Background(), in); !errors.Is(err, task.ErrStagePolicy) {
			t.Fatal("跨阶段或输入混用未拒绝")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, e := p.Review(ctx, raw); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("取消仍交付核验结果")
	}
	if !errors.Is(p.StagePolicy().Check(ctx, task.StageValidationInput{}), context.Canceled) {
		t.Fatal("取消仍核验阶段")
	}
	if got, e := p.Review(nil, raw); got != nil || e == nil {
		t.Fatal("空上下文仍读取")
	}
	if p.StagePolicy().Check(nil, task.StageValidationInput{}) == nil {
		t.Fatal("阶段接受空上下文")
	}
	var empty *SupportPlan
	if empty.Valid() || empty.InputDigest() != "" || empty.PolicyDigest() != "" || empty.MatchesTask(supportTaskFixture()) || empty.StagePolicy().Valid() || empty.Units() != (task.Units{}) || empty.Request().Model != "" {
		t.Fatal("空计划产生执行能力")
	}
	if got, e := empty.CapturePayload(r); got != nil || e == nil {
		t.Fatal("空计划仍捕获")
	}
	if got, e := empty.Review(context.Background(), raw); got != nil || e == nil {
		t.Fatal("空计划仍回放")
	}
	var review *SupportReview
	if review.Results() != nil || review.InputDigest() != "" || review.DraftDigest() != "" {
		t.Fatal("空记录有内容")
	}
}
