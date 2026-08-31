package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type ReportSearch struct {
	Title             *string     `json:"title" form:"title"`
	Type              *int        `json:"type" form:"type"`
	Level             *int        `json:"level" form:"level"`
	Industry          *string     `json:"industry" form:"industry"`
	Institution       *string     `json:"institution" form:"institution"`
	Analyst           *string     `json:"analyst" form:"analyst"`
	PublishDateRange  []time.Time `json:"publish_date_range" form:"publish_date_range[]"`
	DeadlineDateRange []time.Time `json:"deadline_date_range" form:"deadline_date_range[]"`
	Summary           *string     `json:"summary" form:"summary"`
	FileExt           *string     `json:"file_ext" form:"file_ext"`
	Remark            *string     `json:"remark" form:"remark"`
	Status            *int        `json:"status" form:"status"`
	CreatedAtRange    []time.Time `json:"created_at_range" form:"created_at_range[]"`
	request.PageInfo
}
