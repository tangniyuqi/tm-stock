package qa

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

func reportDigest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func reportNow() time.Time { return time.Date(2026, 10, 22, 4, 0, 0, 0, time.UTC) }

func validReportEvidence() ReportEvidence {
	now := reportNow()
	return ReportEvidence{
		TaskID:      9,
		SubjectID:   7,
		SubjectType: "theme",
		SubjectName: "测试示例题材",
		GeneratedAt: now.Add(-time.Hour),
		Digest:      reportDigest("report-9"),
		Quotes: []ReportQuote{
			{Section: "definition", Text: "测试示例题材目录列出测试设备分类。", SourceType: "公告", SourceURL: "https://example.test/a", CollectedAt: now.Add(-2 * time.Hour)},
			{Section: "timeline", Text: "测试示例资料记载部件名称。", SourceType: "年报", SourceURL: "https://example.test/b", CollectedAt: now.Add(-90 * time.Minute)},
		},
	}
}

func validReportConfig() Config {
	model := func(name string) Model {
		return Model{Name: name, FilingNo: "测试备案", ApprovalRef: "测试批准", QualityRef: "测试质量", BudgetRef: "测试预算", ProfileDigest: reportDigest(name)}
	}
	return Config{
		Selector: model("selector"), Entailment: model("entailment"), Semantic: model("semantic"),
		Limits: Limits{MaxSources: 4, MaxSentences: 32, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second},
	}
}

func reportMessage() conversation.MessageInput {
	return conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "报告的题材定义引用了什么分类？"}
}

func TestBuildReportScope(t *testing.T) {
	now := reportNow()
	ev := validReportEvidence()
	scope, err := BuildReportScope(ev, now)
	if err != nil {
		t.Fatal(err)
	}
	if scope.SubjectType != "theme" || scope.SubjectID != 7 || scope.SubjectName != ev.SubjectName || len(scope.Snapshots) != 2 {
		t.Fatal("范围字段不符", scope)
	}
	if scope.ApprovalRef != "report:9:"+ev.Digest || !scope.ValidUntil.After(now) || scope.ValidUntil.Sub(now) != reportScopeTTL {
		t.Fatal("批准引用或有效期不符")
	}
	for n, snapshot := range scope.Snapshots {
		if snapshot.ID != int64(n+1) || snapshot.SourceID != int64(n+1) || snapshot.Body != ev.Quotes[n].Text || snapshot.URL != ev.Quotes[n].SourceURL || snapshot.LicenseTag != reportLicenseTag {
			t.Fatal("逐条快照未保持原引文与出处", n)
		}
		if snapshot.Validate() != nil || snapshot.SourceCategory != []string{"announcement", "annual_report"}[n] {
			t.Fatal("快照结构或类别不符", n)
		}
	}
}

