package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// 审核错误集中定义，供内部接口稳定映射；错误详情不得直接返回客户端。
var (
	ErrInvalidReview   = errors.New("审核参数无效")
	ErrNotFound        = errors.New("候选不存在")
	ErrConflict        = errors.New("审核状态或请求发生冲突")
	ErrEvidenceChanged = errors.New("候选证据摘要不一致")
)

type ReviewStatus int8

const (
	Pending ReviewStatus = iota
	Approved
	Rejected
)

// ReviewRequest 不携带审核人；ActorID 必须由鉴权层单独提供。
type ReviewRequest struct {
	CandidateID    int64  `json:"candidate_id"`
	EvidenceDigest string `json:"evidence_digest"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason"`
	RequestID      string `json:"request_id"`
}

func (r ReviewRequest) Validate() error {
	digest, err := hex.DecodeString(r.EvidenceDigest)
	if r.CandidateID <= 0 || err != nil || len(digest) != sha256.Size || strings.ToLower(r.EvidenceDigest) != r.EvidenceDigest ||
		!validReviewRequestID(r.RequestID) {
		return ErrInvalidReview
	}
	if !utf8.ValidString(r.Reason) || utf8.RuneCountInString(r.Reason) > 250 || strings.TrimSpace(r.Reason) != r.Reason {
		return ErrInvalidReview
	}
	for _, c := range r.Reason {
		if unicode.IsControl(c) {
			return ErrInvalidReview
		}
	}
	switch r.Decision {
	case "approve":
		if r.Reason != "" {
			return ErrInvalidReview
		}
	case "reject":
		if r.Reason == "" {
			return ErrInvalidReview
		}
	default:
		return ErrInvalidReview
	}
	return nil
}

func validReviewRequestID(id string) bool {
	if len(id) < 16 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func (r ReviewRequest) Status() ReviewStatus {
	if r.Decision == "approve" {
		return Approved
	}
	return Rejected
}

type ReviewRecord struct {
	Candidate    Candidate
	Status       ReviewStatus
	MappingID    int64
	ReviewedBy   int64
	ReviewedAt   *time.Time
	RejectReason string
}

// AuditEvent 为一次不可修改的审核决定；时间在接口 DTO 中转换为毫秒。
type AuditEvent struct {
	ID        int64
	Request   ReviewRequest
	ActorID   int64
	MappingID int64
	CreatedAt time.Time
}

func (e AuditEvent) Matches(request ReviewRequest, actorID int64) bool {
	return e.Request == request && e.ActorID == actorID
}

// EvidenceDigest 绑定完整依据及模型追溯字段，防止审核导出后换证据或串行。
// 审核状态不在摘要内，因此同请求终态重放仍可校验。
func EvidenceDigest(item Candidate) (string, error) {
	if err := item.Validate(); err != nil {
		return "", fmt.Errorf("生成审核证据摘要失败: %w", err)
	}
	item.CollectedAt = item.CollectedAt.UTC().Truncate(time.Millisecond)
	raw, err := json.Marshal(item)
	if err != nil {
		return "", fmt.Errorf("序列化审核证据失败: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
