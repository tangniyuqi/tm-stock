package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// QuantAiTask AI执行任务 结构体
// 记录 AI 智能选股 / AI 更新 / AI 分析等异步任务的执行状态与进度，供前端执行进度页面轮询展示
type QuantAiTask struct {
	global.GVA_MODEL_ADDON
	Type        string     `json:"type" gorm:"comment:任务类型;column:type;size:32;index"`       // 任务类型: ai_add=AI智能选股 ai_update_one=AI单只更新 ai_update_batch=题材股票批量更新 ai_analyze=AI股票分析
	Name        string     `json:"name" gorm:"comment:任务名称;column:name;size:128"`            // 任务名称（如"AI智能选股：机器人概念"）
	Status      int        `json:"status" gorm:"comment:任务状态;column:status;size:8;index"`    // 任务状态: 0=运行中 1=成功 2=失败 3=已取消 4=待调度
	Total       int        `json:"total" gorm:"comment:总进度;column:total;size:8"`             // 总进度（AI选股为预估数，实际以完成时为准）
	Done        int        `json:"done" gorm:"comment:已完成进度;column:done;size:8"`             // 已完成进度
	Count       int        `json:"count" gorm:"comment:子任务数量;column:count;size:8"`           // 子任务数量（该任务包含的子任务/对象个数，如批量更新的股票数）
	Current     string     `json:"current" gorm:"comment:当前处理对象;column:current;size:255"`    // 当前处理对象描述（如股票名称）
	Logs        string     `json:"logs" gorm:"comment:执行日志;column:logs;type:longtext"`       // 执行日志（按时间顺序追加，换行分隔）
	Params      string     `json:"params" gorm:"comment:任务参数摘要;column:params;type:longtext"` // 任务参数摘要（JSON，不含密钥等敏感信息）
	Result      string     `json:"result" gorm:"comment:执行结果;column:result;type:longtext"`   // 执行结果（JSON，任务成功后写入）
	Error       string     `json:"error" gorm:"comment:失败原因;column:error;type:text"`         // 失败原因（任务失败时写入）
	ScheduledAt *time.Time `json:"scheduled_at" gorm:"comment:计划执行时间;column:scheduled_at"`   // 计划执行时间（定时任务在到达该时间前处于待调度状态）
	FinishedAt  *time.Time `json:"finished_at" gorm:"comment:完成时间;column:finished_at"`       // 完成时间（成功/失败/取消时写入）
}

// TableName QuantAiTask 自定义表名 addon_quant_ai_task
func (QuantAiTask) TableName() string {
	return "addon_quant_ai_task"
}
