package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

// SmsLogSearch 后台管理 - 短信日志列表搜索条件
type SmsLogSearch struct {
	Mobile          *string    `json:"mobile" form:"mobile"`                 // 手机号
	Type            *uint8     `json:"type" form:"type"`                     // 类型
	Status          *uint8     `json:"status" form:"status"`                 // 状态
	BizType         *string    `json:"bizType" form:"bizType"`               // 业务模块
	SendTimeRange   []time.Time `json:"sendTimeRange" form:"sendTimeRange[]"` // 发送时间范围
	CreatedAtRange  []time.Time `json:"createdAtRange" form:"createdAtRange[]"` // 创建时间范围
	request.PageInfo
}
