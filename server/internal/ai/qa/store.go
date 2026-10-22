package qa

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// Decision只含已解析的闭集协议，不存模型自由文本。
type Decision struct {
	Action    string     `json:"action,omitempty"`
	Verdict   string     `json:"verdict,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	Citations []Citation `json:"citations,omitempty"`
}

func (p Proof) Valid() bool {
	evidenceLimit := 8
	if IsReportApproval(p.ScopeApproval) {
		evidenceLimit = ReportMaxQuotes
	}
	if !p.Identity.Valid() || !digest(p.InputDigest) || !digest(p.PolicyDigest) || !meta(p.ScopeApproval) || !meta(p.SubjectName) || !conversation.ID(p.SubjectID) || !slices.Contains([]string{"theme", "topic"}, p.SubjectType) || !conversation.Time(p.ValidUntil) || len(p.Evidence) > evidenceLimit || len(p.Models) != 3 || p.MaxCalls != 3 || p.MaxTokens < 1 || p.MaxTokens > 1<<20 {
		return false
	}
	for i, v := range p.Evidence {
		if !conversation.ID(v.ID) || !digest(v.Hash) || i > 0 && v.ID <= p.Evidence[i-1].ID {
			return false
		}
	}
	for _, m := range p.Models {
		if !m.valid() {
			return false
		}
	}
	return true
}

type Record struct {
	Proof                 Proof
	MessageHash           string
	State, Reason         string
	Citations             []Citation
	Calls, ReservedTokens int
	QuotaDay              string
	Version               int64
	CreatedAt, UpdatedAt  time.Time
}

func (Record) String() string     { return "问答持久记录（私有输入已隐藏）" }
func (r Record) GoString() string { return r.String() }
func (r Record) Receipt() Receipt {
	return Receipt{State: r.State, Reason: r.Reason, InputDigest: r.Proof.InputDigest, PolicyDigest: r.Proof.PolicyDigest, Citations: clonePicks(r.Citations), Calls: r.Calls}
}
func (r Record) Valid() bool {
	if !r.Proof.Valid() || !digest(r.MessageHash) || !conversation.Time(r.CreatedAt) || !conversation.Time(r.UpdatedAt) || r.UpdatedAt.Before(r.CreatedAt) || !conversation.ID(r.Version) || r.Calls < 0 || r.Calls > 3 || r.ReservedTokens < 0 || r.ReservedTokens > r.Proof.MaxTokens {
		return false
	}
	if r.State == "queued" {
		return r.Version == 1 && r.QuotaDay == "" && r.Calls == 0 && r.ReservedTokens == 0 && r.Reason == "" && len(r.Citations) == 0
	}
	if r.State == "failed" && r.QuotaDay == "" {
		return r.Version == 2 && r.Calls == 0 && r.ReservedTokens == 0 && slices.Contains([]string{"execution_failed", "cancelled", "unconfirmed"}, r.Reason) && len(r.Citations) == 0
	}
	day, err := time.Parse("2006-01-02", r.QuotaDay)
	if err != nil || day.Format("2006-01-02") != r.QuotaDay || r.Version < 2 || r.QuotaDay < r.CreatedAt.UTC().Format("2006-01-02") || r.QuotaDay > r.UpdatedAt.UTC().Format("2006-01-02") {
		return false
	}
	if r.State == "running" {
		return r.Reason == "" && len(r.Citations) == 0
	}
	if r.State == "failed" {
		return r.Version >= 3 && slices.Contains([]string{"execution_failed", "cancelled", "unconfirmed"}, r.Reason) && len(r.Citations) == 0
	}
	if r.Version < 3 {
		return false
	}
	if r.State == "answered" {
		return r.Reason == "" && r.Calls == 3 && len(r.Citations) >= 1 && len(r.Citations) <= 4
	}
	return r.State == "refused" && refusalTemplate(r.Reason) != "" && len(r.Citations) == 0
}

type CallRecord struct {
	Call                 Call
	State                string
	Result               CallResult
	CreatedAt, UpdatedAt time.Time
}

func (CallRecord) String() string     { return "问答调用存证（内部引用已隐藏）" }
func (r CallRecord) GoString() string { return r.String() }
func (r CallRecord) Valid() bool {
	c := r.Call
	if !digest(c.InputDigest) || !digest(c.RequestDigest) || !digest(c.ProfileDigest) || !meta(c.Model) || c.Step < 1 || c.Step > 3 || c.Stage != []string{"select", "entailment", "semantic"}[c.Step-1] || c.MaxTokens < 1 || c.MaxTokens > 1<<20 || !conversation.Time(r.CreatedAt) || !conversation.Time(r.UpdatedAt) || r.UpdatedAt.Before(r.CreatedAt) {
		return false
	}
	if r.State == "reserved" {
		return sameJSON(r.Result, CallResult{}) && r.UpdatedAt.Equal(r.CreatedAt)
	}
	return r.State == "recorded" && validCallResult(r.Result, c.Stage)
}

func validCallResult(r CallResult, stage string) bool {
	for _, v := range []int{r.Usage.Prompt, r.Usage.Completion, r.Usage.Total, r.Usage.Cached, r.Usage.Reasoning} {
		if v < 0 || int64(v) > 1e12 {
			return false
		}
	}
	if r.RequestID != "" && !meta(r.RequestID) || r.Model != "" && !meta(r.Model) || r.ResponseDigest != "" && !digest(r.ResponseDigest) {
		return false
	}
	if r.Failure != "" {
		return slices.Contains([]string{"not_sent", "response_unknown", "invalid_metadata", "invalid_protocol"}, r.Failure) && sameJSON(r.Decision, Decision{})
	}
	return digest(r.ResponseDigest) && meta(r.RequestID) && meta(r.Model) && !r.Estimated && r.Usage.Prompt > 0 && r.Usage.Completion > 0 && r.Usage.Total == r.Usage.Prompt+r.Usage.Completion && validDecision(r.Decision, stage)
}
func validDecision(v Decision, stage string) bool {
	if stage == "select" {
		if v.Verdict != "" {
			return false
		}
		if v.Action == "refuse" {
			return len(v.Citations) == 0 && slices.Contains([]string{"no_evidence", "out_of_scope", "request_not_allowed"}, v.Reason)
		}
		return v.Action == "answer" && v.Reason == "none" && len(v.Citations) >= 1 && len(v.Citations) <= 4
	}
	if stage != "entailment" && stage != "semantic" {
		return false
	}
	return v.Action == "" && len(v.Citations) == 0 && (v.Verdict == "pass" && v.Reason == "verified" || v.Verdict == "uncertain" && v.Reason == "uncertain" || v.Verdict == "block" && slices.Contains([]string{"unsupported", "out_of_scope", "request_not_allowed"}, v.Reason))
}
func sameJSON(a, b any) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && string(x) == string(y)
}

type QuotaPolicy struct {
	Ref                                                        string
	UserRequests, InstanceRequests, UserTokens, InstanceTokens int64
}

func (p QuotaPolicy) Valid() bool {
	return meta(p.Ref) && p.UserRequests > 0 && p.UserRequests <= 1e9 && p.InstanceRequests >= p.UserRequests && p.InstanceRequests <= 1e9 && p.UserTokens > 0 && p.UserTokens <= 1e12 && p.InstanceTokens >= p.UserTokens && p.InstanceTokens <= 1e12
}

type Quota struct {
	Day                      string
	UserID, Requests, Tokens int64
}

// Store中的每次回调必须是独立短事务，不得将事务跨模型请求保存。
type Store interface {
	WithinQA(context.Context, bool, func(Tx) error) error
}
type Tx interface {
	conversation.Tx
	// Authorize须复核当前会员/对象、范围批准与模型政策、来源/快照及有效期，不能仅验格式。
	Authorize(context.Context, Proof) error
	QuotaPolicy(context.Context) (QuotaPolicy, error)
	Quota(context.Context, string, int64) (Quota, error)
	AddQuota(context.Context, Quota, int64) error
	Question(context.Context, Identity, bool) (Record, error)
	QuestionByKey(context.Context, int64, int64, string) (Record, error)
	InsertQuestion(context.Context, Record) error
	SetQuestion(context.Context, Record, Record) error
	Calls(context.Context, string) ([]CallRecord, error)
	InsertCall(context.Context, CallRecord) error
	FinishCall(context.Context, CallRecord, CallResult, time.Time) error
}

func (p *Plan) decision(stage, text string) (Decision, error) {
	if stage == "select" {
		v, err := decodeSelection(text)
		if err != nil {
			return Decision{}, err
		}
		if v.Action == "answer" {
			if _, err = p.quotes(v.Citations); err != nil {
				return Decision{}, err
			}
		}
		return Decision{Action: v.Action, Reason: v.Reason, Citations: clonePicks(v.Citations)}, nil
	}
	v, err := decodeJudge(text)
	if err != nil {
		return Decision{}, err
	}
	return Decision{Verdict: v.Verdict, Reason: v.Reason}, nil
}

// 从固化闭集阶段重建结论，不能仅凭外部Complete收据或调用计数发布。
func (p *Plan) replay(calls []CallRecord) (Receipt, error) {
	base := Receipt{InputDigest: p.digest, PolicyDigest: p.policy, Calls: len(calls)}
	refuse := func(reason string) (Receipt, error) { base.State = "refused"; base.Reason = reason; return base, nil }
	if len(calls) == 0 {
		if !safeQuestion(p.input.Question) {
			return refuse("request_not_allowed")
		}
		if len(p.scope.Snapshots) == 0 {
			return refuse("no_evidence")
		}
		return Receipt{}, ErrProtocol
	}
	if len(calls) > 3 {
		return Receipt{}, ErrProtocol
	}
	var picks []Citation
	for n, c := range calls {
		if !c.Valid() || c.State != "recorded" || c.Result.Failure != "" || c.Call.Step != n+1 || c.Call.InputDigest != p.digest || c.Result.Model != p.Proof().Models[n].Name {
			return Receipt{}, ErrProtocol
		}
		request, model, err := p.request(c.Call.Stage, picks)
		if err != nil {
			return Receipt{}, err
		}
		b, err := json.Marshal(request)
		if err != nil {
			return Receipt{}, ErrInvalid
		}
		if c.Call.Model != model.Name || c.Call.ProfileDigest != model.ProfileDigest || c.Call.RequestDigest != task.StageDigest(b) || c.Call.MaxTokens != p.cfg.Limits.MaxInputTokens+p.cfg.Limits.MaxOutputTokens || c.Result.Usage.Prompt > p.cfg.Limits.MaxInputTokens || c.Result.Usage.Completion > p.cfg.Limits.MaxOutputTokens {
			return Receipt{}, ErrProtocol
		}
		d := c.Result.Decision
		if n == 0 {
			if d.Action == "refuse" {
				if len(calls) != 1 {
					return Receipt{}, ErrProtocol
				}
				return refuse(d.Reason)
			}
			picks = clonePicks(d.Citations)
			if _, err := p.quotes(picks); err != nil {
				return Receipt{}, err
			}
		} else if d.Verdict != "pass" {
			if len(calls) != n+1 {
				return Receipt{}, ErrProtocol
			}
			reason := d.Reason
			if !slices.Contains([]string{"out_of_scope", "request_not_allowed"}, reason) {
				reason = "unverified"
			}
			return refuse(reason)
		}
	}
	if len(calls) != 3 {
		return Receipt{}, ErrProtocol
	}
	base.State = "answered"
	base.Citations = picks
	return base, nil
}
