package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type NewsSearch struct {
	Keyword        *string     `json:"keyword" form:"keyword"`
	Title          *string     `json:"title" form:"title"`
	Content        *string     `json:"content" form:"content"`
	Author         *string     `json:"author" form:"author"`
	Nature         *int        `json:"nature" form:"nature"`
	Level          *int        `json:"level" form:"level"`
	Bold           *int        `json:"bold" form:"bold"`
	Source         *int        `json:"source" form:"source"`
	Status         *int        `json:"status" form:"status"`
	CreatedAtRange []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
	Sort  string `json:"sort" form:"sort"`
	Order string `json:"order" form:"order"`
	LastID int `json:"last_id" form:"last_id"`
}
