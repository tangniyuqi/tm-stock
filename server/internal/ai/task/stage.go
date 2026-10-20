package task

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const MaxSteps = 64
const MaxStageResultBytes = 256 * 1024

var (
	ErrStageRejected = fmt.Errorf("阶段结果未通过内部校验")
	ErrStagePolicy   = fmt.Errorf("阶段校验策略不匹配")
)

func ValidStage(stage string) bool {
	switch stage {
	case "collect", "extract", "judge", "agent_call":
		return true
	}
	return false
}

type StageRequest struct {
	UserID, TaskID, Fence                   int64
	Owner, OperationKey, Stage, InputDigest string
	StepNo                                  int
}

func (r StageRequest) Validate() error {
	if r.UserID <= 0 || r.TaskID <= 0 || r.Fence <= 0 || !ValidOwner(r.Owner) || !points.ValidKey(r.OperationKey) || !ValidStage(r.Stage) || !digestPattern.MatchString(r.InputDigest) || r.StepNo < 1 || r.StepNo > MaxSteps {
		return points.ErrInvalid
	}
	return nil
}
func StageDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// CheckStageJSON 只检查隔离响应的结构，不认定事实、授权或合规通过。
func CheckStageJSON(b []byte) error {
	if len(b) < 2 || len(b) > MaxStageResultBytes || !utf8.Valid(b) {
		return points.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return points.ErrInvalid
	}
	var object func(int) error
	var value func(int) error
	object = func(depth int) error {
		if depth > 32 {
			return points.ErrInvalid
		}
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return points.ErrInvalid
			}
			name, ok := k.(string)
			if !ok || seen[strings.ToLower(name)] {
				return points.ErrInvalid
			}
			seen[strings.ToLower(name)] = true
			if err := value(depth + 1); err != nil {
				return err
			}
		}
		if t, err := d.Token(); err != nil || t != json.Delim('}') {
			return points.ErrInvalid
		}
		return nil
	}
	value = func(depth int) error {
		if depth > 32 {
			return points.ErrInvalid
		}
		v, err := d.Token()
		if err != nil {
			return points.ErrInvalid
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				return object(depth)
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
				if t, err := d.Token(); err != nil || t != json.Delim(']') {
					return points.ErrInvalid
				}
			default:
				return points.ErrInvalid
			}
		}
		return nil
	}
	if err := object(1); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return points.ErrInvalid
	}
	return nil
}

type StageResult struct {
	TaskID, UserID, Fence                   int64
	ReservationSeq, StepNo                  int
	Stage, Owner, InputDigest, OutputDigest string
	Payload                                 []byte `json:"-"`
	CreatedAt                               time.Time
}

func (r StageResult) String() string {
	return fmt.Sprintf("隔离阶段结果(task=%d,step=%d,正文已隐藏)", r.TaskID, r.StepNo)
}
func (r StageResult) GoString() string { return r.String() }

type CaptureReceipt struct {
	TaskID                 int64
	ReservationSeq, StepNo int
	OutputDigest           string
	NewlyCaptured          bool
}

func (r StageResult) Receipt(fresh bool) CaptureReceipt {
	return CaptureReceipt{r.TaskID, r.ReservationSeq, r.StepNo, r.OutputDigest, fresh}
}

type StageCheckpoint struct {
	TaskID, UserID, Fence                                 int64
	StepNo, ReservationSeq                                int
	Stage, Owner, InputDigest, OutputDigest, PolicyDigest string
	EventSeq                                              int64
	CreatedAt                                             time.Time
}
type CompletedStage struct {
	Checkpoint     StageCheckpoint
	NewlyCompleted bool
}
type LoadedStage struct {
	Checkpoint StageCheckpoint
	Result     StageResult
}
type StageValidationInput struct{ Stage, InputDigest, Payload string }
type StagePolicy struct {
	Digest string
	Check  func(context.Context, StageValidationInput) error
}

func (p StagePolicy) Valid() bool { return digestPattern.MatchString(p.Digest) && p.Check != nil }

func CheckStageResult(r StageRequest, res Reservation, v StageResult) error {
	if v.TaskID != r.TaskID || v.UserID != r.UserID || v.ReservationSeq != res.Seq || v.StepNo != r.StepNo || v.Stage != r.Stage || v.InputDigest != r.InputDigest || v.Owner != res.Owner || v.Fence != res.Fence || v.CreatedAt.IsZero() || CheckStageJSON(v.Payload) != nil || v.OutputDigest != StageDigest(v.Payload) {
		return points.ErrInvariant
	}
	return nil
}
func StageUnitsMatch(stage string, u Units) bool {
	if stage == "collect" {
		return u.Pages > 0
	}
	return u.Calls > 0 && u.Tokens >= u.Calls
}

type StageStore interface {
	WithinStageTx(context.Context, func(StageTx) error) error
}
type StageTx interface {
	BudgetTx
	InsertStageResult(context.Context, StageResult) (bool, error)
	StageResult(context.Context, int64, int64, int) (StageResult, error)
	InsertStageCheckpoint(context.Context, StageCheckpoint) error
	StageCheckpoint(context.Context, int64, int64, int) (StageCheckpoint, error)
	StageEvent(context.Context, int64, int64) (Event, error)
}

// ExecutionReadTx 只给终结前核查使用，不要求调用者取得阶段写权限。
type ExecutionReadTx interface {
	Tx
	LockBudget(context.Context, int64, int64) (BudgetState, error)
	Reservations(context.Context, int64, int64) ([]Reservation, error)
	StageResult(context.Context, int64, int64, int) (StageResult, error)
}
