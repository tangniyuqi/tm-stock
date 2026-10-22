package qa

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func manualCall(t *testing.T, p *Plan, step int, picks []Citation) Call {
	t.Helper()
	stage := []string{"select", "entailment", "semantic"}[step-1]
	r, m, err := p.request(stage, picks)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return Call{InputDigest: p.digest, Stage: stage, Step: step, RequestDigest: task.StageDigest(b), ProfileDigest: m.ProfileDigest, Model: m.Name, MaxTokens: p.cfg.Limits.MaxInputTokens + p.cfg.Limits.MaxOutputTokens}
}
func manualRows(t *testing.T, p *Plan) []CallRecord {
	t.Helper()
	now := p.scope.Snapshots[0].FetchedAt
	picks := []Citation{{7, []int{1}}}
	out := []CallRecord{}
	for step := 1; step <= 3; step++ {
		var requestPicks []Citation
		if step > 1 {
			requestPicks = picks
		}
		d := Decision{Verdict: "pass", Reason: "verified"}
		if step == 1 {
			d = Decision{Action: "answer", Reason: "none", Citations: picks}
		}
		out = append(out, CallRecord{Call: manualCall(t, p, step, requestPicks), State: "recorded", CreatedAt: now, UpdatedAt: now, Result: CallResult{ResponseDigest: strings.Repeat("a", 64), RequestID: "test-request", Model: p.cfg.Selector.Name, Usage: testUsage, Decision: d}})
	}
	return out
}

func TestPersistedProofRecordAndCallValidation(t *testing.T) {
	c, _, s, p := durableFixture(t)
	queueQA(t, c)
	base := *s.data.Question
	for n, edit := range []func(*Proof){func(p *Proof) { p.SubjectName = "" }, func(p *Proof) { p.Models = nil }, func(p *Proof) { p.Models[0].Name = "" }, func(p *Proof) { p.Evidence = append(p.Evidence, p.Evidence[0]) }, func(p *Proof) { p.MaxCalls = 4 }, func(p *Proof) { p.MaxTokens = 0 }, func(p *Proof) { p.Identity.UserID = 0 }, func(p *Proof) { p.Evidence[0].Hash = "bad" }} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			v := copyQA(p.Proof())
			edit(&v)
			if v.Valid() {
				t.Fatal("非法执行证明有效")
			}
		})
	}
	for n, edit := range []func(*Record){func(r *Record) { r.CreatedAt = time.Time{} }, func(r *Record) { r.Calls = 4 }, func(r *Record) { r.ReservedTokens = r.Proof.MaxTokens + 1 }, func(r *Record) { r.State = "arbitrary" }, func(r *Record) { r.State = "failed"; r.Reason = "unconfirmed"; r.Version = 2 }, func(r *Record) { r.State = "running"; r.QuotaDay = "bad-date"; r.Version = 2 }} {
		v := copyQA(base)
		edit(&v)
		valid := v.Valid()
		if n == 4 {
			if !valid {
				t.Fatal("未执行的故障状态无效")
			}
		} else if valid {
			t.Fatal("非法执行记录有效", n)
		}
	}
	for _, state := range []string{"running", "refused", "failed"} {
		v := copyQA(base)
		v.State = state
		v.Version = 3
		v.QuotaDay = s.now.Format("2006-01-02")
		if state == "refused" {
			v.Reason = "no_evidence"
		}
		if state == "failed" {
			v.Reason = "unconfirmed"
		}
		if !v.Valid() {
			t.Fatal("合法闭集状态无效", state)
		}
		v.Version = 1
		if v.Valid() {
			t.Fatal("终态版本无效仍通过")
		}
	}
	rows := manualRows(t, p)
	for _, edit := range []func(*CallResult){func(r *CallResult) { r.Usage.Cached = -1 }, func(r *CallResult) { r.RequestID = "坏\n编号" }, func(r *CallResult) { r.ResponseDigest = "bad" }, func(r *CallResult) { r.Estimated = true }, func(r *CallResult) { r.Failure = "arbitrary" }, func(r *CallResult) { r.Decision = Decision{Verdict: "pass", Reason: "verified"} }} {
		v := copyQA(rows[0].Result)
		edit(&v)
		if validCallResult(v, "select") {
			t.Fatal("非法调用结果有效")
		}
	}
	if validDecision(Decision{Verdict: "pass", Reason: "verified"}, "unknown") || validDecision(Decision{Action: "answer", Reason: "none"}, "select") {
		t.Fatal("闭集判定边界失效")
	}
	for _, v := range []any{base, rows[0]} {
		if strings.Contains(fmt.Sprintf("%+v %#v", v, v), p.digest) {
			t.Fatal("格式化泄漏内部引用")
		}
	}
	if sameJSON(make(chan int), nil) || sameJSON(nil, make(chan int)) {
		t.Fatal("序列化失败被当相等")
	}
}