func TestNewReportPlan(t *testing.T) {
	now := reportNow()
	plan, err := NewReportPlan(Identity{UserID: 1, ConversationID: 3, MessageSeq: 5}, reportMessage(), validReportEvidence(), validReportConfig(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Valid() || plan.Proof().MaxCalls != 3 || len(plan.Proof().Evidence) != 2 || plan.Proof().SubjectID != 7 {
		t.Fatal("报告计划或证明不符")
	}
	reference, ok := plan.ReportReference()
	if !ok || reference != "report:9:"+validReportEvidence().Digest {
		t.Fatal("报告批准引用不符", reference, ok)
	}
	request, _, err := plan.request("select", nil)
	if err != nil || !strings.Contains(request.Prompt, "测试示例题材目录列出测试设备分类") || !strings.Contains(request.Prompt, "测试示例资料记载部件名称") {
		t.Fatal("提示词未只包含报告引文", err)
	}
	// 资料问答不能借用报告范围。
	if _, ok := (&Plan{input: reportMessage(), scope: Scope{ApprovalRef: "x"}, policy: reportDigest("p"), digest: reportDigest("d")}).ReportReference(); ok {
		t.Fatal("无效计划不得返回报告引用")
	}
}

func TestNewReportPlanRejects(t *testing.T) {
	now := reportNow()
	base := reportMessage()
	cases := map[string]func(*conversation.MessageInput, *ReportEvidence){
		"wrong_skill":   func(in *conversation.MessageInput, _ *ReportEvidence) { in.Skill = "material_qa" },
		"wrong_task":    func(in *conversation.MessageInput, _ *ReportEvidence) { in.TaskID = 10 },
		"wrong_subject": func(in *conversation.MessageInput, _ *ReportEvidence) { in.SubjectID = 8 },
		"topic_subject": func(in *conversation.MessageInput, ev *ReportEvidence) {
			in.SubjectType = "topic"
			ev.SubjectType = "topic"
		},
		"empty_quotes": func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.Quotes = nil },
		"too_many": func(_ *conversation.MessageInput, ev *ReportEvidence) {
			ev.Quotes = make([]ReportQuote, ReportMaxQuotes+1)
		},
		"bad_digest":    func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.Digest = "ABC" },
		"future_report": func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.GeneratedAt = now.Add(time.Minute) },
		"bad_section":   func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.Quotes[0].Section = "" },
		"empty_text":    func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.Quotes[0].Text = "" },
		"control":       func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.Quotes[0].Text = "含控制\u0000字符" },
		"bad_type":      func(_ *conversation.MessageInput, ev *ReportEvidence) { ev.Quotes[0].SourceType = "其他" },
		"http_url": func(_ *conversation.MessageInput, ev *ReportEvidence) {
			ev.Quotes[0].SourceURL = "http://example.test/a"
		},
		"userinfo_url": func(_ *conversation.MessageInput, ev *ReportEvidence) {
			ev.Quotes[0].SourceURL = "https://user@example.test/a"
		},
		"future_quote": func(_ *conversation.MessageInput, ev *ReportEvidence) {
			ev.Quotes[0].CollectedAt = now.Add(time.Minute)
		},
		"after_report": func(_ *conversation.MessageInput, ev *ReportEvidence) {
			ev.Quotes[0].CollectedAt = ev.GeneratedAt.Add(time.Second)
		},
		"not_normalized": func(_ *conversation.MessageInput, ev *ReportEvidence) {
			ev.Quotes[0].Text = "测试示例题材目录  列出测试设备分类。"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in, ev := base, validReportEvidence()
			mutate(&in, &ev)
			if _, err := NewReportPlan(Identity{UserID: 1, ConversationID: 3, MessageSeq: 5}, in, ev, validReportConfig(), now); err == nil {
				t.Fatal("非法报告范围未被拒绝")
			}
		})
	}
}

func TestReportCategoryLabels(t *testing.T) {
	for label, want := range map[string]string{"公告": "announcement", "年报": "annual_report", "招股书": "prospectus", "官方产业目录": "official_catalog", "互动易问答": "interactive_qa"} {
		got, ok := reportCategory(label)
		if !ok || got != want {
			t.Fatal("来源类别映射不符", label, got, ok)
		}
	}
	if _, ok := reportCategory("其他"); ok {
		t.Fatal("未知来源类别不应通过")
	}
}

func TestReportValueStrings(t *testing.T) {
	quote, evidence := ReportQuote{Text: "测试正文", SourceURL: "https://example.test/a"}, validReportEvidence()
	if strings.Contains(quote.String(), "测试正文") || strings.Contains(quote.GoString(), "测试正文") {
		t.Fatal("引文格式化输出泄漏正文")
	}
	if strings.Contains(evidence.String(), "测试示例题材目录") || strings.Contains(evidence.GoString(), "测试示例题材目录") {
		t.Fatal("证据格式化输出泄漏正文")
	}
}

