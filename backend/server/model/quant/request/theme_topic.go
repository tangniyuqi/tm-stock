package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type ThemeTopicSearch struct {
	ThemeId        *int64      `json:"themeId" form:"themeId"`
	Title          *string     `json:"title" form:"title"`
	Type           *int        `json:"type" form:"type"`
	Sentiment      *int        `json:"sentiment" form:"sentiment"`
	Status         *int        `json:"status" form:"status"`
	PublishDate    *string     `json:"publish_date" form:"publish_date"` // 发布日 YYYY-MM-DD，按当天过滤
	CreatedAtRange []time.Time `json:"createdAtRange" form:"createdAtRange[]"`
	request.PageInfo
}
