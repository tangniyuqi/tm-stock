package finalreport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestDeepSelectionSplitsEachSentence(t *testing.T) {
	p, v, snap := fixture(t)
	second, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: "evidence.example.test", Category: sources.CategoryAnnualReport, LicenseTag: "test-only", LicenseRef: "test-only-private-license", Enabled: true}, "https://evidence.example.test/second", sources.MediaPlain, []byte("示例题材的第二段第一句。示例题材的第二段第二句。"), time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	second.ID = 2
	l2cfg := p.cfg
	l2cfg.AnalysisType, l2cfg.MinQuotes = task.DeepAnalysisType, task.DeepMinQuotes
	l2, err := NewPolicy(l2cfg, p.entailment, p.semantic)
	if err != nil {
		t.Fatal(err)
	}
	snapshots := []sources.Snapshot{snap, second}
	in := SelectionResponse{Selections: []Selection{
		{render.SecDefinition, snap.ID, []int{1, 2}},
		{render.SecTimeline, snap.ID, []int{3, 4}},
		{render.SecSegment, second.ID, []int{1}},
		{render.SecCompanies, second.ID, []int{2}},
	}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	l2stage, err := NewSelectionStage(l2, v.TaskNo, v.ThemeID, v.ThemeName, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := l2stage.Build(context.Background(), string(raw))
	if err != nil {
		t.Fatal("深度转换失败", err)
	}
	deep, err := l2.Decode(payload)
	if err != nil {
		t.Fatal(err)
	}
	total, definition := 0, 0
	for _, section := range deep.Report.Sections {
		total += len(section.Nodes)
		if section.Key == render.SecDefinition {
			definition = len(section.Nodes)
		}
	}
	if total != task.DeepMinQuotes || definition != 2 {
		t.Fatal("深度未按句拆分六条引文", total, definition)
	}
	v1 := task.Task{ID: 1, UserID: 1, No: v.TaskNo, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: v.ThemeID}
	if !l2stage.Matches(v1, l2) {
		t.Fatal("深度选择未绑定类型2任务")
	}
	v1.AnalysisType = task.LightAnalysisType
	if l2stage.Matches(v1, l2) {
		t.Fatal("深度选择错误接受类型1任务")
	}

	l1stage, err := NewSelectionStage(p, v.TaskNo, v.ThemeID, v.ThemeName, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = l1stage.Build(context.Background(), string(raw))
	if err != nil {
		t.Fatal(err)
	}
	l1, err := p.Decode(payload)
	if err != nil {
		t.Fatal(err)
	}
	l1total := 0
	for _, section := range l1.Report.Sections {
		l1total += len(section.Nodes)
	}
	if l1total != 4 {
		t.Fatal("L1合并单条引文行为改变", l1total)
	}
	if p.AnalysisType() != 0 || l2.AnalysisType() != task.DeepAnalysisType || (*Policy)(nil).AnalysisType() != 0 {
		t.Fatal("策略类型访问器不符")
	}
}

func TestCheckPlanBindsDeepTaskType(t *testing.T) {
	_, v, _ := fixture(t)
	cfg := CheckConfig{Model: "测试模型", FilingNo: "test-only-filing", Version: "test-only-v1", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: strings.Repeat("a", 64), MaxInputTokens: 32, AnalysisType: task.DeepAnalysisType}
	proposal, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewEntailmentCheck(cfg, EntailmentInput{ProposalJSON: string(proposal), EvidenceJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	deep := task.Task{ID: 1, UserID: 1, No: v.TaskNo, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: v.ThemeID}
	if !plan.MatchesTask(deep) {
		t.Fatal("深度判定未绑定类型2任务")
	}
	deep.AnalysisType = task.LightAnalysisType
	if plan.MatchesTask(deep) {
		t.Fatal("深度判定错误接受类型1任务")
	}
	cfg.AnalysisType = 3
	if _, err := CheckPolicyVersion(CheckEntailment, cfg); err == nil {
		t.Fatal("非法判定类型未拒绝")
	}
}
