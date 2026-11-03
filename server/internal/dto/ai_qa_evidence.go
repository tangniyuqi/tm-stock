package dto

type AIQAEvidenceSentence struct {
	ID        int    `json:"id"`
	StartByte int    `json:"startByte"`
	EndByte   int    `json:"endByte"`
	Text      string `json:"text"`
}
type AIQAEvidenceSource struct {
	SnapshotID  int64                  `json:"snapshotId"`
	Digest      string                 `json:"digest"`
	SourceType  string                 `json:"sourceType"`
	SourceURL   string                 `json:"sourceUrl"`
	CollectedAt int64                  `json:"collectedAt"`
	Sentences   []AIQAEvidenceSentence `json:"sentences"`
}
type AIQAEvidenceCall struct {
	Step          int    `json:"step"`
	Stage         string `json:"stage"`
	ModelName     string `json:"modelName"`
	Status        string `json:"status"`
	Verdict       string `json:"verdict"`
	RequestDigest string `json:"requestDigest"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	InputTokens   *int   `json:"inputTokens"`
	OutputTokens  *int   `json:"outputTokens"`
	CostMicros    *int64 `json:"costMicros"`
}
type AIQAEvidence struct {
	Seq             int64                `json:"seq"`
	State           string               `json:"state"`
	Skill           string               `json:"skill"`
	SubjectType     string               `json:"subjectType"`
	SubjectID       int64                `json:"subjectId"`
	SubjectName     string               `json:"subjectName"`
	TaskID          int64                `json:"taskId"`
	PromptVersion   string               `json:"promptVersion"`
	CandidateDigest string               `json:"candidateDigest"`
	CreatedAt       int64                `json:"createdAt"`
	UpdatedAt       int64                `json:"updatedAt"`
	Sources         []AIQAEvidenceSource `json:"sources"`
	Calls           []AIQAEvidenceCall   `json:"calls"`
}
