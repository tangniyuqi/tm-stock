package dossier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func comparisonConfig() CardComparisonConfig {
	return CardComparisonConfig{Model: supportConfig(), Focus: "measurement", ReviewStep: 5}
}

func comparisonFields(c *FactCard) []*Field {
	return []*Field{&c.Subject, &c.EffectiveDate, &c.Period, &c.Scope, &c.Stage, &c.Basis, &c.Unit}
}

// 全部为合成资料；回执由测试固定给出，只验证协议，不充当语义准确率样本。
func comparisonDraft(t *testing.T, rightValues []string) (Draft, []sources.Snapshot) {
	t.Helper()
	values := []string{"测试集团甲", "2026年1月1日", "2025年度", "测试子公司甲", "试生产", "测试合并范围", "测试台"}
	d := Draft{Protocol: DraftVersion, ThemeID: 7, Version: 1}
	var snaps []sources.Snapshot
	for i, key := range []string{"left_card", "right_card"} {
		v := values
		if i == 1 && rightValues != nil {
			v = rightValues
		}
		body := "仅测试：" + strings.Join(v, "，") + "，发布于2026年2月1日。仅测试背景：这些字段不是整个集团的全部业务。"
		s := testSnapshot(t, int64(i+1), body)
		s.SourceID = int64(7 + i)
		s.URL = fmt.Sprintf("https://example.test/test-only-%d", i)
		s.LicenseTag = "仅供合成许可标记"
		c := FactCard{Key: key, Section: "progress", Statement: reference(s, s.Sentences[0]), PublishedDate: fieldFor(t, s, "2026年2月1日")}
		for j, dst := range comparisonFields(&c) {
			*dst = fieldFor(t, s, v[j])
		}
		d.Cards = append(d.Cards, c)
		snaps = append(snaps, s)
	}
	d.CreatedAt = snaps[1].FetchedAt.Add(time.Minute).UnixMilli()
	return d, snaps
}

func comparisonPreparation(t *testing.T, cfg CardComparisonConfig, d Draft, snaps []sources.Snapshot) *CardComparisonPreparation {
	t.Helper()
	b, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal("合成两卡档案绑定失败", err)
	}
	p, err := PrepareCardComparison(context.Background(), cfg, "仅测试专题", b, snaps, []string{"right_card", "left_card"})
	if err != nil {
		t.Fatal("合成两卡准备失败", err)
	}
	return p
}

func comparisonPlan(t *testing.T) *CardComparisonPlan {
	t.Helper()
	d, snaps := comparisonDraft(t, nil)
	p, err := comparisonPreparation(t, comparisonConfig(), d, snaps).BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal("合成两卡任务绑定失败", err)
	}
	return p
}

type comparisonTestDocument struct {
	Axes    []map[string]any `json:"axes"`
	Wording string           `json:"wording"`
}

