package dossier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func supportAdmission(t *testing.T, p *SupportPlan, prepared *SupportPreparation) task.AdmissionPlan {
	t.Helper()
	a := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: p.identity.UserID, AnalysisType: p.AnalysisType(), SubjectType: task.Theme, SubjectID: p.identity.ThemeID},
		Price: points.Price{Version: "test-only-price", Points: 30}, Budget: task.Budget{MaxAttempts: 2, MaxCalls: 5, MaxTokens: 100000, MaxPages: 1, MaxDurationMS: 60000},
		SubjectName: "仅测试专题", ModelName: p.Config().Model, FilingNo: p.Config().FilingNo, ProfileDigest: p.Config().ProfileDigest, ExecutionDigest: task.StageDigest([]byte("test-only-overall-execution")), DossierSupportDigest: p.ComponentDigest(),
		ApprovalRef: "test-only-admission", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", PriceRef: "test-only-price", SourceRef: "test-only-materials", Materials: prepared.Materials()}
	if a.Validate(a.Scope) != nil {
		t.Fatal("合成准入无效")
	}
	return a
}

func TestDossierSupportPreparationBeforeTask(t *testing.T) {
	d, s := testDraft(t)
	b, e := BindDraft(d, s)
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := PrepareSupport(context.Background(), supportConfig(), "仅测试专题", b, s, []string{"progress_a"})
	if e != nil || !prepared.Valid() {
		t.Fatal("无任务号时不能准备", e)
	}
	deep, e := prepared.ComponentDigest(2)
	if e != nil || len(deep) != 64 {
		t.Fatal("没有首次准入摘要")
	}
	light, e := prepared.ComponentDigest(1)
	if e != nil || light == deep {
		t.Fatal("轻深执行未隔离")
	}
	v := supportTaskFixture()
	first, e := prepared.BindTask(context.Background(), v)
	if e != nil {
		t.Fatal(e)
	}
	v.ID++
	v.UserID++
	v.No = strings.Repeat("c", 32)
	second, e := prepared.BindTask(context.Background(), v)
	if e != nil || first.InputDigest() == second.InputDigest() || first.ComponentDigest() != second.ComponentDigest() || first.ComponentDigest() != deep {
		t.Fatal("任务绑定与创建前摘要混淆")
	}
	// 保留原v1捕获输入编码，不能因新增准入准备而悄悄改写旧语义。
	raw, e := json.Marshal([]any{SupportVersion, first.policy, first.identity, first.request})
	if e != nil || task.StageDigest(raw) != first.InputDigest() {
		t.Fatal("原v1计划编码变化")
	}
	if first.Config() != supportConfig() || first.AnalysisType() != 2 {
		t.Fatal("适配元数据不符")
	}
	materials := prepared.Materials()
	materials[0].Digest = "modified"
	if prepared.Materials()[0].Digest == "modified" {
		t.Fatal("准备元数据被外部修改")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", prepared, prepared), "progress_a") {
		t.Fatal("准备结果格式化泄露")
	}
	key := first.OperationKey(supportTaskFixture(), 3)
	changed := supportTaskFixture()
	changed.Fence = 8
	changed.LeaseOwner = "test-only-next-owner"
	if len(key) != 64 || first.OperationKey(changed, 3) != key || first.OperationKey(changed, 4) == key {
		t.Fatal("原键不稳定或步骤串用")
	}
	changed.UserID++
	if first.OperationKey(changed, 3) != "" || first.OperationKey(supportTaskFixture(), 0) != "" || first.OperationKey(supportTaskFixture(), task.MaxSteps+1) != "" {
		t.Fatal("错身份或步骤仍有原键")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := prepared.BindTask(ctx, supportTaskFixture()); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("取消后绑定")
	}
	if got, err := prepared.BindTask(nil, supportTaskFixture()); got != nil || err == nil {
		t.Fatal("空上下文绑定")
	}
	var empty *SupportPreparation
	if empty.Valid() || empty.Materials() != nil {
		t.Fatal("空准备有数据")
	}
	if digest, err := empty.ComponentDigest(1); digest != "" || err == nil {
		t.Fatal("空准备可准入")
	}
	if digest, err := prepared.ComponentDigest(0); digest != "" || err == nil {
		t.Fatal("非法深度可准入")
	}
	if got, err := empty.BindTask(context.Background(), supportTaskFixture()); got != nil || err == nil {
		t.Fatal("空准备可执行")
	}
	var plan *SupportPlan
	if plan.Config() != (SupportConfig{}) || plan.AnalysisType() != 0 || plan.ComponentDigest() != "" || plan.OperationKey(supportTaskFixture(), 3) != "" || plan.MatchesAdmission(task.AdmissionPlan{}) {
		t.Fatal("空计划可准入")
	}
}

