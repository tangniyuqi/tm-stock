package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type DealSearch struct {
	Title          *string    `json:"title" form:"title"`
	Type           *int       `json:"type" form:"type"`
	Stage          *int       `json:"stage" form:"stage"`
	Source         *int       `json:"source" form:"source"`
	Status         *int       `json:"status" form:"status"`
	StartCreatedAt *time.Time `json:"start_created_at" form:"start_created_at"`
	EndCreatedAt   *time.Time `json:"end_created_at" form:"end_created_at"`
	CreatedBy      uint       `json:"created_by" form:"created_by"`
	request.PageInfo
}
