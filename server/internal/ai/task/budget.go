package task

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const MaxReservations = 1000

var ErrBudget = fmt.Errorf("任务执行预算不足")
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Units 是调用前的保守上界，不是供应商实际用量，更不是用户应扣积分。
type Units struct{ Calls, Tokens, Pages int64 }

func (u Units) Validate() error {
	if u.Calls < 0 || u.Tokens < 0 || u.Pages < 0 || (u.Calls == 0 && u.Pages == 0) || (u.Calls == 0 && u.Tokens != 0) || (u.Calls > 0 && u.Tokens < u.Calls) {
		return points.ErrInvalid
	}
	return nil
}
func (u Units) Add(v Units) (Units, error) {
	if u.Calls < 0 || u.Tokens < 0 || u.Pages < 0 || v.Calls < 0 || v.Tokens < 0 || v.Pages < 0 {
		return Units{}, points.ErrInvariant
	}
	if u.Calls > math.MaxInt64-v.Calls || u.Tokens > math.MaxInt64-v.Tokens || u.Pages > math.MaxInt64-v.Pages {
		return Units{}, points.ErrOverflow
	}
	return Units{u.Calls + v.Calls, u.Tokens + v.Tokens, u.Pages + v.Pages}, nil
}
func (u Units) Fits(limit Units) bool {
	return u.Calls >= 0 && u.Tokens >= 0 && u.Pages >= 0 && u.Calls <= limit.Calls && u.Tokens <= limit.Tokens && u.Pages <= limit.Pages
}
func (b Budget) Limits() Units { return Units{b.MaxCalls, b.MaxTokens, b.MaxPages} }

type ReserveRequest struct {
	UserID, TaskID, Fence   int64
	Owner, Key, InputDigest string
	Units                   Units
}

func (r ReserveRequest) Validate() error {
	if r.UserID <= 0 || r.TaskID <= 0 || r.Fence <= 0 || !ValidOwner(r.Owner) || !points.ValidKey(r.Key) || !digestPattern.MatchString(r.InputDigest) {
		return points.ErrInvalid
	}
	return r.Units.Validate()
}

type BudgetState struct {
	TaskID, UserID   int64
	Limits, Reserved Units
	Count            int
}
type Reservation struct {
	TaskID, UserID, Fence   int64
	Seq                     int
	Owner, Key, InputDigest string
	Units                   Units
	CreatedAt               time.Time
}

// ReservationReceipt 只在确认提交后返回；重放 NewlyReserved=false，不能再次执行网络请求。
type ReservationReceipt struct {
	Reservation   Reservation
	NewlyReserved bool
}

type BudgetStore interface {
	WithinBudgetTx(context.Context, func(BudgetTx) error) error
}
type BudgetTx interface {
	Tx
	LockBudget(context.Context, int64, int64) (BudgetState, error)
	InitBudget(context.Context, BudgetState) (bool, error)
	Reservations(context.Context, int64, int64) ([]Reservation, error)
	InsertReservation(context.Context, Reservation) error
}

// CheckBudget 核对计数器与不可变预留明细。当前读及任务锁由事务适配器保证。
func CheckBudget(v Task, state BudgetState, rows []Reservation) error {
	if v.Budget.Validate() != nil || state.TaskID != v.ID || state.UserID != v.UserID || state.Limits != v.Budget.Limits() || state.Count < 0 || state.Count > MaxReservations || len(rows) != state.Count || !state.Reserved.Fits(state.Limits) {
		return points.ErrInvariant
	}
	used := Units{}
	seen := make(map[string]bool, len(rows))
	for i, row := range rows {
		if row.TaskID != v.ID || row.UserID != v.UserID || row.Seq != i+1 || row.Fence <= 0 || row.Fence > v.Fence || !ValidOwner(row.Owner) || !points.ValidKey(row.Key) || seen[row.Key] || !digestPattern.MatchString(row.InputDigest) || row.Units.Validate() != nil || row.CreatedAt.IsZero() {
			return points.ErrInvariant
		}
		seen[row.Key] = true
		var err error
		used, err = used.Add(row.Units)
		if err != nil {
			return err
		}
	}
	if used != state.Reserved {
		return points.ErrInvariant
	}
	return nil
}
