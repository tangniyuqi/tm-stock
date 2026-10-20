package candidate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var (
	ErrInvalidInspection  = errors.New("复核参数或记录无效")
	ErrInspectionNotFound = errors.New("复核记录不存在")
	ErrInspectionActor    = errors.New("复核身份不匹配或不独立")
)

type InspectionBatchRequest struct {
	RequestID     string `json:"request_id"`
	AfterReviewID int64  `json:"after_review_id"`
	WindowSize    int    `json:"window_size"`
	SampleSize    int    `json:"sample_size"`
}

func (r InspectionBatchRequest) Validate() error {
	if !validReviewRequestID(r.RequestID) || r.AfterReviewID < 0 || r.WindowSize < 1 || r.WindowSize > 2000 || r.SampleSize < 1 || r.SampleSize > 200 || r.SampleSize > r.WindowSize {
		return ErrInvalidInspection
	}
	return nil
}

type InspectionBatch struct {
	ID                             int64
	Request                        InspectionBatchRequest
	ActorID                        int64
	PopulationCount, EligibleCount int
	NextReviewID                   int64
	PopulationDigest               string
	PopulationReviewIDs            []int64
	CreatedAt                      time.Time
	Items                          []InspectionSelection
}
type InspectionSelection struct{ ID, BatchID, ReviewID, CandidateID int64 }
type InspectionItem struct {
	Selection InspectionSelection
	ActorID   int64
	Original  AuditEvent
	Candidate Candidate
	Result    *InspectionEvent `json:"-"`
}

// InspectionDigest 绑定本条分派、原决定和完整依据，不含可变化的复核状态。
func InspectionDigest(item InspectionItem) (string, error) {
	if item.Selection.ID <= 0 || item.Selection.BatchID <= 0 || item.Selection.ReviewID <= 0 || item.ActorID <= 0 || item.Selection.ReviewID != item.Original.ID || item.Selection.CandidateID != item.Candidate.ID || item.Original.Request.CandidateID != item.Candidate.ID || item.Original.Request.Decision != "approve" || item.Original.MappingID <= 0 || item.Original.ActorID <= 0 || item.Original.CreatedAt.IsZero() || item.Original.Request.Validate() != nil {
		return "", ErrInvalidInspection
	}
	digest, err := EvidenceDigest(item.Candidate)
	if err != nil || digest != item.Original.Request.EvidenceDigest {
		return "", ErrEvidenceChanged
	}
	item.Original.CreatedAt = item.Original.CreatedAt.UTC().Truncate(time.Millisecond)
	item.Candidate.CollectedAt = item.Candidate.CollectedAt.UTC().Truncate(time.Millisecond)
	raw, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type InspectionRequest struct {
	ItemID         int64  `json:"item_id"`
	EvidenceDigest string `json:"evidence_digest"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason"`
	RequestID      string `json:"request_id"`
}

func (r InspectionRequest) Validate() error {
	decision := ""
	switch r.Decision {
	case "confirm":
		decision = "approve"
	case "issue":
		decision = "reject"
	default:
		return ErrInvalidInspection
	}
	// 复用人审请求的摘要、编号、原因长度/控制字符约束，避免两套口径漂移。
	if err := (ReviewRequest{CandidateID: r.ItemID, EvidenceDigest: r.EvidenceDigest, Decision: decision, Reason: r.Reason, RequestID: r.RequestID}).Validate(); err != nil {
		return ErrInvalidInspection
	}
	return nil
}

type InspectionEvent struct {
	ID                 int64
	Request            InspectionRequest
	ActorID, MappingID int64
	CreatedAt          time.Time
}
type InspectionMapping struct {
	ID, ThemeID, StockID     int64
	TsCode                   string
	SourceType               int8
	SourceExcerpt, SourceURL string
	ExcerptTrimmed           bool
	CollectedAt              time.Time
	AuditStatus, Status      int8
	Deleted                  bool
}

func (m InspectionMapping) Matches(c Candidate) bool {
	return !m.Deleted && m.ID > 0 && m.ThemeID == c.ThemeID && m.StockID == c.StockID && m.TsCode == c.TsCode && m.SourceType == c.SourceType && m.SourceExcerpt == c.SourceExcerpt && m.ExcerptTrimmed == c.ExcerptTrimmed && m.SourceURL == c.SourceURL && m.CollectedAt.Equal(c.CollectedAt) && m.AuditStatus == 2 && (m.Status == 0 || m.Status == 1)
}

type InspectionTx interface {
	BatchByRequest(context.Context, string) (InspectionBatch, bool, error)
	SamplingWindow(context.Context, int64, int) ([]AuditEvent, error)
	InsertBatch(context.Context, InspectionBatch, []AuditEvent) (InspectionBatch, error)
	LockItem(context.Context, int64) (InspectionItem, error)
	EventByRequest(context.Context, string) (InspectionEvent, bool, error)
	EventByItem(context.Context, int64) (InspectionEvent, bool, error)
	ApprovalSnapshot(context.Context, Candidate) (sources.Snapshot, error)
	LockCatalog(context.Context, Candidate) (ReviewCatalog, error)
	LockMapping(context.Context, int64, int64) (InspectionMapping, error)
	Quarantine(context.Context, int64, int64) error
	AppendInspection(context.Context, InspectionRequest, int64, int64) (InspectionEvent, error)
	RefreshThemeCount(context.Context, int64) error
}

type InspectionStore interface {
	WithinInspectionTx(context.Context, func(InspectionTx) error) error
	BatchByRequest(context.Context, string) (InspectionBatch, bool, error)
	ItemByID(context.Context, int64) (InspectionItem, error)
	ListIssues(context.Context, int64, int) ([]InspectionEvent, error)
}
