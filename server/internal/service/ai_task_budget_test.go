package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type budgetScript struct {
	*scriptedTaskStore
	state               task.BudgetState
	rows                []task.Reservation
	exists, skipCounter bool
	budgetCommitError   error
}

func (s *budgetScript) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	if err := s.WithinTaskTx(ctx, func(task.Tx) error { return fn(s) }); err != nil {
		return err
	}
	return s.budgetCommitError
}
func (s *budgetScript) LockBudget(ctx context.Context, _, _ int64) (task.BudgetState, error) {
	if err := s.p.step(ctx); err != nil {
		return task.BudgetState{}, err
	}
	if !s.exists {
		return task.BudgetState{}, points.ErrNotFound
	}
	return s.state, nil
}
func (s *budgetScript) InitBudget(ctx context.Context, v task.BudgetState) (bool, error) {
	if err := s.p.step(ctx); err != nil {
		return false, err
	}
	if s.exists {
		return false, nil
	}
	s.state = v
	s.exists = true
	return true, nil
}
func (s *budgetScript) Reservations(ctx context.Context, _, _ int64) ([]task.Reservation, error) {
	return s.rows, s.p.step(ctx)
}
func (s *budgetScript) InsertReservation(ctx context.Context, v task.Reservation) error {
	if err := s.p.step(ctx); err != nil {
		return err
	}
	s.rows = append(s.rows, v)
	if !s.skipCounter {
		u, err := s.state.Reserved.Add(v.Units)
		if err != nil {
			return err
		}
		s.state.Reserved = u
		s.state.Count++
	}
	return nil
}
func budgetFixture(t *testing.T) (*AITaskBudgetService, *budgetScript, task.ReserveRequest) {
	t.Helper()
	_, old := existingTask(t)
	old.v.Status = task.Running
	old.v.Fence = 1
	old.v.Attempt = 1
	old.v.LeaseOwner = "worker_test_00001"
	until := old.p.now.Add(time.Minute)
	old.v.LeaseUntil = &until
	st := &budgetScript{scriptedTaskStore: old}
	s, err := NewAITaskBudgetService(st)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, task.ReserveRequest{UserID: 1, TaskID: 40, Fence: 1, Owner: old.v.LeaseOwner, Key: "operation_test_00001", InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}}
}

func TestAITaskBudgetReplayTakeoverAndLimits(t *testing.T) {
	s, st, r := budgetFixture(t)
	first, err := s.Reserve(context.Background(), r)
	if err != nil || !first.NewlyReserved || st.state.Count != 1 {
		t.Fatal(first, err)
	}
	again, err := s.Reserve(context.Background(), r)
	if err != nil || again.NewlyReserved || again.Reservation != first.Reservation || len(st.rows) != 1 {
		t.Fatal(again, err)
	}
	r.InputDigest = strings.Repeat("b", 64)
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
	r.InputDigest = strings.Repeat("a", 64)
	r.Units.Tokens++
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
	r.Units.Tokens--
	st.v.Fence = 2
	st.v.LeaseOwner = "worker_test_00002"
	r.Fence = 2
	r.Owner = st.v.LeaseOwner
	again, err = s.Reserve(context.Background(), r)
	if err != nil || again.NewlyReserved || again.Reservation.Fence != 1 {
		t.Fatal(again, err)
	}
	r.Key = "operation_test_00002"
	r.Units = task.Units{Pages: 1}
	if got, err := s.Reserve(context.Background(), r); err != nil || !got.NewlyReserved {
		t.Fatal(got, err)
	}
	for _, u := range []task.Units{{Calls: 4, Tokens: 100}, {Calls: 1, Tokens: 1000}, {Pages: 3}} {
		r.Key = "operation_test_limit"
		r.Units = u
		if _, err := s.Reserve(context.Background(), r); !errors.Is(err, task.ErrBudget) {
			t.Fatal(u, err)
		}
	}
}

