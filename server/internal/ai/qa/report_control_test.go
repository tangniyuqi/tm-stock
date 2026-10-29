package qa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
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

func (a *qaReportAccess) ScopeForRead(ctx context.Context, p Proof) (Scope, error) {
	return a.Scope(ctx, p)
}

// 直接构造显著测试存证，不冒充真实模型调用；实际回放核必须核对三步及原摘要。
func TestReportExpiredHistoryReplaysAllStoredChecks(t *testing.T) {
	now := reportNow()
	p, err := NewReportPlan(Identity{1, 3, 1}, reportMessage(), validReportEvidence(), validReportConfig(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !p.scope.ValidUntil.Before(time.Now()) {
		t.Fatal("夹具必须已过执行期")
	}
	rows := []CallRecord{}
	var picks []Citation
	for n, stage := range []string{"select", "entailment", "semantic"} {
		req, model, err := p.request(stage, picks)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		text := `{"verdict":"pass","reason":"verified"}`
		if n == 0 {
			text = `{"action":"answer","reason":"none","citations":[{"snapshot_id":1,"sentence_ids":[1]}]}`
		}
		decision, err := p.decision(stage, text)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			picks = decision.Citations
		}
		rows = append(rows, CallRecord{Call: Call{InputDigest: p.digest, Stage: stage, RequestDigest: task.StageDigest(body), ProfileDigest: model.ProfileDigest, Model: model.Name, Step: n + 1, MaxTokens: p.cfg.Limits.MaxInputTokens + p.cfg.Limits.MaxOutputTokens}, State: "recorded", Result: CallResult{ResponseDigest: reportDigest(stage), RequestID: "test-only-request", Model: model.Name, Usage: testUsage, Decision: decision}, CreatedAt: now, UpdatedAt: now})
	}
	hash, err := p.input.Digest(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{Proof: p.Proof(), MessageHash: hash, State: "answered", Citations: picks, Calls: 3, ReservedTokens: 3 * (p.cfg.Limits.MaxInputTokens + p.cfg.Limits.MaxOutputTokens), QuotaDay: now.Format("2006-01-02"), Version: 6, CreatedAt: now, UpdatedAt: now}
	store := &qaMemoryStore{data: qaMemoryData{Conv: conversation.Record{ID: 3, UserID: 1, Key: "test_conversation_key_01", RequestHash: conversation.CreationHash("测试会话"), Title: "测试会话", Version: 2, LastSeq: 1, CreatedAt: now, UpdatedAt: now}, Messages: []conversation.Message{{ConversationID: 3, UserID: 1, Seq: 1, Key: "test_question_key_01", RequestHash: hash, Input: p.input, CreatedAt: now}}, Question: &record, Calls: rows}}
	readPlan, err := NewReportReadPlanScoped(p.identity, p.input, p.scope, p.cfg, time.Now().UTC().Truncate(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	access := &qaReportAccess{scope: p.scope}
	ctl, err := NewDurableControlWithReports(store, readPlan, access)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		view, err := ctl.Read(context.Background())
		if err != nil || len(view.Quotes) != 1 || view.Quotes[0].Text != validReportEvidence().Quotes[0].Text {
			t.Fatal("过期历史无法从三步存证回放", err)
		}
	}
	if len(store.data.Calls) != 3 || store.data.Question.Version != 6 {
		t.Fatal("读取改变存证或执行次数")
	}
	store.data.Calls[1].Result.Decision.Verdict = "uncertain"
	store.data.Calls[1].Result.Decision.Reason = "uncertain"
	if v, err := ctl.Read(context.Background()); err == nil || len(v.Quotes) != 0 {
		t.Fatal("损坏的判定存证仍可交付")
	}
	store.data.Calls[1].Result.Decision = Decision{Verdict: "pass", Reason: "verified"}
	access.err = ErrUnavailable
	if v, err := ctl.Read(context.Background()); err == nil || len(v.Quotes) != 0 {
		t.Fatal("当前报告撤权仍可读历史")
	}
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
