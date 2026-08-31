package quant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/datascope"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AI 执行任务状态常量
const (
	AiTaskStatusRunning  = 0 // 运行中
	AiTaskStatusSuccess  = 1 // 成功
	AiTaskStatusFailed   = 2 // 失败
	AiTaskStatusCanceled = 3 // 已取消
	AiTaskStatusPending  = 4 // 待调度（定时任务，等待到达计划时间后自动执行）
)

// AiTaskService AI 执行任务：负责任务的创建、进度上报与查询
type AiTaskService struct{}

// sysDB 后台任务/调度器专用 DB 访问：以系统身份执行，避免"无身份上下文"告警
// （行级数据权限引擎要求 Service 透传请求 ctx；无请求链路的后台任务按约定用 WithSystem）
func (aiTaskService *AiTaskService) sysDB() *gorm.DB {
	return global.GVA_DB.WithContext(datascope.WithSystem(context.Background()))
}

// 任务取消注册表：taskID -> cancel 函数
// 后台执行任务启动时注册、goroutine 退出时注销；用户停止任务时触发 cancel，
// 可真正中断正在飞行的大模型 HTTP 请求（HTTP 请求基于该 context 创建，见 utils/request）
var (
	aiTaskCancelMu    sync.RWMutex
	aiTaskCancelFuncs = make(map[uint]context.CancelFunc)
)

// RegisterTaskCancel 注册任务取消函数（后台任务启动时调用）
func (aiTaskService *AiTaskService) RegisterTaskCancel(taskID uint, cancel context.CancelFunc) {
	aiTaskCancelMu.Lock()
	aiTaskCancelFuncs[taskID] = cancel
	aiTaskCancelMu.Unlock()
}

// UnregisterTaskCancel 注销任务取消函数（后台任务 goroutine 退出时调用，防止内存泄漏）
func (aiTaskService *AiTaskService) UnregisterTaskCancel(taskID uint) {
	aiTaskCancelMu.Lock()
	delete(aiTaskCancelFuncs, taskID)
	aiTaskCancelMu.Unlock()
}

// CancelTask 触发任务取消（用户停止任务时调用）
// 任务仍在后台执行则触发 cancel 并返回 true；未注册（已完成/未启动）为空操作
func (aiTaskService *AiTaskService) CancelTask(taskID uint) bool {
	aiTaskCancelMu.RLock()
	cancel, ok := aiTaskCancelFuncs[taskID]
	aiTaskCancelMu.RUnlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

// withAiNamePrefix 为任务名称统一添加"AI"前缀
func withAiNamePrefix(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	return "AI" + name
}

// CreateAiTask 创建 AI 执行任务，返回任务ID
// taskName 为任务名称（如"分析题材股票：机器人概念"），入库前统一添加"AI"前缀；
// subTaskCount 为该任务包含的子任务数量
// params 为脱敏后的任务参数摘要（不含 API 密钥等敏感信息），仅用于追溯展示
func (aiTaskService *AiTaskService) CreateAiTask(ctx context.Context, taskType, taskName string, params any, total, subTaskCount int, userID uint, scheduledAt ...time.Time) (uint, error) {
	paramsJSON := ""
	if params != nil {
		if b, err := json.Marshal(params); err == nil {
			paramsJSON = string(b)
		}
	}
	status := AiTaskStatusRunning
	var scheduledAtPtr *time.Time
	if len(scheduledAt) > 0 && !scheduledAt[0].IsZero() && scheduledAt[0].After(time.Now()) {
		status = AiTaskStatusPending
		scheduledAtPtr = &scheduledAt[0]
	}
	task := quant.QuantAiTask{
		Type:        taskType,
		Name:        withAiNamePrefix(taskName),
		Status:      status,
		Total:       total,
		Done:        0,
		Count:       subTaskCount,
		Params:      paramsJSON,
		ScheduledAt: scheduledAtPtr,
		GVA_MODEL_ADDON: global.GVA_MODEL_ADDON{
			CreatedBy: userID,
			UpdatedBy: userID,
		},
	}
	if err := global.GVA_DB.WithContext(ctx).Create(&task).Error; err != nil {
		global.GVA_LOG.Error("创建AI执行任务失败!", zap.Error(err))
		return 0, err
	}
	return task.ID, nil
}

// UpdateAiTaskProgress 更新任务进度并追加日志
// 任务由单个 goroutine 串行执行，日志按时间顺序追加，无并发写问题
func (aiTaskService *AiTaskService) UpdateAiTaskProgress(taskID uint, done int, current, logLine string) error {
	// 读取现有日志以便追加
	var task quant.QuantAiTask
	if err := aiTaskService.sysDB().Select("id", "logs").First(&task, taskID).Error; err != nil {
		return err
	}
	logs := task.Logs
	if logLine != "" {
		if logs != "" {
			logs += "\n"
		}
		logs += "[" + time.Now().Format("15:04:05") + "] " + logLine
	}
	return aiTaskService.sysDB().Model(&quant.QuantAiTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"done":    done,
		"current": current,
		"logs":    logs,
	}).Error
}

