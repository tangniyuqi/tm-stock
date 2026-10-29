package qa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// Control由持久层实现：Begin必须先确认唯一执行权及用户/实例配额，任何未知不能返回nil。
// Start/FinishCall须先预约最坏预算再记录真实用量，Complete须持久存证；不能用内存通过器装配生产。
type Control interface {
	Begin(context.Context, Proof) error
	Recheck(context.Context, Proof) error
	StartCall(context.Context, Call) error
	FinishCall(context.Context, Call, CallResult) error
	Complete(context.Context, Proof, Receipt) error
}
type Call struct {
	InputDigest, Stage, RequestDigest, ProfileDigest, Model string
	Step, MaxTokens                                         int
}
type CallResult struct {
	ResponseDigest, RequestID, Model, Failure string
	// 失败时仍保留适配器已取得的用量；未知/估计或元数据无效时，零值绝不表示已确认零成本。
	Usage     provider.Usage
	Estimated bool
	Decision  Decision
}
type Receipt struct {
	State, Reason, InputDigest, PolicyDigest string
	Citations                                []Citation
	Calls                                    int
}

func (Receipt) String() string     { return "问答核验收据（内部引用已隐藏）" }
func (r Receipt) GoString() string { return r.String() }

type Engine struct {
	models  [3]*provider.OpenAICompat
	control Control
}

func NewEngine(selector, entailment, semantic *provider.OpenAICompat, c Control) (*Engine, error) {
	if selector == nil || entailment == nil || semantic == nil || c == nil || selector.DirectProfileDigest() == "" || entailment.DirectProfileDigest() == "" || semantic.DirectProfileDigest() == "" {
		return nil, ErrInvalid
	}
	return &Engine{[3]*provider.OpenAICompat{selector, entailment, semantic}, c}, nil
}

// Outcome不可从外部字段伪造；仅经本核Complete成功后签发，不含模型原文。
type Outcome struct {
	planDigest, policy, state, reason string
	citations                         []Citation
	calls                             int
	seal                              string
}