func comparisonResponse(t *testing.T, p *CardComparisonPlan, change func(*comparisonTestDocument)) provider.Response {
	t.Helper()
	doc := comparisonTestDocument{Wording: "same_meaning"}
	for _, a := range p.axes {
		left, right, relation := "supported", "supported", "aligned"
		if a.LeftCheckID == 0 {
			left, relation = "unknown", "uncertain"
		}
		if a.RightCheckID == 0 {
			right, relation = "unknown", "uncertain"
		}
		if !a.Required && a.LeftCheckID == 0 && a.RightCheckID == 0 {
			relation = "not_applicable"
		}
		doc.Axes = append(doc.Axes, map[string]any{"id": a.ID, "left": left, "right": right, "relation": relation})
	}
	if change != nil {
		change(&doc)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return provider.Response{Text: string(raw), Model: p.Config().Model.Model, FinishReason: "stop", RequestID: "test-only-two-card-request", Usage: provider.Usage{Prompt: 1500, Completion: 500, Total: 2000, Cached: 100}}
}

func comparisonReview(t *testing.T, p *CardComparisonPlan, change func(*comparisonTestDocument)) (*CardComparisonReview, []byte) {
	t.Helper()
	raw, err := p.CapturePayload(comparisonResponse(t, p, change))
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Review(context.Background(), raw)
	if err != nil {
		t.Fatal("合成两卡回执被拒绝", err)
	}
	return r, raw
}

func TestDossierCardComparisonInputBinding(t *testing.T) {
	d, snaps := comparisonDraft(t, nil)
	unused := testSnapshot(t, 3, "仅测试未选资料：不可出现在两卡模型输入。")
	c := d.Cards[0]
	c.Key, c.Statement = "unused_card", reference(unused, unused.Sentences[0])
	c.PublishedDate = Field{State: "unknown"}
	for _, dst := range comparisonFields(&c) {
		*dst = Field{State: "unknown"}
	}
	d.Cards = append(d.Cards, c)
	snaps = append(snaps, unused)
	prep := comparisonPreparation(t, comparisonConfig(), d, snaps)
	p, err := prep.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	r := p.Request()
	if !p.Valid() || !prep.Valid() || p.StepNo() != 5 || !p.MatchesTask(supportTaskFixture()) || len(p.InputDigest()) != 64 || len(p.PolicyDigest()) != 64 || len(p.ComponentDigest()) != 64 ||
		r.Model != comparisonConfig().Model.Model || r.Step != 5 || r.TaskID != supportTaskFixture().No || r.Attempt != 1 || r.Format != provider.FormatJSONSchema || !r.Strict || r.Temperature == nil || *r.Temperature != 0 || !json.Valid(r.Schema) {
		t.Fatal("没有绑定原任务或严格受控请求")
	}
	for _, s := range snaps[:2] {
		if !strings.Contains(r.Prompt, s.Body) || !strings.Contains(r.Prompt, s.Hash) || strings.Contains(r.Prompt, s.LicenseRef) || strings.Contains(r.Prompt, s.LicenseTag) || strings.Contains(r.Prompt, s.URL) {
			t.Fatal("完整证据缺失或模型输入泄露控制信息")
		}
	}
	if strings.Contains(r.Prompt, unused.Body) || strings.Contains(r.Prompt, "userId") || strings.Contains(r.Prompt, comparisonConfig().Model.ApprovalRef) || len(prep.Materials()) != 2 || p.Units() != prep.Units() || p.Units().Calls != 1 {
		t.Fatal("未选资料隔离或容量绑定失效")
	}
	// 发布日期保留为证据，但不得自动变为所属期间。
	if !strings.Contains(r.Prompt, "publishedDate") || p.axes[2].Field != "period" || p.axes[2].LeftCheckID == p.baseCheckIDForTest(t, "publishedDate") {
		t.Fatal("业务时间混同发布日期")
	}
	slices.Reverse(snaps)
	slices.Reverse(d.Cards)
	second := comparisonPreparation(t, comparisonConfig(), d, snaps)
	p2, err := second.BindTask(context.Background(), supportTaskFixture())
	if err != nil || p2.InputDigest() != p.InputDigest() || p2.OperationKey(supportTaskFixture(), 5) != p.OperationKey(supportTaskFixture(), 5) {
		t.Fatal("输入顺序改变了规范计划")
	}
	b, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	b.cards[0].Section = "policy"
	b.cards[0].Subject.Text = "篡改非规范副本"
	p3, err := PrepareCardComparison(context.Background(), comparisonConfig(), "仅测试专题", b, snaps, []string{"left_card", "right_card"})
	if err != nil || p3.digest != prep.digest {
		t.Fatal("未从已绑定规范档案重建")
	}
	for _, value := range []any{p, *prep, p.Config()} {
		if text := fmt.Sprintf("%+v %#v", value, value); strings.Contains(text, "测试集团甲") || strings.Contains(text, supportTaskFixture().No) || strings.Contains(text, comparisonConfig().Model.ApprovalRef) {
			t.Fatal("格式化泄露任务、资料或批准身份")
		}
	}
}

// 仅供断言输入中字段的真实编号，不增加生产公开入口。
func (p *CardComparisonPlan) baseCheckIDForTest(t *testing.T, field string) int {
	t.Helper()
	var doc struct {
		Evidence struct {
			Checks []supportCheck
		}
	}
	if json.Unmarshal([]byte(p.Request().Prompt), &doc) != nil {
		t.Fatal("请求不是JSON")
	}
	for _, c := range doc.Evidence.Checks {
		if c.Card == "left_card" && c.Field == field {
			return c.ID
		}
	}
	return 0
}

func TestDossierCardComparisonRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name, focus string
		policy      bool
		required    []string
	}{
		{"定义", "definition", false, []string{"subject", "scope"}},
		{"进展", "progress", false, []string{"subject", "period", "scope", "stage"}},
		{"测量", "measurement", false, []string{"subject", "period", "scope", "stage", "basis", "unit"}},
		{"政策另核生效", "definition", true, []string{"subject", "effectiveDate", "scope"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, snaps := comparisonDraft(t, nil)
			if tc.policy {
				d.Cards[1].Section = "policy"
			}
			for i := range d.Cards {
				for _, dst := range comparisonFields(&d.Cards[i]) {
					*dst = Field{State: "unknown"}
				}
			}
			cfg := comparisonConfig()
			cfg.Focus = tc.focus
			p, err := comparisonPreparation(t, cfg, d, snaps).BindTask(context.Background(), supportTaskFixture())
			if err != nil {
				t.Fatal(err)
			}
			r, _ := comparisonReview(t, p, nil)
			if r.Outcome() != "insufficient_context" || r.ResolvedCount() != 0 {
				t.Fatal("全未知被推成已对齐")
			}
			for i, a := range p.axes {
				if a.Required != slices.Contains(tc.required, a.Field) || a.LeftCheckID != 0 || a.RightCheckID != 0 {
					t.Fatal("必核轴或缺失字段绑定错误")
				}
				expected := "not_applicable"
				if a.Required {
					expected = "uncertain"
				}
				if r.Axes()[i].Relation != expected {
					t.Fatal("未知必核字段被忽略")
				}
				if a.Required {
					raw, err := p.CapturePayload(comparisonResponse(t, p, func(doc *comparisonTestDocument) { doc.Axes[i]["relation"] = "not_applicable" }))
					if err != nil {
						t.Fatal(err)
					}
					if _, err := p.Review(context.Background(), raw); !errors.Is(err, ErrCardComparison) {
						t.Fatal("双方未知的必核字段被忽略", err)
					}
				}
			}
		})
	}
}

