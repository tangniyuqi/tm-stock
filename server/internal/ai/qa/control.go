package qa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// ReportAccess 在每次执行/读取前重建报告证据范围；实现必须重新读取本人当前可读报告。
type ReportAccess interface {
	Scope(context.Context, Proof) (Scope, error)
}

// ReportReadAccess 单独复核当前历史读取权，不能替代执行授权接口。
type ReportReadAccess interface {
	ScopeForRead(context.Context, Proof) (Scope, error)
}

// DurableControl只依赖事务存储；生产装配须另验结构保护和真实批准，不能用测试Store替代。
type DurableControl struct {
	store   Store
	plan    *Plan
	reports ReportAccess
}

func NewDurableControl(store Store, p *Plan) (*DurableControl, error) {
	return NewDurableControlWithReports(store, p, nil)
}

// NewDurableControlWithReports 对报告追问强制注入当前报告读取器；资料问答不得走该分支。
func NewDurableControlWithReports(store Store, p *Plan, reports ReportAccess) (*DurableControl, error) {
	if store == nil || !p.Valid() || reports != nil && p.input.Skill != "report_qa" || IsReportApproval(p.Proof().ScopeApproval) && (reports == nil || p.input.Skill != "report_qa") {
		return nil, ErrInvalid
	}
	return &DurableControl{store: store, plan: p, reports: reports}, nil
}

