package dossier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

const ManagementProtocol = "dossier-management-v1"
const ManagedApprovalPrefix = "dossier-managed:"

// MySQL DATETIME(3)的真实上界，不能先接受浏览器Date极值再把参数错误当提交未知。
const ManagementMaxTime int64 = 253402300799999

var ErrInvalidManagement = errors.New("专题管理请求或存证无效")
var ErrManagementConflict = errors.New("专题管理修订或原请求冲突")
var ErrManagementEvidence = errors.New("专题管理存证未通过核验")
var managementRequestID = regexp.MustCompile("^[A-Za-z0-9_-]{16,64}$")

type ManagementObject struct {
	Kind    string `json:"kind"`
	UserID  int64  `json:"userId"`
	ThemeID int64  `json:"themeId"`
	Version int64  `json:"version"`
}

func (t ManagementObject) Valid() bool {
	return conversation.ID(t.ThemeID) && conversation.ID(t.Version) && (t.Kind == "publication" && t.UserID == 0 || t.Kind == "member_read" && conversation.ID(t.UserID))
}
func ValidManagementRequestID(id string) bool { return managementRequestID.MatchString(id) }

type ManagementRequest struct {
	RequestID        string           `json:"requestId"`
	Object           ManagementObject `json:"object"`
	ExpectedRevision int64            `json:"expectedRevision"`
	VersionDigest    string           `json:"versionDigest"`
	Enabled          bool             `json:"enabled"`
	ValidUntil       int64            `json:"validUntil"`
	ReviewReference  string           `json:"reviewReference"`
}

func managementDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (r ManagementRequest) Validate() error {
	if !ValidManagementRequestID(r.RequestID) || !r.Object.Valid() || r.ExpectedRevision < 0 || r.ExpectedRevision >= 9007199254740991 || !managementDigest(r.VersionDigest) || !conversation.Text(r.ReviewReference, 128) || r.Enabled && (r.ValidUntil <= 0 || r.ValidUntil > ManagementMaxTime) || !r.Enabled && r.ValidUntil != 0 {
		return ErrInvalidManagement
	}
	return nil
}

// 先拒绝重复键与畸形JSON，再逐层核必填字段、null和实际类型；不把缺enabled当撤回。
func ParseManagementRequest(raw []byte) (ManagementRequest, error) {
	var r ManagementRequest
	if len(raw) > 8192 || task.CheckStageJSON(raw) != nil {
		return r, ErrInvalidManagement
	}
	shape := func(raw []byte, names []string) bool {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || len(obj) != len(names) {
			return false
		}
		for _, name := range names {
			v, ok := obj[name]
			if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return false
			}
		}
		return true
	}
	if !shape(raw, []string{"requestId", "object", "expectedRevision", "versionDigest", "enabled", "validUntil", "reviewReference"}) {
		return r, ErrInvalidManagement
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || !shape(fields["object"], []string{"kind", "userId", "themeId", "version"}) || json.Unmarshal(raw, &r) != nil || r.Validate() != nil {
		return ManagementRequest{}, ErrInvalidManagement
	}
	return r, nil
}

type ManagementState struct {
	Digest     string `json:"digest"`
	Reference  string `json:"reference"`
	Enabled    bool   `json:"enabled"`
	ValidUntil int64  `json:"validUntil"`
}

func (s ManagementState) valid() bool {
	return managementDigest(s.Digest) && conversation.Text(s.Reference, 128) && s.ValidUntil > 0 && s.ValidUntil <= ManagementMaxTime
}

type ManagementHead struct {
	Revision  int64
	RequestID string
}
type ManagementEvent struct {
	Protocol  string            `json:"protocol"`
	ActorID   int64             `json:"actorId"`
	Request   ManagementRequest `json:"request"`
	Before    *ManagementState  `json:"before"`
	After     ManagementState   `json:"after"`
	Revision  int64             `json:"revision"`
	CreatedAt int64             `json:"createdAt"`
}

func (ManagementRequest) String() string     { return "专题管理请求（内部引用已隐藏）" }
func (r ManagementRequest) GoString() string { return r.String() }
func (ManagementEvent) String() string       { return "专题管理审计（内部状态已隐藏）" }
func (e ManagementEvent) GoString() string   { return e.String() }
func (e ManagementEvent) Canonical() ([]byte, string, error) {
	if e.Protocol != ManagementProtocol || !conversation.ID(e.ActorID) || e.Request.Validate() != nil || e.Revision != e.Request.ExpectedRevision+1 || !conversation.ID(e.Revision) || e.CreatedAt <= 0 || e.CreatedAt > ManagementMaxTime || !e.After.valid() || e.After.Digest != e.Request.VersionDigest || e.After.Reference != ManagedApprovalPrefix+e.Request.RequestID || e.After.Enabled != e.Request.Enabled {
		return nil, "", ErrInvalidManagement
	}
	if e.Before != nil && (!e.Before.valid() || e.Before.Digest != e.Request.VersionDigest) || e.Request.ExpectedRevision > 0 && e.Before == nil {
		return nil, "", ErrInvalidManagement
	}
	if e.Request.Enabled {
		if e.After.ValidUntil != e.Request.ValidUntil || e.After.ValidUntil <= e.CreatedAt {
			return nil, "", ErrInvalidManagement
		}
	} else if e.Before == nil || e.After.ValidUntil != e.Before.ValidUntil {
		return nil, "", ErrInvalidManagement
	}
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > 16384 {
		return nil, "", ErrInvalidManagement
	}
	return raw, task.StageDigest(raw), nil
}
func RestoreManagementEvent(raw []byte, digest string) (ManagementEvent, error) {
	var e ManagementEvent
	if len(raw) > 16384 || task.StageDigest(raw) != digest || task.CheckStageJSON(raw) != nil || json.Unmarshal(raw, &e) != nil {
		return e, ErrInvalidManagement
	}
	canonical, _, err := e.Canonical()
	if err != nil || !bytes.Equal(raw, canonical) {
		return ManagementEvent{}, ErrInvalidManagement
	}
	return e, nil
}
func (e ManagementEvent) Matches(actor int64, r ManagementRequest) bool {
	return e.ActorID == actor && e.Request == r
}
func (e ManagementEvent) MatchesApproval(t ManagementObject, a Approval) bool {
	return e.Request.Object == t && e.After.Digest == a.Digest && e.After.Reference == a.Reference && e.After.Enabled == a.Enabled && e.After.ValidUntil == a.Until.UnixMilli()
}
func IsManagedApproval(reference string) bool {
	return strings.HasPrefix(reference, ManagedApprovalPrefix)
}

type ManagementTx interface {
	Tx
	ManagementHead(context.Context, ManagementObject, bool) (ManagementHead, error)
	ManagementEvent(context.Context, string) (ManagementEvent, error)
	CurrentPermission(context.Context, ManagementObject) (*ManagementState, error)
	AppendManagementEvent(context.Context, ManagementEvent) error
	AdvanceManagementHead(context.Context, ManagementObject, ManagementHead, ManagementEvent) error
	WritePermission(context.Context, ManagementObject, *ManagementState, ManagementState, int64) error
}
type ManagementStore interface {
	WithinManagement(context.Context, func(ManagementTx) error) error
}
