package dto

import (
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
)

// AIReviewItem 仅供内部接口；时点统一毫秒，不直接序列化领域 time.Time。
type AIReviewItem struct {
	ID             int64    `json:"id"`
	SnapshotID     int64    `json:"snapshot_id"`
	ClaimIndex     int      `json:"claim_index"`
	ThemeID        int64    `json:"theme_id"`
	ThemeKey       string   `json:"theme_key"`
	StockID        int64    `json:"stock_id"`
	EntityKey      string   `json:"entity_key"`
	TsCode         string   `json:"ts_code"`
	Predicate      string   `json:"predicate"`
	SourceType     int8     `json:"source_type"`
	SourceExcerpt  string   `json:"source_excerpt"`
	SentenceIDs    []int    `json:"sentence_ids"`
	QuoteSegments  []string `json:"quote_segments"`
	ExcerptTrimmed bool     `json:"excerpt_trimmed"`
	SourceURL      string   `json:"source_url"`
	CollectedAt    int64    `json:"collected_at"`
	Provider       string   `json:"provider"`
	Model          string   `json:"model"`
	PromptVersion  string   `json:"prompt_version"`
	EvidenceDigest string   `json:"evidence_digest"`
}

func NewAIReviewItem(c candidate.Candidate) (AIReviewItem, error) {
	digest, err := candidate.EvidenceDigest(c)
	if err != nil {
		return AIReviewItem{}, err
	}
	return AIReviewItem{ID: c.ID, SnapshotID: c.SnapshotID, ClaimIndex: c.ClaimIndex, ThemeID: c.ThemeID, ThemeKey: c.ThemeKey,
		StockID: c.StockID, EntityKey: c.EntityKey, TsCode: c.TsCode, Predicate: c.Predicate, SourceType: c.SourceType,
		SourceExcerpt: c.SourceExcerpt, SentenceIDs: c.SentenceIDs, QuoteSegments: c.QuoteSegments, ExcerptTrimmed: c.ExcerptTrimmed,
		SourceURL: c.SourceURL, CollectedAt: c.CollectedAt.UnixMilli(), Provider: c.Provider, Model: c.Model, PromptVersion: c.PromptVersion, EvidenceDigest: digest}, nil
}

func (d AIReviewItem) Candidate() candidate.Candidate {
	return candidate.Candidate{ID: d.ID, SnapshotID: d.SnapshotID, ClaimIndex: d.ClaimIndex, ThemeID: d.ThemeID, ThemeKey: d.ThemeKey,
		StockID: d.StockID, EntityKey: d.EntityKey, TsCode: d.TsCode, Predicate: d.Predicate, SourceType: d.SourceType,
		SourceExcerpt: d.SourceExcerpt, SentenceIDs: d.SentenceIDs, QuoteSegments: d.QuoteSegments, ExcerptTrimmed: d.ExcerptTrimmed,
		SourceURL: d.SourceURL, CollectedAt: time.UnixMilli(d.CollectedAt).UTC(), Provider: d.Provider, Model: d.Model, PromptVersion: d.PromptVersion}
}

type AIReviewQueue struct {
	Items       []AIReviewItem `json:"items"`
	NextAfterID int64          `json:"next_after_id"`
}

type AIReviewEvent struct {
	ID          int64  `json:"id"`
	RequestID   string `json:"request_id"`
	CandidateID int64  `json:"candidate_id"`
	ActorID     int64  `json:"actor_id"`
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
	MappingID   int64  `json:"mapping_id"`
	CreatedAt   int64  `json:"created_at"`
}
