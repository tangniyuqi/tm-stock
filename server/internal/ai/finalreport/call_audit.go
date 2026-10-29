package finalreport

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// CallAudit 只存身份与内容摘要，不复制提示、资料、模型原文或错误消息。
type CallAudit struct {
	Request                                                                             task.StageRequest
	Phase, TaskNo, AdmissionDigest, ProfileDigest, RequestDigest, ResultDigest, Outcome string
	CreatedAt                                                                           time.Time
}

func (CallAudit) String() string     { return "报告调用审计（身份及摘要已隐藏）" }
func (a CallAudit) GoString() string { return a.String() }
func (a CallAudit) Validate() error {
	if a.Request.Validate() != nil || !taskNoPattern.MatchString(a.TaskNo) || !ValidDigest(a.AdmissionDigest) || !ValidDigest(a.ProfileDigest) || !ValidDigest(a.RequestDigest) || a.CreatedAt.UnixMilli() <= 0 || a.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 {
		return points.ErrInvariant
	}
	r := a.Request
	if !(r.StepNo == 2 && r.Stage == "extract" || (r.StepNo == 3 || r.StepNo == 4) && r.Stage == "judge") {
		return points.ErrInvariant
	}
	if a.Phase == "start" && a.Outcome == "started" && a.ResultDigest == "" || a.Phase == "finish" && (a.Outcome == "returned" || a.Outcome == "errored") && ValidDigest(a.ResultDigest) {
		return nil
	}
	return points.ErrInvariant
}

func ReportRequestDigest(r task.StageRequest, taskNo, model string, req provider.Request) (string, error) {
	if r.Validate() != nil || !taskNoPattern.MatchString(taskNo) || model == "" || req.Model != model || req.TaskID != taskNo || req.Step != r.StepNo || req.Attempt != 1 || req.MaxTokens <= 0 || req.Format != provider.FormatJSONSchema || !req.Strict || req.Temperature == nil || *req.Temperature != 0 || req.Seed != nil || req.AttemptTimeout != 0 || len(req.System)+len(req.Prompt)+len(req.Schema) > 12<<20 {
		return "", points.ErrInvalid
	}
	b, err := json.Marshal([]any{"report-call-request-v1", r, req})
	if err != nil {
		return "", points.ErrInvalid
	}
	return task.StageDigest(b), nil
}

func ReportResultDigest(out provider.Response, failed bool) (string, error) {
	// 返回是否报错是封闭状态；不记录可能带端点、凭据或正文的error.Error()。
	b, err := json.Marshal([]any{"report-call-result-v1", out, failed})
	if err != nil {
		return "", points.ErrInvalid
	}
	return task.StageDigest(b), nil
}

type CallAuditTx interface {
	task.StageTx
	task.AdmissionReadTx
	ReadCallAudit(context.Context, int64, int64, int, string) (CallAudit, error)
	InsertCallAudit(context.Context, CallAudit) error
}
type CallAuditStore interface {
	WithinCallAuditTx(context.Context, func(CallAuditTx) error) error
}
