package dto

// 只附着于当前可读成功报告；没有原始回执、批准引用或执行配置。
type AIReportDossierReview struct {
	Version    int64                      `json:"version"`
	CompiledAt int64                      `json:"compiledAt"`
	Notice     string                     `json:"notice"`
	Items      []AIReportDossierField     `json:"items"`
	Comparison *AIReportDossierComparison `json:"comparison,omitempty"`
}

// 比较只携带固定状态和当前获授权原文，不含模型包或内部执行定位。
type AIReportDossierComparison struct {
	Notice           string                   `json:"notice"`
	Focus            string                   `json:"focus"`
	Outcome          string                   `json:"outcome"`
	Wording          string                   `json:"wording"`
	StatementOrigins string                   `json:"statementOrigins"`
	Left             AIReportComparisonCard   `json:"left"`
	Right            AIReportComparisonCard   `json:"right"`
	Axes             []AIReportComparisonAxis `json:"axes"`
}
type AIReportComparisonCard struct {
	Key       string         `json:"key"`
	Section   string         `json:"section"`
	Statement AIDossierQuote `json:"statement"`
}
type AIReportComparisonAxis struct {
	Field    string                 `json:"field"`
	Required bool                   `json:"required"`
	Left     AIReportComparisonSide `json:"left"`
	Right    AIReportComparisonSide `json:"right"`
	Relation string                 `json:"relation"`
	Reason   string                 `json:"reason"`
}
type AIReportComparisonSide struct {
	State   string          `json:"state"`
	Verdict string          `json:"verdict"`
	Quote   *AIDossierQuote `json:"quote,omitempty"`
}

func (AIReportDossierComparison) String() string {
	return "报告两卡对照展示（正文已隐藏）"
}
func (v AIReportDossierComparison) GoString() string { return v.String() }

type AIReportDossierField struct {
	CardKey   string         `json:"cardKey"`
	Section   string         `json:"section"`
	Field     string         `json:"field"`
	Verdict   string         `json:"verdict"`
	Reason    string         `json:"reason"`
	Checked   AIDossierQuote `json:"checked"`
	Statement AIDossierQuote `json:"statement"`
}

func (AIReportDossierReview) String() string     { return "报告字段核验展示（正文已隐藏）" }
func (v AIReportDossierReview) GoString() string { return v.String() }
