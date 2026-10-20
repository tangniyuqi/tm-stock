package dto

// AITaskProgress 不含幂等键、提示词、预算、租约、围栏、原始 Agent 报告或供应商异常。
type AITaskProgress struct {
	ID              int64  `json:"id"`
	AnalysisType    int    `json:"analysisType"`
	SubjectType     string `json:"subjectType"`
	SubjectID       int64  `json:"subjectId"`
	Status          int    `json:"status"`
	CurrentStep     int    `json:"currentStep"`
	EventSeq        int64  `json:"eventSeq"`
	CancelRequested bool   `json:"cancelRequested"`
	ErrorCode       string `json:"errorCode"`
	DeadlineAt      int64  `json:"deadlineAt"`
	ReportAvailable bool   `json:"reportAvailable"`
}

type AITaskHistory struct {
	Items        []AITaskSummary `json:"items"`
	NextBeforeID int64           `json:"nextBeforeId"`
	HasMore      bool            `json:"hasMore"`
}

// AITaskSummary 是独立的列表 DTO，不复用进度详情或加载关联内容。
type AITaskSummary struct {
	ID              int64  `json:"id"`
	AnalysisType    int    `json:"analysisType"`
	SubjectType     string `json:"subjectType"`
	SubjectID       int64  `json:"subjectId"`
	Status          int    `json:"status"`
	DeadlineAt      int64  `json:"deadlineAt"`
	ReportAvailable bool   `json:"reportAvailable"`
}
