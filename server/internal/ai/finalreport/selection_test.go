package finalreport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func selectionJSON(t *testing.T, id int64) string {
	t.Helper()
	in := SelectionResponse{}
	for i, key := range sectionOrder {
		in.Selections = append(in.Selections, Selection{key, id, []int{i + 1}})
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func selectionFixture(t *testing.T) (*SelectionStage, *Policy, sources.Snapshot) {
	t.Helper()
	p, v, snap := fixture(t)
	s, err := NewSelectionStage(p, v.TaskNo, v.ThemeID, v.ThemeName, []sources.Snapshot{snap})
	if err != nil {
		t.Fatal(err)
	}
	return s, p, snap
}

func TestReportSelectionBuildAndIsolation(t *testing.T) {
	s, p, snap := selectionFixture(t)
	raw := selectionJSON(t, snap.ID)
	got, err := s.Build(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Decode(got)
	if err != nil || p.Bind(v, []sources.Snapshot{snap}, v.ThemeName) != nil || s.Policy().Check(context.Background(), task.StageValidationInput{Stage: "extract", InputDigest: s.InputDigest(), Payload: raw}) != nil {
		t.Fatal("编号回取未通过最终结构/证据门禁", err)
	}
	for i, sec := range v.Report.Sections {
		if sec.Nodes[0].Text != snap.Sentences[i].Text || sec.Nodes[0].Quote.Start != snap.Sentences[i].StartByte {
			t.Fatal("未逐字回取")
		}
	}
	if strings.Contains(got, snap.LicenseRef) || strings.Contains(fmt.Sprintf("%+v %#v", s, s), snap.Body) {
		t.Fatal("内部授权或正文泄漏")
	}
	wantTask := task.Task{No: v.TaskNo, SubjectID: v.ThemeID, SubjectType: task.Theme, AnalysisType: 1}
	if !s.Matches(wantTask, p) || !s.MatchesEvidence([]sources.Snapshot{snap}, v.ThemeName) {
		t.Fatal("绑定错配")
	}
	wantTask.SubjectID++
	if s.Matches(wantTask, p) {
		t.Fatal("跨题材复用")
	}
	ids := s.SnapshotIDs()
	ids[0]++
	if s.SnapshotIDs()[0] != snap.ID {
		t.Fatal("编号列表可被外部篡改")
	}
	// 策略持有完整私有副本；原始切片和授权时点在构造后发生变化不能改变它。
	expiry := snap.FetchedAt.Add(time.Hour)
	snap.LicenseExpires = &expiry
	copyInput := []sources.Snapshot{snap}
	clone, err := NewSelectionStage(p, v.TaskNo, v.ThemeID, v.ThemeName, copyInput)
	if err != nil {
		t.Fatal(err)
	}
	baseline := clone.InputDigest()
	copyInput[0].Sentences[0].Text = "仅用于篡改测试"
	expiry = expiry.Add(time.Hour)
	if _, err := clone.Build(context.Background(), raw); err != nil || clone.InputDigest() != baseline {
		t.Fatal("输入别名改变策略", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.Build(context.Background(), raw)
			if err != nil || out != got {
				t.Error("并发构建不确定", err)
			}
		}()
	}
	wg.Wait()
}

func TestReportSelectionClosedInput(t *testing.T) {
	s, _, snap := selectionFixture(t)
	raw := selectionJSON(t, snap.ID)
	for name, bad := range map[string]string{
		"空": `{}`, "空数组": `{"selections":[]}`, "空值": `{"selections":null}`,
		"顶层原文":  strings.Replace(raw, `"selections":`, `"report":"测试原文","selections":`, 1),
		"大小写顶层": strings.Replace(raw, `"selections"`, `"Selections"`, 1),
		"大小写字段": strings.Replace(raw, `"section"`, `"Section"`, 1),
		"重复字段":  strings.Replace(raw, `"section":`, `"section":"companies","section":`, 1),
		"缺字段":   strings.Replace(raw, `"section":`, `"missing":`, 1),
		"自写引文":  strings.Replace(raw, `"section":`, `"text":"测试自写原文","section":`, 1),
		"未知来源":  strings.Replace(raw, `"snapshot_id":1`, `"snapshot_id":99`, 1),
		"编号零":   strings.Replace(raw, `[1]`, `[0]`, 1), "编号越界": strings.Replace(raw, `[1]`, `[999]`, 1),
		"编号空": strings.Replace(raw, `[1]`, `[]`, 1), "编号null": strings.Replace(raw, `[1]`, `null`, 1),
		"编号重复": strings.Replace(raw, `[1]`, `[1,1]`, 1), "编号倒序": strings.Replace(raw, `[1]`, `[2,1]`, 1),
		"编号不连续": strings.Replace(raw, `[1]`, `[1,3]`, 1),
		"编号太多":  strings.Replace(raw, `[1]`, `[1,2,3,4,5,6,7,8,9]`, 1),
		"编号类型":  strings.Replace(raw, `[1]`, `["1"]`, 1),
		"额外章节":  strings.Replace(raw, `"definition"`, `"verification"`, 1),
		"缺章节":   strings.Replace(raw, `"definition"`, `"companies"`, 1),
		"跨章节重叠": strings.Replace(raw, `[2]`, `[1]`, 1),
		"范围重叠":  strings.Replace(raw, `[1]`, `[1,2]`, 1),
		"尾随":    raw + `{}`, "非对象": `[]`, "空条目": `{"selections":[null,null,null,null]}`,
		"截断": raw[:len(raw)-3],
	} {
		t.Run(name, func(t *testing.T) {
			if out, err := s.Build(context.Background(), bad); err == nil || out != "" {
				t.Fatal("畸形输入产生部分报告", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := s.Build(ctx, raw); err != context.Canceled || out != "" {
		t.Fatal("取消返回正文", err)
	}
	for _, in := range []task.StageValidationInput{{Stage: "judge", InputDigest: s.InputDigest(), Payload: raw}, {Stage: "extract", InputDigest: strings.Repeat("f", 64), Payload: raw}} {
		if s.Policy().Check(context.Background(), in) != task.ErrStagePolicy {
			t.Fatal("策略输入错配")
		}
	}
}

func TestReportSelectionTrustedInputBoundaries(t *testing.T) {
	s, p, snap := selectionFixture(t)
	for name, change := range map[string]func(*sources.Snapshot){
		"编号": func(v *sources.Snapshot) { v.ID = 0 }, "正文": func(v *sources.Snapshot) { v.Body += "测试变更" },
		"授权": func(v *sources.Snapshot) { v.LicenseRef = "" }, "句表": func(v *sources.Snapshot) { v.Sentences = nil },
		"链接": func(v *sources.Snapshot) { v.URL = "https://other.example.test/" }, "非规范链接": func(v *sources.Snapshot) { v.URL += "#fragment" },
		"畸形链接": func(v *sources.Snapshot) { v.URL = ":bad" },
		"超大":   func(v *sources.Snapshot) { v.Body = strings.Repeat("a", MaxEvidenceBodyBytes+1) },
	} {
		t.Run(name, func(t *testing.T) {
			v := snap
			change(&v)
			if _, err := NewSelectionStage(p, s.taskNo, s.themeID, s.themeName, []sources.Snapshot{v}); err == nil {
				t.Fatal("错误可信输入通过")
			}
		})
	}
	for _, input := range [][]sources.Snapshot{nil, {snap, snap}, make([]sources.Snapshot, MaxSnapshots+1)} {
		if _, err := NewSelectionStage(p, s.taskNo, s.themeID, s.themeName, input); err == nil {
			t.Fatal("数量/重复未拒绝")
		}
	}
	for _, input := range []struct {
		no, name string
		id       int64
	}{{"bad", s.themeName, 1}, {s.taskNo, "", 1}, {s.taskNo, s.themeName, 0}, {s.taskNo, strings.Repeat("题", 33), 1}} {
		if _, err := NewSelectionStage(p, input.no, input.id, input.name, []sources.Snapshot{snap}); err == nil {
			t.Fatal("无效题材身份未拒绝")
		}
	}
	if _, err := NewSelectionStage(nil, s.taskNo, 1, s.themeName, []sources.Snapshot{snap}); err == nil {
		t.Fatal("空终检策略通过")
	}
	var empty *SelectionStage
	if empty.Valid() || empty.InputDigest() != "" || empty.Policy().Valid() || empty.SnapshotIDs() != nil || empty.MatchesEvidence(nil, "") || empty.Matches(task.Task{}, p) {
		t.Fatal("空策略意外有效")
	}
	if out, err := empty.Build(context.Background(), "{}"); err == nil || out != "" {
		t.Fatal("空策略产生候选")
	}
	for _, change := range []func(*sources.Snapshot){func(v *sources.Snapshot) { v.LicenseRef += "-changed" }, func(v *sources.Snapshot) { v.FetchedAt = v.FetchedAt.Add(time.Second) }, func(v *sources.Snapshot) { v.ID++ }} {
		v := snap
		change(&v)
		if s.MatchesEvidence([]sources.Snapshot{v}, s.themeName) {
			t.Fatal("来源绑定变化未识别")
		}
	}
	if s.MatchesEvidence([]sources.Snapshot{snap}, "另一示例题材") {
		t.Fatal("题材重命名未识别")
	}
}

func TestReportSelectionMultipleSourcesAndCanonicalOrder(t *testing.T) {
	s, p, snap := selectionFixture(t)
	other := snap
	other.ID = 2
	a, err := NewSelectionStage(p, s.taskNo, 1, s.themeName, []sources.Snapshot{other, snap})
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSelectionStage(p, s.taskNo, 1, s.themeName, []sources.Snapshot{snap, other})
	if err != nil || a.InputDigest() != b.InputDigest() {
		t.Fatal("来源顺序影响绑定")
	}
	var in SelectionResponse
	if err := json.Unmarshal([]byte(selectionJSON(t, 1)), &in); err != nil {
		t.Fatal(err)
	}
	in.Selections = append(in.Selections, Selection{render.SecDefinition, 2, []int{1}})
	firstBytes, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	first := string(firstBytes)
	in.Selections[0], in.Selections[4] = in.Selections[4], in.Selections[0]
	x, err := a.Build(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	y, err := a.Build(context.Background(), string(secondBytes))
	if err != nil || x != y {
		t.Fatal("模型次序成为显示排序")
	}
	// 已授权的候选来源可以未被选择；最终报告只绑定实际引用，不凭空填充章节。
	if _, err := a.Build(context.Background(), selectionJSON(t, 1)); err != nil {
		t.Fatal(err)
	}
}

type selectionCancelContext struct {
	context.Context
	calls, at int
	cancel    context.CancelFunc
}

func (c *selectionCancelContext) Err() error {
	c.calls++
	if c.calls == c.at {
		c.cancel()
	}
	return c.Context.Err()
}

func TestReportSelectionQuoteLimitsAndLateCancellation(t *testing.T) {
	s, p, snap := selectionFixture(t)
	for _, body := range []string{
		"示例题材" + strings.Repeat("字", 1001) + "。示例题材事项。示例题材环节。示例题材公司。",
		"其他名称定义。示例题材事项。示例题材环节。示例题材公司。",
	} {
		v, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: p.cfg.AllowedDomains[0], Category: sources.CategoryAnnualReport, LicenseTag: "test-only", LicenseRef: "test-only", Enabled: true}, snap.URL, sources.MediaPlain, []byte(body), snap.FetchedAt, MaxEvidenceBodyBytes)
		if err != nil {
			t.Fatal(err)
		}
		v.ID = 1
		stage, err := NewSelectionStage(p, s.taskNo, 1, s.themeName, []sources.Snapshot{v})
		if err != nil {
			t.Fatal(err)
		}
		if out, err := stage.Build(context.Background(), selectionJSON(t, 1)); err == nil || out != "" {
			t.Fatal("超长或无题材原文产生候选")
		}
	}
	// 两句连续回取是一个原文字节区间，不拼接不相邻段落。
	v, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: p.cfg.AllowedDomains[0], Category: sources.CategoryAnnualReport, LicenseTag: "test-only", LicenseRef: "test-only", Enabled: true}, snap.URL, sources.MediaPlain, []byte("示例题材第一句。示例题材第二句。示例题材事项。示例题材环节。示例题材公司。"), snap.FetchedAt, MaxEvidenceBodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	v.ID = 1
	stage, err := NewSelectionStage(p, s.taskNo, 1, s.themeName, []sources.Snapshot{v})
	if err != nil {
		t.Fatal(err)
	}
	in := SelectionResponse{Selections: []Selection{{render.SecDefinition, 1, []int{1, 2}}, {render.SecTimeline, 1, []int{3}}, {render.SecSegment, 1, []int{4}}, {render.SecCompanies, 1, []int{5}}}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := stage.Build(context.Background(), string(b))
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := p.Decode(out)
	if err != nil || proposal.Report.Sections[0].Nodes[0].Text != v.Body[:v.Sentences[1].EndByte] {
		t.Fatal("连续区间回取失败", err)
	}
	for at := 1; at <= 6; at++ {
		base, cancel := context.WithCancel(context.Background())
		c := &selectionCancelContext{Context: base, at: at, cancel: cancel}
		out, err := s.Build(c, selectionJSON(t, 1))
		cancel()
		if err != context.Canceled || out != "" {
			t.Fatal("中途/最终取消泄露候选", at, err)
		}
	}
}