func TestDossierCardComparisonScopeOutcomes(t *testing.T) {
	for i, tc := range []struct{ name, value string }{
		{"集团与子公司", "测试子公司乙"}, {"生效日期", "2026年7月1日"}, {"历史所属期间", "2024年度"}, {"适用范围", "测试海外范围"},
		{"计划与实际送样与试生产", "计划送样"}, {"统计依据", "测试单体范围"}, {"计量单位", "测试万台"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := []string{"测试集团甲", "2026年1月1日", "2025年度", "测试子公司甲", "试生产", "测试合并范围", "测试台"}
			values[i] = tc.value
			d, snaps := comparisonDraft(t, values)
			p, err := comparisonPreparation(t, comparisonConfig(), d, snaps).BindTask(context.Background(), supportTaskFixture())
			if err != nil || !strings.Contains(p.Request().Prompt, tc.value) {
				t.Fatal("实际差异原文未带入计划", err)
			}
			r, _ := comparisonReview(t, p, func(doc *comparisonTestDocument) {
				doc.Axes[i]["relation"] = "different"
				doc.Wording = "different_wording"
			})
			if r.Outcome() != "not_comparable" || r.Axes()[i].Reason != p.axes[i].Field+"_differs" || r.ResolvedCount() != 7 {
				t.Fatal("口径差异被当成事实冲突或资料缺失")
			}
		})
	}
	for _, wording := range []string{"same_meaning", "different_wording", "insufficient_context"} {
		t.Run(wording, func(t *testing.T) {
			p := comparisonPlan(t)
			r, _ := comparisonReview(t, p, func(doc *comparisonTestDocument) { doc.Wording = wording })
			expect := map[string]string{"same_meaning": "scope_aligned", "different_wording": "wording_differs", "insufficient_context": "insufficient_context"}[wording]
			if r.Outcome() != expect || r.Wording() != wording {
				t.Fatal("不同表述与口径状态混同")
			}
		})
	}
	for _, support := range []string{"partial", "unsupported", "uncertain", "supported"} {
		t.Run("语境不足_"+support, func(t *testing.T) {
			p := comparisonPlan(t)
			r, _ := comparisonReview(t, p, func(doc *comparisonTestDocument) { doc.Axes[0]["left"], doc.Axes[0]["relation"] = support, "uncertain" })
			expect := "field_meaning_unverified"
			if support == "supported" {
				expect = "insufficient_context"
			}
			if r.Outcome() != "insufficient_context" || r.Axes()[0].Reason != expect || r.ResolvedCount() != 6 {
				t.Fatal("原文支持不足仍称口径已核")
			}
		})
	}
}

