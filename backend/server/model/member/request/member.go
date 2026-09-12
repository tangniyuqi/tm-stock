package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

// MemberSearch 后台管理 - 用户列表搜索条件
type MemberSearch struct {
	Name           *string    `json:"name" form:"name"`
	Nickname       *string    `json:"nickname" form:"nickname"`
	Mobile         *string    `json:"mobile" form:"mobile"`
	Email          *string    `json:"email" form:"email"`
	Status         *int8      `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}