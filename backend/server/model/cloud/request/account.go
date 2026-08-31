package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type AccountSearch struct {
	Title          *string     `json:"title" form:"title"`
	Type           *int        `json:"type" form:"type"`
	Remark         *string     `json:"remark" form:"remark"`
	Balance        *int        `json:"balance" form:"balance"`
	Review         *int        `json:"review" form:"review"`
	Status         *int        `json:"status" form:"status"`
	CreatedBy      uint        `json:"created_by" form:"created_by"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
