// Package task 定义持久化 AI 任务及只读查询契约，不执行真实模型调用。
package task

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const (
	Theme = "theme"
	Topic = "topic"

	Queued    = 1
	Running   = 2
	Succeeded = 3
	Failed    = 4
	Cancelled = 5
	Expired   = 6
)

var (
	ErrLease       = fmt.Errorf("任务租约或围栏已失效")
	ErrCancelled   = fmt.Errorf("任务已请求取消")
	ErrNotReady    = fmt.Errorf("任务尚不可抢占")
	ErrUnconfirmed = fmt.Errorf("任务仍有未确认调用，须按原键核查")
)

// Budget 由可信服务端价格簿提供，创建时固化，不接受 C 端金额或预算。
type Budget struct {
	MaxAttempts   int   `json:"max_attempts"`
	MaxCalls      int64 `json:"max_calls"`
	MaxTokens     int64 `json:"max_tokens"`
	MaxPages      int64 `json:"max_pages"`
	MaxDurationMS int64 `json:"max_duration_ms"`
}

func (b Budget) Validate() error {
	if b.MaxAttempts < 1 || b.MaxAttempts > 100 || b.MaxCalls < 1 || b.MaxTokens < 1 || b.MaxPages < 1 || b.MaxDurationMS < 1000 || b.MaxDurationMS > 86400000 {
		return points.ErrInvalid
	}
	return nil
}

type CreateRequest struct {
	UserID       int64
	Key          string
	AnalysisType int
	SubjectType  string
	SubjectID    int64
	PriceVersion string
}

func (r CreateRequest) Validate() error {
	if r.UserID <= 0 || !points.ValidKey(r.Key) || (r.AnalysisType != 1 && r.AnalysisType != 2) || (r.SubjectType != Theme && r.SubjectType != Topic) || r.SubjectID <= 0 || !identifier.MatchString(r.PriceVersion) || len(r.PriceVersion) > 32 {
		return points.ErrInvalid
	}
	return nil
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

func ValidOwner(owner string) bool { return len(owner) <= 64 && points.ValidKey(owner) }
func ValidFailure(code string) bool {
	switch code {
	case "provider_unavailable", "invalid_output", "guard_rejected", "budget_exhausted", "attempts_exhausted", "internal_failure":
		return true
	}
	return false
}

type Task struct {
	ID, UserID, HoldID           int64
	No, Key, Hash                string
	AnalysisType                 int
	SubjectType                  string
	SubjectID                    int64
	PriceVersion                 string
	PricePoints                  int64
	Budget                       Budget
	Status, Attempt, CurrentStep int
	NotBefore, Deadline          time.Time
	LeaseOwner                   string
	LeaseUntil                   *time.Time
	Fence, EventSeq              int64
	CancelRequested              bool
	ErrorCode                    string
}

func (t Task) Terminal() bool { return t.Status >= Succeeded && t.Status <= Expired }

// Created 是稳定创建收据，重放不把已变化的状态/租约伪装成首次响应。
type Created struct {
	TaskID, UserID, HoldID, PricePoints int64
	TaskNo, PriceVersion                string
}

func (t Task) Created() Created {
	return Created{t.ID, t.UserID, t.HoldID, t.PricePoints, t.No, t.PriceVersion}
}

// Event 不含提示词、供应商错误、正文或任何自由文本，只保存进度白名单。
type Event struct {
	TaskID, Seq  int64
	Type         string
	Status, Step int
	ErrorCode    string
}

type Store interface {
	WithinTaskTx(context.Context, func(Tx) error) error
}
type Tx interface {
	Points() points.Tx
	FindByKey(context.Context, int64, string) (Task, error)
	LockTask(context.Context, int64, int64) (Task, error)
	InsertTask(context.Context, Task) (int64, error)
	SetTask(context.Context, Task, Task) error
	InsertEvent(context.Context, Event) error
}
