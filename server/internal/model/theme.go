// Package model 是领域实体层，对应数据库表结构。
// 只放数据结构与最基本的自校验，不放业务编排（那属于 service）。
package model

import "time"

// MappingStatusApproved 是唯一对 C 端可见的映射状态。
// 常量集中在此，避免各处硬编码字符串（见 methodology 常量管理条）。
const (
	MappingStatusDraft    = "DRAFT"
	MappingStatusPending  = "PENDING"
	MappingStatusApproved = "APPROVED"
	MappingStatusRejected = "REJECTED"
)

// Theme 题材。
type Theme struct {
	ID         int64
	Name       string
	CategoryID int64
	Summary    string // 一句话通俗解读（客观）
	DataSource string
	UpdatedAt  time.Time
}

// Category 题材分类（一/二/三级）。
type Category struct {
	ID        int64
	Name      string
	Level     int
	ParentID  int64
	SortOrder int
}

// ChainNode 产业链环节。题材详情页的中间层。
type ChainNode struct {
	ID          int64
	ThemeID     int64
	Name        string
	Description string
	SortOrder   int
}

// Stock 个股标识。
//
// 🔴 禁止在此结构体扩展基本面/财务/技术指标字段——
// 这是红线而非排期（ADR-0003），且 scripts/check-architecture.sh 规则 8 会拦。
type Stock struct {
	Code     string
	Name     string
	Market   string // SH|SZ|BJ
	Delisted bool
}

// ThemeStockMapping 题材↔个股归属映射，本项目的合规命门。
//
// 四项证据字段（SourceType/SourceExcerpt/SourceURL/CollectedAt）缺任一项，
// 该映射即不成立——因为"凭什么把这家公司归到这个环节"就没有客观依据，
// 会从「事实归集」退化成「品种选择」（落入荐股要件二）。
type ThemeStockMapping struct {
	ID          int64
	ThemeID     int64
	ChainNodeID int64
	StockCode   string

	SourceType    string // 公告|年报|招股书|官方目录
	SourceExcerpt string // 原文摘录
	SourceURL     string // 原文链接
	CollectedAt   time.Time

	Status    string
	SortOrder int
}

// HasCompleteEvidence 判断证据是否齐全。
//
// 为什么服务层还要判一次（DB 已有 NOT NULL + CHECK）：
//   - CHECK 约束在 MySQL 8.0.16 之前不生效，且可能被误删/被 --skip 绕过
//   - 历史数据可能早于约束
//   - 这是唯一可被单元测试覆盖的那一层
//
// 三层防线的价值在于任意一层失效时其余仍然拦得住，不是重复劳动。
func (m ThemeStockMapping) HasCompleteEvidence() bool {
	return m.SourceType != "" &&
		m.SourceExcerpt != "" &&
		m.SourceURL != "" &&
		!m.CollectedAt.IsZero()
}

// IsVisibleToPublic 仅已审核通过的映射才对 C 端可见。
func (m ThemeStockMapping) IsVisibleToPublic() bool {
	return m.Status == MappingStatusApproved
}

// ThemeEvent 题材关联事件。
type ThemeEvent struct {
	ID        int64
	ThemeID   int64
	Title     string
	Source    string
	SourceURL string
	PublishAt time.Time
}