// authorize 对报告追问使用当前报告重建的范围并逐字段比对；资料问答继续用原平台范围批准。
func (c *DurableControl) authorize(ctx context.Context, tx Tx, proof Proof) error {
	if !IsReportApproval(proof.ScopeApproval) {
		return tx.Authorize(ctx, proof)
	}
	if c.reports == nil || c.plan.input.Skill != "report_qa" {
		return ErrUnavailable
	}
	var scope Scope
	var err error
	if c.plan.replayOnly {
		reader, ok := c.reports.(ReportReadAccess)
		if !ok {
			return ErrUnavailable
		}
		scope, err = reader.ScopeForRead(ctx, proof)
	} else {
		scope, err = c.reports.Scope(ctx, proof)
	}
	if err != nil {
		return err
	}
	if !sameJSON(scope, c.plan.scope) {
		return fmt.Errorf("报告范围与当前报告不符: %w", ErrUnavailable)
	}
	return nil
}
func (c *DurableControl) within(parent context.Context, write bool, fn func(context.Context, Tx) error) error {
	if c == nil || c.store == nil || !c.plan.Valid() || parent == nil || fn == nil || write && c.plan.replayOnly {
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	return c.store.WithinQA(ctx, write, func(tx Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
}
func (c *DurableControl) load(ctx context.Context, tx Tx, lock, authorize bool) (Record, error) {
	id := c.plan.identity
	conv, err := tx.ByID(ctx, id.UserID, id.ConversationID, lock)
	if err != nil {
		return Record{}, err
	}
	if !conv.Valid() || conv.UserID != id.UserID || conv.ID != id.ConversationID {
		return Record{}, ErrProtocol
	}
	if conv.LastSeq < id.MessageSeq {
		return Record{}, ErrNotFound
	}
	r, err := tx.Question(ctx, id, lock)
	if err != nil {
		return Record{}, err
	}
	hash, err := c.plan.input.Digest(id.UserID, id.ConversationID)
	if err != nil {
		return Record{}, err
	}
	if !r.Valid() || !sameJSON(r.Proof, c.plan.Proof()) || r.MessageHash != hash {
		return Record{}, ErrConflict
	}
	ms, err := tx.Messages(ctx, id.UserID, id.ConversationID, id.MessageSeq-1, id.MessageSeq, 1)
	if err != nil {
		return Record{}, err
	}
	if len(ms) != 1 || !ms[0].Valid() || ms[0].Seq != id.MessageSeq || ms[0].ConversationID != id.ConversationID || ms[0].UserID != id.UserID || ms[0].RequestHash != hash || !sameJSON(ms[0].Input, c.plan.input) {
		return Record{}, ErrProtocol
	}
	if authorize {
		if r.State == "failed" || conv.Archived && (r.State == "queued" || r.State == "running") {
			return Record{}, ErrUnavailable
		}
		if err := c.authorize(ctx, tx, c.plan.Proof()); err != nil {
			return Record{}, err
		}
	}
	return r, nil
}

// Enqueue原子保存用户输入及queued引用；提交未确认只能用原键查，不调用模型。
func (c *DurableControl) Enqueue(ctx context.Context, key string, version int64) (Record, error) {
	if !points.ValidKey(key) || !conversation.ID(version) {
		return Record{}, ErrInvalid
	}
	var out Record
	err := c.within(ctx, true, func(ctx context.Context, tx Tx) error {
		id := c.plan.identity
		conv, err := tx.ByID(ctx, id.UserID, id.ConversationID, true)
		if err != nil {
			return err
		}
		if !conv.Valid() || conv.UserID != id.UserID || conv.ID != id.ConversationID {
			return ErrProtocol
		}
		previous, err := tx.QuestionByKey(ctx, id.UserID, id.ConversationID, key)
		if err == nil {
			if !previous.Valid() || !sameJSON(previous.Proof, c.plan.Proof()) {
				return ErrConflict
			}
			out, err = c.load(ctx, tx, true, false)
			if err != nil {
				return err
			}
			// 这里只恢复本人原输入/状态收据，不返回引文或授予执行权限；撤权后也须能确认是否已保存。
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		if conv.Archived || conv.Version != version || conv.LastSeq+1 != id.MessageSeq || conv.LastSeq >= conversation.MaxID-1 || conv.Version >= conversation.MaxID {
			return ErrConflict
		}
		if err := c.authorize(ctx, tx, c.plan.Proof()); err != nil {
			return err
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		if !conversation.Time(now) || !now.Before(c.plan.scope.ValidUntil) {
			return ErrUnavailable
		}
		if now.Before(conv.UpdatedAt) {
			now = conv.UpdatedAt
		}
		hash, err := c.plan.input.Digest(id.UserID, id.ConversationID)
		if err != nil {
			return err
		}
		out = Record{Proof: c.plan.Proof(), MessageHash: hash, State: "queued", Version: 1, CreatedAt: now, UpdatedAt: now}
		if !out.Valid() {
			return ErrInvalid
		}
		next := conv
		next.LastSeq++
		next.Version++
		next.UpdatedAt = now
		if err := tx.Update(ctx, conv, next); err != nil {
			return err
		}
		if err := tx.Append(ctx, conversation.Message{UserID: id.UserID, ConversationID: id.ConversationID, Seq: id.MessageSeq, Key: key, RequestHash: hash, Input: c.plan.input, CreatedAt: now}); err != nil {
			return err
		}
		return tx.InsertQuestion(ctx, out)
	})
	if err != nil {
		return Record{}, err
	}
	return out, nil
}
func (c *DurableControl) Begin(ctx context.Context, p Proof) error {
	if c == nil || !sameJSON(c.plan.Proof(), p) {
		return ErrInvalid
	}
	return c.within(ctx, true, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, true, true)
		if err != nil {
			return err
		}
		if r.State != "queued" {
			return ErrConflict
		}
		policy, err := tx.QuotaPolicy(ctx)
		if err != nil {
			return err
		}
		if !policy.Valid() {
			return ErrUnavailable
		}
		for _, m := range p.Models {
			if m.BudgetRef != policy.Ref {
				return ErrUnavailable
			}
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		day := now.UTC().Format("2006-01-02")
		for _, uid := range []int64{0, p.Identity.UserID} {
			q, err := tx.Quota(ctx, day, uid)
			if err != nil {
				return err
			}
			maxRequests, maxTokens := policy.InstanceRequests, policy.InstanceTokens
			if uid != 0 {
				maxRequests, maxTokens = policy.UserRequests, policy.UserTokens
			}
			if q.UserID != uid || q.Day != day || q.Requests < 0 || q.Tokens < 0 || q.Requests >= maxRequests || q.Tokens > maxTokens-int64(p.MaxTokens) {
				return ErrQuota
			}
			if err := tx.AddQuota(ctx, q, int64(p.MaxTokens)); err != nil {
				return err
			}
		}
		next := r
		next.State = "running"
		next.Version++
		next.QuotaDay = day
		next.UpdatedAt = maxTime(now, r.UpdatedAt)
		return tx.SetQuestion(ctx, r, next)
	})
}
func (c *DurableControl) Recheck(ctx context.Context, p Proof) error {
	if c == nil || !sameJSON(c.plan.Proof(), p) {
		return ErrInvalid
	}
	return c.within(ctx, false, func(ctx context.Context, tx Tx) error { _, err := c.load(ctx, tx, false, true); return err })
}
func (c *DurableControl) StartCall(ctx context.Context, v Call) error {
	return c.within(ctx, true, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, true, true)
		if err != nil {
			return err
		}
		if r.State != "running" || r.Calls >= 3 || v.Step != r.Calls+1 {
			return ErrConflict
		}
		calls, err := tx.Calls(ctx, c.plan.digest)
		if err != nil {
			return err
		}
		if len(calls) != r.Calls {
			return ErrProtocol
		}
		var picks []Citation
		for n, previous := range calls {
			if !previous.Valid() || previous.State != "recorded" || previous.Call.Step != n+1 || previous.Result.Failure != "" {
				return ErrProtocol
			}
			if n == 0 {
				if previous.Result.Decision.Action != "answer" {
					return ErrProtocol
				}
				picks = previous.Result.Decision.Citations
			} else if previous.Result.Decision.Verdict != "pass" {
				return ErrProtocol
			}
		}
		request, model, err := c.plan.request(v.Stage, picks)
		if err != nil {
			return err
		}
		b, err := json.Marshal(request)
		if err != nil {
			return ErrInvalid
		}
		if v.InputDigest != c.plan.digest || v.Model != model.Name || v.ProfileDigest != model.ProfileDigest || v.RequestDigest != task.StageDigest(b) || v.MaxTokens != c.plan.cfg.Limits.MaxInputTokens+c.plan.cfg.Limits.MaxOutputTokens || v.MaxTokens > r.Proof.MaxTokens-r.ReservedTokens {
			return ErrProtocol
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		call := CallRecord{Call: v, State: "reserved", CreatedAt: now, UpdatedAt: now}
		if !call.Valid() {
			return ErrInvalid
		}
		if err := tx.InsertCall(ctx, call); err != nil {
			return err
		}
		next := r
		next.Calls++
		next.ReservedTokens += v.MaxTokens
		next.Version++
		next.UpdatedAt = maxTime(now, r.UpdatedAt)
		return tx.SetQuestion(ctx, r, next)
	})
}
func (c *DurableControl) FinishCall(parent context.Context, v Call, result CallResult) error {
	if parent == nil {
		return ErrInvalid
	}
	// 调用已发生，取消后也做有界审计；不撤销取消，不再执行模型或交付答案。
	return c.within(context.WithoutCancel(parent), true, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, true, false)
		if err != nil {
			return err
		}
		if r.State != "running" && r.State != "failed" || v.InputDigest != c.plan.digest || !validCallResult(result, v.Stage) {
			return ErrProtocol
		}
		rows, err := tx.Calls(ctx, c.plan.digest)
		if err != nil {
			return err
		}
		if v.Step < 1 || v.Step > len(rows) {
			return ErrProtocol
		}
		old := rows[v.Step-1]
		if !old.Valid() || !sameJSON(old.Call, v) {
			return ErrProtocol
		}
		if old.State == "recorded" {
			if sameJSON(old.Result, result) {
				return nil
			}
			return ErrConflict
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		return tx.FinishCall(ctx, old, result, maxTime(now, old.UpdatedAt))
	})
}
func (c *DurableControl) Complete(ctx context.Context, p Proof, v Receipt) error {
	if c == nil || !sameJSON(c.plan.Proof(), p) {
		return ErrInvalid
	}
	return c.within(ctx, true, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, true, true)
		if err != nil {
			return err
		}
		if r.State != "running" {
			return ErrConflict
		}
		calls, err := tx.Calls(ctx, c.plan.digest)
		if err != nil {
			return err
		}
		expected, err := c.plan.replay(calls)
		if err != nil {
			return err
		}
		if !matchesReservations(r, calls) || !sameJSON(v, expected) {
			return ErrProtocol
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		next := r
		next.State = v.State
		next.Reason = v.Reason
		next.Citations = clonePicks(v.Citations)
		next.Version++
		next.UpdatedAt = maxTime(now, r.UpdatedAt)
		return tx.SetQuestion(ctx, r, next)
	})
}
func (c *DurableControl) ReadReceipt(ctx context.Context, p Proof) (Receipt, error) {
	if c == nil || !sameJSON(c.plan.Proof(), p) {
		return Receipt{}, ErrInvalid
	}
	var out Receipt
	err := c.within(ctx, false, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, false, true)
		if err != nil {
			return err
		}
		if r.State != "answered" && r.State != "refused" {
			return ErrUnavailable
		}
		calls, err := tx.Calls(ctx, c.plan.digest)
		if err != nil {
			return err
		}
		expected, err := c.plan.replay(calls)
		if err != nil {
			return err
		}
		if !matchesReservations(r, calls) || !sameJSON(r.Receipt(), expected) {
			return ErrProtocol
		}
		out = expected
		return nil
	})
	if err != nil {
		return Receipt{}, err
	}
	return out, nil
}
func matchesReservations(r Record, rows []CallRecord) bool {
	total := 0
	for _, row := range rows {
		total += row.Call.MaxTokens
	}
	return len(rows) == r.Calls && total == r.ReservedTokens
}
func (c *DurableControl) Fail(ctx context.Context, reason string) error {
	if !slices.Contains([]string{"execution_failed", "cancelled", "unconfirmed"}, reason) {
		return ErrInvalid
	}
	return c.within(ctx, true, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, true, false)
		if err != nil {
			return err
		}
		if r.State == "answered" || r.State == "refused" || r.State == "failed" {
			return nil
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		next := r
		next.State = "failed"
		next.Reason = reason
		next.Version++
		next.UpdatedAt = maxTime(now, r.UpdatedAt)
		return tx.SetQuestion(ctx, r, next)
	})
}