func TestPersistedReplayRequiresEntireOriginalStageChain(t *testing.T) {
	_, _, _, p := durableFixture(t)
	base := manualRows(t, p)
	for name, edit := range map[string]func([]CallRecord) []CallRecord{
		"无阶段":   func([]CallRecord) []CallRecord { return nil },
		"不完整":   func(r []CallRecord) []CallRecord { return r[:1] },
		"过多":    func(r []CallRecord) []CallRecord { return append(r, r[0]) },
		"未完成":   func(r []CallRecord) []CallRecord { r[0].State = "reserved"; return r },
		"异体请求":  func(r []CallRecord) []CallRecord { r[0].Call.RequestDigest = strings.Repeat("b", 64); return r },
		"预算不同":  func(r []CallRecord) []CallRecord { r[0].Call.MaxTokens++; return r },
		"模型不同":  func(r []CallRecord) []CallRecord { r[0].Result.Model = "other-model"; return r },
		"跨范围引文": func(r []CallRecord) []CallRecord { r[0].Result.Decision.Citations[0].SnapshotID = 9; return r },
		"不支持但仍后续执行": func(r []CallRecord) []CallRecord {
			r[1].Result.Decision = Decision{Verdict: "block", Reason: "unsupported"}
			return r
		},
		"先拒绝仍后续执行": func(r []CallRecord) []CallRecord {
			r[0].Result.Decision = Decision{Action: "refuse", Reason: "no_evidence"}
			return r
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := p.replay(edit(copyQA(base))); err == nil {
				t.Fatal("非法存证链可发布")
			}
		})
	}
	if r, err := p.replay(base); err != nil || r.State != "answered" {
		t.Fatal("合法链不能发布", err)
	}
	for _, reason := range []string{"unsupported", "out_of_scope", "request_not_allowed"} {
		r := copyQA(base[:2])
		r[1].Result.Decision = Decision{Verdict: "block", Reason: reason}
		if v, err := p.replay(r); err != nil || v.State != "refused" {
			t.Fatal("独立拒答不能回放", err)
		}
	}
	r := copyQA(base[:1])
	r[0].Result.Decision = Decision{Action: "refuse", Reason: "no_evidence"}
	if v, err := p.replay(r); err != nil || v.State != "refused" {
		t.Fatal("无证据拒答不能回放")
	}
	copy := *p
	copy.scope.Snapshots = nil
	if v, err := copy.replay(nil); err != nil || v.Reason != "no_evidence" {
		t.Fatal("空资料拒答不能回放")
	}
	copy.input.Question = "请提供买卖点"
	if v, err := copy.replay(nil); err != nil || v.Reason != "request_not_allowed" {
		t.Fatal("红线拒答不能回放")
	}
}

