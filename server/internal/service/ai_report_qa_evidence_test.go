package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

type fakeReportReader struct {
	report dto.AIReport
	err    error
	uid    int64
	taskID int64
	calls  int
}

func (f *fakeReportReader) Read(_ context.Context, uid, taskID int64) (dto.AIReport, error) {
	f.calls++
	f.uid, f.taskID = uid, taskID
	return f.report, f.err
}

func qaEvidenceReport() dto.AIReport {
	now := time.Date(2026, 10, 22, 4, 0, 0, 0, time.UTC)
	return dto.AIReport{
		TaskID: 9, SubjectType: "theme", SubjectID: 7, SubjectName: "测试示例题材", GeneratedAt: now.UnixMilli(),
		Sections: []dto.AIReportSection{
			{Key: "definition", Title: "题材定义", AIGenerated: true, Quotes: []dto.AIReportQuote{{Text: "测试示例题材目录列出测试设备分类。", SourceType: "公告", SourceURL: "https://example.test/a", CollectedAt: now.Add(-2 * time.Hour).UnixMilli()}}},
			{Key: "timeline", Title: "事件时间线", AIGenerated: true, Quotes: []dto.AIReportQuote{{Text: "测试示例资料记载部件名称。", SourceType: "年报", SourceURL: "https://example.test/b", CollectedAt: now.Add(-time.Hour).UnixMilli()}}},
			{Key: "companies", Title: "披露公司", AIGenerated: false, Quotes: []dto.AIReportQuote{}, Companies: []dto.AIReportCompany{{Name: "测试示例公司", Code: "600001", Node: "测试环节", Evidence: []dto.AIReportCompanyEvidence{{Text: "测试示例公司证据", SourceType: "年报", SourceURL: "https://example.test/c", CollectedAt: now.UnixMilli(), Trimmed: true}}}}},
		},
	}
}

func qaEvidenceConfig() qa.Config {
	model := func(name string) qa.Model {
		sum := sha256.Sum256([]byte(name))
		return qa.Model{Name: name, FilingNo: "测试备案", ApprovalRef: "测试批准", QualityRef: "测试质量", BudgetRef: "测试预算", ProfileDigest: hex.EncodeToString(sum[:])}
	}
	return qa.Config{Selector: model("selector"), Entailment: model("entailment"), Semantic: model("semantic"), Limits: qa.Limits{MaxSources: 4, MaxSentences: 32, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second}}
}

func TestAIReportQAEvidenceBuildsScopeAndPlan(t *testing.T) {
	reader := &fakeReportReader{report: qaEvidenceReport()}
	adapter, err := NewAIReportQAEvidence(reader)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := adapter.ForQA(context.Background(), 1, 9)
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || reader.uid != 1 || reader.taskID != 9 {
		t.Fatal("证据必须通过本人报告读取入口", reader)
	}
	if ev.TaskID != 9 || ev.SubjectID != 7 || ev.SubjectType != "theme" || ev.SubjectName != "测试示例题材" || len(ev.Quotes) != 2 || len(ev.Digest) != 64 {
		t.Fatal("证据字段不符", ev)
	}
	if ev.Quotes[0].Section != "definition" || ev.Quotes[0].SourceType != "公告" || ev.Quotes[1].Section != "timeline" {
		t.Fatal("只应包含 AI 章节引文")
	}
	scope, err := qa.BuildReportScope(ev, time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.Snapshots) != 2 {
		t.Fatal("报告引文未转为快照")
	}
	plan, err := qa.NewReportPlan(qa.Identity{UserID: 1, ConversationID: 3, MessageSeq: 5}, conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "报告的题材定义引用了什么分类？"}, ev, qaEvidenceConfig(), time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC))
	if err != nil || !plan.Valid() {
		t.Fatal("报告证据未生成有效计划", err)
	}
	// 同一报告重复读取必须得到相同摘要；报告变化必须改变摘要。
	again, err := adapter.ForQA(context.Background(), 1, 9)
	if err != nil || again.Digest != ev.Digest {
		t.Fatal("同一报告摘要不稳定", again.Digest, ev.Digest)
	}
	changed := qaEvidenceReport()
	changed.Sections[0].Quotes[0].Text = "测试示例题材目录列出另一分类。"
	changedReader := &fakeReportReader{report: changed}
	changedAdapter, _ := NewAIReportQAEvidence(changedReader)
	changedEvidence, err := changedAdapter.ForQA(context.Background(), 1, 9)
	if err != nil || changedEvidence.Digest == ev.Digest {
		t.Fatal("报告变化后摘要未变化")
	}
}

func TestAIReportQAEvidenceFailures(t *testing.T) {
	ctx := context.Background()
	cases := map[string]struct {
		reader *fakeReportReader
		want   error
	}{
		"unreadable": {&fakeReportReader{err: finalreport.ErrUnavailable}, qa.ErrUnavailable},
		"other":      {&fakeReportReader{err: errors.New("资料服务暂不可用")}, nil},
		"empty":      {&fakeReportReader{report: dto.AIReport{TaskID: 9, SubjectType: "theme", SubjectID: 7, SubjectName: "测试示例题材", GeneratedAt: time.Now().UnixMilli()}}, qa.ErrUnavailable},
	}
	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			adapter, _ := NewAIReportQAEvidence(item.reader)
			_, err := adapter.ForQA(ctx, 1, 9)
			if err == nil {
				t.Fatal("异常报告未被拒绝")
			}
			if item.want != nil && !errors.Is(err, item.want) {
				t.Fatal("错误语义不符", err)
			}
		})
	}
	if _, err := NewAIReportQAEvidence(nil); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("缺少读取入口必须失败")
	}
	adapter, _ := NewAIReportQAEvidence(&fakeReportReader{report: qaEvidenceReport()})
	if _, err := adapter.ForQA(ctx, 0, 9); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("无效会员必须失败")
	}
	if _, err := adapter.ForQA(ctx, 1, 0); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("无效任务必须失败")
	}
}

func TestAIReportQAEvidenceRejectsOtherOrOversizedReports(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	base := qaEvidenceReport()
	mismatch := base
	mismatch.TaskID = 10
	topic := base
	topic.SubjectType = "topic"
	noName := base
	noName.SubjectName = ""
	noTime := base
	noTime.GeneratedAt = 0
	cases := []dto.AIReport{mismatch, topic, noName, noTime}
	for _, report := range cases {
		adapter, _ := NewAIReportQAEvidence(&fakeReportReader{report: report})
		if _, err := adapter.ForQA(ctx, 1, 9); !errors.Is(err, qa.ErrProtocol) {
			t.Fatal("不一致报告未按协议拒绝", report.TaskID, report.SubjectType)
		}
	}
	oversized := base
	oversized.Sections = []dto.AIReportSection{{Key: "definition", Title: "题材定义", AIGenerated: true}}
	for n := 0; n < qa.ReportMaxQuotes+1; n++ {
		oversized.Sections[0].Quotes = append(oversized.Sections[0].Quotes, dto.AIReportQuote{Text: "测试引文" + string(rune('A'+n)), SourceType: "公告", SourceURL: "https://example.test/a", CollectedAt: now.UnixMilli()})
	}
	adapter, _ := NewAIReportQAEvidence(&fakeReportReader{report: oversized})
	if _, err := adapter.ForQA(ctx, 1, 9); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("超过引文上限未拒绝")
	}
}
