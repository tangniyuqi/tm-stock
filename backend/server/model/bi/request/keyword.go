package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type KeywordSearch struct {
	Name           *string     `json:"name" form:"name"`
	Times          *int        `json:"times" form:"times"`
	Status         *int        `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	SortType       string      `json:"sort_type" form:"sort_type"`
	request.PageInfo
}
