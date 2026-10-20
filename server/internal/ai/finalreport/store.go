package finalreport

import (
	"context"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var ErrDisabled = fmt.Errorf("报告终检尚未启用")
var ErrUnavailable = fmt.Errorf("报告暂不可读取")

type Request struct {
	Stage  task.StageRequest
	TaskNo string
}

// Commit 是不可变终检提交，失败时 Payload 必须为空；不向公开 DTO 暴露候选正文。
type Commit struct {
	TaskID, UserID, Fence, EventSeq, TerminalTxnID                 int64
	StepNo, Status                                                 int
	TaskNo, OperationKey, InputDigest, ContentDigest, PolicyDigest string
	Entailment, Semantic                                           Verdict
	Reason                                                         string
	Payload                                                        []byte `json:"-"`
	CreatedAt                                                      time.Time
}

func (c Commit) String() string {
	return fmt.Sprintf("终检提交(task=%d,status=%d,正文已隐藏)", c.TaskID, c.Status)
}
func (c Commit) GoString() string { return c.String() }

type Receipt struct {
	TaskID        int64  `json:"task_id"`
	Status        int    `json:"status"`
	TerminalTxnID int64  `json:"terminal_txn_id"`
	ReportDigest  string `json:"report_digest,omitempty"`
}

func (c Commit) Validate() error {
	if c.TaskID <= 0 || c.UserID <= 0 || c.Fence <= 0 || c.EventSeq <= 1 || c.TerminalTxnID <= 0 || c.StepNo < 1 || c.StepNo > task.MaxSteps || !taskNoPattern.MatchString(c.TaskNo) || !points.ValidKey(c.OperationKey) || !digestPattern.MatchString(c.InputDigest) || !digestPattern.MatchString(c.ContentDigest) || !digestPattern.MatchString(c.PolicyDigest) || !c.Entailment.Valid() || !c.Semantic.Valid() || c.CreatedAt.IsZero() {
		return points.ErrInvariant
	}
	if c.Status == task.Succeeded {
		if c.Reason != "" || c.Entailment != Pass || c.Semantic != Pass || task.CheckStageJSON(c.Payload) != nil || task.StageDigest(c.Payload) != c.ContentDigest {
			return points.ErrInvariant
		}
	} else if c.Status == task.Failed {
		if len(c.Payload) != 0 {
			return points.ErrInvariant
		}
		switch c.Reason {
		case "structure", "evidence":
		case "entailment":
			if c.Entailment == Pass || c.Semantic != Uncertain {
				return points.ErrInvariant
			}
		case "semantic":
			if c.Entailment != Pass || c.Semantic == Pass {
				return points.ErrInvariant
			}
		default:
			return points.ErrInvariant
		}
	} else {
		return points.ErrInvariant
	}
	return nil
}

func (c Commit) Receipt() Receipt {
	r := Receipt{TaskID: c.TaskID, Status: c.Status, TerminalTxnID: c.TerminalTxnID}
	if c.Status == task.Succeeded {
		r.ReportDigest = c.ContentDigest
	}
	return r
}

type Store interface {
	WithinReportTx(context.Context, func(Tx) error) error
}
type Tx interface {
	task.StageTx
	task.AdmissionReadTx
	ReportCommit(context.Context, int64, int64) (Commit, error)
	InsertReportCommit(context.Context, Commit) error
	ReportEvidence(context.Context, []int64, int64) ([]sources.Snapshot, string, error)
}
