package dto

// 专题响应只包含固定标签、获授权原文及定位；不序列化内部候选、许可或批准记录。
type AIDossierView struct {
	ThemeID    int64                `json:"themeId"`
	Notice     string               `json:"notice"`
	Versions   []AIDossierVersion   `json:"versions"`
	Comparison *AIDossierComparison `json:"comparison,omitempty"`
}

type AIDossierComparison struct {
	Notice                     string                `json:"notice"`
	Added                      []string              `json:"added"`
	NotPresent                 []string              `json:"notPresent"`
	Changed                    []AIDossierCardChange `json:"changed"`
	Gaps                       []AIDossierGapChange  `json:"gaps"`
	QuotedSentenceOrderChanged bool                  `json:"quotedSentenceOrderChanged"`
}
type AIDossierCardChange struct {
	BeforeKey        string   `json:"beforeKey"`
	AfterKey         string   `json:"afterKey"`
	QuoteChanged     bool     `json:"quoteChanged"`
	SourceChanged    bool     `json:"sourceChanged"`
	ReferenceChanged bool     `json:"referenceChanged"`
	SectionChanged   bool     `json:"sectionChanged"`
	Fields           []string `json:"fields"`
}
type AIDossierGapChange struct {
	BeforeKey string `json:"beforeKey"`
	AfterKey  string `json:"afterKey"`
}
type AIDossierVersionItem struct {
	Version    int64 `json:"version"`
	CompiledAt int64 `json:"compiledAt"`
}
type AIDossierVersionList struct {
	ThemeID           int64                  `json:"themeId"`
	Items             []AIDossierVersionItem `json:"items"`
	HasMore           bool                   `json:"hasMore"`
	NextBeforeVersion int64                  `json:"nextBeforeVersion"`
}
type AIDossierVersion struct {
	Version    int64           `json:"version"`
	CompiledAt int64           `json:"compiledAt"`
	Cards      []AIDossierCard `json:"cards"`
	Gaps       []AIDossierGap  `json:"gaps"`
}
type AIDossierCard struct {
	Key     string           `json:"key"`
	Section string           `json:"section"`
	Quote   AIDossierQuote   `json:"quote"`
	Fields  []AIDossierField `json:"fields"`
}
type AIDossierQuote struct {
	Text        string `json:"text"`
	Context     string `json:"context"`
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	CollectedAt int64  `json:"collectedAt"`
	SnapshotID  int64  `json:"snapshotId"`
	Sentence    int    `json:"sentence"`
	StartByte   int    `json:"startByte"`
	EndByte     int    `json:"endByte"`
}
type AIDossierField struct {
	Key   string          `json:"key"`
	State string          `json:"state"`
	Quote *AIDossierQuote `json:"quote,omitempty"`
}
type AIDossierGap struct {
	Key     string `json:"key"`
	Section string `json:"section"`
	Reason  string `json:"reason"`
}

func (AIDossierView) String() string     { return "专题资料响应（正文已隐藏）" }
func (v AIDossierView) GoString() string { return v.String() }
