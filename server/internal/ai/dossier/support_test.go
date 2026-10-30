package dossier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func supportConfig() SupportConfig {
	return SupportConfig{Model: "test-only-support-model", Version: "test-only-v1", FilingNo: "test-only-filing", ApprovalRef: "test-only-approval",
		QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: strings.Repeat("a", 64), MaxInputTokens: 16000, MaxOutputTokens: 2048, MaxPromptBytes: 128 << 10}
}
func supportTaskFixture() task.Task {
	return task.Task{ID: 17, UserID: 23, No: strings.Repeat("b", 32), SubjectType: task.Theme, SubjectID: 7, AnalysisType: 2}
}
func supportPlan(t *testing.T, d Draft, snaps []sources.Snapshot, keys ...string) *SupportPlan {
	t.Helper()
	b, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal("合成档案绑定失败", err)
	}
	p, err := NewSupportPlan(context.Background(), supportConfig(), supportTaskFixture(), "仅测试专题", b, snaps, keys)
	if err != nil {
		t.Fatal("合成核验计划失败", err)
	}
	return p
}

func TestDossierSupportPlanEvidence(t *testing.T) {
	d, snaps := testDraft(t)
	// 第一整句是不能从局部摘录丢掉的上下文；字段位于第二句。
	s := testSnapshot(t, 1, "仅测试：下述样品进展不代表集团其他主体的状态。仅测试：测试主体甲的样品处于送样阶段。")
	s.LicenseTag = "test-only-private-license-marker"
	c := d.Cards[0]
	c.Statement = reference(s, s.Sentences[1])
	for _, pair := range []struct {
		text string
		dst  *Field
	}{{"测试主体甲", &c.Subject}, {"送样", &c.Stage}} {
		ref := reference(s, s.Sentences[1])
		ref.StartByte = strings.Index(s.Body, pair.text)
		ref.EndByte = ref.StartByte + len(pair.text)
		*pair.dst = Field{State: "known", Ref: ref}
	}
	d.Cards = []FactCard{c}
	snaps = []sources.Snapshot{s}
	p := supportPlan(t, d, snaps, c.Key)
	r := p.Request()
	if !p.Valid() || len(p.InputDigest()) != 64 || len(p.PolicyDigest()) != 64 || !p.MatchesTask(supportTaskFixture()) ||
		r.Model != supportConfig().Model || r.Format != provider.FormatJSONSchema || !r.Strict || r.Temperature == nil || *r.Temperature != 0 ||
		r.Attempt != 1 || r.TaskID != supportTaskFixture().No || r.MaxTokens != supportConfig().MaxOutputTokens || !json.Valid(r.Schema) ||
		!strings.Contains(r.Prompt, "不代表集团其他主体") || !strings.Contains(r.Prompt, s.Hash) {
		t.Fatal("计划未绑定原身份、严格请求或完整原始语境")
	}
	if len(p.checks) != 3 || p.checks[0].Field != "section" || p.checks[1].Field != "subject" || p.checks[2].Field != "stage" ||
		strings.Contains(r.Prompt, s.LicenseRef) || strings.Contains(r.Prompt, s.LicenseTag) || strings.Contains(r.Prompt, "userId") {
		t.Fatal("未知字段被补出或请求泄露许可/账户")
	}
	if p.Units() != (task.Units{Calls: 1, Tokens: 18048}) || !p.StagePolicy().Valid() {
		t.Fatal("原阶段与预留单位不完整")
	}
	// 请求可写副本与调用者快照均不能改变计划；这里不执行模型。
	*r.Temperature = 9
	r.Schema[0] = 'x'
	snaps[0].Sentences[0].Text = "外部修改"
	snaps[0].Body = "外部修改"
	if *p.Request().Temperature != 0 || p.Request().Schema[0] != '{' || !strings.Contains(p.Request().Prompt, "不代表集团其他主体") {
		t.Fatal("计划被外部修改")
	}
	for _, value := range []any{p, supportConfig()} {
		text := fmt.Sprintf("%+v %#v", value, value)
		for _, hidden := range []string{"测试主体甲", supportConfig().Model, supportConfig().ApprovalRef, supportTaskFixture().No} {
			if strings.Contains(text, hidden) {
				t.Fatal("格式化泄露内部输入")
			}
		}
	}
}