func TestDurableControlManualStageAndReceiptBoundaries(t *testing.T) {
	c, _, s, p := durableFixture(t)
	queueQA(t, c)
	ctx := context.Background()
	rows := manualRows(t, p)
	if c.StartCall(ctx, rows[0].Call) == nil {
		t.Fatal("未取得执行权预约")
	}
	if err := c.Begin(ctx, p.Proof()); err != nil {
		t.Fatal(err)
	}
	bad := rows[0].Call
	bad.Step = 2
	if c.StartCall(ctx, bad) == nil {
		t.Fatal("跳步预约")
	}
	bad = rows[0].Call
	bad.RequestDigest = strings.Repeat("f", 64)
	if c.StartCall(ctx, bad) == nil {
		t.Fatal("请求摘要错绑")
	}
	if err := c.StartCall(ctx, rows[0].Call); err != nil {
		t.Fatal(err)
	}
	if c.StartCall(ctx, rows[1].Call) == nil {
		t.Fatal("前步未存证仍预约")
	}
	if err := c.FinishCall(ctx, rows[0].Call, rows[0].Result); err != nil {
		t.Fatal(err)
	}
	if err := c.FinishCall(ctx, rows[0].Call, rows[0].Result); err != nil {
		t.Fatal("同内容审计重放失败", err)
	}
	badResult := rows[0].Result
	badResult.RequestID = "different-test-id"
	if c.FinishCall(ctx, rows[0].Call, badResult) == nil {
		t.Fatal("审计异体重放")
	}
	if c.Complete(ctx, p.Proof(), Receipt{State: "answered"}) == nil {
		t.Fatal("部分链能Complete")
	}
	for n := 1; n < 3; n++ {
		if err := c.StartCall(ctx, rows[n].Call); err != nil {
			t.Fatal(err)
		}
		if err := c.FinishCall(ctx, rows[n].Call, rows[n].Result); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := p.replay(s.data.Calls)
	if err != nil {
		t.Fatal(err)
	}
	wrong := expected
	wrong.Calls = 2
	if c.Complete(ctx, p.Proof(), wrong) == nil {
		t.Fatal("最终收据不能只信调用方")
	}
	if err := c.Complete(ctx, p.Proof(), expected); err != nil {
		t.Fatal(err)
	}
	if c.Complete(ctx, p.Proof(), expected) == nil {
		t.Fatal("终态重复写入")
	}
}

func TestDurableControlExplicitFailureAndArchivedRead(t *testing.T) {
	t.Run("已失败仍保存迟到用量不恢复答案", func(t *testing.T) {
		c, _, s, p := durableFixture(t)
		queueQA(t, c)
		ctx := context.Background()
		rows := manualRows(t, p)
		if err := c.Begin(ctx, p.Proof()); err != nil {
			t.Fatal(err)
		}
		if err := c.StartCall(ctx, rows[0].Call); err != nil {
			t.Fatal(err)
		}
		if err := c.Fail(ctx, "cancelled"); err != nil {
			t.Fatal(err)
		}
		if err := c.FinishCall(ctx, rows[0].Call, rows[0].Result); err != nil || s.data.Question.State != "failed" || s.data.Calls[0].State != "recorded" || s.data.Quotas[0].Tokens != 39072 {
			t.Fatal("迟到用量丢失或恢复了失败执行", err)
		}
	})
	t.Run("未执行故障不占日配额", func(t *testing.T) {
		c, _, s, _ := durableFixture(t)
		queueQA(t, c)
		if err := c.Fail(context.Background(), "execution_failed"); err != nil || !s.data.Question.Valid() || len(s.data.Quotas) != 0 {
			t.Fatal("未执行失败不合法", err)
		}
	})
	t.Run("归档阻止执行", func(t *testing.T) {
		c, e, s, _ := durableFixture(t)
		queueQA(t, c)
		s.data.Conv.Archived = true
		s.data.Conv.Version++
		if _, err := c.Run(context.Background(), e); err == nil || s.data.Question.State != "queued" {
			t.Fatal("归档还执行/误终结")
		}
	})
	t.Run("完成后归档仍可读", func(t *testing.T) {
		c, e, s, p := durableFixture(t)
		queueQA(t, c)
		if _, err := c.Run(context.Background(), e); err != nil {
			t.Fatal(err)
		}
		s.data.Conv.Archived = true
		s.data.Conv.Version++
		if _, err := e.ReadStored(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	})
	if !maxTime(time.Now().Add(-time.Hour), time.Now()).After(time.Now().Add(-time.Minute)) {
		t.Fatal("时间不能回退")
	}
}