// UpdateAiTaskMeta 更新任务元信息（子任务数量、总进度）
// 用于 AI 选股等任务：创建时总进度/子任务数为预估，AI 返回实际结果后修正，
// 保证"子任务数 / 总进度"与实际一致（任务名称保持创建时的预估支数不变，不随结果更新）
// name 参数保留用于可选更新任务名称（非空时统一添加"AI"前缀）
func (aiTaskService *AiTaskService) UpdateAiTaskMeta(taskID uint, count, total int, name string) error {
	updates := map[string]any{}
	if count > 0 {
		updates["count"] = count
	}
	if total > 0 {
		updates["total"] = total
	}
	if trimmed := withAiNamePrefix(name); trimmed != "" {
		updates["name"] = trimmed
	}
	if len(updates) == 0 {
		return nil
	}
	return aiTaskService.sysDB().Model(&quant.QuantAiTask{}).Where("id = ?", taskID).Updates(updates).Error
}

// IsAiTaskCanceled 判断任务是否已被用户取消（后台执行 goroutine 轮询检测，提前退出）
func (aiTaskService *AiTaskService) IsAiTaskCanceled(taskID uint) bool {
	var status int
	if err := aiTaskService.sysDB().Model(&quant.QuantAiTask{}).Select("status").Where("id = ?", taskID).Scan(&status).Error; err != nil {
		return false
	}
	return status == AiTaskStatusCanceled
}

// ResetAiTask 重置任务为运行中状态（供重启任务使用），清空进度/日志/结果并置完成时间为空
func (aiTaskService *AiTaskService) ResetAiTask(taskID uint, total, count int, userID uint) error {
	return aiTaskService.sysDB().Model(&quant.QuantAiTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":      AiTaskStatusRunning,
		"done":        0,
		"total":       total,
		"count":       count,
		"current":     "",
		"logs":        "",
		"result":      "",
		"error":       "",
		"finished_at": nil,
		"updated_by":  userID,
	}).Error
}

// StopAiTask 停止运行中/待调度的AI执行任务：置为已取消并写入完成时间与取消日志
// 定时任务（待调度）也可以停止，无需触发 HTTP 取消
func (aiTaskService *AiTaskService) StopAiTask(ctx context.Context, id uint, userID uint, isAdmin bool) error {
	var task quant.QuantAiTask
	db := global.GVA_DB.WithContext(ctx).Model(&quant.QuantAiTask{})
	if !isAdmin {
		db = db.Where("created_by = ?", userID)
	}
	if err := db.Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	if task.Status != AiTaskStatusRunning && task.Status != AiTaskStatusPending {
		return fmt.Errorf("仅运行中或待调度的任务可以停止")
	}
	// 触发 context 取消：真正中断正在飞行的大模型 HTTP 请求
	// （协作式边界检查 IsAiTaskCanceled 保留为双保险，覆盖不依赖 HTTP context 的入库段）
	aiTaskService.CancelTask(id)
	logs := task.Logs
	if logs != "" {
		logs += "\n"
	}
	logs += "[" + time.Now().Format("15:04:05") + "] 任务已被用户停止"
	return global.GVA_DB.WithContext(ctx).Model(&quant.QuantAiTask{}).Where("id = ?", id).Updates(map[string]any{
		"status":      AiTaskStatusCanceled,
		"finished_at": time.Now(),
		"logs":        logs,
		"updated_by":  userID,
	}).Error
}

