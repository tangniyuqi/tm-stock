package dto

// 原始请求、内部上下文、批准引用、上游请求号和凭据不得进入会员投影。
type AIReportInputCall struct {
	Seq           int    `json:"seq"`
	Node          string `json:"node"`
	Model         string `json:"model"`
	State         string `json:"state"`
	RequestDigest string `json:"requestDigest"`
	PromptDigest  string `json:"promptDigest"`
	InputTokens   int    `json:"inputTokens"`
	OutputTokens  int    `json:"outputTokens"`
	CostMicros    *int64 `json:"costMicros"`
}
type AIReportInputFact struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type AIReportInputs struct {
	TaskID         int64               `json:"taskId"`
	SubjectID      int64               `json:"subjectId"`
	Schema         string              `json:"schema"`
	Scope          string              `json:"scope"`
	UpstreamCommit string              `json:"upstreamCommit"`
	Provider       string              `json:"provider"`
	FixtureDigest  string              `json:"fixtureDigest"`
	FixtureSubject string              `json:"fixtureSubject"`
	FixtureNotice  string              `json:"fixtureNotice"`
	Facts          []AIReportInputFact `json:"facts"`
	Calls          []AIReportInputCall `json:"calls"`
}
