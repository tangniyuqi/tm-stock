package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type ClientSearch struct {
	Name           *string     `json:"name" form:"name"`
	Server         *string     `json:"server" form:"server"`
	Status         *int        `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
