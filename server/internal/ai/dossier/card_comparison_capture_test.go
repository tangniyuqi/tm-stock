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
)

func TestDossierCardComparisonResponseRejections(t *testing.T) {
	p := comparisonPlan(t)
	for _, tc := range []struct {
		name   string
		change func(*comparisonTestDocument)
	}{
		{"漏轴", func(d *comparisonTestDocument) { d.Axes = d.Axes[:6] }},
		{"重复轴", func(d *comparisonTestDocument) { d.Axes[1]["id"] = 1 }},
		{"越界轴", func(d *comparisonTestDocument) { d.Axes[0]["id"] = 8 }},
		{"浮点轴", func(d *comparisonTestDocument) { d.Axes[0]["id"] = 1.5 }},
		{"额外轴键", func(d *comparisonTestDocument) { d.Axes[0]["explanation"] = "自由推断" }},
		{"轴空值", func(d *comparisonTestDocument) { d.Axes[0]["left"] = nil }},
		{"轴键大小写", func(d *comparisonTestDocument) { delete(d.Axes[0], "left"); d.Axes[0]["Left"] = "supported" }},
		{"已知字段伪称未知", func(d *comparisonTestDocument) { d.Axes[0]["left"] = "unknown" }},
		{"字段部分支持仍对齐", func(d *comparisonTestDocument) { d.Axes[0]["left"] = "partial" }},
		{"字段不支持仍判不同", func(d *comparisonTestDocument) {
			d.Axes[0]["left"], d.Axes[0]["relation"] = "unsupported", "different"
			d.Wording = "different_wording"
		}},
		{"必核轴被忽略", func(d *comparisonTestDocument) { d.Axes[0]["relation"] = "not_applicable" }},
		{"非必核已有字段被忽略", func(d *comparisonTestDocument) { d.Axes[1]["relation"] = "not_applicable" }},
		{"差异与同义矛盾", func(d *comparisonTestDocument) { d.Axes[0]["relation"] = "different" }},
		{"事实冲突不在闭集", func(d *comparisonTestDocument) { d.Wording = "fact_conflict" }},
		{"自由关系", func(d *comparisonTestDocument) { d.Axes[0]["relation"] = "independent" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := p.CapturePayload(comparisonResponse(t, p, tc.change))
			if err != nil {
				t.Fatal("协议应在回放层拒绝", err)
			}
			if _, err := p.Review(context.Background(), raw); !errors.Is(err, ErrCardComparison) {
				t.Fatal("接受了无效两卡回执", err)
			}
		})
	}
	valid := comparisonResponse(t, p, nil)
	for _, tc := range []struct{ name, text string }{
		{"无JSON", "不是JSON"}, {"额外顶层键", strings.TrimSuffix(valid.Text, "}") + `,"independent":true}`},
		{"重复顶层键", strings.TrimSuffix(valid.Text, "}") + `,"wording":"same_meaning"}`},
		{"顶层空值", `{"axes":null,"wording":"same_meaning"}`}, {"根空值", "null"},
		{"尾随对象", valid.Text + "{}"}, {"重复轴键", strings.Replace(valid.Text, `"id":1`, `"id":1,"id":1`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid
			r.Text = tc.text
			raw, err := p.CapturePayload(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Review(context.Background(), raw); !errors.Is(err, ErrCardComparison) {
				t.Fatal("畸形模型JSON被接受", err)
			}
		})
	}
}

func TestDossierCardComparisonMissingCannotBeSupported(t *testing.T) {
	d, snaps := comparisonDraft(t, nil)
	d.Cards[0].Subject, d.Cards[0].EffectiveDate, d.Cards[1].EffectiveDate = Field{State: "unknown"}, Field{State: "unknown"}, Field{State: "unknown"}
	p, err := comparisonPreparation(t, comparisonConfig(), d, snaps).BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	r, _ := comparisonReview(t, p, nil)
	if r.Outcome() != "insufficient_context" || r.Axes()[0].Reason != "missing_field" || r.Axes()[1].Relation != "not_applicable" {
		t.Fatal("缺字段未保留未知")
	}
	for _, tc := range []struct {
		name   string
		change func(*comparisonTestDocument)
	}{
		{"补造未知字段", func(d *comparisonTestDocument) { d.Axes[0]["left"] = "supported" }},
		{"缺主体仍对齐", func(d *comparisonTestDocument) { d.Axes[0]["relation"] = "aligned" }},
		{"缺主体仍判不同", func(d *comparisonTestDocument) { d.Axes[0]["relation"] = "different"; d.Wording = "different_wording" }},
		{"有单边字段被忽略", func(d *comparisonTestDocument) { d.Axes[0]["relation"] = "not_applicable" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := p.CapturePayload(comparisonResponse(t, p, tc.change))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Review(context.Background(), raw); !errors.Is(err, ErrCardComparison) {
				t.Fatal("字段缺失被模型改写为已核", err)
			}
		})
	}
}

func TestDossierCardComparisonCaptureAndPolicy(t *testing.T) {
	p := comparisonPlan(t)
	r, raw := comparisonReview(t, p, nil)
	if r.InputDigest() != p.InputDigest() || r.DraftDigest() != p.draftDigest || r.PayloadDigest() != task.StageDigest(raw) || !p.StagePolicy().Valid() {
		t.Fatal("实际捕获包或策略未绑定")
	}
	in := task.StageValidationInput{Stage: "judge", InputDigest: p.InputDigest(), Payload: string(raw)}
	if err := p.StagePolicy().Check(context.Background(), in); err != nil {
		t.Fatal("原judge阶段不能回放已捕获结果", err)
	}
	for _, tc := range []struct{ stage, digest string }{{"semantic", p.InputDigest()}, {"judge", strings.Repeat("a", 64)}} {
		wrong := in
		wrong.Stage, wrong.InputDigest = tc.stage, tc.digest
		if !errors.Is(p.StagePolicy().Check(context.Background(), wrong), task.ErrStagePolicy) {
			t.Fatal("错误阶段或输入进入原策略")
		}
	}
	response := comparisonResponse(t, p, nil)
	response.Raw = []byte("不可转存的原始提供商数据")
	withRaw, err := p.CapturePayload(response)
	if err != nil || !bytes.Equal(withRaw, raw) || bytes.Contains(withRaw, response.Raw) {
		t.Fatal("原始提供商包混入阶段捕获", err)
	}
	response.RequestID += "-other"
	otherRaw, err := p.CapturePayload(response)
	if err != nil {
		t.Fatal(err)
	}
	otherReview, err := p.Review(context.Background(), otherRaw)
	if err != nil || r.PayloadDigest() == otherReview.PayloadDigest() {
		t.Fatal("只按相同判定而非实际包绑定", err)
	}
	// 允许模型按任意次序返回七轴，但呈现和状态必须始终采用已绑定的固定次序。
	reordered, _ := comparisonReview(t, p, func(d *comparisonTestDocument) { slices.Reverse(d.Axes) })
	if !slices.Equal(reordered.Axes(), r.Axes()) {
		t.Fatal("模型输出顺序改变固定轴")
	}
	var packet cardComparisonPacket
	if json.Unmarshal(raw, &packet) != nil {
		t.Fatal("捕获包解析失败")
	}
	for _, tc := range []struct {
		name   string
		change func(*cardComparisonPacket)
	}{
		{"输入摘要", func(p *cardComparisonPacket) { p.InputDigest = strings.Repeat("a", 64) }},
		{"旧协议", func(p *cardComparisonPacket) { p.Protocol = SupportVersion }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := packet
			tc.change(&bad)
			b, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Review(context.Background(), b); !errors.Is(err, ErrCardComparison) {
				t.Fatal("不同计划包被接受", err)
			}
		})
	}
	for _, bad := range [][]byte{nil, append([]byte(" "), raw...), append(bytes.TrimSuffix(raw, []byte("}")), []byte(`,"extra":1}`)...)} {
		if _, err := p.Review(context.Background(), bad); !errors.Is(err, ErrCardComparison) {
			t.Fatal("不规范捕获包被接受", err)
		}
	}
	d, snaps := comparisonDraft(t, nil)
	old := supportPlan(t, d, snaps, "left_card", "right_card")
	if _, err := old.Review(context.Background(), raw); !errors.Is(err, ErrSupportReview) {
		t.Fatal("旧字段协议解释了新两卡协议", err)
	}
}

