package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"

type AIConversationReceipt struct {
	ID        int64 `json:"id"`
	CreatedAt int64 `json:"createdAt"`
}
type AIConversation struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Archived  bool   `json:"archived"`
	Version   int64  `json:"version"`
	LastSeq   int64  `json:"lastSeq"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// 摘要与详情分开定义，禁止列表增加关联正文或报告可用性。
type AIConversationSummary struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Archived  bool   `json:"archived"`
	Version   int64  `json:"version"`
	UpdatedAt int64  `json:"updatedAt"`
}
type AIConversationPage struct {
	Items        []AIConversationSummary `json:"items"`
	NextBeforeID int64                   `json:"nextBeforeId"`
	HasMore      bool                    `json:"hasMore"`
}
type AIConversationMessage struct {
	Seq         int64  `json:"seq"`
	Kind        string `json:"kind"`
	SubjectType string `json:"subjectType"`
	SubjectID   int64  `json:"subjectId"`
	Skill       string `json:"skill"`
	Question    string `json:"question"`
	TaskID      int64  `json:"taskId"`
	CreatedAt   int64  `json:"createdAt"`
}
type AIConversationMessages struct {
	ConversationID int64                   `json:"conversationId"`
	Items          []AIConversationMessage `json:"items"`
	UpperSeq       int64                   `json:"upperSeq"`
	NextAfterSeq   int64                   `json:"nextAfterSeq"`
	HasMore        bool                    `json:"hasMore"`
}

func NewAIConversation(r conversation.Record) AIConversation {
	return AIConversation{r.ID, r.Title, r.Archived, r.Version, r.LastSeq, r.CreatedAt.UnixMilli(), r.UpdatedAt.UnixMilli()}
}
