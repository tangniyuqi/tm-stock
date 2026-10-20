package task

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestUnitsBoundsAndRequests(t *testing.T) {
	for _, u := range []Units{{1, 1, 0}, {0, 0, 1}, {2, 10, 3}, {math.MaxInt64, math.MaxInt64, math.MaxInt64}} {
		if err := u.Validate(); err != nil {
			t.Fatal(u, err)
		}
	}
	for _, u := range []Units{{}, {-1, 0, 1}, {0, -1, 1}, {0, 0, -1}, {0, 1, 1}, {2, 1, 0}} {
		if err := u.Validate(); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(u, err)
		}
	}
	if n, err := (Units{1, 2, 3}).Add(Units{2, 3, 4}); err != nil || n != (Units{3, 5, 7}) {
		t.Fatal(n, err)
	}
	for _, u := range []Units{{math.MaxInt64, 0, 0}, {0, math.MaxInt64, 0}, {0, 0, math.MaxInt64}} {
		if _, err := u.Add(Units{1, 1, 1}); !errors.Is(err, points.ErrOverflow) {
			t.Fatal(err)
		}
	}
	if _, err := (Units{-1, 0, 0}).Add(Units{}); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	if _, err := (Units{}).Add(Units{0, 0, -1}); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	r := ReserveRequest{UserID: 1, TaskID: 2, Fence: 1, Owner: "worker_test_00001", Key: "operation_test_00001", InputDigest: strings.Repeat("a", 64), Units: Units{1, 100, 0}}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ReserveRequest){func(r *ReserveRequest) { r.UserID = 0 }, func(r *ReserveRequest) { r.TaskID = 0 }, func(r *ReserveRequest) { r.Fence = 0 }, func(r *ReserveRequest) { r.Owner = "bad" }, func(r *ReserveRequest) { r.Key = "bad" }, func(r *ReserveRequest) { r.InputDigest = "bad" }, func(r *ReserveRequest) { r.Units = Units{} }} {
		c := r
		change(&c)
		if err := c.Validate(); err == nil {
			t.Fatal(c)
		}
	}
}

func TestCheckBudgetRejectsCorruption(t *testing.T) {
	v := Task{ID: 2, UserID: 1, Fence: 1, Budget: Budget{MaxAttempts: 1, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 1000}}
	row := Reservation{TaskID: 2, UserID: 1, Fence: 1, Seq: 1, Owner: "worker_test_00001", Key: "operation_test_00001", InputDigest: strings.Repeat("a", 64), Units: Units{1, 100, 0}, CreatedAt: time.Now()}
	s := BudgetState{TaskID: 2, UserID: 1, Limits: v.Budget.Limits(), Reserved: row.Units, Count: 1}
	if err := CheckBudget(v, s, []Reservation{row}); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Reservation){func(r *Reservation) { r.TaskID = 3 }, func(r *Reservation) { r.UserID = 3 }, func(r *Reservation) { r.Seq = 2 }, func(r *Reservation) { r.Fence = 0 }, func(r *Reservation) { r.Fence = 2 }, func(r *Reservation) { r.Owner = "bad" }, func(r *Reservation) { r.Key = "bad" }, func(r *Reservation) { r.InputDigest = "bad" }, func(r *Reservation) { r.Units = Units{} }, func(r *Reservation) { r.CreatedAt = time.Time{} }} {
		r := row
		change(&r)
		if err := CheckBudget(v, s, []Reservation{r}); err == nil {
			t.Fatal(r)
		}
	}
	for _, change := range []func(*BudgetState){func(s *BudgetState) { s.TaskID = 3 }, func(s *BudgetState) { s.UserID = 3 }, func(s *BudgetState) { s.Limits.Calls++ }, func(s *BudgetState) { s.Reserved.Tokens++ }, func(s *BudgetState) { s.Reserved.Calls = 5 }, func(s *BudgetState) { s.Count = -1 }, func(s *BudgetState) { s.Count = 1001 }, func(s *BudgetState) { s.Count = 2 }} {
		c := s
		change(&c)
		if err := CheckBudget(v, c, []Reservation{row}); err == nil {
			t.Fatal(c)
		}
	}
	v.Budget.MaxTokens = 0
	if err := CheckBudget(v, s, []Reservation{row}); err == nil {
		t.Fatal("坏预算放行")
	}
	v.Budget.MaxTokens = math.MaxInt64
	v.Budget.MaxCalls = math.MaxInt64
	s.Limits = v.Budget.Limits()
	s.Count = 2
	s.Reserved = Units{1, 100, 0}
	r2 := row
	r2.Seq = 2
	r2.Key = "operation_test_00002"
	r2.Units = Units{1, math.MaxInt64, 0}
	if err := CheckBudget(v, s, []Reservation{row, r2}); !errors.Is(err, points.ErrOverflow) {
		t.Fatal(err)
	}
	r2 = row
	r2.Seq = 2
	if err := CheckBudget(v, s, []Reservation{row, r2}); err == nil {
		t.Fatal("重复键放行")
	}
}
