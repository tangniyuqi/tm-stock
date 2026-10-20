package finalreport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func fixture(t *testing.T) (*Policy, Proposal, sources.Snapshot) {
	t.Helper()
	cfg := Config{Ref: "test-only-policy", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{"evidence.example.test"}}
	p, err := NewPolicy(cfg, func(context.Context, EntailmentInput) (Verdict, error) { return Pass, nil }, func(context.Context, string) (Verdict, error) { return Pass, nil })
	if err != nil {
		t.Fatal(err)
	}
	s, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: cfg.AllowedDomains[0], Category: sources.CategoryAnnualReport, LicenseTag: "test-only", LicenseRef: "test-only-private-license", Enabled: true}, "https://evidence.example.test/example", sources.MediaPlain, []byte("示例题材是本测试资料的主题。示例题材的事项发生于资料记载日期。示例题材包含资料所述环节。示例题材的公司信息见本段。"), time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	s.ID = 1
	v := Proposal{TaskNo: strings.Repeat("a", 32), ThemeID: 1, ThemeName: "示例题材", Report: render.Report{SchemaVer: render.CurrentSchemaVer, Disclosure: render.Disclosure{AIGenerated: true, ModelName: cfg.ModelName, FilingNo: cfg.FilingNo}}}
	if len(s.Sentences) != 4 {
		t.Fatal("测试句子数量", len(s.Sentences))
	}
	for i, key := range sectionOrder {
		sentence := s.Sentences[i]
		node, err := render.QuoteNode(render.Quote{SnapshotID: s.ID, Start: sentence.StartByte, End: sentence.EndByte, Text: sentence.Text, SourceType: "年报", URL: s.URL, FetchedAt: s.FetchedAt.UnixMilli(), ContentHash: s.Hash}, render.ProvAIExtractedQuote)
		if err != nil {
			t.Fatal(err)
		}
		title, _ := render.SectionTitle(key)
		v.Report.Sections = append(v.Report.Sections, render.Section{Key: key, Title: title, AILabel: true, Nodes: []render.Node{node}})
	}
	return p, v, s
}

