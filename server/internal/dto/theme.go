// Package dto 是对外契约层。字段一旦发布就难改，因此这里的约束最严。
//
// 🔴 契约层合规约束（来自 docs/specs/theme-query/design.md §3）：
//  1. 不得出现价值评价字段（IsLeader/Purity/RecommendLevel/BenefitLevel/Importance）
//  2. 不得出现个股详情字段（Kline/Fundamental/PeRatio/Macd/TechnicalIndicator）
//  3. 不得出现服务端下发的高亮/置顶字段——高亮只能是前端的用户选中态
//  4. 时间字段一律 int64 毫秒时间戳，禁 string
//
// 上述 1–3 由 scripts/check-architecture.sh 规则 7/8/9 机器守护；
// 第 4 条由规则 6 守护。
package dto

// ThemeDetailResp 题材详情。付费内容在无权限时置 nil 并 Locked=true，
// 不返回"空壳字段"——避免前端误以为"有数据但为空"。
type ThemeDetailResp struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CategoryPath string `json:"categoryPath"`
	Summary      string `json:"summary"`
	UpdatedAt    int64  `json:"updatedAt"` // 毫秒时间戳
	DataSource   string `json:"dataSource"`

	// 行情口径。一期 QuoteEnabled=false（ADR-0006：涨跌幅不是差异化，砍出最小路径）
	QuoteEnabled  bool  `json:"quoteEnabled"`
	QuoteDelayMin int   `json:"quoteDelayMin"` // 启用时固定 15
	QuoteAt       int64 `json:"quoteAt"`       // 行情【数据时点】，不是刷新时间
	QuoteIsMock   bool  `json:"quoteIsMock"`   // true 时前端必须显著标注「示例数据」

	Locked     bool             `json:"locked"`
	TrialLeft  int              `json:"trialLeft"`
	ChainNodes []ChainNodeResp  `json:"chainNodes"`
	Events     []ThemeEventResp `json:"events"`
}

// ChainNodeResp 产业链环节。
type ChainNodeResp struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// ChangePct 用指针：nil 表示【无数据】，前端显示「—」。
	// 绝不能用 0 表示无数据——0 会被读成"平盘"，那是假数据（数据真实铁律）。
	ChangePct *float64 `json:"changePct"`
	// Caliber 聚合口径说明。有 ChangePct 就必须有口径，否则数字来源不可解释。
	Caliber string `json:"caliber"`

	Stocks []StockItemResp `json:"stocks"`
}

// StockItemResp 成分股条目。
//
// 刻意【只有】标识 + 涨跌幅 + 依据标记三类字段。
// 任何"评价""排名""推荐"维度都不得加入——那是荐股定义的核心要件。
type StockItemResp struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Market string `json:"market"`

	ChangePct *float64 `json:"changePct"` // nil = 无数据，前端显示「—」

	// HasEvidence 恒为 true：无依据的映射在服务层已被过滤，不会出现在此列表中。
	// 保留该字段是为了让前端可以断言（若出现 false 说明服务端过滤失效）。
	HasEvidence bool `json:"hasEvidence"`
}

// ThemeEventResp 题材关联事件。只做客观转述 + 原文跳转，不加利好利空定性。
type ThemeEventResp struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	PublishAt int64  `json:"publishAt"`
	Source    string `json:"source"`
	SourceURL string `json:"sourceUrl"`
}

// EvidenceResp 归属依据。本项目差异化的核心，也是合规抓手。
//
// 四项字段缺一不可：服务层保证不会返回不完整的依据，
// 宁可返回 404 也不返回"空依据"——空依据等于承认"我们也不知道为什么"。
type EvidenceResp struct {
	StockCode     string `json:"stockCode"`
	StockName     string `json:"stockName"`
	ChainNodeName string `json:"chainNodeName"`

	SourceType    string `json:"sourceType"`
	SourceExcerpt string `json:"sourceExcerpt"`
	SourceURL     string `json:"sourceUrl"`
	CollectedAt   int64  `json:"collectedAt"` // 毫秒时间戳
}
