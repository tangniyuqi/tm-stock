package qa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

type qaReportAccess struct {
	scope Scope
	err   error
	calls int
}

func (a *qaReportAccess) Scope(_ context.Context, _ Proof) (Scope, error) {
	a.calls++
	return a.scope, a.err
}

func qaReportPlan(t *testing.T, count int) *Plan {
	t.Helper()
	now := time.Date(2026, 10, 22, 4, 0, 0, 0, time.UTC)
	quotes := make([]ReportQuote, 0, count)
	for n := 0; n < count; n++ {
		quotes = append(quotes, ReportQuote{
			Section: "definition", Text: fmt.Sprintf("测试报告引文第%d条。", n+1),
			SourceType: "公告", SourceURL: fmt.Sprintf("https://example.test/%d", n+1), CollectedAt: now.Add(-time.Hour),
		})
	}
	ev := ReportEvidence{TaskID: 9, SubjectID: 7, SubjectType: "theme", SubjectName: "测试示例题材", GeneratedAt: now.Add(-time.Minute), Digest: reportDigest("report-control"), Quotes: quotes}
	model := func(name string) Model {
		sum := sha256.Sum256([]byte(name))
		return Model{Name: name, FilingNo: "测试备案", ApprovalRef: "测试批准", QualityRef: "测试质量", BudgetRef: "测试预算", ProfileDigest: hex.EncodeToString(sum[:])}
	}
	cfg := Config{Selector: model("selector"), Entailment: model("entailment"), Semantic: model("semantic"), Limits: Limits{MaxSources: 4, MaxSentences: 32, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second}}
	in := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "报告引用了什么分类？"}
	plan, err := NewReportPlan(Identity{UserID: 1, ConversationID: 3, MessageSeq: 5}, in, ev, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestReportProofEvidenceLimit(t *testing.T) {
	plan := qaReportPlan(t, 9)
	if !plan.Proof().Valid() {
		t.Fatal("报告证明应允许最多16条证据", len(plan.Proof().Evidence))
	}
	material := plan.Proof()
	material.ScopeApproval = "material:1"
	if material.Valid() {
		t.Fatal("资料问答不得借用报告的证据上限")
	}
	if IsReportApproval("report:9:abc") != true || IsReportApproval("material:9") != false {
		t.Fatal("报告批准前缀识别错误")
	}
	if _, err := NewReportPlan(Identity{UserID: 1, ConversationID: 3, MessageSeq: 5},
		conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "report_qa", TaskID: 9, Question: "问题"},
		ReportEvidence{TaskID: 9, SubjectID: 7, SubjectType: "theme", SubjectName: "测试示例题材", GeneratedAt: time.Now(), Digest: reportDigest("x")}, Config{}, time.Now()); err == nil {
		t.Fatal("空引文计划未被拒绝")
	}
}

func TestDurableControlReportAuthorization(t *testing.T) {
	ctx := context.Background()
	plan := qaReportPlan(t, 2)
	store := &qaMemoryStore{}
	if _, err := NewDurableControlWithReports(store, plan, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("报告计划缺少当前报告读取器必须失败")
	}
	pass := &qaReportAccess{scope: plan.scope}
	control, err := NewDurableControlWithReports(store, plan, pass)
	if err != nil {
		t.Fatal(err)
	}
	if err := control.authorize(ctx, nil, plan.Proof()); err != nil || pass.calls != 1 {
		t.Fatal("当前报告范围未通过授权", err, pass.calls)
	}
	changed := copyQA(plan.scope)
	changed.ApprovalRef = "report:9:" + reportDigest("changed")
	fail := &qaReportAccess{scope: changed}
	control, _ = NewDurableControlWithReports(store, plan, fail)
	if err := control.authorize(ctx, nil, plan.Proof()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("报告变化未拒绝旧计划", err)
	}
	down := &qaReportAccess{err: ErrUnavailable}
	control, _ = NewDurableControlWithReports(store, plan, down)
	if err := control.authorize(ctx, nil, plan.Proof()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("报告不可读未拒绝")
	}
}

func TestDurableControlMaterialStillUsesPlatformScope(t *testing.T) {
	plan := qaReportPlan(t, 2)
	plan.input.Skill = "material_qa"
	plan.input.TaskID = 0
	plan.scope.ApprovalRef = "material:1"
	store := &qaMemoryStore{}
	control, err := NewDurableControl(store, plan)
	if err != nil {
		t.Fatal(err)
	}
	tx := &qaMemoryTx{qaMemoryStore: store}
	store.authorized = false
	if err := control.authorize(context.Background(), tx, plan.Proof()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("资料问答仍应走平台范围授权")
	}
	store.authorized = true
	if err := control.authorize(context.Background(), tx, plan.Proof()); err != nil {
		t.Fatal("平台授权通过后不应被报告分支拦截", err)
	}
}