func TestBuildReportScopeMalformed(t *testing.T) {
	now := reportNow()
	cases := map[string]func(*ReportEvidence){
		"task":         func(ev *ReportEvidence) { ev.TaskID = 0 },
		"subject":      func(ev *ReportEvidence) { ev.SubjectID = 0 },
		"subject_type": func(ev *ReportEvidence) { ev.SubjectType = "topic" },
		"name":         func(ev *ReportEvidence) { ev.SubjectName = " 含空白" },
		"generated":    func(ev *ReportEvidence) { ev.GeneratedAt = time.Time{} },
		"quote_time":   func(ev *ReportEvidence) { ev.Quotes[0].CollectedAt = time.Time{} },
		"section":      func(ev *ReportEvidence) { ev.Quotes[0].Section = " 定义 " },
		"long_text":    func(ev *ReportEvidence) { ev.Quotes[0].Text = strings.Repeat("测试", 501) },
		"host":         func(ev *ReportEvidence) { ev.Quotes[0].SourceURL = "https:///a" },
		"empty_url":    func(ev *ReportEvidence) { ev.Quotes[0].SourceURL = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			ev := validReportEvidence()
			mutate(&ev)
			if _, err := BuildReportScope(ev, now); err == nil {
				t.Fatal("非法报告范围未被拒绝")
			}
		})
	}
	if _, err := BuildReportScope(validReportEvidence(), time.Time{}); err == nil {
		t.Fatal("无效当前时点未被拒绝")
	}
}

func TestReportScopeKeepsReportLineBreaks(t *testing.T) {
	ev := validReportEvidence()
	ev.Quotes[0].Text = "第一行\n第二行"
	scope, err := BuildReportScope(ev, reportNow())
	if err != nil {
		t.Fatal(err)
	}
	if scope.Snapshots[0].Body != ev.Quotes[0].Text {
		t.Fatal("报告引文换行必须逐字保留")
	}
}

func TestReportScopeDeduplicatesIdenticalEvidence(t *testing.T) {
	ev := validReportEvidence()
	ev.Quotes[1] = ev.Quotes[0]
	scope, err := BuildReportScope(ev, reportNow())
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.Snapshots) != 1 || scope.Snapshots[0].Body != ev.Quotes[0].Text {
		t.Fatal("重复引文应合并为一条证据", len(scope.Snapshots))
	}
}

func TestParseReportApproval(t *testing.T) {
	sum := reportDigest("approval")
	cases := []struct {
		ref    string
		taskID int64
		ok     bool
	}{
		{"report:9:" + sum, 9, true},
		{"report:1:" + sum, 1, true},
		{"report:0:" + sum, 0, false},
		{"report:09:" + sum, 0, false},
		{"report:-1:" + sum, 0, false},
		{"report:9007199254740992:" + sum, 0, false},
		{"report:9", 0, false},
		{"report:9:ABC", 0, false},
		{"report:9:" + sum[:63], 0, false},
		{"material:9:" + sum, 0, false},
		{"", 0, false},
	}
	for _, item := range cases {
		taskID, digestValue, ok := ParseReportApproval(item.ref)
		if ok != item.ok || taskID != item.taskID || ok && digestValue != sum {
			t.Fatal("报告批准解析不符", item.ref, taskID, ok)
		}
	}
}

func TestBuildReportScopeUntilBounds(t *testing.T) {
	now := reportNow()
	ev := validReportEvidence()
	if _, err := BuildReportScopeUntil(ev, now, now); err == nil {
		t.Fatal("批准时点等于当前时点未被拒绝")
	}
	if _, err := BuildReportScopeUntil(ev, now, now.Add(reportScopeTTL+time.Second)); err == nil {
		t.Fatal("超过报告范围时限未被拒绝")
	}
	if _, err := BuildReportScopeUntil(ev, now, now.Add(reportScopeTTL)); err != nil {
		t.Fatal("边界内批准时点应通过", err)
	}
}

func TestReportPlanRequiresValidIdentity(t *testing.T) {
	now := reportNow()
	for _, identity := range []Identity{{}, {UserID: 1}, {UserID: 1, ConversationID: 3}, {UserID: 0, ConversationID: 3, MessageSeq: 5}, {UserID: 1, ConversationID: 3, MessageSeq: 0}} {
		if _, err := NewReportPlan(identity, reportMessage(), validReportEvidence(), validReportConfig(), now); err == nil {
			t.Fatal("无效身份未被拒绝", identity)
		}
	}
}
