// Package agents 定义外部 Agent 执行契约；上游完成不等于本项目报告通过审核。
package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"unicode/utf8"
)

var (
	ErrInvalid   = errors.New("Agent 请求或记录无效")
	ErrDisabled  = errors.New("外部 Agent 尚未启用")
	ErrConflict  = errors.New("Agent 请求与已有记录冲突")
	ErrNotFound  = errors.New("Agent 请求不存在")
	ErrLimit     = errors.New("该批准配置的提交限额已用尽")
	ErrUncertain = errors.New("执行状态待核对；禁止重新提交")
)

const (
	Submitting        = "submitting"
	SubmissionUnknown = "submission_unknown"
	Accepted          = "accepted"
	Running           = "running"
	Suspended         = "suspended"
	Completed         = "upstream_completed"
	Failed            = "upstream_failed"
	Cancelled         = "upstream_cancelled"
	Quarantined       = "result_quarantined"
	MaxResultBytes    = 4 << 20
)

var requestPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{7,63}$`)
var remotePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var symbolPattern = regexp.MustCompile(`^[0-9]{6}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Request struct {
	RequestID string `json:"request_id"`
	Symbol    string `json:"symbol"`
}

func (r Request) Valid() bool {
	return requestPattern.MatchString(r.RequestID) && symbolPattern.MatchString(r.Symbol)
}
func ValidRequestID(v string) bool { return requestPattern.MatchString(v) }
func ValidRemoteID(v string) bool  { return remotePattern.MatchString(v) }
func ValidDigest(v string) bool    { return digestPattern.MatchString(v) }

// Profile 的摘要绑定批准地址、上游版本、模型与授权引用；不包含任何凭据。
type Profile struct {
	Digest         string
	MaxSubmissions int
}

func (p Profile) Valid() bool {
	return ValidDigest(p.Digest) && p.MaxSubmissions >= 1 && p.MaxSubmissions <= 1000
}

// Record 刻意不含上游正文、错误原文或凭据，只供内部工具查看执行状态。
type Record struct {
	ID              int64   `json:"id"`
	ActorID         int64   `json:"actor_id"`
	Request         Request `json:"request"`
	ProfileDigest   string  `json:"profile_digest"`
	RemoteID        string  `json:"remote_id,omitempty"`
	State           string  `json:"state"`
	CancelRequested bool    `json:"cancel_requested"`
	ResultDigest    string  `json:"result_digest,omitempty"`
	ErrorCode       string  `json:"error_code,omitempty"`
}

func Terminal(state string) bool {
	return state == Completed || state == Failed || state == Cancelled || state == Quarantined
}
func ValidRemoteState(state string) bool {
	return state == Accepted || state == Running || state == Suspended || state == Completed || state == Failed || state == Cancelled
}

type Acceptance struct{ RemoteID string }
type Observation struct {
	RemoteID string
	State    string
}

// Result 的正文只能进入隔离表，不能序列化给 C 端；真实账单用量仍未知。
type Result struct {
	RemoteID string
	Payload  json.RawMessage `json:"-"`
}

func (r Result) String() string   { return "隔离 Agent 结果(正文已隐藏)" }
func (r Result) GoString() string { return r.String() }

func (r Record) Valid() bool {
	if r.ID <= 0 || r.ActorID <= 0 || !r.Request.Valid() || !ValidDigest(r.ProfileDigest) {
		return false
	}
	if r.State == Submitting || r.State == SubmissionUnknown {
		return r.RemoteID == "" && !r.CancelRequested && r.ResultDigest == "" && ((r.State == Submitting && r.ErrorCode == "") || (r.State == SubmissionUnknown && r.ErrorCode == "submission_unknown"))
	}
	if !ValidRemoteID(r.RemoteID) || (!ValidRemoteState(r.State) && r.State != Quarantined) {
		return false
	}
	if (r.State == Quarantined) != ValidDigest(r.ResultDigest) || (r.State != Quarantined && r.ResultDigest != "") {
		return false
	}
	return r.ErrorCode == "" || (r.CancelRequested && (r.ErrorCode == "cancel_pending" || r.ErrorCode == "cancel_unknown"))
}

// ValidChange 把可写状态转换收口；上游终态不会因旧的轮询响应倒退。
func ValidChange(old, next Record, event string, result *Result) bool {
	if !old.Valid() || !next.Valid() || old.ID != next.ID || old.ActorID != next.ActorID || old.Request != next.Request || old.ProfileDigest != next.ProfileDigest {
		return false
	}
	want := old
	switch event {
	case "submitted":
		if old.State != Submitting {
			return false
		}
		want.RemoteID = next.RemoteID
		want.State = Accepted
	case "submission_unknown":
		if old.State != Submitting {
			return false
		}
		want.State = SubmissionUnknown
		want.ErrorCode = event
	case "observed":
		if !ValidRemoteState(next.State) || old.RemoteID == "" || Terminal(old.State) && old.State != next.State || old.State == Running && next.State == Accepted {
			return false
		}
		want.State = next.State
	case "cancel_requested":
		if old.CancelRequested || Terminal(old.State) || old.RemoteID == "" {
			return false
		}
		want.CancelRequested = true
		want.ErrorCode = "cancel_pending"
	case "cancel_acknowledged", "cancel_unknown":
		if !old.CancelRequested || old.ErrorCode != "cancel_pending" {
			return false
		}
		want.ErrorCode = ""
		if event == "cancel_unknown" {
			want.ErrorCode = event
		}
	case "quarantined":
		if old.State != Completed || result == nil || !result.Valid() || result.RemoteID != old.RemoteID {
			return false
		}
		want.State = Quarantined
		want.ResultDigest = result.Digest()
	default:
		return false
	}
	return want == next && (event == "quarantined" || result == nil)
}

func (r Result) Valid() bool {
	if !ValidRemoteID(r.RemoteID) || len(r.Payload) == 0 || len(r.Payload) > MaxResultBytes || !utf8.Valid(r.Payload) || !json.Valid(r.Payload) {
		return false
	}
	var v struct {
		ID      string                     `json:"analysis_id"`
		Reports map[string]json.RawMessage `json:"reports"`
	}
	return json.Unmarshal(r.Payload, &v) == nil && v.ID == r.RemoteID && len(v.Reports) > 0
}
func (r Result) Digest() string {
	sum := sha256.Sum256(r.Payload)
	return hex.EncodeToString(sum[:])
}

type Executor interface {
	Submit(context.Context, string) (Acceptance, error)
	Status(context.Context, string) (Observation, error)
	Result(context.Context, string) (Result, error)
	Cancel(context.Context, string) error
}

// Store 的写方法由 service 在事务内调用。网络操作永远不在数据库事务内等待。
type Store interface {
	WithinAgentTx(context.Context, func(Tx) error) error
	ByRequest(context.Context, string) (Record, error)
}

// PayloadReader 仅供受信任编排读取隔离区，不注册为会员或管理端 HTTP 能力。
type PayloadReader interface {
	Quarantined(context.Context, int64, string) (Record, Result, error)
}

type Tx interface {
	Reserve(context.Context, Profile, Request, int64) (Record, bool, error)
	Lock(context.Context, int64) (Record, error)
	Save(context.Context, Record, string, *Result) error
}
