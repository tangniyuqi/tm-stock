package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type EventSearch struct {
	Name           *string     `json:"name" form:"name"`
	Date           *time.Time  `json:"date" form:"date""`
	City           *string     `json:"city" form:"city"`
	Industry       *string     `json:"industry" form:"industry"`
	Level          *int        `json:"level" form:"level"`
	Content        *string     `json:"content" form:"content"`
	Remark         *string     `json:"remark" form:"remark"`
	Status         *int        `json:"status" form:"status"`
	DateRange      []string    `json:"date_range" form:"date_range[]"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	OrderKey       string      `json:"orderKey" form:"orderKey"`   // 排序字段
	OrderDesc      bool        `json:"orderDesc" form:"orderDesc"` // 排序方式: true 倒序, false 正序
	request.PageInfo
}
