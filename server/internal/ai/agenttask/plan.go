// Package agenttask 定义任务与外部 Agent 的内部接线，不授予公开执行或计费权限。
package agenttask

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// Plan 必须来自可信规划器。Symbol 不能从题材编号猜测，Units 不是供应商真实用量。
type Plan struct {
	UserID, TaskID, SubjectID, ActorID int64
	TaskNo, SubjectType, OperationKey  string
	Symbol, ProfileDigest              string
	Units                              task.Units
}

func (p Plan) Validate(v task.Task) error {
	if v.CurrentStep < 0 || v.CurrentStep > 1 {
		return points.ErrInvalid
	}
	return p.ValidateScope(v)
}

// ValidateScope 用于完整编排对已完成 Agent 步骤的身份复核，不授予再次执行权。
func (p Plan) ValidateScope(v task.Task) error {
	if v.ID <= 0 || v.UserID <= 0 || v.SubjectID <= 0 || (v.SubjectType != task.Theme && v.SubjectType != task.Topic) {
		return points.ErrInvalid
	}
	if p.UserID != v.UserID || p.TaskID != v.ID || p.TaskNo == "" || p.TaskNo != v.No || p.SubjectType != v.SubjectType || p.SubjectID != v.SubjectID || p.ActorID <= 0 || !points.ValidKey(p.OperationKey) || !(agents.Request{RequestID: "validate_plan", Symbol: p.Symbol}).Valid() || !agents.ValidDigest(p.ProfileDigest) || p.Units.Validate() != nil || p.Units.Calls < 1 || v.Budget.Validate() != nil || !p.Units.Fits(v.Budget.Limits()) || v.CurrentStep < 0 || v.CurrentStep > task.MaxSteps {
		return points.ErrInvalid
	}
	if p.OperationKey != p.RequestID() {
		return points.ErrInvalid
	}
	return nil
}

func (p Plan) InputDigest(v task.Task) string {
	// 固定字段编码；改主体、操作人、模型配置或资源上界均不能复用旧预留。
	b, _ := json.Marshal(struct {
		Version string
		Plan    Plan
		Type    int
	}{"task_agent_input_v1", p, v.AnalysisType})
	return task.StageDigest(b)
}

func (p Plan) RequestID() string {
	// 不包含可变 symbol/profile/actor；变更计划必须冲突，不能换远端请求规避原预留。
	b, _ := json.Marshal([]any{"task_agent_request_v1", p.UserID, p.TaskID, p.TaskNo, "agent_call:1"})
	return task.StageDigest(b)
}

// Planner 同时负责当前主体↔股票的可信映射、来源/许可与远端硬预算准入。
// 当前 V2 API 没有这些证明，因此产品尚无可放行的 Planner；测试规划器不能被当成真实授权。
type Planner interface {
	Plan(context.Context, task.Task) (Plan, error)
	Recheck(context.Context, task.Task, Plan) error
}

type Options struct {
	Enabled      bool
	Lease        task.RecoveryOptions
	PollInterval time.Duration
}

func (o Options) Validate() error {
	if o.Lease.Validate() != nil || o.PollInterval < 50*time.Millisecond || o.PollInterval > 5*time.Second {
		return points.ErrInvalid
	}
	return nil
}

// Receipt 仅证明隔离结果身份，不含正文，也不认定事实或报告通过。
type Receipt struct {
	RequestID, RemoteID, ProfileDigest, ResultDigest string
}

func ReceiptPolicy(p Plan, input string, r agents.Record) ([]byte, task.StagePolicy, error) {
	if !r.Valid() || r.State != agents.Quarantined || r.Request.RequestID != p.RequestID() || r.Request.Symbol != p.Symbol || r.ActorID != p.ActorID || r.ProfileDigest != p.ProfileDigest || !agents.ValidDigest(input) {
		return nil, task.StagePolicy{}, agents.ErrConflict
	}
	payload, _ := json.Marshal(Receipt{r.Request.RequestID, r.RemoteID, r.ProfileDigest, r.ResultDigest})
	policyBytes, _ := json.Marshal([]string{"task_agent_receipt_policy_v1", input, string(payload)})
	policy := task.StagePolicy{Digest: task.StageDigest(policyBytes), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if in.Stage != "agent_call" || in.InputDigest != input || in.Payload != string(payload) {
			return task.ErrStageRejected
		}
		return nil
	}}
	return payload, policy, nil
}

// TerminalReceiptPolicy 只确认原请求已持久记录的上游失败/取消，不生成研究结果。
// 使用独立版本和结构，保留已有成功隔离回执的编码/策略摘要不变。
func TerminalReceiptPolicy(p Plan, input string, r agents.Record) ([]byte, task.StagePolicy, error) {
	if !r.Valid() || r.State != agents.Failed && r.State != agents.Cancelled || r.Request.RequestID != p.RequestID() || r.Request.Symbol != p.Symbol || r.ActorID != p.ActorID || r.ProfileDigest != p.ProfileDigest || !agents.ValidDigest(input) {
		return nil, task.StagePolicy{}, agents.ErrConflict
	}
	payload, _ := json.Marshal(struct {
		RequestID, RemoteID, ProfileDigest, State string
	}{r.Request.RequestID, r.RemoteID, r.ProfileDigest, r.State})
	policyBytes, _ := json.Marshal([]string{"task_agent_terminal_receipt_policy_v1", input, string(payload)})
	policy := task.StagePolicy{Digest: task.StageDigest(policyBytes), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if in.Stage != "agent_call" || in.InputDigest != input || in.Payload != string(payload) {
			return task.ErrStageRejected
		}
		return nil
	}}
	return payload, policy, nil
}
