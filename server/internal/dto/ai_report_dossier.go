package dto

// 只附着于当前可读成功报告；没有原始回执、批准引用或执行配置。
type AIReportDossierReview struct {
	Version    int64                  `json:"version"`
	CompiledAt int64                  `json:"compiledAt"`
	Notice     string                 `json:"notice"`
	Items      []AIReportDossierField `json:"items"`
}
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