// RestartAiTask 重启已取消或失败的AI执行任务
// 从任务参数摘要(params)重建执行请求并重新发起后台执行，任务记录复用（不新建任务）
func (aiTaskService *AiTaskService) RestartAiTask(ctx context.Context, id uint, userID uint, isAdmin bool) error {
	var task quant.QuantAiTask
	db := global.GVA_DB.WithContext(ctx).Model(&quant.QuantAiTask{})
	if !isAdmin {
		db = db.Where("created_by = ?", userID)
	}
	if err := db.Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	if task.Status != AiTaskStatusCanceled && task.Status != AiTaskStatusFailed {
		return fmt.Errorf("仅已取消或失败的任务可以重启")
	}
	if task.Params == "" {
		return fmt.Errorf("任务参数缺失，无法重启")
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return fmt.Errorf("任务参数解析失败: %w", err)
	}
	switch task.Type {
	case "ai_add":
		return themeStockService.RestartAiAddTask(task, params, userID)
	case "ai_update_one":
		return themeStockService.RestartAiUpdateOneTask(task, params, userID)
	case "ai_update_batch":
		return themeStockService.RestartAiUpdateBatchTask(task, params, userID)
	case "ai_analyze":
		return baseStockService.RestartAiAnalyzeTask(task, params, userID)
	default:
		return fmt.Errorf("不支持的任务类型: %s", task.Type)
	}
}

// ==================== 任务参数解析辅助（params JSON → 请求字段） ====================

