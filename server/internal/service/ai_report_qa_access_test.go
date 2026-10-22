package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
)

type fakeReportEvidenceSource struct {
	evidence qa.ReportEvidence
	err      error
	uid      int64
	taskID   int64
	calls    int
}

func (f *fakeReportEvidenceSource) ForQA(_ context.Context, uid, taskID int64) (qa.ReportEvidence, error) {
	f.calls++
	f.uid, f.taskID = uid, taskID
	return f.evidence, f.err
}

func accessDigest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func reportAccessPlan(t *testing.T, now time.Time) (*qa.Plan, qa.ReportEvidence) {
	t.Helper()
	reader := &fakeReportReader{report: qaEvidenceReport()}
	adapter, _ := NewAIReportQAEvidence(reader)
	evidence, err := adapter.ForQA(context.Background(), 1, 9)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := qa.NewReportPlan(qa.Identity{UserID: 1, ConversationID: 3, MessageSeq: 5},
		conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "报告的题材定义引用了什么分类？"},
		evidence, qaEvidenceConfig(), now)
	if err != nil {
		t.Fatal(err)
	}
	return plan, evidence
}

func TestAIReportQAAccessRebuildsExactScope(t *testing.T) {
	now := time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC)
	plan, evidence := reportAccessPlan(t, now)
	source := &fakeReportEvidenceSource{evidence: evidence}
	access, err := NewAIReportQAAccess(source)
	if err != nil {
		t.Fatal(err)
	}
	access.now = func() time.Time { return now }
	scope, err := access.Scope(context.Background(), plan.Proof())
	if err != nil {
		t.Fatal(err)
	}
	expected, err := qa.BuildReportScope(evidence, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope, expected) {
		t.Fatal("重建范围与原计划范围不一致")
	}
	if source.calls != 1 || source.uid != 1 || source.taskID != 9 {
		t.Fatal("未按会员与任务重新读取报告", source)
	}
	// 摘要变化（报告更新）必须拒绝旧证明。
	changed := evidence
	changed.Digest = accessDigest("changed-evidence")
	changedSource := &fakeReportEvidenceSource{evidence: changed}
	changedAccess, _ := NewAIReportQAAccess(changedSource)
	changedAccess.now = func() time.Time { return now }
	if _, err := changedAccess.Scope(context.Background(), plan.Proof()); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("报告摘要变化未拒绝", err)
	}
}

func TestAIReportQAAccessFailures(t *testing.T) {
	now := time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC)
	plan, evidence := reportAccessPlan(t, now)
	source := &fakeReportEvidenceSource{evidence: evidence}
	access, _ := NewAIReportQAAccess(source)
	access.now = func() time.Time { return now }
	if _, err := NewAIReportQAAccess(nil); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("缺少证据源必须失败")
	}
	bad := plan.Proof()
	bad.ScopeApproval = "report:"
	if _, err := access.Scope(context.Background(), bad); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("无效报告批准未拒绝")
	}
	bad = plan.Proof()
	bad.ScopeApproval = "report:9:" + accessDigest("other")
	if _, err := access.Scope(context.Background(), bad); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("摘要不一致未拒绝")
	}
	bad = plan.Proof()
	bad.Identity.UserID = 0
	if _, err := access.Scope(context.Background(), bad); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("无效会员未拒绝")
	}
	unavailable := &fakeReportEvidenceSource{err: qa.ErrUnavailable}
	failing, _ := NewAIReportQAAccess(unavailable)
	failing.now = func() time.Time { return now }
	if _, err := failing.Scope(context.Background(), plan.Proof()); !errors.Is(err, qa.ErrUnavailable) {
		t.Fatal("不可读报告未失败关闭")
	}
	// 证明有效期与重建时刻不一致时不能复现同一范围。
	late := plan.Proof()
	late.ValidUntil = now.Add(time.Hour)
	if _, err := access.Scope(context.Background(), late); !errors.Is(err, qa.ErrInvalid) {
		t.Fatal("超出报告范围时限未拒绝")
	}
}