// RejectQueued仅处理已知未获执行权的配额拒绝；不能终结并发已进入running的执行。
func (c *DurableControl) RejectQueued(ctx context.Context) error {
	return c.within(ctx, true, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, true, false)
		if err != nil {
			return err
		}
		if r.State != "queued" {
			return nil
		}
		now, err := checkedQANow(ctx, tx)
		if err != nil {
			return err
		}
		next := r
		next.State = "failed"
		next.Reason = "execution_failed"
		next.Version++
		next.UpdatedAt = maxTime(now, r.UpdatedAt)
		return tx.SetQuestion(ctx, r, next)
	})
}

type ownedAttempt struct {
	*DurableControl
	begun bool
	check func(context.Context) error
}

func (a *ownedAttempt) Begin(ctx context.Context, p Proof) error {
	if a.check != nil {
		if err := a.check(ctx); err != nil {
			return err
		}
	}
	err := a.DurableControl.Begin(ctx, p)
	if err == nil {
		a.begun = true
	}
	return err
}
func (a *ownedAttempt) Recheck(ctx context.Context, p Proof) error {
	if a.check != nil {
		if err := a.check(ctx); err != nil {
			return err
		}
	}
	return a.DurableControl.Recheck(ctx, p)
}
func (a *ownedAttempt) StartCall(ctx context.Context, v Call) error {
	if a.check != nil {
		if err := a.check(ctx); err != nil {
			return err
		}
	}
	return a.DurableControl.StartCall(ctx, v)
}
func (a *ownedAttempt) Complete(ctx context.Context, p Proof, v Receipt) error {
	if a.check != nil {
		if err := a.check(ctx); err != nil {
			return err
		}
	}
	return a.DurableControl.Complete(ctx, p, v)
}

