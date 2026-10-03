package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

// ThemeStockSearch 用于分页和查询题材-股票关联（仅后台使用）
//
// 旧版的 tier（梯队）筛选与 reason（入选逻辑）模糊搜索已随评价类字段一起移除；
// 现在按依据摘录与审核状态检索。
type ThemeStockSearch struct {
	ID             *int64      `json:"id" form:"id"`
	ThemeId        *int32      `json:"theme_id" form:"theme_id"`
	StockId        *int64      `json:"stock_id" form:"stock_id"`
	SourceType     *int        `json:"source_type" form:"source_type"`
	SourceExcerpt  *string     `json:"source_excerpt" form:"source_excerpt"` // 依据摘录（模糊匹配）
	AuditStatus    *int        `json:"audit_status" form:"audit_status"`     // 审核：0草稿 1待审 2已通过 3已驳回
	Status         *int        `json:"status" form:"status"`
	Sort           *int32      `json:"sort" form:"sort"`
	CreatedAtRange []time.Time `json:"createdAtRange" form:"createdAtRange[]"`
	OrderKey       string      `json:"orderKey" form:"orderKey"` // 排序字段
	Desc           bool        `json:"desc" form:"desc"`         // 是否倒序
	request.PageInfo
}
