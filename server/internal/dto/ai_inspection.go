package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"

type AIInspectionSelection struct {
	ID          int64 `json:"id"`
	BatchID     int64 `json:"batch_id"`
	ReviewID    int64 `json:"review_id"`
	CandidateID int64 `json:"candidate_id"`
}
type AIInspectionBatch struct {
	ID                  int64                   `json:"id"`
	RequestID           string                  `json:"request_id"`
	ActorID             int64                   `json:"actor_id"`
	AfterReviewID       int64                   `json:"after_review_id"`
	WindowSize          int                     `json:"window_size"`
	SampleSize          int                     `json:"sample_size"`
	PopulationCount     int                     `json:"population_count"`
	EligibleCount       int                     `json:"eligible_count"`
	NextReviewID        int64                   `json:"next_review_id"`
	PopulationDigest    string                  `json:"population_digest"`
	PopulationReviewIDs []int64                 `json:"population_review_ids"`
	CreatedAt           int64                   `json:"created_at"`
	Items               []AIInspectionSelection `json:"items"`
}

func NewAIInspectionBatch(b candidate.InspectionBatch) AIInspectionBatch {
	d := AIInspectionBatch{ID: b.ID, RequestID: b.Request.RequestID, ActorID: b.ActorID, AfterReviewID: b.Request.AfterReviewID, WindowSize: b.Request.WindowSize, SampleSize: b.Request.SampleSize, PopulationCount: b.PopulationCount, EligibleCount: b.EligibleCount, NextReviewID: b.NextReviewID, PopulationDigest: b.PopulationDigest, CreatedAt: b.CreatedAt.UnixMilli(), Items: []AIInspectionSelection{}}
	for _, item := range b.Items {
		d.Items = append(d.Items, AIInspectionSelection{ID: item.ID, BatchID: item.BatchID, ReviewID: item.ReviewID, CandidateID: item.CandidateID})
	}
	d.PopulationReviewIDs = b.PopulationReviewIDs
	return d
}

type AIInspectionEvent struct {
	ID             int64  `json:"id"`
	RequestID      string `json:"request_id"`
	ItemID         int64  `json:"item_id"`
	EvidenceDigest string `json:"evidence_digest"`
	ActorID        int64  `json:"actor_id"`
	MappingID      int64  `json:"mapping_id"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason"`
	CreatedAt      int64  `json:"created_at"`
}

func NewAIInspectionEvent(e candidate.InspectionEvent) AIInspectionEvent {
	return AIInspectionEvent{ID: e.ID, RequestID: e.Request.RequestID, ItemID: e.Request.ItemID, EvidenceDigest: e.Request.EvidenceDigest, ActorID: e.ActorID, MappingID: e.MappingID, Decision: e.Request.Decision, Reason: e.Request.Reason, CreatedAt: e.CreatedAt.UnixMilli()}
}

type AIInspectionItem struct {
	ID             int64              `json:"id"`
	BatchID        int64              `json:"batch_id"`
	ActorID        int64              `json:"actor_id"`
	EvidenceDigest string             `json:"evidence_digest"`
	OriginalReview AIReviewEvent      `json:"original_review"`
	Candidate      AIReviewItem       `json:"candidate"`
	Result         *AIInspectionEvent `json:"result"`
}

func NewAIInspectionItem(item candidate.InspectionItem) (AIInspectionItem, error) {
	digest, err := candidate.InspectionDigest(item)
	if err != nil {
		return AIInspectionItem{}, err
	}
	c, err := NewAIReviewItem(item.Candidate)
	if err != nil {
		return AIInspectionItem{}, err
	}
	e := item.Original
	d := AIInspectionItem{ID: item.Selection.ID, BatchID: item.Selection.BatchID, ActorID: item.ActorID, EvidenceDigest: digest, Candidate: c, OriginalReview: AIReviewEvent{ID: e.ID, RequestID: e.Request.RequestID, CandidateID: e.Request.CandidateID, ActorID: e.ActorID, Decision: e.Request.Decision, Reason: e.Request.Reason, MappingID: e.MappingID, CreatedAt: e.CreatedAt.UnixMilli()}}
	if item.Result != nil {
		result := NewAIInspectionEvent(*item.Result)
		d.Result = &result
	}
	return d, nil
}

type AIInspectionIssues struct {
	Items       []AIInspectionEvent `json:"items"`
	NextAfterID int64               `json:"next_after_id"`
}
