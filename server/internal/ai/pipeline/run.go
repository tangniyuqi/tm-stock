package pipeline

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrDisabled         = errors.New("内部抽取未启用")
	ErrRunNotFound      = errors.New("抽取记录不存在")
	ErrInvalidRun       = errors.New("抽取请求无效")
	ErrRunConflict      = errors.New("抽取请求编号已绑定其他载荷或身份")
	ErrRunBusy          = errors.New("抽取正在执行或结果待核对")
	ErrAuditUnavailable = errors.New("模型调用审计不可用")
)

type RunRequest struct {
	SnapshotID int64  `json:"snapshot_id"`
	RequestID  string `json:"request_id"`
}

func (r RunRequest) Validate() error {
	if r.SnapshotID <= 0 || !ValidRequestID(r.RequestID) {
		return ErrInvalidRun
	}
	return nil
}

func ValidRequestID(value string) bool {
	if len(value) < 16 || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", c) {
			return false
		}
	}
	return true
}

// RunPolicy 固化每次调用的模型与书面批准引用，不含密钥或可由请求覆盖的参数。
type RunPolicy struct{ Provider, Model, ApprovalRef string }

type RunRecord struct {
	ID         int64
	Request    RunRequest
	ActorID    int64
	Policy     RunPolicy
	State      string
	Result     ExtractionResult
	ErrorCode  string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

// CallEvent 在调用前和调用后分别追加，不能修改。未有 finish 的 start 表示结果不确定。
type CallEvent struct {
	RunID   int64
	Attempt int
	Phase   string
	Payload []byte
}

type RunStore interface {
	BeginRun(context.Context, RunRequest, int64, RunPolicy) (RunRecord, bool, error)
	RunByRequest(context.Context, string) (RunRecord, error)
	FinishRun(context.Context, int64, ExtractionResult, string) (RunRecord, error)
	AppendCallEvent(context.Context, CallEvent) error
}

type runKey struct{}

func WithRunID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, runKey{}, id)
}
func RunID(ctx context.Context) int64 { id, _ := ctx.Value(runKey{}).(int64); return id }
