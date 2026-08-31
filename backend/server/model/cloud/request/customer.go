package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type CustomerSearch struct {
	Name           *string     `json:"name" form:"name"`
	Type           *int        `json:"type" form:"type"`
	Level          *int        `json:"level " form:"level"`
	Tips           *string     `json:"tips" form:"tips"`
	Mobile         *string     `json:"mobile" form:"mobile"`
	Wechat         *string     `json:"wechat" form:"wechat"`
	Douyin         *string     `json:"douyin" form:"douyin"`
	Qq             *string     `json:"qq" form:"qq"`
	Company        *string     `json:"company" form:"company"`
	Industry       *int        `json:"industry" form:"industry"`
	Business       *string     `json:"business" form:"business"`
	Source         *int        `json:"source" form:"source"`
	Remark         *string     `json:"remark" form:"remark"`
	Status         *int        `json:"status" form:"status"`
	CreatedBy      uint        `json:"created_by" form:"created_by"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