func TestAITaskBudgetSingleOperationScope(t *testing.T) {
	s, st, r := budgetFixture(t)
	if _, err := s.ReserveSingleOperation(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ReserveSingleOperation(context.Background(), r); err != nil || got.NewlyReserved {
		t.Fatal(got, err)
	}
	r.Key = "test_different_operation"
	if _, err := s.ReserveSingleOperation(context.Background(), r); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
	// 普通多阶段预留保持原语义；单阶段入口随后必须拒绝这个混合任务。
	if _, err := s.Reserve(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReserveSingleOperation(context.Background(), r); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
	s, st, r = budgetFixture(t)
	st.v.CurrentStep = 1
	if _, err := s.ReserveSingleOperation(context.Background(), r); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
}

func TestAITaskBudgetStorageFaultsAndUnsafeLease(t *testing.T) {
	s, st, r := budgetFixture(t)
	if _, err := s.Reserve(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	calls := st.p.calls
	for n := 1; n <= calls; n++ {
		s, st, r := budgetFixture(t)
		st.p.failAt = n
		out, err := s.Reserve(context.Background(), r)
		if err == nil || !reflect.DeepEqual(out, task.ReservationReceipt{}) {
			t.Fatal(n, out, err)
		}
	}
	s, st, r = budgetFixture(t)
	st.commitError = errPointsInjected
	if out, err := s.Reserve(context.Background(), r); !errors.Is(err, errPointsInjected) || out != (task.ReservationReceipt{}) {
		t.Fatal(out, err)
	}
	st.commitError = nil
	if out, err := s.Reserve(context.Background(), r); err != nil || out.NewlyReserved {
		t.Fatal("未知提交重放不能新预留", out, err)
	}
	st.p.failAt = st.p.calls + 5
	if out, err := s.Reserve(context.Background(), r); !errors.Is(err, errPointsInjected) || out != (task.ReservationReceipt{}) {
		t.Fatal("已有预留读取故障未关闭", out, err)
	}
	s, st, r = budgetFixture(t)
	st.v.Budget.MaxTokens = 0
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, points.ErrInvariant) {
		t.Fatal("损坏任务预算未拒绝", err)
	}
	for _, change := range []func(*budgetScript){func(s *budgetScript) { s.v.CancelRequested = true }, func(s *budgetScript) { s.v.Status = task.Failed }, func(s *budgetScript) { s.v.LeaseOwner = "worker_test_other" }, func(s *budgetScript) { s.v.Fence++ }, func(s *budgetScript) { s.v.LeaseUntil = &s.p.now }, func(s *budgetScript) { s.v.Deadline = s.p.now }, func(s *budgetScript) { s.skipCounter = true }, func(s *budgetScript) { s.exists = true; s.state.Count = 1 }} {
		s, st, r := budgetFixture(t)
		change(st)
		if out, err := s.Reserve(context.Background(), r); err == nil || out != (task.ReservationReceipt{}) {
			t.Fatal(out, err)
		}
	}
	s, st, r = budgetFixture(t)
	r.UserID = 2
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, points.ErrNotFound) {
		t.Fatal(err)
	}
	r.UserID = 0
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.UserID = 1
	if _, err := s.Reserve(ctx, r); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewAITaskBudgetService(nil); err == nil {
		t.Fatal("空仓储")
	}
	var absent *AITaskBudgetService
	if _, err := absent.Reserve(ctx, r); err == nil {
		t.Fatal("空服务")
	}
}

func TestAITaskBudgetConservativeCapacityAndOverflow(t *testing.T) {
	s, st, r := budgetFixture(t)
	st.v.Budget.MaxCalls = 1001
	st.v.Budget.MaxTokens = 1001
	st.exists = true
	st.state = task.BudgetState{TaskID: st.v.ID, UserID: st.v.UserID, Limits: st.v.Budget.Limits(), Reserved: task.Units{Calls: 1000, Tokens: 1000}, Count: 1000}
	for i := 1; i <= 1000; i++ {
		st.rows = append(st.rows, task.Reservation{TaskID: st.v.ID, UserID: st.v.UserID, Fence: 1, Seq: i, Owner: r.Owner, Key: fmt.Sprintf("budget_full_%06d", i), InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 1}, CreatedAt: st.p.now})
	}
	r.Units = task.Units{Calls: 1, Tokens: 1}
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, task.ErrBudget) {
		t.Fatal("明细数量上限未生效", err)
	}
	s, st, r = budgetFixture(t)
	if _, err := s.Reserve(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	st.v.Budget.MaxTokens = math.MaxInt64
	st.state.Limits.Tokens = math.MaxInt64
	r.Key = "budget_overflow_request"
	r.Units.Tokens = math.MaxInt64
	if _, err := s.Reserve(context.Background(), r); !errors.Is(err, points.ErrOverflow) {
		t.Fatal(err)
	}
}
