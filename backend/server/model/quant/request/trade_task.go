package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type TradeTaskSearch struct {
	MemberId       *uint       `json:"member_id" form:"member_id"`
	AccountId      *uint       `json:"account_id" form:"account_id"`
	StrategyId     *uint       `json:"strategy_id" form:"strategy_id"`
	Name           *string     `json:"name" form:"name"`
	Remark         *string     `json:"remark" form:"remark"`
	Status         *int        `json:"status" form:"status"`
	Keyword        string      `json:"keyword" form:"keyword"`
	CreatedBy      *uint       `json:"created_by" form:"created_by"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
