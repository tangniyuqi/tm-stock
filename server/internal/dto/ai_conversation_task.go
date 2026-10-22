package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"

type AIConversationTask struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversationId"`
	AnalysisType   string `json:"analysisType"`
	SubjectType    string `json:"subjectType"`
	SubjectID      int64  `json:"subjectId"`
	PriceVersion   string `json:"priceVersion"`
	PricePoints    int64  `json:"pricePoints"`
	State          string `json:"state"`
	RejectionCode  string `json:"rejectionCode"`
	TaskID         int64  `json:"taskId"`
	MessageSeq     int64  `json:"messageSeq"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}
type AIConversationTaskSummary struct {
	ID           int64  `json:"id"`
	State        string `json:"state"`
	AnalysisType string `json:"analysisType"`
	SubjectType  string `json:"subjectType"`
	SubjectID    int64  `json:"subjectId"`
	UpdatedAt    int64  `json:"updatedAt"`
}
type AIConversationTasks struct {
	Items        []AIConversationTaskSummary `json:"items"`
	NextBeforeID int64                       `json:"nextBeforeId"`
	HasMore      bool                        `json:"hasMore"`
}

func NewAIConversationTask(i conversation.Intent) AIConversationTask {
	return AIConversationTask{ID: i.ID, ConversationID: i.ConversationID, AnalysisType: i.Input.AnalysisType, SubjectType: i.Input.SubjectType, SubjectID: i.Input.SubjectID, PriceVersion: i.Input.PriceVersion, PricePoints: i.PricePoints, State: i.State, RejectionCode: i.RejectionCode, TaskID: i.TaskID, MessageSeq: i.MessageSeq, CreatedAt: i.CreatedAt.UnixMilli(), UpdatedAt: i.UpdatedAt.UnixMilli()}
}
