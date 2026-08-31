package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type MsgSearch struct {
	ID             *int        `json:"id" form:"id"`
	ClientId       *int        `json:"client_id" form:"client_id"`
	SenderId       *string     `json:"sender_id" form:"sender_id"`
	MsgId          *string     `json:"msg_id" form:"msg_id"`
	Gid            *string     `json:"gid" form:"gid"`
	Group          *string     `json:"group" form:"group"`
	Type           *string     `json:"type" form:"type"`
	Sender         *string     `json:"sender" form:"sender"`
	SenderRemark   *string     `json:"sender_remark" form:"sender_remark"`
	Content        *string     `json:"content" form:"content"`
	Priority       *int        `json:"priority" form:"priority"`
	Oppty          *int        `json:"oppty" form:"oppty"`
	Level          *int        `json:"level" form:"level"`
	Remark         *string     `json:"remark" form:"remark"`
	Status         *int        `json:"status" form:"status"`
	SendTimeRange  []time.Time `json:"send_time_range" form:"send_time_range[]"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	MatchType      string      `json:"match_type" form:"match_type"`
	Q              *string     `json:"q" form:"q"`
	StatusStr      *string     `json:"status_str" form:"status_str"`
	request.PageInfo
}
