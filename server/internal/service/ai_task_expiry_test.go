package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type expiryStub struct {
	rangeHint task.ExpiryRange
	rows      []task.ExpiryCandidate
	pageFn    func(int64, int64, int) ([]task.ExpiryCandidate, error)
	finishFn  func(int64, int64) (task.Task, error)
	rangeErr  error
	calls     []int64
	deadline  time.Time
}

func (s *expiryStub) ExpiryRange(ctx context.Context) (task.ExpiryRange, error) {
	s.deadline, _ = ctx.Deadline()
	return s.rangeHint, s.rangeErr
}
func (s *expiryStub) ExpiryPage(_ context.Context, after, upper int64, limit int) ([]task.ExpiryCandidate, error) {
	if s.pageFn != nil {
		return s.pageFn(after, upper, limit)
	}
	var out []task.ExpiryCandidate
	for _, v := range s.rows {
		if v.ID > after && v.ID <= upper && len(out) < limit {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *expiryStub) Expire(_ context.Context, uid, id int64) (task.Task, error) {
	s.calls = append(s.calls, id)
	if s.finishFn != nil {
		return s.finishFn(uid, id)
	}
	return task.Task{ID: id, UserID: uid, Status: task.Expired}, nil
}
func expiryFixture(t *testing.T) (*AITaskExpiryService, *expiryStub) {
	t.Helper()
	at := time.Date(2026, 10, 16, 0, 0, 0, 123000000, time.UTC)
	s := &expiryStub{rangeHint: task.ExpiryRange{UpperID: 9, AsOf: at}, rows: []task.ExpiryCandidate{
		{ID: 1, UserID: 1, Status: task.Queued, Deadline: at},
		{ID: 3, UserID: 2, Status: task.Running, Deadline: at.Add(-time.Hour)},
		{ID: 5, UserID: 1, Status: task.Cancelled, Deadline: at},
		{ID: 7, UserID: 2, Status: task.Queued, Deadline: at.Add(time.Millisecond)},
		{ID: 9, UserID: 2, Status: task.Running, Deadline: at},
	}}
	v, err := NewAITaskExpiryService(s, s)
	if err != nil {
		t.Fatal(err)
	}
	return v, s
}

func TestAITaskExpiryPaginationAndResume(t *testing.T) {
	s, st := expiryFixture(t)
	opts := task.ExpiryOptions{PageSize: 2, MaxTasks: 3}
	a, err := s.Sweep(context.Background(), opts)
	if err != nil || a.Complete || a.LastID != 5 || a.Visited != 3 || a.ExpiredObserved != 2 || a.Skipped != 1 || !reflect.DeepEqual(st.calls, []int64{1, 3}) || time.Until(st.deadline) > 5*time.Minute {
		t.Fatal(a, err, st.calls)
	}
	opts.AfterID = a.LastID
	b, err := s.Sweep(context.Background(), opts)
	if err != nil || !b.Complete || b.LastID != 9 || b.Visited != 2 || b.ExpiredObserved != 1 || b.Skipped != 1 {
		t.Fatal(b, err)
	}
	opts.AfterID = b.LastID
	c, err := s.Sweep(context.Background(), opts)
	if err != nil || !c.Complete || c.Visited != 0 {
		t.Fatal(c, err)
	}
	st.rangeHint.UpperID, st.rows = 0, nil
	opts.AfterID = 0
	if v, err := s.Sweep(context.Background(), opts); err != nil || !v.Complete || v.Visited != 0 {
		t.Fatal(v, err)
	}
}

func TestAITaskExpiryFailClosed(t *testing.T) {
	fault := errors.New("fake expiry failure")
	for _, kind := range []string{"range_error", "range_upper", "range_time", "range_precision", "page_error", "empty", "too_many", "duplicate", "backwards", "outside", "user", "status", "deadline", "precision", "finish_error", "receipt_id", "receipt_user", "receipt_status", "second_page"} {
		t.Run(kind, func(t *testing.T) {
			s, st := expiryFixture(t)
			switch kind {
			case "range_error":
				st.rangeErr = fault
			case "range_upper":
				st.rangeHint.UpperID = -1
			case "range_time":
				st.rangeHint.AsOf = time.Time{}
			case "range_precision":
				st.rangeHint.AsOf = st.rangeHint.AsOf.Add(time.Nanosecond)
			case "page_error":
				st.pageFn = func(int64, int64, int) ([]task.ExpiryCandidate, error) { return nil, fault }
			case "empty":
				st.rows = nil
			case "too_many":
				st.pageFn = func(int64, int64, int) ([]task.ExpiryCandidate, error) { return st.rows, nil }
			case "duplicate":
				st.rows[1].ID = 1
			case "backwards":
				st.rows[0].ID = 0
				st.pageFn = func(int64, int64, int) ([]task.ExpiryCandidate, error) { return st.rows[:2], nil }
			case "outside":
				st.rows[1].ID = 10
				st.pageFn = func(int64, int64, int) ([]task.ExpiryCandidate, error) { return st.rows[:2], nil }
			case "user":
				st.rows[1].UserID = 0
			case "status":
				st.rows[1].Status = 99
			case "deadline":
				st.rows[1].Deadline = time.Time{}
			case "precision":
				st.rows[1].Deadline = st.rows[1].Deadline.Add(time.Nanosecond)
			case "finish_error":
				st.finishFn = func(int64, int64) (task.Task, error) { return task.Task{ID: 1, UserID: 1, Status: task.Expired}, fault }
			case "receipt_id", "receipt_user", "receipt_status":
				st.finishFn = func(uid, id int64) (task.Task, error) {
					v := task.Task{ID: id, UserID: uid, Status: task.Expired}
					if kind == "receipt_id" {
						v.ID++
					}
					if kind == "receipt_user" {
						v.UserID++
					}
					if kind == "receipt_status" {
						v.Status = task.Running
					}
					return v, nil
				}
			case "second_page":
				st.pageFn = func(after, _ int64, _ int) ([]task.ExpiryCandidate, error) {
					if after == 0 {
						return st.rows[:2], nil
					}
					return nil, fault
				}
			}
			out, err := s.Sweep(context.Background(), task.ExpiryOptions{PageSize: 2, MaxTasks: 9})
			if err == nil || out.Complete {
				t.Fatal(out, err)
			}
			if kind == "second_page" {
				if out.LastID != 3 || out.Visited != 2 || out.ExpiredObserved != 2 {
					t.Fatal(out)
				}
			} else if out.Visited != 0 || len(st.calls) > 1 {
				t.Fatal("坏页未整体拒绝或未知提交越过游标", out, st.calls)
			}
		})
	}
}

func TestAITaskExpiryRacesAndContext(t *testing.T) {
	for _, code := range []int{task.Expired, task.Cancelled, task.Failed, task.Succeeded, 0} {
		s, st := expiryFixture(t)
		st.finishFn = func(uid, id int64) (task.Task, error) {
			if code == 0 {
				return task.Task{}, task.ErrNotReady
			}
			return task.Task{ID: id, UserID: uid, Status: code}, nil
		}
		out, err := s.Sweep(context.Background(), task.ExpiryOptions{PageSize: 2, MaxTasks: 9})
		want := 0
		if code == task.Expired {
			want = 3
		}
		if err != nil || !out.Complete || out.ExpiredObserved != want || out.Skipped != 5-want {
			t.Fatal(out, err)
		}
	}
	s, st := expiryFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.Sweep(ctx, task.ExpiryOptions{PageSize: 2, MaxTasks: 1}); err != nil || time.Until(st.deadline) > time.Second {
		t.Fatal(err)
	}
	for _, when := range []string{"before", "page", "finish", "last"} {
		t.Run(when, func(t *testing.T) {
			s, st := expiryFixture(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			if when == "before" {
				stop()
			}
			if when == "page" {
				st.pageFn = func(int64, int64, int) ([]task.ExpiryCandidate, error) { stop(); return st.rows[:2], nil }
			}
			if when == "finish" || when == "last" {
				st.finishFn = func(uid, id int64) (task.Task, error) {
					stop()
					return task.Task{ID: id, UserID: uid, Status: task.Expired}, nil
				}
			}
			max := 9
			if when == "last" {
				max = 1
			}
			out, err := s.Sweep(ctx, task.ExpiryOptions{PageSize: 2, MaxTasks: max})
			if !errors.Is(err, context.Canceled) || out.Complete {
				t.Fatal(out, err)
			}
		})
	}
	for _, s := range []*AITaskExpiryService{nil, {}} {
		if _, err := s.Sweep(context.Background(), task.ExpiryOptions{PageSize: 1, MaxTasks: 1}); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := s.Sweep(context.Background(), task.ExpiryOptions{}); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := NewAITaskExpiryService(nil, st); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := NewAITaskExpiryService(st, nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
}
