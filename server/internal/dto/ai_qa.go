package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"

// AIQAStatus仅是本人问题的存储状态，不授予当前答案读取权。
type AIQAStatus struct {
	Seq         int64  `json:"seq"`
	State       string `json:"state"`
	SubjectType string `json:"subjectType"`
	SubjectID   int64  `json:"subjectId"`
	Skill       string `json:"skill"`
	Question    string `json:"question"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}
type AIQAQuote struct {
	SnapshotID  int64  `json:"snapshotId"`
	StartByte   int    `json:"startByte"`
	EndByte     int    `json:"endByte"`
	Excerpt     string `json:"excerpt"`
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	CollectedAt int64  `json:"collectedAt"`
}
type AIQAAnswer struct {
	Seq         int64       `json:"seq"`
	State       string      `json:"state"`
	Template    string      `json:"template"`
	SubjectName string      `json:"subjectName"`
	AIGenerated bool        `json:"aiGenerated"`
	ModelName   string      `json:"modelName"`
	FilingNo    string      `json:"filingNo"`
	Quotes      []AIQAQuote `json:"quotes"`
}
type AIQAInput struct {
	Version     int64  `json:"version"`
	SubjectType string `json:"subjectType"`
	SubjectID   int64  `json:"subjectId"`
	Skill       string `json:"skill"`
	Question    string `json:"question"`
	TaskID      int64  `json:"taskId,omitempty"`
}

// Valid 固定技能与任务编号的关系：资料问答不得携带任务号，报告追问必须绑定一个任务。
func (in AIQAInput) Valid() bool {
	if !conversation.ID(in.Version) || !conversation.ID(in.SubjectID) || in.SubjectType != "theme" || !conversation.Text(in.Question, 1000) || len(in.Question) > 4000 {
		return false
	}
	switch in.Skill {
	case "material_qa":
		return in.TaskID == 0
	case "report_qa":
		return conversation.ID(in.TaskID)
	default:
		return false
	}
}
