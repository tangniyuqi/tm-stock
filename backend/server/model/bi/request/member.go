package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type MemberSearch struct {
	ClientId       *int        `json:"client_id" form:"client_id"`
	MsgId          *int        `json:"msg_id" form:"msg_id"`
	Name           *string     `json:"name" form:"name"`
	Nickname       *string     `json:"nickname" form:"nickname"`
	Mobile         *string     `json:"mobile" form:"mobile"`
	Wechat         *string     `json:"wechat" form:"wechat"`
	Business       *string     `json:"business" form:"business"`
	Remark         *string     `json:"remark" form:"remark"`
	Status         *int        `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