func TestDossierSupportPlanIdentityAndOrdering(t *testing.T) {
	d, snaps := testDraft(t)
	c := d.Cards[0]
	c.Key = "another_card"
	d.Cards = append(d.Cards, c)
	a := supportPlan(t, d, snaps, "progress_a", "another_card")
	b := supportPlan(t, d, snaps, "another_card", "progress_a")
	if a.InputDigest() != b.InputDigest() || a.Request().Prompt != b.Request().Prompt || a.checks[0].Card != "another_card" {
		t.Fatal("输入顺序影响计划")
	}
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*task.Task)
	}{
		{"任务", func(v *task.Task) { v.ID++ }}, {"会员", func(v *task.Task) { v.UserID++ }}, {"任务号", func(v *task.Task) { v.No = strings.Repeat("c", 32) }},
		{"深度", func(v *task.Task) { v.AnalysisType = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := supportTaskFixture()
			tc.change(&v)
			changed, err := NewSupportPlan(context.Background(), supportConfig(), v, "仅测试专题", bound, snaps, []string{"another_card", "progress_a"})
			if err != nil || a.MatchesTask(v) || changed.InputDigest() == a.InputDigest() || changed.PolicyDigest() != a.PolicyDigest() {
				t.Fatal("任务身份未隔离")
			}
		})
	}
	v := supportTaskFixture()
	v.Fence = 9
	v.LeaseOwner = "test-only-new-lease"
	v.Status = task.Running
	lease, err := NewSupportPlan(context.Background(), supportConfig(), v, "仅测试专题", bound, snaps, []string{"another_card", "progress_a"})
	if err != nil || !a.MatchesTask(v) || lease.InputDigest() != a.InputDigest() {
		t.Fatal("租约变化改变原输入；围栏应由原阶段服务核验")
	}
	for _, tc := range []struct {
		name      string
		change    func(*Draft, []sources.Snapshot)
		nameValue string
		keys      []string
	}{
		{"档案版本", func(d *Draft, _ []sources.Snapshot) { d.Version++ }, "仅测试专题", []string{"another_card", "progress_a"}},
		{"编制时点", func(d *Draft, _ []sources.Snapshot) { d.CreatedAt++ }, "仅测试专题", []string{"another_card", "progress_a"}},
		{"许可元数据", func(_ *Draft, s []sources.Snapshot) { s[0].LicenseRef = "test-only-revised-license" }, "仅测试专题", []string{"another_card", "progress_a"}},
		{"题材名", func(_ *Draft, _ []sources.Snapshot) {}, "仅测试另一个名称", []string{"another_card", "progress_a"}},
		{"所核字段集合", func(_ *Draft, _ []sources.Snapshot) {}, "仅测试专题", []string{"progress_a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d2 := d
			d2.Cards = slices.Clone(d.Cards)
			s2 := slices.Clone(snaps)
			tc.change(&d2, s2)
			b2, e := BindDraft(d2, s2)
			if e != nil {
				t.Fatal(e)
			}
			p2, e := NewSupportPlan(context.Background(), supportConfig(), supportTaskFixture(), tc.nameValue, b2, s2, tc.keys)
			if e != nil || p2.InputDigest() == a.InputDigest() || p2.PolicyDigest() != a.PolicyDigest() {
				t.Fatal("精确输入未绑定")
			}
		})
	}
}

