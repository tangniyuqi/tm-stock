package dto

// AITaskCancellation 只表达本地当前状态，不表示远端已停止或冻结已释放。
type AITaskCancellation struct {
	ID              int64  `json:"id"`
	Status          int    `json:"status"`
	EventSeq        int64  `json:"eventSeq"`
	CancelRequested bool   `json:"cancelRequested"`
	ErrorCode       string `json:"errorCode"`
	ReportAvailable bool   `json:"reportAvailable"`
}
