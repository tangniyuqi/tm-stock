package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type ThemeSearch struct {
	Name           *string     `json:"name" form:"name"`
	Code           *string     `json:"code" form:"code"`
	Level          *int        `json:"level" form:"level"`
	ParentID       *int        `json:"parent_id" form:"parent_id"`
	Description    string      `json:"description" form:"description"`
	Remark         string      `json:"remark" form:"remark"`
	Source         *int        `json:"source" form:"source"`
	ChangePct      *float64    `json:"change_pct" form:"change_pct"`
	Sort           int         `json:"sort" form:"sort"`
	Status         *int        `json:"status" form:"status"`
	StockCount     *int32      `json:"stock_count" form:"stock_count"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