func TestDossierSupportOnlyRelatedFullSources(t *testing.T) {
	d, snaps := testDraft(t)
	other := testSnapshot(t, 2, "仅测试：未选卡片的私有原文不应传给本次模型。")
	c := d.Cards[0]
	c.Key, c.Section = "other_card", "definition"
	c.Statement = reference(other, other.Sentences[0])
	c.Subject, c.Stage = Field{State: "unknown"}, Field{State: "unknown"}
	d.Cards = append(d.Cards, c)
	snaps = append(snaps, other)
	p := supportPlan(t, d, snaps, "progress_a")
	if strings.Contains(p.Request().Prompt, other.Body) || strings.Contains(p.Request().Prompt, other.Hash) {
		t.Fatal("请求夹带未选卡片的来源正文")
	}
	a := supportPlan(t, d, snaps, "progress_a", "other_card")
	slices.Reverse(snaps)
	b := supportPlan(t, d, snaps, "other_card", "progress_a")
	if a.InputDigest() != b.InputDigest() || a.Request().Prompt != b.Request().Prompt {
		t.Fatal("来源顺序改变原计划")
	}
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	// 仅发送相关全文，但构造时仍要求完整档案绑定集合，不能跳过另一快照的完整性。
	if got, e := NewSupportPlan(context.Background(), supportConfig(), supportTaskFixture(), "仅测试专题", bound, snaps[:1], []string{"progress_a"}); got != nil || e == nil {
		t.Fatal("省略档案绑定证据仍能构造计划")
	}
	cfg := supportConfig()
	cfg.QualityRef = ""
	if got, e := NewSupportPlan(context.Background(), cfg, supportTaskFixture(), "仅测试专题", bound, snaps, []string{"progress_a"}); got != nil || e == nil {
		t.Fatal("无效模型配置仍有计划")
	}
}

func TestDossierSupportConfigBoundaries(t *testing.T) {
	base := supportConfig()
	digest, err := base.PolicyDigest()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*SupportConfig)
		valid  bool
	}{
		{"空模型", func(c *SupportConfig) { c.Model = "" }, false}, {"空版本", func(c *SupportConfig) { c.Version = "" }, false},
		{"空公示", func(c *SupportConfig) { c.FilingNo = "" }, false}, {"空批准", func(c *SupportConfig) { c.ApprovalRef = "" }, false},
		{"空质量", func(c *SupportConfig) { c.QualityRef = "" }, false}, {"空硬预算", func(c *SupportConfig) { c.HardBudgetRef = "" }, false},
		{"错误摘要", func(c *SupportConfig) { c.ProfileDigest = "bad" }, false}, {"大写摘要", func(c *SupportConfig) { c.ProfileDigest = strings.Repeat("A", 64) }, false},
		{"元数据控制符", func(c *SupportConfig) { c.Model = "test\nmodel" }, false}, {"元数据隐形符", func(c *SupportConfig) { c.Version = "test\u200bversion" }, false},
		{"元数据过长", func(c *SupportConfig) { c.Model = strings.Repeat("m", 129) }, false}, {"零输入", func(c *SupportConfig) { c.MaxInputTokens = 0 }, false},
		{"过大输入", func(c *SupportConfig) { c.MaxInputTokens = 1<<20 + 1 }, false}, {"少输出", func(c *SupportConfig) { c.MaxOutputTokens = 127 }, false},
		{"多输出", func(c *SupportConfig) { c.MaxOutputTokens = 8193 }, false}, {"少正文", func(c *SupportConfig) { c.MaxPromptBytes = 1023 }, false},
		{"过大正文", func(c *SupportConfig) { c.MaxPromptBytes = 256<<10 + 1 }, false},
		{"模型变更", func(c *SupportConfig) { c.Model += "-new" }, true}, {"版本变更", func(c *SupportConfig) { c.Version += "-new" }, true},
		{"批准变更", func(c *SupportConfig) { c.ApprovalRef += "-new" }, true}, {"质量变更", func(c *SupportConfig) { c.QualityRef += "-new" }, true},
		{"预算变更", func(c *SupportConfig) { c.HardBudgetRef += "-new" }, true}, {"公示变更", func(c *SupportConfig) { c.FilingNo += "-new" }, true},
		{"模型摘要变更", func(c *SupportConfig) { c.ProfileDigest = strings.Repeat("f", 64) }, true},
		{"输入上限变更", func(c *SupportConfig) { c.MaxInputTokens++ }, true}, {"输出上限变更", func(c *SupportConfig) { c.MaxOutputTokens++ }, true},
		{"提示上限变更", func(c *SupportConfig) { c.MaxPromptBytes++ }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.change(&c)
			got, e := c.PolicyDigest()
			if (e == nil) != tc.valid || tc.valid && (got == digest || len(got) != 64) || !tc.valid && got != "" {
				t.Fatal("配置边界或版本绑定失效")
			}
		})
	}
}

