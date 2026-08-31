package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type StrategySearch struct {
	Name           *string     `json:"name" form:"name"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	OrderKey       string      `json:"orderKey" form:"orderKey"` // 排序字段
	Desc           bool        `json:"desc" form:"desc"`         // 是否倒序
	request.PageInfo
}
