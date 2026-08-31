package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type IcSearch struct {
	MsgId          *int        `json:"msg_id" form:"msg_id"`
	Sender         *string     `json:"sender" form:"sender"`
	Type           *int        `json:"type" form:"type"`
	Name           *string     `json:"name" form:"name"`
	Times          *int        `json:"times" form:"times"`
	Status         *int        `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