func TestDossierCardComparisonStatementOrigins(t *testing.T) {
	for _, tc := range []struct {
		name, expected string
		change         func([]sources.Snapshot)
	}{
		{"同网址", "same_document", func(s []sources.Snapshot) { s[1].URL = s[0].URL }},
		{"同正文不同网址", "duplicate_body", func([]sources.Snapshot) {}},
		{"同登记不同正文", "same_registered_source", func(s []sources.Snapshot) { s[1].SourceID = s[0].SourceID }},
		{"不同登记仍未知", "unverified", func([]sources.Snapshot) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var values []string
			if tc.expected != "duplicate_body" {
				values = []string{"测试主体乙", "2026年1月1日", "2025年度", "测试子公司甲", "试生产", "测试合并范围", "测试台"}
			}
			d, snaps := comparisonDraft(t, values)
			tc.change(snaps)
			p, err := comparisonPreparation(t, comparisonConfig(), d, snaps).BindTask(context.Background(), supportTaskFixture())
			if err != nil {
				t.Fatal(err)
			}
			r, _ := comparisonReview(t, p, nil)
			if r.StatementOriginRelation() != tc.expected {
				t.Fatal("原句出处提示错误")
			}
			visible, err := p.VisibleReview(r)
			if err != nil || strings.Contains(visible, "independent") || strings.Contains(visible, "fact_conflict") || strings.Contains(visible, "测试主体乙") {
				t.Fatal("自动推导了独立性、事实冲突或自由正文")
			}
		})
	}
}