func TestFinalReportCommitAndNullPolicy(t *testing.T) {
	if !ValidDigest(strings.Repeat("a", 64)) || ValidDigest(strings.Repeat("A", 64)) || ValidDigest("bad") {
		t.Fatal("摘要格式未封闭")
	}
	p, v, snap := fixture(t)
	raw := []byte(encoded(t, v))
	c := Commit{TaskID: 1, UserID: 1, Fence: 1, EventSeq: 2, TerminalTxnID: 1, StepNo: 1, Status: task.Succeeded, TaskNo: v.TaskNo, OperationKey: "test_report_operation", InputDigest: strings.Repeat("a", 64), ContentDigest: task.StageDigest(raw), PolicyDigest: p.Digest(), Entailment: Pass, Semantic: Pass, Payload: raw, CreatedAt: snap.FetchedAt}
	if c.Validate() != nil || c.Receipt().ReportDigest == "" || strings.Contains(fmt.Sprintf("%+v %#v", c, c), v.ThemeName) {
		t.Fatal("成功提交/安全输出错误")
	}
	for _, change := range []func(*Commit){func(c *Commit) { c.TaskID = 0 }, func(c *Commit) { c.Semantic = Uncertain }, func(c *Commit) { c.Payload = nil }, func(c *Commit) { c.ContentDigest = strings.Repeat("f", 64) }, func(c *Commit) { c.Reason = "semantic" }, func(c *Commit) { c.Status = task.Cancelled }} {
		bad := c
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("坏提交放行")
		}
	}
	c.Status = task.Failed
	c.Payload = nil
	c.Reason = "semantic"
	c.Semantic = Uncertain
	if c.Validate() != nil || c.Receipt().ReportDigest != "" {
		t.Fatal("拒绝收据错误")
	}
	for _, reason := range []string{"structure", "evidence", "entailment", "semantic", "unknown"} {
		bad := c
		bad.Reason = reason
		if reason == "entailment" {
			bad.Entailment = Block
		}
		if reason == "unknown" {
			if bad.Validate() == nil {
				t.Fatal("未知原因放行")
			}
		} else if bad.Validate() != nil {
			t.Fatal("有效原因拒绝")
		}
	}
	bad := c
	bad.Payload = raw
	if bad.Validate() == nil {
		t.Fatal("拒绝保存正文")
	}
	bad = c
	bad.Semantic = Pass
	if bad.Validate() == nil {
		t.Fatal("语义通过却记录语义拒绝")
	}
	bad = c
	bad.Reason = "entailment"
	if bad.Validate() == nil {
		t.Fatal("蕴含通过却记录蕴含拒绝")
	}
	var absent *Policy
	if absent.Valid() || absent.Digest() != "" || absent.StagePolicy().Valid() {
		t.Fatal("空策略放行")
	}
	if _, err := absent.Decode(string(raw)); err == nil {
		t.Fatal("空策略解码放行")
	}
	if absent.Bind(v, []sources.Snapshot{snap}, v.ThemeName) == nil {
		t.Fatal("空策略证据放行")
	}
	if _, _, reason, err := absent.Review(context.Background(), string(raw), v, []sources.Snapshot{snap}); err != nil || reason != "entailment" {
		t.Fatal("空策略未关闭")
	}
	if err := p.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "extract", Payload: string(raw)}); err == nil {
		t.Fatal("错误阶段放行")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.StagePolicy().Check(ctx, task.StageValidationInput{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, which := range []string{"entailment", "semantic"} {
		p, v, snap := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if which == "entailment" {
			p.entailment = func(context.Context, EntailmentInput) (Verdict, error) { cancel(); return Pass, nil }
		} else {
			p.semantic = func(context.Context, string) (Verdict, error) { cancel(); return Pass, nil }
		}
		if _, _, _, err := p.Review(ctx, encoded(t, v), v, []sources.Snapshot{snap}); !errors.Is(err, context.Canceled) {
			t.Fatal("回调取消被吞")
		}
	}
	for _, category := range []string{sources.CategoryAnnouncement, sources.CategoryAnnualReport, sources.CategoryProspectus, sources.CategoryOfficialList, sources.CategoryInteractive} {
		if sourceType(category) == "" {
			t.Fatal("合法来源类型缺失")
		}
	}
	if sourceType("unknown") != "" {
		t.Fatal("未知来源被映射")
	}
	p.semantic = func(context.Context, string) (Verdict, error) { return Pass, errors.New("test-only:校验器错误") }
	if _, s, reason, err := p.Review(context.Background(), string(raw), v, []sources.Snapshot{snap}); err != nil || s != Uncertain || reason != "semantic" {
		t.Fatal("语义错误默认通过")
	}
}
func encoded(t *testing.T, v Proposal) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFinalReportPolicyShapeAndEvidence(t *testing.T) {
	p, v, s := fixture(t)
	raw := encoded(t, v)
	got, err := p.Decode(raw)
	if err != nil || p.Bind(got, []sources.Snapshot{s}, v.ThemeName) != nil {
		t.Fatal(err)
	}
	if err := p.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "judge", Payload: raw}); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Proposal){
		"无章节": func(v *Proposal) { v.Report.Sections = nil }, "空节": func(v *Proposal) { v.Report.Sections[0].Nodes = nil }, "节重复": func(v *Proposal) { v.Report.Sections[0] = v.Report.Sections[1] },
		"无备案": func(v *Proposal) { v.Report.Disclosure.FilingNo = "" }, "错模型": func(v *Proposal) { v.Report.Disclosure.ModelName = "另一模型" }, "假资料库": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Provenance = render.ProvLibrary },
		"任意句": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Text = "任意正文" }, "伪标题": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Quote.SourceTitle = "自报标题" },
		"节选": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Quote.Trimmed = true }, "转载": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Quote.Reprint = &render.Reprint{By: "测试", At: 1} },
		"重复引文": func(v *Proposal) { v.Report.Sections[1].Nodes[0] = v.Report.Sections[0].Nodes[0] }, "非题材": func(v *Proposal) { v.ThemeName = "另一主题" },
		"非引文": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Kind = render.KindDataRow }, "越域": func(v *Proposal) { v.Report.Sections[0].Nodes[0].Quote.URL = "https://other.example.test/x" },
		"无主题编号": func(v *Proposal) { v.ThemeID = 0 }, "坏任务号": func(v *Proposal) { v.TaskNo = "bad" },
	} {
		t.Run(name, func(t *testing.T) {
			_, mut, _ := fixture(t)
			change(&mut)
			if _, err := p.Decode(encoded(t, mut)); err == nil {
				t.Fatal("变异被放行")
			}
		})
	}
	for _, bad := range []string{raw + " ", strings.Replace(raw, "task_no", "Task_No", 1), strings.Replace(raw, `"task_no":`, `"extra":1,"task_no":`, 1), strings.Replace(raw, `"task_no":`, `"task_no":"x","task_no":`, 1), "[]", string([]byte{0xff})} {
		if _, err := p.Decode(bad); err == nil {
			t.Fatal("坏编码被放行")
		}
	}
	for _, change := range []func(*sources.Snapshot){func(s *sources.Snapshot) { s.ID++ }, func(s *sources.Snapshot) { s.Hash = strings.Repeat("f", 64) }, func(s *sources.Snapshot) { s.URL += "?other=1" }, func(s *sources.Snapshot) { s.FetchedAt = s.FetchedAt.Add(time.Second) }, func(s *sources.Snapshot) { s.SourceCategory = sources.CategoryAnnouncement }} {
		bad := s
		change(&bad)
		if p.Bind(v, []sources.Snapshot{bad}, v.ThemeName) == nil {
			t.Fatal("错证据被放行")
		}
	}
	if p.Bind(v, []sources.Snapshot{s}, "主数据已改名") == nil || p.Bind(v, nil, v.ThemeName) == nil {
		t.Fatal("题材/来源不足被放行")
	}
}