func taskParamsString(params map[string]any, key string) string {
	if v, ok := params[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func taskParamsInt(params map[string]any, key string) int {
	if v, ok := params[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 0
}

func taskParamsFloat(params map[string]any, key string) float64 {
	if v, ok := params[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func taskParamsBoolPtr(params map[string]any, key string) *bool {
	v, ok := params[key]
	if !ok || v == nil {
		return nil
	}
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}

func taskParamsUintSlice(params map[string]any, key string) []uint {
	arr, ok := params[key].([]any)
	if !ok {
		return nil
	}
	var ids []uint
	for _, item := range arr {
		if f, ok := item.(float64); ok {
			ids = append(ids, uint(f))
		}
	}
	return ids
}

func taskParamsInt64Slice(params map[string]any, key string) []int64 {
	arr, ok := params[key].([]any)
	if !ok {
		return nil
	}
	var ids []int64
	for _, item := range arr {
		if f, ok := item.(float64); ok {
			ids = append(ids, int64(f))
		}
	}
	return ids
}

// FinishAiTask 完成任务（成功/失败），写入结果、错误信息与完成时间
func (aiTaskService *AiTaskService) FinishAiTask(taskID uint, status int, result any, errMsg string) {
	// 任务已被用户取消时，不再覆盖取消状态（避免停止后后台 goroutine 改写为成功/失败）
	var cur quant.QuantAiTask
	if err := aiTaskService.sysDB().Select("status").First(&cur, taskID).Error; err != nil {
		return
	}
	if cur.Status == AiTaskStatusCanceled {
		return
	}
	updates := map[string]any{
		"status":      status,
		"finished_at": time.Now(),
	}
	if status == AiTaskStatusSuccess {
		// 成功时修正总进度为实际完成数（如 AI 选股返回的股票数少于预估），保证进度显示 100%
		var task quant.QuantAiTask
		if err := aiTaskService.sysDB().Select("done").First(&task, taskID).Error; err == nil {
			updates["total"] = task.Done
		}
	}
	if result != nil {
		if b, err := json.Marshal(result); err == nil {
			updates["result"] = string(b)
		}
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	if err := aiTaskService.sysDB().Model(&quant.QuantAiTask{}).Where("id = ?", taskID).Updates(updates).Error; err != nil {
		global.GVA_LOG.Error("更新AI执行任务完成状态失败!", zap.Uint("taskID", taskID), zap.Error(err))
	}
}

// GetAiTask 根据ID获取AI执行任务
func (aiTaskService *AiTaskService) GetAiTask(ctx context.Context, id string, userID uint, isAdmin bool) (task quant.QuantAiTask, err error) {
	db := global.GVA_DB.WithContext(ctx).Model(&quant.QuantAiTask{})
	if !isAdmin {
		// 非管理员只能查看自己创建的任务
		db = db.Where("created_by = ?", userID)
	}
	err = db.Where("id = ?", id).First(&task).Error
	return
}

// GetAiTaskInfoList 分页获取AI执行任务列表
func (aiTaskService *AiTaskService) GetAiTaskInfoList(ctx context.Context, info quantReq.AiTaskSearch, userID uint, isAdmin bool) (list []quant.QuantAiTask, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.WithContext(ctx).Model(&quant.QuantAiTask{})
	if !isAdmin {
		db = db.Where("created_by = ?", userID)
	}
	if info.Type != "" {
		db = db.Where("type = ?", info.Type)
	}
	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}
	err = db.Count(&total).Error
	if err != nil {
		return
	}
	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}
	// 运行中 + 待调度的任务排前，其余按创建时间倒序
	db = db.Order("CASE WHEN status IN (0, 4) THEN 0 ELSE 1 END, id DESC")
	err = db.Find(&list).Error
	return list, total, err
}

// ==================== 定时任务调度器 ====================

var (
	aiTaskSchedulerOnce sync.Once
)

// StartAiTaskScheduler 启动 AI 定时任务调度器（后台 goroutine，每 30 秒扫描一次待调度任务）
// 调用 StartAiTaskSchedulerOnce 确保只启动一个实例
func StartAiTaskScheduler() {
	aiTaskSchedulerOnce.Do(func() {
		scheduler := AiTaskService{}
		go func() {
			global.GVA_LOG.Info("AI定时任务调度器已启动（间隔30秒）")
			// 启动时立即执行一次，避免首次等待 30 秒
			scheduler.dispatchScheduledTasks()
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				scheduler.dispatchScheduledTasks()
			}
		}()
	})
}

// dispatchScheduledTasks 扫描并调度所有到期的定时任务
// 查询 status=4(待调度) 且 scheduled_at <= now 的任务，将其置为运行中并发起后台执行
func (aiTaskService *AiTaskService) dispatchScheduledTasks() {
	var tasks []quant.QuantAiTask
	now := time.Now()
	if err := aiTaskService.sysDB().Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?", AiTaskStatusPending, now).Find(&tasks).Error; err != nil {
		global.GVA_LOG.Error("查询待调度AI任务失败", zap.Error(err))
		return
	}
	for _, task := range tasks {
		// 加一行调度启动日志到任务日志（任务本身已被 ResetAiTask 重置，但不影响）
		_ = aiTaskService.UpdateAiTaskProgress(task.ID, 0, "", "定时调度启动，准备执行")
		aiTaskService.dispatchScheduledTask(task)
	}
	if len(tasks) > 0 {
		global.GVA_LOG.Info("AI定时任务调度完成", zap.Int("count", len(tasks)))
	}
}

// dispatchScheduledTask 调度单个定时任务：解析参数 -> 重置状态 -> 发起后台执行
func (aiTaskService *AiTaskService) dispatchScheduledTask(task quant.QuantAiTask) {
	var params map[string]any
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		global.GVA_LOG.Error("定时任务参数解析失败，标记为失败", zap.Uint("taskID", task.ID), zap.Error(err))
		aiTaskService.FinishAiTask(task.ID, AiTaskStatusFailed, nil, "定时任务参数解析失败: "+err.Error())
		return
	}

	var runErr error
	switch task.Type {
	case "ai_add":
		runErr = themeStockService.RestartAiAddTask(task, params, task.CreatedBy)
	case "ai_update_one":
		runErr = themeStockService.RestartAiUpdateOneTask(task, params, task.CreatedBy)
	case "ai_update_batch":
		runErr = themeStockService.RestartAiUpdateBatchTask(task, params, task.CreatedBy)
	case "ai_analyze":
		runErr = baseStockService.RestartAiAnalyzeTask(task, params, task.CreatedBy)
	default:
		runErr = fmt.Errorf("不支持的定时任务类型: %s", task.Type)
	}

	if runErr != nil {
		global.GVA_LOG.Error("定时任务启动失败", zap.Uint("taskID", task.ID), zap.Error(runErr))
		_ = aiTaskService.UpdateAiTaskProgress(task.ID, 0, "", "任务启动失败："+runErr.Error())
		aiTaskService.FinishAiTask(task.ID, AiTaskStatusFailed, nil, runErr.Error())
	}
}