func TestDossierSupportAdmissionExactBinding(t *testing.T) {
	d, s := testDraft(t)
	b, e := BindDraft(d, s)
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := PrepareSupport(context.Background(), supportConfig(), "仅测试专题", b, s, []string{"progress_a"})
	if e != nil {
		t.Fatal(e)
	}
	p, e := prepared.BindTask(context.Background(), supportTaskFixture())
	if e != nil {
		t.Fatal(e)
	}
	base := supportAdmission(t, p, prepared)
	if !p.MatchesAdmission(base) {
		t.Fatal("原准入未匹配")
	}
	for _, tc := range []struct {
		name   string
		change func(*task.AdmissionPlan)
	}{
		{"核验组件摘要", func(a *task.AdmissionPlan) { a.DossierSupportDigest = strings.Repeat("e", 64) }},
		{"旧记录缺核验组件", func(a *task.AdmissionPlan) { a.DossierSupportDigest = "" }},
		{"跨会员", func(a *task.AdmissionPlan) { a.Scope.UserID++ }}, {"跨题材", func(a *task.AdmissionPlan) { a.Scope.SubjectID++ }},
		{"跨轻深", func(a *task.AdmissionPlan) { a.Scope.AnalysisType = 1 }}, {"对象类别", func(a *task.AdmissionPlan) { a.Scope.SubjectType = task.Topic }},
		{"题材名称", func(a *task.AdmissionPlan) { a.SubjectName = "仅测试另一名称" }}, {"模型", func(a *task.AdmissionPlan) { a.ModelName = "other-model" }},
		{"公示", func(a *task.AdmissionPlan) { a.FilingNo = "other-filing" }}, {"配置摘要", func(a *task.AdmissionPlan) { a.ProfileDigest = strings.Repeat("e", 64) }},
		{"预算不足", func(a *task.AdmissionPlan) { a.Budget.MaxTokens = 1 }}, {"缺资料", func(a *task.AdmissionPlan) { a.Materials = nil }},
		{"资料编号", func(a *task.AdmissionPlan) { a.Materials[0].SnapshotID++ }}, {"资料哈希", func(a *task.AdmissionPlan) { a.Materials[0].Digest = strings.Repeat("e", 64) }},
		{"资料域名", func(a *task.AdmissionPlan) { a.Materials[0].Domain = "other.example.test" }}, {"资料类别", func(a *task.AdmissionPlan) { a.Materials[0].Category = "unknown" }},
		{"资料链接", func(a *task.AdmissionPlan) { a.Materials[0].URL += "?changed=1" }}, {"资料时点", func(a *task.AdmissionPlan) {
			a.Materials[0].CollectedAt = a.Materials[0].CollectedAt.Add(time.Millisecond)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := base.Clone()
			tc.change(&a)
			if p.MatchesAdmission(a) {
				t.Fatal("错误首次准入仍通过")
			}
		})
	}
	extra := base.Clone()
	other := extra.Materials[0]
	other.SnapshotID++
	extra.Materials = append(extra.Materials, other)
	if !p.MatchesAdmission(extra) {
		t.Fatal("额外获准资料误改实际核验范围")
	}
	zone := base.Clone()
	zone.Materials[0].CollectedAt = zone.Materials[0].CollectedAt.In(time.FixedZone("test-zone", 3600))
	if !p.MatchesAdmission(zone) {
		t.Fatal("相同时点因时区表示被误拒")
	}
}

func TestDossierSupportReviewStepBinding(t *testing.T) {
	d, snapshots := testDraft(t)
	bound, err := BindDraft(d, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	digests := map[string]bool{}
	for _, step := range []int{0, 3, 4, -1, 2, 5} {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			cfg := supportConfig()
			cfg.ReviewStep = step
			p, err := NewSupportPlan(context.Background(), cfg, supportTaskFixture(), "仅测试专题", bound, snapshots, []string{"progress_a"})
			if step != 0 && step != 3 && step != 4 {
				if err == nil || p != nil {
					t.Fatal("未允许的步骤可执行")
				}
				return
			}
			want := step
			if want == 0 {
				want = 3
			}
			if err != nil || p.StepNo() != want || p.OperationKey(supportTaskFixture(), want) == "" || p.OperationKey(supportTaskFixture(), 7-want) != "" || digests[p.ComponentDigest()] {
				t.Fatal("步骤未固化或计划可串用", err)
			}
			digests[p.ComponentDigest()] = true
		})
	}
	var empty *SupportPlan
	if empty.StepNo() != 0 {
		t.Fatal("空计划有步骤")
	}
}
