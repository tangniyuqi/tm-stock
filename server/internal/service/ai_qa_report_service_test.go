package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type qaStubStore struct{ qa.AccessStore }

type qaStubAuth struct{ uid int64 }

func (a qaStubAuth) Authenticate(context.Context, string) (int64, error) { return a.uid, nil }

type qaStubPolicies struct{}

func (qaStubPolicies) ForPolicy(context.Context, string) (qa.Config, error) {
	return qa.Config{}, qa.ErrUnavailable
}

type qaStubModels struct{}

func (qaStubModels) ForPlan(context.Context, qa.Proof) ([3]*provider.OpenAICompat, error) {
	return [3]*provider.OpenAICompat{}, qa.ErrUnavailable
}

func reportQAInput() dto.AIQAInput {
	return dto.AIQAInput{Version: 1, SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "报告的题材定义引用了什么分类？"}
}

func TestReportQAServiceRequiresCompleteDependencies(t *testing.T) {
	store := &qaStubStore{}
	auth := qaStubAuth{uid: 1}
	if _, err := NewAIQAServiceWithReports(store, qaStubPolicies{}, qaStubModels{}, auth, true, &fakeReportEvidenceSource{}, nil); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("只提供证据源必须失败")
	}
	if _, err := NewAIQAServiceWithReports(store, qaStubPolicies{}, qaStubModels{}, auth, true, nil, &fakeReportConfig{}); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("只提供配置必须失败")
	}
	svc, err := NewAIQAServiceWithReports(store, qaStubPolicies{}, qaStubModels{}, auth, true, &fakeReportEvidenceSource{}, &fakeReportConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !svc.reportReady() {
		t.Fatal("完整报告依赖未就绪")
	}
	plain, err := NewAIQAService(store, qaStubPolicies{}, qaStubModels{}, auth, true)
	if err != nil || plain.reportReady() {
		t.Fatal("未装配报告依赖时不应就绪", err)
	}
}

func TestReportQASubmitFailures(t *testing.T) {
	ctx := context.Background()
	auth := qaStubAuth{uid: 1}
	plain, _ := NewAIQAService(&qaStubStore{}, qaStubPolicies{}, qaStubModels{}, auth, true)
	key := "question_0123456789abcdef"
	if _, err := plain.Submit(ctx, "token", 1, 3, key, reportQAInput()); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("缺少报告依赖时必须拒绝", err)
	}
	disabled, _ := NewAIQAServiceWithReports(&qaStubStore{}, qaStubPolicies{}, qaStubModels{}, auth, false, &fakeReportEvidenceSource{}, &fakeReportConfig{})
	if _, err := disabled.Submit(ctx, "token", 1, 3, key, reportQAInput()); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("关闭提交时必须拒绝", err)
	}
	invalid := reportQAInput()
	invalid.TaskID = 0
	if _, err := disabled.Submit(ctx, "token", 1, 3, key, invalid); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("报告追问缺少任务号必须拒绝", err)
	}
	material := reportQAInput()
	material.Skill, material.TaskID = "material_qa", 9
	if _, err := disabled.Submit(ctx, "token", 1, 3, key, material); !errors.Is(err, qa.ErrInvalid) && !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("资料问答携带任务号必须拒绝", err)
	}
	if _, err := disabled.Submit(ctx, "token", 1, 3, "", reportQAInput()); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("缺少幂等键必须拒绝", err)
	}
	if _, err := disabled.Submit(ctx, "token", 1, 0, "k", reportQAInput()); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("缺少会话必须拒绝", err)
	}
}

func TestReportQAReadAccessDoesNotEnableSubmit(t *testing.T) {
	for _, submit := range []bool{false, true} {
		svc, err := NewAIQAServiceWithReportAccess(&qaStubStore{}, qaStubPolicies{}, qaStubModels{}, qaStubAuth{uid: 1}, submit, false, &fakeReportEvidenceSource{}, &fakeReportConfig{})
		if err != nil || !svc.reportReady() {
			t.Fatal("报告只读依赖未保留", err)
		}
		if _, err := svc.Submit(context.Background(), "token", 1, 3, "question_read_only_01", reportQAInput()); !errors.Is(err, qa.ErrUnavailable) {
			t.Fatal("资料提交或报告只读装配隐含开放报告提交", err)
		}
	}
	if _, err := NewAIQAServiceWithReportAccess(&qaStubStore{}, nil, nil, qaStubAuth{uid: 1}, false, true, &fakeReportEvidenceSource{}, &fakeReportConfig{}); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("总体关闭提交时仍接受报告提交授权")
	}
}

func TestReportQAAnswerRequiresDependencies(t *testing.T) {
	svc, _ := NewAIQAService(&qaStubStore{}, qaStubPolicies{}, qaStubModels{}, qaStubAuth{uid: 1}, true)
	record := qa.Record{}
	if _, err := svc.answerReport(context.Background(), "token", 1, 3, 1, record, conversation.Message{}); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("缺少报告依赖时答案读取必须失败关闭", err)
	}
}
