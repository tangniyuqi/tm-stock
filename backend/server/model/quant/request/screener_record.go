package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type ScreenerRecordSearch struct {
	MemberId       *uint     `json:"member_id" form:"member_id"`
	Prompt         *string     `json:"prompt" form:"prompt"`
	Status         *int8       `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
