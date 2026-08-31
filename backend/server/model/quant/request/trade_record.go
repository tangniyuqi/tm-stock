package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type TradeRecordSearch struct {
	MemberId       *uint       `json:"member_id" form:"member_id"`
	AccountId      *uint       `json:"account_id" form:"account_id"`
	TaskId         *uint       `json:"task_id" form:"task_id"`
	Action         *string     `json:"action" form:"action"`
	Direction      *string     `json:"direction" form:"direction"`
	Type           *string     `json:"type" form:"type"`
	Symbol         *string     `json:"symbol" form:"symbol"`
	Name           *string     `json:"name" form:"name"`
	Keyword        string      `json:"keyword" form:"keyword"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