func (Outcome) String() string     { return "问答结果（正文及内部引用已隐藏）" }
func (v Outcome) GoString() string { return v.String() }
func outcomeSeal(digest, policy, state, reason string, picks []Citation, calls int) string {
	b, err := json.Marshal([]any{Version, digest, policy, state, reason, picks, calls})
	if err != nil {
		return ""
	}
	return task.StageDigest(b)
}
func (v *Outcome) valid(p *Plan) bool {
	return v != nil && p.Valid() && v.planDigest == p.digest && v.policy == p.policy && v.calls >= 0 && v.calls <= 3 && v.seal == outcomeSeal(v.planDigest, v.policy, v.state, v.reason, v.citations, v.calls) && (v.state == "answered" && v.reason == "" && v.calls == 3 && len(v.citations) > 0 || v.state == "refused" && refusalTemplate(v.reason) != "" && len(v.citations) == 0)
}
func (e *Engine) check(ctx context.Context, p *Plan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.replayOnly && !time.Now().Before(p.scope.ValidUntil) {
		return ErrUnavailable
	}
	if err := e.control.Recheck(ctx, p.Proof()); err != nil {
		return fmt.Errorf("问答当前范围复核未通过: %w", ErrUnavailable)
	}
	return ctx.Err()
}
func responseValid(r provider.Response, m Model, l Limits) bool {
	u := r.Usage
	if r.Model != m.Name || r.FinishReason != "stop" || r.UsageEstimated || u.Prompt <= 0 || u.Completion <= 0 || u.Total != u.Prompt+u.Completion || u.Prompt > l.MaxInputTokens || u.Completion > l.MaxOutputTokens || u.Cached < 0 || u.Cached > u.Prompt || u.Reasoning < 0 || u.Reasoning > u.Completion || !meta(r.RequestID) {
		return false
	}
	// 通用适配器允许补全缺失model/total并舍入token；本核不能把这些兼容值冒充原始存证。
	var envelope struct {
		Model   string                     `json:"model"`
		Usage   map[string]json.RawMessage `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if task.CheckStageJSON(r.Raw) != nil || json.Unmarshal(r.Raw, &envelope) != nil || envelope.Model != m.Name || len(envelope.Choices) != 1 || envelope.Choices[0].FinishReason != "stop" {
		return false
	}
	for key, want := range map[string]int{"prompt_tokens": u.Prompt, "completion_tokens": u.Completion, "total_tokens": u.Total} {
		if !rawCount(envelope.Usage, key, want, true) {
			return false
		}
	}
	for _, detail := range []struct {
		key, field string
		want       int
	}{{"prompt_tokens_details", "cached_tokens", u.Cached}, {"completion_tokens_details", "reasoning_tokens", u.Reasoning}} {
		if raw, ok := envelope.Usage[detail.key]; ok {
			var fields map[string]json.RawMessage
			if json.Unmarshal(raw, &fields) != nil || fields == nil || !rawCount(fields, detail.field, detail.want, detail.want != 0) {
				return false
			}
		}
	}
	if _, ok := envelope.Usage["prompt_cache_hit_tokens"]; ok && !rawCount(envelope.Usage, "prompt_cache_hit_tokens", u.Cached, true) {
		return false
	}
	return true
}

func rawCount(fields map[string]json.RawMessage, key string, want int, required bool) bool {
	raw, ok := fields[key]
	if !ok {
		return !required
	}
	var actual *int
	return json.Unmarshal(raw, &actual) == nil && actual != nil && *actual == want
}
func (e *Engine) call(ctx context.Context, p *Plan, index int, stage string, picks []Citation) (provider.Response, error) {
	request, model, err := p.request(stage, picks)
	if err != nil {
		return provider.Response{}, err
	}
	if !e.models[index].MatchesDirect(model.ProfileDigest, model.Name) {
		return provider.Response{}, ErrUnavailable
	}
	if err := e.check(ctx, p); err != nil {
		return provider.Response{}, err
	}
	b, err := json.Marshal(request)
	if err != nil {
		return provider.Response{}, ErrInvalid
	}
	call := Call{InputDigest: p.digest, Stage: stage, RequestDigest: task.StageDigest(b), ProfileDigest: model.ProfileDigest, Model: model.Name, Step: index + 1, MaxTokens: p.cfg.Limits.MaxInputTokens + p.cfg.Limits.MaxOutputTokens}
	if err := e.control.StartCall(ctx, call); err != nil {
		return provider.Response{}, fmt.Errorf("问答调用预约未确认: %w", ErrUnavailable)
	}
	if err := e.check(ctx, p); err != nil {
		// 已有预约但尚未发送：仍须配对记录，不能把预约当实际供应商用量。
		if audit := e.control.FinishCall(ctx, call, CallResult{Failure: "not_sent"}); audit != nil {
			return provider.Response{}, fmt.Errorf("问答未发送记录未确认: %w", ErrUnavailable)
		}
		return provider.Response{}, err
	}
	r, cause := e.models[index].Generate(ctx, request)
	result := CallResult{RequestID: r.RequestID, Model: r.Model, Usage: r.Usage, Estimated: r.UsageEstimated, ResponseDigest: task.StageDigest([]byte(r.Text))}
	if cause != nil {
		result.Failure = "response_unknown"
		var upstream *provider.Error
		if errors.As(cause, &upstream) && upstream != nil {
			result.RequestID, result.Usage = upstream.RequestID, upstream.Usage
		}
	} else if !responseValid(r, model, p.cfg.Limits) {
		result.Failure = "invalid_metadata"
	} else {
		result.Decision, err = p.decision(stage, r.Text)
		if err != nil {
			result.Failure = "invalid_protocol"
			result.Decision = Decision{}
		}
	}
	if !meta(result.Model) {
		result.Model = ""
	}
	if !meta(result.RequestID) {
		result.RequestID = ""
	}
	// 模型原文/Raw不进入可交付收据；实际用量不补0、不估算冒充供应商记录。
	if err := e.control.FinishCall(ctx, call, result); err != nil {
		return provider.Response{}, fmt.Errorf("问答调用结果存证未确认: %w", ErrUnavailable)
	}
	if cause != nil {
		return provider.Response{}, ErrUnavailable
	}
	if result.Failure != "" {
		return provider.Response{}, ErrProtocol
	}
	if err := e.check(ctx, p); err != nil {
		return provider.Response{}, err
	}
	r.Raw = nil
	return r, nil
}
func (e *Engine) complete(ctx context.Context, p *Plan, state, reason string, picks []Citation, calls int) (*Outcome, error) {
	if err := e.check(ctx, p); err != nil {
		return nil, err
	}
	r := Receipt{State: state, Reason: reason, InputDigest: p.digest, PolicyDigest: p.policy, Citations: clonePicks(picks), Calls: calls}
	if err := e.control.Complete(ctx, p.Proof(), r); err != nil {
		return nil, fmt.Errorf("问答最终结果存证未确认: %w", ErrUnavailable)
	}
	if err := e.check(ctx, p); err != nil {
		return nil, err
	}
	v := &Outcome{p.digest, p.policy, state, reason, clonePicks(picks), calls, ""}
	v.seal = outcomeSeal(v.planDigest, v.policy, v.state, v.reason, v.citations, v.calls)
	if !v.valid(p) {
		return nil, ErrProtocol
	}
	return v, nil
}

// Run只有一次执行权限，至多三个单次调用；异常不重试、不回退、不隐式生成报告。
func (e *Engine) Run(parent context.Context, p *Plan) (*Outcome, error) {
	if e == nil || e.control == nil || parent == nil || !p.Valid() || p.replayOnly {
		return nil, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, p.cfg.Limits.Timeout)
	defer cancel()
	for n, m := range []Model{p.cfg.Selector, p.cfg.Entailment, p.cfg.Semantic} {
		if !e.models[n].MatchesDirect(m.ProfileDigest, m.Name) {
			return nil, ErrUnavailable
		}
	}
	if err := e.check(ctx, p); err != nil {
		return nil, err
	}
	if err := e.control.Begin(ctx, p.Proof()); err != nil {
		if errors.Is(err, ErrQuota) {
			return nil, ErrQuota
		}
		return nil, fmt.Errorf("问答执行权限未确认: %w", ErrUnavailable)
	}
	if !safeQuestion(p.input.Question) {
		return e.complete(ctx, p, "refused", "request_not_allowed", nil, 0)
	}
	if len(p.scope.Snapshots) == 0 {
		return e.complete(ctx, p, "refused", "no_evidence", nil, 0)
	}
	r, err := e.call(ctx, p, 0, "select", nil)
	if err != nil {
		return nil, err
	}
	selected, err := decodeSelection(r.Text)
	if err != nil {
		return nil, err
	}
	if selected.Action == "refuse" {
		return e.complete(ctx, p, "refused", selected.Reason, nil, 1)
	}
	if _, err := p.quotes(selected.Citations); err != nil {
		return nil, err
	}
	for n, stage := range []string{"entailment", "semantic"} {
		r, err = e.call(ctx, p, n+1, stage, selected.Citations)
		if err != nil {
			return nil, err
		}
		decision, err := decodeJudge(r.Text)
		if err != nil {
			return nil, err
		}
		if decision.Verdict != "pass" {
			reason := decision.Reason
			if !slices.Contains([]string{"out_of_scope", "request_not_allowed"}, reason) {
				reason = "unverified"
			}
			return e.complete(ctx, p, "refused", reason, nil, n+2)
		}
	}
	return e.complete(ctx, p, "answered", "", selected.Citations, 3)
}

// View每次展示都重新复核当前范围；不能把旧Outcome或已缓存引文视为永久许可。
func (e *Engine) View(ctx context.Context, p *Plan, v *Outcome) (View, error) {
	if e == nil || e.control == nil || ctx == nil || !v.valid(p) {
		return View{}, ErrInvalid
	}
	if err := e.check(ctx, p); err != nil {
		return View{}, err
	}
	out := View{State: v.state, SubjectName: p.scope.SubjectName, Quotes: []Quote{}}
	if v.state == "refused" {
		out.Template = refusalTemplate(v.reason)
		return out, nil
	}
	quotes, err := p.quotes(v.citations)
	if err != nil {
		return View{}, err
	}
	if err := e.check(ctx, p); err != nil {
		return View{}, err
	}
	out.Template = "AI 资料问答：以下为所选授权资料中经核验的原文引文，不是新的报告。"
	out.ModelName, out.FilingNo, out.Quotes = p.cfg.Selector.Name, p.cfg.Selector.FilingNo, quotes
	return out, nil
}
