package dto

import (
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type AISourceRegistration struct {
	Domain           string `json:"domain"`
	Category         string `json:"category"`
	LicenseTag       string `json:"license_tag"`
	LicenseRef       string `json:"license_ref"`
	LicenseExpiresAt *int64 `json:"license_expires_at"`
	MinIntervalMS    int64  `json:"min_interval_ms"`
}
type AISourceRecord struct {
	ID int64 `json:"id"`
	AISourceRegistration
	Enabled bool `json:"enabled"`
}

func NewAISourceRecord(s sources.Source) AISourceRecord {
	var expires *int64
	if s.LicenseExpires != nil {
		value := s.LicenseExpires.UnixMilli()
		expires = &value
	}
	return AISourceRecord{ID: s.ID, Enabled: s.Enabled, AISourceRegistration: AISourceRegistration{Domain: s.Domain, Category: s.Category, LicenseTag: s.LicenseTag, LicenseRef: s.LicenseRef, LicenseExpiresAt: expires, MinIntervalMS: s.MinInterval.Milliseconds()}}
}

type AISourceAudit struct {
	ID          int64           `json:"id"`
	ActorID     int64           `json:"actor_id"`
	Operation   string          `json:"operation"`
	Before      *AISourceRecord `json:"before"`
	After       AISourceRecord  `json:"after"`
	SnapshotID  int64           `json:"snapshot_id"`
	ContentHash string          `json:"content_hash"`
	URL         string          `json:"url"`
	CreatedAt   int64           `json:"created_at"`
}

func NewAISourceAudit(e sources.SourceAudit) AISourceAudit {
	d := AISourceAudit{ID: e.ID, ActorID: e.ActorID, Operation: e.Operation, After: NewAISourceRecord(e.After),
		SnapshotID: e.SnapshotID, ContentHash: e.ContentHash, URL: e.URL, CreatedAt: e.CreatedAt.UnixMilli()}
	if e.Before != nil {
		before := NewAISourceRecord(*e.Before)
		d.Before = &before
	}
	return d
}

type AISourceAudits struct {
	Items       []AISourceAudit `json:"items"`
	NextAfterID int64           `json:"next_after_id"`
}

type AISnapshotImport struct {
	SourceID  int64  `json:"source_id"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Body      []byte `json:"body"`
}
type AISnapshotFetch struct {
	SourceID int64  `json:"source_id"`
	URL      string `json:"url"`
}
type AISnapshotRecord struct {
	ID             int64  `json:"id"`
	SourceID       int64  `json:"source_id"`
	SourceCategory string `json:"source_category"`
	URL            string `json:"url"`
	ContentHash    string `json:"content_hash"`
	Body           string `json:"body"`
	SentenceCount  int    `json:"sentence_count"`
	FetchedAt      int64  `json:"fetched_at"`
}

func NewAISnapshotRecord(s sources.Snapshot) AISnapshotRecord {
	return AISnapshotRecord{ID: s.ID, SourceID: s.SourceID, SourceCategory: s.SourceCategory, URL: s.URL, ContentHash: s.Hash, Body: s.Body, SentenceCount: len(s.Sentences), FetchedAt: s.FetchedAt.UnixMilli()}
}

type AIL0RunRecord struct {
	ID               int64                `json:"id"`
	RequestID        string               `json:"request_id"`
	SnapshotID       int64                `json:"snapshot_id"`
	State            string               `json:"state"`
	ErrorCode        string               `json:"error_code"`
	CandidateIDs     []int64              `json:"candidate_ids"`
	Rejected         []pipeline.Rejection `json:"rejected"`
	Attempts         int                  `json:"attempts"`
	Provider         string               `json:"provider"`
	Model            string               `json:"model"`
	PromptTokens     int                  `json:"prompt_tokens"`
	CompletionTokens int                  `json:"completion_tokens"`
	TotalTokens      int                  `json:"total_tokens"`
	UsageEstimated   bool                 `json:"usage_estimated"`
	CreatedAt        int64                `json:"created_at"`
	FinishedAt       *int64               `json:"finished_at"`
}

func NewAIL0RunRecord(r pipeline.RunRecord) AIL0RunRecord {
	d := AIL0RunRecord{ID: r.ID, RequestID: r.Request.RequestID, SnapshotID: r.Request.SnapshotID, State: r.State, ErrorCode: r.ErrorCode, CandidateIDs: []int64{}, Rejected: r.Result.Rejected, Attempts: r.Result.Attempts,
		Provider: r.Policy.Provider, Model: r.Policy.Model, PromptTokens: r.Result.Usage.Prompt, CompletionTokens: r.Result.Usage.Completion, TotalTokens: r.Result.Usage.Total, UsageEstimated: r.Result.UsageEstimated, CreatedAt: r.CreatedAt.UnixMilli()}
	if r.FinishedAt != nil {
		value := r.FinishedAt.UnixMilli()
		d.FinishedAt = &value
	}
	for _, c := range r.Result.Candidates {
		d.CandidateIDs = append(d.CandidateIDs, c.ID)
	}
	return d
}
