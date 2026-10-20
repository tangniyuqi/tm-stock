package dto

type AITaskQuoteInput struct {
	AnalysisType string `json:"analysis_type"`
	SubjectType  string `json:"subject_type"`
	SubjectID    int64  `json:"subject_id"`
}
type AITaskCreateInput struct {
	AnalysisType string `json:"analysis_type"`
	SubjectType  string `json:"subject_type"`
	SubjectID    int64  `json:"subject_id"`
	PriceVersion string `json:"price_version"`
}
type AITaskMaterialScope struct {
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	CollectedAt int64  `json:"collectedAt"`
}
type AITaskQuote struct {
	AnalysisType    string                `json:"analysisType"`
	SubjectType     string                `json:"subjectType"`
	SubjectID       int64                 `json:"subjectId"`
	SubjectName     string                `json:"subjectName"`
	PricePoints     int64                 `json:"pricePoints"`
	PriceVersion    string                `json:"priceVersion"`
	AvailablePoints int64                 `json:"availablePoints"`
	QuotedAt        int64                 `json:"quotedAt"`
	HoldExpireSec   int64                 `json:"holdExpireSec"`
	MaterialScope   []AITaskMaterialScope `json:"materialScope"`
}

// AITaskCreation 是首次创建收据，不声称当前任务状态、报告可用或已交付。
type AITaskCreation struct {
	TaskID       int64  `json:"taskId"`
	PricePoints  int64  `json:"pricePoints"`
	PriceVersion string `json:"priceVersion"`
}
