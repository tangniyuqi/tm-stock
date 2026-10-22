package task

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type RecoveryOptions struct{ Lease, Heartbeat, Timeout time.Duration }

// 2026-10-22 用户批准：真实多 Agent 约9分钟，单任务恢复上限由5分钟提高到30分钟。
var maxRecoveryTimeout = 30 * time.Minute

// SetMaxRecoveryTimeout 仅供启动装配/本地验收收紧或调整上限；不得超过30分钟。
func SetMaxRecoveryTimeout(d time.Duration) error {
	if d < time.Second || d > 30*time.Minute {
		return points.ErrInvalid
	}
	maxRecoveryTimeout = d
	return nil
}

func (o RecoveryOptions) Validate() error {
	if o.Lease < time.Second || o.Lease > 10*time.Minute || o.Lease%time.Millisecond != 0 || o.Heartbeat < 50*time.Millisecond || o.Heartbeat > o.Lease/3 || o.Timeout < time.Second || o.Timeout > maxRecoveryTimeout {
		return points.ErrInvalid
	}
	return nil
}

// RecoveryStep 不带外部执行器、正文或资源上界；只能恢复已有预留/响应。
type RecoveryStep struct {
	OperationKey, Stage, InputDigest string
	Policy                           StagePolicy
	Recheck                          func(context.Context) error
}
type RecoveryPlan struct {
	UserID, TaskID int64
	TaskNo         string
	Steps          []RecoveryStep
}

func (p RecoveryPlan) Validate(v Task) error {
	if p.UserID != v.UserID || p.TaskID != v.ID || p.TaskNo == "" || p.TaskNo != v.No || len(p.Steps) < 1 || len(p.Steps) > MaxSteps || v.CurrentStep < 0 || v.CurrentStep > len(p.Steps) {
		return points.ErrInvariant
	}
	keys := map[string]bool{}
	for _, s := range p.Steps {
		if !points.ValidKey(s.OperationKey) || keys[s.OperationKey] || !ValidStage(s.Stage) || !digestPattern.MatchString(s.InputDigest) || !s.Policy.Valid() || s.Recheck == nil {
			return points.ErrInvalid
		}
		keys[s.OperationKey] = true
	}
	return nil
}

type RecoveryResult struct {
	TaskID          int64  `json:"task_id"`
	TaskStatus      int    `json:"task_status"`
	CurrentStep     int    `json:"current_step"`
	PlannedSteps    int    `json:"planned_steps"`
	Outcome         string `json:"outcome"`
	ReportAvailable bool   `json:"report_available"`
}