func TestFinalReportReviewFailClosedAndInputIsolation(t *testing.T) {
	for _, tc := range []struct {
		e, s    Verdict
		failure bool
		reason  string
	}{{Pass, Pass, false, ""}, {Block, Pass, false, "entailment"}, {Uncertain, Pass, false, "entailment"}, {"bad", Pass, false, "entailment"}, {Pass, Block, false, "semantic"}, {Pass, Uncertain, false, "semantic"}, {Pass, "bad", false, "semantic"}, {Pass, Pass, true, "entailment"}} {
		p, v, snap := fixture(t)
		semanticCalls := 0
		p.entailment = func(_ context.Context, in EntailmentInput) (Verdict, error) {
			if strings.Contains(in.EvidenceJSON, snap.LicenseRef) || !strings.Contains(in.EvidenceJSON, snap.Body) {
				t.Fatal("证据输入越界")
			}
			if tc.failure {
				return Pass, errors.New("test-only:上游原文不得透出")
			}
			return tc.e, nil
		}
		p.semantic = func(_ context.Context, text string) (Verdict, error) {
			semanticCalls++
			if strings.Contains(text, snap.URL) || strings.Contains(text, snap.LicenseRef) || !strings.Contains(text, v.ThemeName) {
				t.Fatal("语义输入越界")
			}
			return tc.s, nil
		}
		e, sem, reason, err := p.Review(context.Background(), encoded(t, v), v, []sources.Snapshot{snap})
		if err != nil || reason != tc.reason || !e.Valid() || !sem.Valid() {
			t.Fatal(e, sem, reason, err)
		}
		if reason == "entailment" && semanticCalls != 0 {
			t.Fatal("蕴含失败继续执行")
		}
	}
	p, v, s := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := p.Review(ctx, encoded(t, v), v, []sources.Snapshot{s}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewPolicy(p.cfg, nil, p.semantic); err == nil {
		t.Fatal("缺蕴含默认通过")
	}
	if _, err := NewPolicy(p.cfg, p.entailment, nil); err == nil {
		t.Fatal("缺语义默认通过")
	}
	for _, change := range []func(*Config){func(c *Config) { c.Ref = "" }, func(c *Config) { c.AllowedDomains = nil }, func(c *Config) { c.AllowedDomains = []string{"127.0.0.1"} }, func(c *Config) { c.AllowedDomains = append(c.AllowedDomains, c.AllowedDomains[0]) }} {
		c := p.cfg
		c.AllowedDomains = append([]string(nil), c.AllowedDomains...)
		change(&c)
		if _, err := NewPolicy(c, p.entailment, p.semantic); err == nil {
			t.Fatal("坏策略放行")
		}
	}
	for _, change := range []func(*Config){func(c *Config) { c.ModelName = strings.Repeat("x", 65) }, func(c *Config) { c.Ref = "test-only\x01" }} {
		c := p.cfg
		change(&c)
		if _, err := NewPolicy(c, p.entailment, p.semantic); err == nil {
			t.Fatal("坏公示/控制字符被放行")
		}
	}
	changed := p.cfg
	changed.SemanticVersion = "test-only-changed"
	other, err := NewPolicy(changed, p.entailment, p.semantic)
	if err != nil || other.Digest() == p.Digest() {
		t.Fatal("校验版本不绑定")
	}
}
