package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type FollowSearch struct {
	DealId          *int        `json:"deal_id" form:"deal_id"`
	MsgId           *int        `json:"msg_id" form:"msg_id"`
	MemberId        *int        `json:"member_id" form:"member_id"`
	CustomerId      *int        `json:"customer_id" form:"customer_id"`
	FollowTimeRange []time.Time `json:"follow_timeRange" form:"follow_timeRange[]"`
	Type            *int        `json:"type" form:"type"`
	Content         *string     `json:"content" form:"content"`
	Plan            *string     `json:"plan" form:"plan"`
	Feedback        *int        `json:"feedback" form:"feedback"`
	Reason          *string     `json:"reason" form:"reason"`
	CreatedAtRange  []time.Time `json:"createdAtRange" form:"createdAtRange[]"`
	request.PageInfo
}
