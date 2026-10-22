package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
)

type fakeReportConfig struct {
	config qa.Config
	err    error
	calls  int
}

func (f *fakeReportConfig) ForReport(_ context.Context, _ qa.ReportEvidence) (qa.Config, error) {
	f.calls++
	return f.config, f.err
}

func reportPlanFixture(t *testing.T) (qa.Identity, conversation.MessageInput, qa.ReportEvidence) {
	t.Helper()
	reader := &fakeReportReader{report: qaEvidenceReport()}
	adapter, _ := NewAIReportQAEvidence(reader)
	evidence, err := adapter.ForQA(context.Background(), 1, 9)
	if err != nil {
		t.Fatal(err)
	}
	in := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "报告的题材定义引用了什么分类？"}
	return qa.Identity{UserID: 1, ConversationID: 3, MessageSeq: 5}, in, evidence
}

func TestBuildReportPlanUsesTrustedConfig(t *testing.T) {
	now := time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC)
	id, in, evidence := reportPlanFixture(t)
	provider := &fakeReportConfig{config: qaEvidenceConfig()}
	plan, err := buildReportPlan(context.Background(), id, in, evidence, provider, now)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Valid() || provider.calls != 1 {
		t.Fatal("报告计划未使用可信配置", plan.Valid(), provider.calls)
	}
	reference, ok := plan.ReportReference()
	if !ok || reference != "report:9:"+evidence.Digest {
		t.Fatal("报告计划引用不符", reference)
	}
}

func TestBuildReportPlanRejects(t *testing.T) {
	now := time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC)
	id, in, evidence := reportPlanFixture(t)
	provider := &fakeReportConfig{config: qaEvidenceConfig()}
	cases := []struct {
		name     string
		ctx      context.Context
		id       qa.Identity
		in       conversation.MessageInput
		provider AIReportQAConfigProvider
	}{
		{"nil_context", nil, id, in, provider},
		{"nil_provider", context.Background(), id, in, nil},
		{"material", context.Background(), id, func() conversation.MessageInput { v := in; v.Skill = "material_qa"; v.TaskID = 0; return v }(), provider},
		{"no_task", context.Background(), id, func() conversation.MessageInput { v := in; v.TaskID = 0; return v }(), provider},
		{"other_task", context.Background(), id, func() conversation.MessageInput { v := in; v.TaskID = 10; return v }(), provider},
		{"bad_identity", context.Background(), qa.Identity{}, in, provider},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if _, err := buildReportPlan(item.ctx, item.id, item.in, evidence, item.provider, now); !errors.Is(err, qa.ErrInvalid) {
				t.Fatal("非法输入未被拒绝", err)
			}
		})
	}
	failing := &fakeReportConfig{err: qa.ErrUnavailable}
	if _, err := buildReportPlan(context.Background(), id, in, evidence, failing, now); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("配置不可用未失败关闭", err)
	}
	invalid := &fakeReportConfig{config: qa.Config{}}
	if _, err := buildReportPlan(context.Background(), id, in, evidence, invalid, now); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("无效配置未拒绝", err)
	}
}
