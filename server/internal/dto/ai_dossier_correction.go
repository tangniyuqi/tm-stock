package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"

const AIDossierCorrectionNotice = "仅展示所选版本中经操作人审核的明确更正声明及双方引文；不代表事实本身或全部变化均已核实。未展示声明不表示不存在更正。"

type AIDossierCorrectionQuote struct {
	Version int64          `json:"version"`
	Key     string         `json:"key"`
	Field   string         `json:"field"`
	Quote   AIDossierQuote `json:"quote"`
}
type AIDossierCorrectionItem struct {
	Before    AIDossierCorrectionQuote `json:"before"`
	After     AIDossierCorrectionQuote `json:"after"`
	Statement AIDossierCorrectionQuote `json:"statement"`
}
type AIDossierCorrections struct {
	ThemeID       int64                     `json:"themeId"`
	BeforeVersion int64                     `json:"beforeVersion"`
	AfterVersion  int64                     `json:"afterVersion"`
	Notice        string                    `json:"notice"`
	State         string                    `json:"state"`
	Items         []AIDossierCorrectionItem `json:"items"`
}
type AIDossierCorrectionReceipt struct {
	RequestID  string                 `json:"requestId"`
	Object     dossier.CorrectionPair `json:"object"`
	ActorID    int64                  `json:"actorId"`
	Revision   int64                  `json:"revision"`
	Enabled    bool                   `json:"enabled"`
	RecordedAt int64                  `json:"recordedAt"`
}
type AIDossierCorrectionInspect struct {
	Notice       string                 `json:"notice"`
	Object       dossier.CorrectionPair `json:"object"`
	BeforeDigest string                 `json:"beforeDigest"`
	AfterDigest  string                 `json:"afterDigest"`
	Revision     int64                  `json:"revision"`
	Enabled      bool                   `json:"enabled"`
	ValidUntil   int64                  `json:"validUntil"`
	SourceState  string                 `json:"sourceState"`
	Preview      *AIDossierView         `json:"preview"`
}

func (AIDossierCorrections) String() string     { return "明确更正声明响应（正文已隐藏）" }
func (v AIDossierCorrections) GoString() string { return v.String() }
func (AIDossierCorrectionInspect) String() string {
	return "明确更正声明预览（内部内容已隐藏）"
}
func (v AIDossierCorrectionInspect) GoString() string { return v.String() }
