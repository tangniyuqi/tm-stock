package dto

// AIReport 是会员可见白名单，不复用内部渲染/证据/结算结构。
type AIReport struct {
	TaskID      int64              `json:"taskId"`
	SubjectType string             `json:"subjectType"`
	SubjectID   int64              `json:"subjectId"`
	SubjectName string             `json:"subjectName"`
	GeneratedAt int64              `json:"generatedAt"`
	Disclosure  AIReportDisclosure `json:"disclosure"`
	Sections    []AIReportSection  `json:"sections"`
}
type AIReportDisclosure struct {
	AIGenerated bool   `json:"aiGenerated"`
	ModelName   string `json:"modelName"`
	FilingNo    string `json:"filingNo"`
}
type AIReportSection struct {
	Key         string            `json:"key"`
	Title       string            `json:"title"`
	AIGenerated bool              `json:"aiGenerated"`
	Quotes      []AIReportQuote   `json:"quotes"`
	Companies   []AIReportCompany `json:"companies,omitempty"`
}
type AIReportCompany struct {
	Name     string                    `json:"name"`
	Code     string                    `json:"code"`
	Node     string                    `json:"node"`
	Evidence []AIReportCompanyEvidence `json:"evidence"`
}
type AIReportCompanyEvidence struct {
	Text        string `json:"text"`
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	CollectedAt int64  `json:"collectedAt"`
	Trimmed     bool   `json:"trimmed"`
}
type AIReportQuote struct {
	Text        string `json:"text"`
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	CollectedAt int64  `json:"collectedAt"`
}

func (r AIReport) String() string   { return "会员报告(格式化日志不输出正文)" }
func (r AIReport) GoString() string { return r.String() }
