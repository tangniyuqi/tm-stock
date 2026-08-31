package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type GroupSearch struct {
	Name           *string     `json:"name" form:"name"`
	Status         *int        `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
