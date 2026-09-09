package dto

type ThemeDailyItemResp struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	ThemeID   int64    `json:"themeId"`
	ThemeName string   `json:"themeName"`
	ChangePct *float64 `json:"changePct"`
	Caliber   string   `json:"caliber"`
	PublishAt int64    `json:"publishAt"`
	Source    string   `json:"source"`
	SourceURL string   `json:"sourceUrl"`
}
type ThemeDailyPageResp struct {
	BizDay        string               `json:"date"` // YYYY-MM-DD；日期参数按设计契约为字符串
	IsTradingDay  bool                 `json:"isTradingDay"`
	EmptyReason   string               `json:"emptyReason"`
	DataFrom      string               `json:"dataFrom"`
	QuoteDelayMin int                  `json:"quoteDelayMin"`
	SnapshotAt    int64                `json:"snapshotAt"`
	QuoteIsMock   bool                 `json:"quoteIsMock"`
	Sort          string               `json:"sort"`
	List          []ThemeDailyItemResp `json:"list"`
	Total         int                  `json:"total"`
	HasMore       bool                 `json:"hasMore"`
}
