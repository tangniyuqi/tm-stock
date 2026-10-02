package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 题材-股票关联的依据类型（与 server/migrations 里 source_type 的注释一致）
const (
	ThemeStockSourceAnnouncement = 1 // 公告
	ThemeStockSourceAnnualReport = 2 // 年报
	ThemeStockSourceProspectus   = 3 // 招股书
	ThemeStockSourceCatalog      = 4 // 官方产业目录
	ThemeStockSourceInteractive  = 5 // 互动易问答
)

// 题材-股票关联的审核状态。仅 ThemeStockAuditPassed 对 C 端可见。
const (
	ThemeStockAuditDraft    = 0 // 草稿
	ThemeStockAuditPending  = 1 // 待审
	ThemeStockAuditPassed   = 2 // 已通过
	ThemeStockAuditRejected = 3 // 已驳回
)

// ThemeStock 题材-股票关联（"这家公司凭什么归到这个题材"）。
//
// 🔴 合规命门（ADR-0003）：每条归属必须带可溯源的客观依据——依据类型、原文摘录、原文链接、采集时点——
// 四项在库里 NOT NULL 且 CHECK 非空，无依据禁止入库。归属依据若来自我方主观判断，
// "事实归集"就退化成"品种选择"，落入荐股认定要件。
//
// ⚠️ 本表由 tm-stock 的 SQL 迁移管理（server/migrations/20260730_addon_quant_theme_stock.sql 与
// 20261002_converge_addon_quant_theme_stock.sql），不在 GVA 的 AutoMigrate 列表里（initialize/gorm_biz.go）。
// 历史上 GVA 的 AutoMigrate 会静默改写这张表、削弱上述约束（见 docs/specs/ai-analysis/requirements.md 的 F7）。
// 这里的 gorm 标签只描述列，用于读写，不会改表；不要把本模型加回 AutoMigrate。
//
// 旧版的梯队（tier）、相关度（relevance）、AI 入选逻辑（ai_reason）、人工入选逻辑（reason）、纳入日期（in_date）
// 五列已移除：它们是对个股的价值评价与排序，不是客观事实，不得再加回来
// （AC-G5 的字段名守卫会在 server/ 一侧拦同类字段；这里靠评审与合规词门禁）。
type ThemeStock struct {
	global.GVA_MODEL_ADDON
	ThemeId *int32 `json:"theme_id" form:"theme_id" gorm:"column:theme_id;type:bigint unsigned;not null;comment:题材/环节ID（addon_quant_theme.id，可为任意层级）"` // 题材/环节ID
	StockId *int64 `json:"stock_id" form:"stock_id" gorm:"column:stock_id;type:bigint unsigned;not null;comment:股票ID（addon_quant_base_stock.id，权威字段）"` // 股票ID（权威）
	TsCode  string `json:"ts_code" form:"ts_code" gorm:"column:ts_code;size:20;not null;comment:TS代码冗余（仅供排查对账，权威以 stock_id 为准）"`                       // 由服务层按 stock_id 回填，不信任请求值

	// ── 归属依据（四项缺一不可）──
	SourceType    *int8      `json:"source_type" form:"source_type" gorm:"column:source_type;not null;comment:依据类型：1公告 2年报 3招股书 4官方产业目录 5互动易问答"`           // 依据类型
	SourceExcerpt string     `json:"source_excerpt" form:"source_excerpt" gorm:"column:source_excerpt;size:1000;not null;comment:原文摘录。禁止填“见链接”“详见公告”之类占位"` // 原文摘录
	SourceUrl     string     `json:"source_url" form:"source_url" gorm:"column:source_url;size:512;not null;comment:原文链接"`                                 // 原文链接
	CollectedAt   *time.Time `json:"collected_at" form:"collected_at" gorm:"column:collected_at;not null;comment:采集时点"`                                    // 采集时点

	// ── 审核（仅已通过的对 C 端可见）──
	AuditStatus  int8       `json:"audit_status" form:"audit_status" gorm:"column:audit_status;not null;default:0;comment:审核：0草稿 1待审 2已通过 3已驳回。仅 2 对 C 端可见"` // 审核状态
	AuditBy      *uint      `json:"audit_by" form:"audit_by" gorm:"column:audit_by;type:bigint unsigned;comment:审核人"`                                        // 审核人
	AuditAt      *time.Time `json:"audit_at" form:"audit_at" gorm:"column:audit_at;comment:审核时间"`                                                            // 审核时间
	RejectReason *string    `json:"reject_reason" form:"reject_reason" gorm:"column:reject_reason;size:250;comment:驳回原因"`                                    // 驳回原因

	Remark *string    `json:"remark" form:"remark" gorm:"column:remark;size:250;comment:备注"`          // 备注（仅后台可见）
	Sort   *int32     `json:"sort" form:"sort" gorm:"column:sort;default:0;comment:排序"`               // 排序
	Status *int8      `json:"status" form:"status" gorm:"column:status;default:1;comment:状态：1启用 0停用"` // 状态
	Theme  *Theme     `json:"theme,omitempty" gorm:"foreignKey:ThemeId"`
	Stock  *BaseStock `json:"stock,omitempty" gorm:"foreignKey:StockId"`
}

// TableName 题材股票 ThemeStock自定义表名 addon_quant_theme_stock
func (ThemeStock) TableName() string {
	return "addon_quant_theme_stock"
}
