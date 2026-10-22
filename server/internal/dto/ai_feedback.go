package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"

type AIFeedback struct {
	ID         int64  `json:"id"`
	TaskID     int64  `json:"taskId"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	Version    int64  `json:"version"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}
type AIFeedbackPage struct {
	Items       []AIFeedback `json:"items"`
	NextAfterID int64        `json:"nextAfterId"`
	HasMore     bool         `json:"hasMore"`
}
type AIFeedbackList struct {
	TaskID int64        `json:"taskId"`
	Items  []AIFeedback `json:"items"`
}
type AIFeedbackEvent struct {
	Version    int64  `json:"version"`
	ActorID    int64  `json:"actorId"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	At         int64  `json:"at"`
}
type AIInternalFeedback struct {
	Item   AIFeedback        `json:"item"`
	Events []AIFeedbackEvent `json:"events"`
}

func NewAIFeedback(r feedback.Record) AIFeedback {
	return AIFeedback{r.ID, r.TaskID, r.Category, r.Status, r.Resolution, r.Version, r.CreatedAt.UnixMilli(), r.UpdatedAt.UnixMilli()}
}
