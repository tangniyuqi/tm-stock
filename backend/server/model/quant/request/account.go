package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type AccountSearch struct {
	Name              *string     `json:"name" form:"name"`
	MemberId          *uint32     `json:"member_id" form:"member_id"`
	Broker            *string     `json:"broker" form:"broker"`
	AccountNo         *string     `json:"account_no" form:"account_no"`
	ExpirationDay     *int        `json:"expiration_day" form:"expiration_day"`
	CreatedAtRange    []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