func (c *DurableControl) Run(ctx context.Context, e *Engine) (*Outcome, error) {
	return c.RunChecked(ctx, e, nil)
}
func (c *DurableControl) RunChecked(ctx context.Context, e *Engine, check func(context.Context) error) (*Outcome, error) {
	if c == nil || ctx == nil || e == nil || e.control != c {
		return nil, ErrInvalid
	}
	// 只有本次已确认取得唯一执行权，才有权写失败；并发/重放失败不能终结别人的在途执行。
	bound := *e
	attempt := &ownedAttempt{DurableControl: c, check: check}
	bound.control = attempt
	v, err := bound.Run(ctx, c.plan)
	if err == nil {
		return v, nil
	}
	if !attempt.begun {
		return nil, err
	}
	reason := "execution_failed"
	if errors.Is(ctx.Err(), context.Canceled) {
		reason = "cancelled"
	}
	if audit := c.Fail(context.WithoutCancel(ctx), reason); audit != nil {
		return nil, ErrUnavailable
	}
	return nil, err
}

// Read不依赖模型实例/凭据，仍完整回放原存证及每次当前授权。
func (c *DurableControl) Read(ctx context.Context) (View, error) {
	if c == nil {
		return View{}, ErrInvalid
	}
	return (&Engine{control: c}).ReadStored(ctx, c.plan)
}
func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
func checkedQANow(ctx context.Context, tx Tx) (time.Time, error) {
	now, err := tx.Now(ctx)
	if err != nil {
		return time.Time{}, err
	}
	if !conversation.Time(now) {
		return time.Time{}, ErrProtocol
	}
	return now, nil
}

// ReadStored只回放原存证，不调用模型，不用旧Outcome替代当前授权。
func (e *Engine) ReadStored(ctx context.Context, p *Plan) (View, error) {
	if e == nil || ctx == nil || !p.Valid() {
		return View{}, ErrInvalid
	}
	reader, ok := e.control.(interface {
		ReadReceipt(context.Context, Proof) (Receipt, error)
	})
	if !ok {
		return View{}, ErrUnavailable
	}
	r, err := reader.ReadReceipt(ctx, p.Proof())
	if err != nil {
		return View{}, err
	}
	if r.InputDigest != p.digest || r.PolicyDigest != p.policy {
		return View{}, ErrProtocol
	}
	v := &Outcome{planDigest: p.digest, policy: p.policy, state: r.State, reason: r.Reason, citations: clonePicks(r.Citations), calls: r.Calls}
	v.seal = outcomeSeal(v.planDigest, v.policy, v.state, v.reason, v.citations, v.calls)
	return e.View(ctx, p, v)
}
