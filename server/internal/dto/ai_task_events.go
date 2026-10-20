package dto

// AITaskEvent 只表达真实状态进度，不承载自由文本或内部执行身份。
type AITaskEvent struct {
	Seq       int64  `json:"seq"`
	Type      string `json:"type"`
	Status    int    `json:"status"`
	Step      int    `json:"step"`
	ErrorCode string `json:"errorCode"`
	At        int64  `json:"at"`
}

type AITaskEventPage struct {
	Items        []AITaskEvent `json:"items"`
	NextAfterSeq int64         `json:"nextAfterSeq"`
	HasMore      bool          `json:"hasMore"`
	UpperSeq     int64         `json:"upperSeq"`
}