func TestDossierCardComparisonInputRejections(t *testing.T) {
	d, snaps := comparisonDraft(t, nil)
	b, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*CardComparisonConfig)
	}{
		{"错误步位", func(c *CardComparisonConfig) { c.ReviewStep = 4 }},
		{"旧字段步位不能冒充", func(c *CardComparisonConfig) { c.Model.ReviewStep = 4 }},
		{"未知焦点", func(c *CardComparisonConfig) { c.Focus = "auto" }},
		{"输出容量不足", func(c *CardComparisonConfig) { c.Model.MaxOutputTokens = 1023 }},
		{"空模型", func(c *CardComparisonConfig) { c.Model.Model = "" }},
		{"完整输入不得截断", func(c *CardComparisonConfig) { c.Model.MaxPromptBytes = 1024 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := comparisonConfig()
			tc.change(&cfg)
			if _, err := PrepareCardComparison(context.Background(), cfg, "仅测试专题", b, snaps, []string{"left_card", "right_card"}); !errors.Is(err, ErrCardComparison) {
				t.Fatal("无效配置被接受", err)
			}
		})
	}
	for _, keys := range [][]string{nil, {"left_card"}, {"left_card", "left_card"}, {"left_card", "missing"}, {"left_card", "right_card", "unused"}} {
		if _, err := PrepareCardComparison(context.Background(), comparisonConfig(), "仅测试专题", b, snaps, keys); !errors.Is(err, ErrCardComparison) {
			t.Fatal("未限制两张不同存在的卡", err)
		}
	}
	tampered := slices.Clone(snaps)
	tampered[0].Body += "篡改"
	if _, err := PrepareCardComparison(context.Background(), comparisonConfig(), "仅测试专题", b, tampered, []string{"left_card", "right_card"}); !errors.Is(err, ErrCardComparison) {
		t.Fatal("正文篡改被接受", err)
	}
	badBound := *b
	badBound.digest = strings.Repeat("a", 64)
	if _, err := PrepareCardComparison(context.Background(), comparisonConfig(), "仅测试专题", &badBound, snaps, []string{"left_card", "right_card"}); !errors.Is(err, ErrCardComparison) {
		t.Fatal("档案摘要篡改被接受", err)
	}
	var record storedDraft
	if json.Unmarshal(b.Canonical(), &record) != nil {
		t.Fatal("规范记录解析失败")
	}
	record.Draft.Cards[0].Subject.Ref.StartByte++
	badBound.canonical, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	badBound.digest = task.StageDigest(badBound.canonical)
	if _, err := PrepareCardComparison(context.Background(), comparisonConfig(), "仅测试专题", &badBound, snaps, []string{"left_card", "right_card"}); !errors.Is(err, ErrCardComparison) {
		t.Fatal("重新计算摘要掩盖了字段引用篡改", err)
	}
	prep := comparisonPreparation(t, comparisonConfig(), d, snaps)
	p, err := prep.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*task.Task)
	}{
		{"账户", func(v *task.Task) { v.UserID++ }}, {"任务", func(v *task.Task) { v.ID++ }}, {"编号", func(v *task.Task) { v.No = strings.Repeat("c", 32) }},
		{"题材", func(v *task.Task) { v.SubjectID++ }}, {"轻任务", func(v *task.Task) { v.AnalysisType = task.LightAnalysisType }},
		{"非法任务", func(v *task.Task) { v.ID = 0 }}, {"非题材", func(v *task.Task) { v.SubjectType = "stock" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := supportTaskFixture()
			tc.change(&v)
			if p.MatchesTask(v) || p.OperationKey(v, 5) != "" {
				t.Fatal("跨身份复用了计划或原键")
			}
			newPlan, err := prep.BindTask(context.Background(), v)
			if tc.name == "账户" || tc.name == "任务" || tc.name == "编号" {
				if err != nil || newPlan.InputDigest() == p.InputDigest() || newPlan.OperationKey(v, 5) == p.OperationKey(supportTaskFixture(), 5) {
					t.Fatal("真实任务身份未进入输入和原键", err)
				}
			} else if !errors.Is(err, ErrCardComparison) {
				t.Fatal("无效任务被绑定", err)
			}
		})
	}
	if p.OperationKey(supportTaskFixture(), 4) != "" {
		t.Fatal("允许调用其他步位")
	}
	if _, err := prep.ComponentDigest(task.LightAnalysisType); !errors.Is(err, ErrCardComparison) {
		t.Fatal("轻量任务被准入两卡组件")
	}
	if !reflect.DeepEqual(p.Config(), comparisonConfig()) {
		t.Fatal("配置副本失真")
	}
	// 合法的新版本或比较焦点必须取得新组件/输入/原键，不能冒用旧捕获包。
	for _, change := range []func(*Draft, *CardComparisonConfig){
		func(d *Draft, _ *CardComparisonConfig) { d.Version++ },
		func(_ *Draft, c *CardComparisonConfig) { c.Focus = "definition" },
	} {
		cfg := comparisonConfig()
		next := d
		change(&next, &cfg)
		other, err := comparisonPreparation(t, cfg, next, snaps).BindTask(context.Background(), supportTaskFixture())
		if err != nil || other.InputDigest() == p.InputDigest() || other.ComponentDigest() == p.ComponentDigest() || other.OperationKey(supportTaskFixture(), 5) == p.OperationKey(supportTaskFixture(), 5) {
			t.Fatal("版本或焦点变更未隔离计划", err)
		}
		_, raw := comparisonReview(t, p, nil)
		if _, err := other.Review(context.Background(), raw); !errors.Is(err, ErrCardComparison) {
			t.Fatal("新版本或焦点复用了旧捕获包", err)
		}
	}
}