func TestDossierSupportPlanRejectsUnboundInput(t *testing.T) {
	d, snaps := testDraft(t)
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		change    func(*task.Task)
		keys      []string
		nameValue string
		nilBound  bool
	}{
		{"缺卡", func(_ *task.Task) {}, nil, "仅测试专题", false}, {"重复卡", func(_ *task.Task) {}, []string{"progress_a", "progress_a"}, "仅测试专题", false},
		{"未知卡", func(_ *task.Task) {}, []string{"missing"}, "仅测试专题", false}, {"超四卡", func(_ *task.Task) {}, []string{"a", "b", "c", "d", "e"}, "仅测试专题", false},
		{"无档案", func(_ *task.Task) {}, []string{"progress_a"}, "仅测试专题", true}, {"无名称", func(_ *task.Task) {}, []string{"progress_a"}, "", false},
		{"无任务", func(v *task.Task) { v.ID = 0 }, []string{"progress_a"}, "仅测试专题", false}, {"超安全会员", func(v *task.Task) { v.UserID = conversation.MaxID + 1 }, []string{"progress_a"}, "仅测试专题", false},
		{"错误任务号", func(v *task.Task) { v.No = "bad" }, []string{"progress_a"}, "仅测试专题", false}, {"专题未接", func(v *task.Task) { v.SubjectType = task.Topic }, []string{"progress_a"}, "仅测试专题", false},
		{"跨题材", func(v *task.Task) { v.SubjectID++ }, []string{"progress_a"}, "仅测试专题", false}, {"未知深度", func(v *task.Task) { v.AnalysisType = 3 }, []string{"progress_a"}, "仅测试专题", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := supportTaskFixture()
			tc.change(&v)
			b := bound
			if tc.nilBound {
				b = nil
			}
			p, e := NewSupportPlan(context.Background(), supportConfig(), v, tc.nameValue, b, snaps, tc.keys)
			if p != nil || e == nil {
				t.Fatal("接受未绑定输入")
			}
		})
	}
	s2 := slices.Clone(snaps)
	s2[0].Body += "篡改"
	if p, e := NewSupportPlan(context.Background(), supportConfig(), supportTaskFixture(), "仅测试专题", bound, s2, []string{"progress_a"}); p != nil || e == nil {
		t.Fatal("未重验原文完整性")
	}
	if p, e := NewSupportPlan(context.Background(), supportConfig(), supportTaskFixture(), "仅测试专题", bound, nil, []string{"progress_a"}); p != nil || e == nil {
		t.Fatal("接受缺失原文")
	}
	p := supportPlan(t, d, snaps, "progress_a")
	cfg := supportConfig()
	size := len(p.Request().System) + len(p.Request().Prompt) + len(p.Request().Schema)
	cfg.MaxPromptBytes = size
	if _, e := NewSupportPlan(context.Background(), cfg, supportTaskFixture(), "仅测试专题", bound, snaps, []string{"progress_a"}); e != nil {
		t.Fatal("精确容量误拒", e)
	}
	cfg.MaxPromptBytes--
	if p, e := NewSupportPlan(context.Background(), cfg, supportTaskFixture(), "仅测试专题", bound, snaps, []string{"progress_a"}); p != nil || e == nil {
		t.Fatal("容量不足仍截断语境通过")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p, e := NewSupportPlan(ctx, supportConfig(), supportTaskFixture(), "仅测试专题", bound, snaps, []string{"progress_a"}); p != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("取消后仍构造计划")
	}
	if p, e := NewSupportPlan(nil, supportConfig(), supportTaskFixture(), "仅测试专题", bound, snaps, []string{"progress_a"}); p != nil || e == nil {
		t.Fatal("接受空上下文")
	}
}