func TestDossierCardComparisonUsageRejections(t *testing.T) {
	p := comparisonPlan(t)
	for _, tc := range []struct {
		name   string
		change func(*provider.Response)
	}{
		{"错模型", func(r *provider.Response) { r.Model = "other-model" }},
		{"不完整输出", func(r *provider.Response) { r.FinishReason = "length" }},
		{"估计用量", func(r *provider.Response) { r.UsageEstimated = true }},
		{"缺请求编号", func(r *provider.Response) { r.RequestID = "" }},
		{"缺用量", func(r *provider.Response) { r.Usage = provider.Usage{} }},
		{"总量错误", func(r *provider.Response) { r.Usage.Total++ }},
		{"超过输入", func(r *provider.Response) {
			r.Usage.Prompt = int(p.Config().Model.MaxInputTokens) + 1
			r.Usage.Total = r.Usage.Prompt + r.Usage.Completion
		}},
		{"超过输出", func(r *provider.Response) {
			r.Usage.Completion = p.Config().Model.MaxOutputTokens + 1
			r.Usage.Total = r.Usage.Prompt + r.Usage.Completion
		}},
		{"缓存超量", func(r *provider.Response) { r.Usage.Cached = r.Usage.Prompt + 1 }},
		{"推理负数", func(r *provider.Response) { r.Usage.Reasoning = -1 }},
		{"正文超限", func(r *provider.Response) { r.Text = strings.Repeat("a", 64<<10+1) }},
		{"模型超限", func(r *provider.Response) { r.Model = strings.Repeat("a", 129) }},
		{"编号超限", func(r *provider.Response) { r.RequestID = strings.Repeat("a", 257) }},
		{"非法UTF8", func(r *provider.Response) { r.Text = string([]byte{0xff}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := comparisonResponse(t, p, nil)
			tc.change(&r)
			raw, err := p.CapturePayload(r)
			if err == nil {
				_, err = p.Review(context.Background(), raw)
			}
			if !errors.Is(err, ErrCardComparison) {
				t.Fatal("无效完整性或用量被接受", err)
			}
		})
	}
}

func TestDossierCardComparisonCopiesAndPresentation(t *testing.T) {
	p := comparisonPlan(t)
	r, _ := comparisonReview(t, p, nil)
	axes, req := r.Axes(), p.Request()
	axes[0].Relation, req.Schema[0], *req.Temperature = "different", 'x', 1
	if r.Axes()[0].Relation != "aligned" || p.Request().Schema[0] != '{' || *p.Request().Temperature != 0 {
		t.Fatal("返回副本改变封闭计划或结果")
	}
	d, snaps := comparisonDraft(t, nil)
	prep := comparisonPreparation(t, comparisonConfig(), d, snaps)
	materials := prep.Materials()
	materials[0].URL = "https://changed.example.test/"
	if prep.Materials()[0].URL == materials[0].URL {
		t.Fatal("资料副本改变准入绑定")
	}
	for _, tc := range []struct {
		name   string
		change func(*CardComparisonReview)
	}{
		{"输入", func(r *CardComparisonReview) { r.inputDigest = strings.Repeat("a", 64) }},
		{"档案", func(r *CardComparisonReview) { r.draftDigest = strings.Repeat("a", 64) }},
		{"包摘要缺失", func(r *CardComparisonReview) { r.payloadDigest = "" }},
		{"伪造独立来源", func(r *CardComparisonReview) { r.origins = "independent" }},
		{"漏轴", func(r *CardComparisonReview) { r.axes = r.axes[:6] }},
		{"字段", func(r *CardComparisonReview) { r.axes[0].Field = "unit" }},
		{"原因", func(r *CardComparisonReview) { r.axes[0].Reason = "自由说明" }},
		{"结果", func(r *CardComparisonReview) { r.outcome = "fact_conflict" }},
		{"数量", func(r *CardComparisonReview) { r.resolved++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := *r
			bad.axes = r.Axes()
			tc.change(&bad)
			if _, err := p.VisibleReview(&bad); !errors.Is(err, ErrCardComparison) {
				t.Fatal("篡改内部呈现被接受", err)
			}
		})
	}
	for _, value := range []any{*r, r.Axes()[0]} {
		if strings.Contains(fmt.Sprintf("%+v %#v", value, value), p.InputDigest()) {
			t.Fatal("结果格式化泄露内部定位")
		}
	}
}

func TestDossierCardComparisonNilAndCancellation(t *testing.T) {
	var prep *CardComparisonPreparation
	var p *CardComparisonPlan
	var r *CardComparisonReview
	if prep.Valid() || prep.Materials() != nil || prep.Units() != (task.Units{}) || p.Valid() || p.Config() != (CardComparisonConfig{}) || p.StepNo() != 0 || p.Units() != (task.Units{}) ||
		p.InputDigest() != "" || p.ComponentDigest() != "" || p.PolicyDigest() != "" || p.MatchesTask(supportTaskFixture()) || p.OperationKey(supportTaskFixture(), 5) != "" || p.StagePolicy().Valid() || p.Request().Prompt != "" ||
		r.InputDigest() != "" || r.DraftDigest() != "" || r.PayloadDigest() != "" || r.Outcome() != "" || r.Wording() != "" || r.StatementOriginRelation() != "" || r.Axes() != nil || r.ResolvedCount() != 0 {
		t.Fatal("空值意外提供计划、结果或执行能力")
	}
	if _, err := prep.ComponentDigest(task.DeepAnalysisType); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	if _, err := prep.BindTask(context.Background(), supportTaskFixture()); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	if _, err := p.CapturePayload(provider.Response{}); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	if _, err := p.VisibleReview(r); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	d, snaps := comparisonDraft(t, nil)
	prep = comparisonPreparation(t, comparisonConfig(), d, snaps)
	p, err := prep.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	_, raw := comparisonReview(t, p, nil)
	b, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, canceled} {
		expect := ErrCardComparison
		if ctx != nil {
			expect = context.Canceled
		}
		if _, err := PrepareCardComparison(ctx, comparisonConfig(), "仅测试专题", b, snaps, []string{"left_card", "right_card"}); !errors.Is(err, expect) {
			t.Fatal("准备未保留取消或空上下文拒绝", err)
		}
		if _, err := prep.BindTask(ctx, supportTaskFixture()); !errors.Is(err, expect) {
			t.Fatal("绑定未保留取消或空上下文拒绝", err)
		}
		if _, err := p.Review(ctx, raw); !errors.Is(err, expect) {
			t.Fatal("回放未保留取消或空上下文拒绝", err)
		}
		if err := p.StagePolicy().Check(ctx, task.StageValidationInput{Stage: "judge", InputDigest: p.InputDigest(), Payload: string(raw)}); !errors.Is(err, expect) {
			t.Fatal("策略未保留取消或空上下文拒绝", err)
		}
	}
}
