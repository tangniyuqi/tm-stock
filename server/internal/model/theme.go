// Package model 是领域实体层，对应数据库表结构。
// 只放数据结构与最基本的自校验，不放业务编排（那属于 service）。
//
// 🔴 表归属（ADR-0007）：
//   - addon_quant_theme / addon_quant_base_stock 由【现有系统】维护，本项目只读；
//   - addon_quant_theme_stock（题材↔股票关联）由本项目负责。
//
// 现有表的约定：主键 bigint unsigned、时间 datetime(3)、软删除 deleted_at、
// 另有 created_by / updated_by / deleted_by。本层跟随该约定。
package model

import "time"

// 审核状态。现有系统用 tinyint 枚举风格，这里跟随。
//
// ⚠️ 与表里的 status 字段不是一回事：status 是【启用/停用】，
// AuditStatus 是【审核流转】。命名刻意区分，避免日后有人混用。
const (
	AuditStatusDraft    int8 = 0 // 草稿（默认值，fail-safe：误插入的行不对外可见）
	AuditStatusPending  int8 = 1 // 待审
	AuditStatusApproved int8 = 2 // 已通过 —— 唯一对 C 端可见的状态
	AuditStatusRejected int8 = 3 // 已驳回
)

// 依据来源类型（ADR-0007 确认的五类，均为可核对原文的公开信息源）。
//
// 刻意不含【券商研报】与【媒体报道】：研报本身就含投资意见，
// 拿它当归属依据等于间接引用他人的评价，合规强度远低于公告原文。
const (
	SourceTypeAnnouncement int8 = 1 // 公告
	SourceTypeAnnualReport int8 = 2 // 年报
	SourceTypeProspectus   int8 = 3 // 招股书
	SourceTypeOfficialDir  int8 = 4 // 官方产业目录
	SourceTypeInteractive  int8 = 5 // 互动易问答
)

// 启用状态。
const (
	StatusDisabled int8 = 0
	StatusEnabled  int8 = 1
)

// Theme 题材节点（addon_quant_theme）。
//
// 这张表是一棵树：Level=1 为题材本身，Level=2 为其下的产业链环节
// （ADR-0007 确定的语义）。不用它表达分类导航。
type Theme struct {
	ID          int64
	Name        string
	Code        string // 可空，来源侧编码
	Level       int8   // 1=题材 2=产业链环节
	ParentID    int64  // 顶层为 0
	Description string
	Sort        int
	Status      int8
	UpdatedAt   time.Time
}

// IsChainNode 是否为产业链环节（题材的子节点）。
func (t Theme) IsChainNode() bool { return t.Level >= 2 && t.ParentID != 0 }

// Stock 个股（addon_quant_base_stock，Tushare stock_basic 结构）。
//
// 🔴 本项目【只读】此表，且禁止推动它扩展基本面/财务/技术指标字段——
// 那是红线而非排期（ADR-0003）。
type Stock struct {
	ID         int64  // 主键，关联表按它关联
	TsCode     string // 000001.SZ，唯一业务标识
	Symbol     string // 000001，用户看到的代码
	Name       string
	Industry   string
	CnSpell    string // 拼音缩写，支撑首字母搜索
	Market     string // 主板/创业板/科创板…
	Exchange   string // SSE/SZSE/BSE
	ListStatus string // L上市 D退市 P暂停
}

// ThemeStockMapping 题材↔股票归属映射（addon_quant_theme_stock）。
//
// 本项目的合规命门：四项证据字段缺任一项，该归属即不成立——
// "凭什么把这家公司归到这个题材"没有客观依据时，
// 归属会从【事实归集】退化成【品种选择】，落入荐股认定要件二（ADR-0003）。
type ThemeStockMapping struct {
	ID      int64
	ThemeID int64  // 指向 addon_quant_theme.id，可为任意层级（题材本身或其环节）
	StockID int64  // 指向 addon_quant_base_stock.id（权威）
	TsCode  string // 冗余，仅供对账排查

	SourceType    int8
	SourceExcerpt string
	SourceURL     string
	CollectedAt   time.Time

	AuditStatus int8
	Status      int8
	Sort        int
}

// HasCompleteEvidence 判断证据是否齐全。
//
// 为什么 DB 已有 NOT NULL + CHECK 还要在服务层判一次：
//   - CHECK 在 MySQL 8.0.16 之前不生效，5.7 更是静默忽略
//   - 约束可能被误删；历史数据可能早于约束
//   - 这是三层防线里唯一能被单元测试覆盖的一层
//
// 三层的价值在于任意一层失效时其余仍拦得住，不是重复劳动。
func (m ThemeStockMapping) HasCompleteEvidence() bool {
	return m.SourceType > 0 &&
		m.SourceExcerpt != "" &&
		m.SourceURL != "" &&
		!m.CollectedAt.IsZero()
}

// IsVisibleToPublic 对 C 端可见的两个条件：已审核通过 + 启用中。
// （软删除由 SQL 层的 deleted_at IS NULL 负责，不在内存里判。）
func (m ThemeStockMapping) IsVisibleToPublic() bool {
	return m.AuditStatus == AuditStatusApproved && m.Status == StatusEnabled
}
