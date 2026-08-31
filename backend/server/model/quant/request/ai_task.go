package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

// AiTaskSearch AI执行任务分页查询
type AiTaskSearch struct {
	Type   string `json:"type" form:"type"`     // 任务类型: ai_add / ai_update_one / ai_update_batch / ai_analyze
	Status *int   `json:"status" form:"status"` // 任务状态: 0=运行中 1=成功 2=失败 3=已取消
	request.PageInfo
}

// AiTaskActionReq AI执行任务操作请求（停止/重启）
type AiTaskActionReq struct {
	ID uint `json:"id" form:"id"` // 任务ID
}
