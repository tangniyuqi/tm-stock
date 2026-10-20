package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func eventReadFixture(t *testing.T) (*AITaskReadService, *taskReadStub) {
	t.Helper()
	v := readTask(9)
	v.EventSeq = 3
	rows := []task.EventRecord{
		{Event: task.Event{TaskID: 9, Seq: 1, Type: "created", Status: task.Queued}, CreatedAt: time.UnixMilli(1000)},
		{Event: task.Event{TaskID: 9, Seq: 2, Type: "claimed", Status: task.Running}, CreatedAt: time.UnixMilli(999)},
		{Event: task.Event{TaskID: 9, Seq: 3, Type: "retried", Status: task.Queued}, CreatedAt: time.UnixMilli(2000)},
	}
	st := &taskReadStub{rows: []task.Task{v}, events: rows}
	svc, err := NewAITaskReadService(st)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func TestAITaskEventsPaginationAndBounds(t *testing.T) {
	s, st := eventReadFixture(t)
	out, err := s.Events(context.Background(), 1, 9, 0, 2)
	if err != nil || len(out.Items) != 2 || out.NextAfterSeq != 2 || !out.HasMore || out.UpperSeq != 3 || st.uid != 1 || st.before != 0 || st.through != 3 || st.limit != 3 || out.Items[1].At != 999 {
		t.Fatal(out, err)
	}
	st.events = st.events[2:]
	out, err = s.Events(context.Background(), 1, 9, 2, 2)
	if err != nil || len(out.Items) != 1 || out.NextAfterSeq != 3 || out.HasMore {
		t.Fatal(out, err)
	}
	st.events = nil
	out, err = s.Events(context.Background(), 1, 9, 3, 20)
	if err != nil || out.Items == nil || len(out.Items) != 0 || out.NextAfterSeq != 3 || out.HasMore {
		t.Fatal(out, err)
	}
	st.rows[0].EventSeq = math.MaxInt64
	st.rows[0].Status = task.Cancelled
	st.rows[0].ErrorCode = "cancelled"
	st.events = []task.EventRecord{{Event: task.Event{TaskID: 9, Seq: math.MaxInt64, Type: "cancelled", Status: task.Cancelled, ErrorCode: "cancelled"}, CreatedAt: time.UnixMilli(2000)}}
	out, err = s.Events(context.Background(), 1, 9, math.MaxInt64-1, 100)
	if err != nil || out.NextAfterSeq != math.MaxInt64 || out.HasMore {
		t.Fatal("游标整数上界溢出", out, err)
	}
	st.events = nil
	if _, err := s.Events(context.Background(), 1, 9, math.MaxInt64, 100); err != nil {
		t.Fatal(err)
	}
}

func TestAITaskEventProjectionClosedStates(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		status, step int
		code         string
	}{
		{"created", task.Queued, 0, ""}, {"retried", task.Queued, 0, ""}, {"claimed", task.Running, 0, ""}, {"cancel_requested", task.Running, 0, ""}, {"step_completed", task.Running, 1, ""}, {"succeeded", task.Succeeded, 1, ""}, {"failed", task.Failed, 1, "guard_rejected"}, {"cancelled", task.Cancelled, 1, "cancelled"}, {"expired", task.Expired, 1, "expired"},
	} {
		v := readTask(9)
		v.EventSeq, v.CurrentStep, v.Status, v.ErrorCode = 3, tc.step, tc.status, tc.code
		seq := int64(3)
		if tc.kind == "created" {
			seq = 1
			v.EventSeq = 1
		}
		row := task.EventRecord{Event: task.Event{TaskID: 9, Seq: seq, Type: tc.kind, Status: tc.status, Step: tc.step, ErrorCode: tc.code}, CreatedAt: time.UnixMilli(1000)}
		if _, err := taskEventProjection(row, v); err != nil {
			t.Fatal(tc.kind, err)
		}
	}
	v := readTask(9)
	v.EventSeq, v.Status = 3, task.Running
	base := task.EventRecord{Event: task.Event{TaskID: 9, Seq: 3, Type: "claimed", Status: task.Running}, CreatedAt: time.UnixMilli(1000)}
	for _, mutate := range []func(*task.EventRecord){
		func(r *task.EventRecord) { r.TaskID++ }, func(r *task.EventRecord) { r.Seq = 0 }, func(r *task.EventRecord) { r.Seq = 4 }, func(r *task.EventRecord) { r.Step = -1 }, func(r *task.EventRecord) { r.Step = task.MaxSteps + 1 }, func(r *task.EventRecord) { r.Step = 1 }, func(r *task.EventRecord) { r.CreatedAt = time.Time{} }, func(r *task.EventRecord) { r.CreatedAt = time.UnixMilli(0) }, func(r *task.EventRecord) { r.CreatedAt = r.CreatedAt.Add(time.Nanosecond) }, func(r *task.EventRecord) { r.Type = "fake-sensitive-marker" }, func(r *task.EventRecord) { r.Type = "created" }, func(r *task.EventRecord) { r.Type = "step_completed" }, func(r *task.EventRecord) { r.Seq = 1 }, func(r *task.EventRecord) { r.Status = task.Queued }, func(r *task.EventRecord) { r.ErrorCode = "fake-sensitive-marker" }, func(r *task.EventRecord) {
			r.Type = "cancelled"
			r.Status = task.Cancelled
			r.ErrorCode = "cancelled"
			r.Seq = 2
		}, func(r *task.EventRecord) { r.Type = "retried"; r.Status = task.Queued },
	} {
		row := base
		mutate(&row)
		if out, err := taskEventProjection(row, v); !errors.Is(err, points.ErrInvariant) || out != (dto.AITaskEvent{}) {
			t.Fatal(out, err)
		}
	}
	badCreated := base
	badCreated.Type, badCreated.Status, badCreated.Seq, badCreated.Step = "created", task.Queued, 1, 1
	v.CurrentStep = 1
	if _, err := taskEventProjection(badCreated, v); err == nil {
		t.Fatal("created 步骤非零")
	}
}

func TestAITaskEventsRejectGapsAndFailures(t *testing.T) {
	for _, mutate := range []func(*taskReadStub){
		func(s *taskReadStub) { s.events = s.events[1:] }, func(s *taskReadStub) { s.events = append(s.events, s.events[2]) }, func(s *taskReadStub) { s.events[1].Seq = 1 }, func(s *taskReadStub) { s.rows[0].ID = 8 }, func(s *taskReadStub) { s.rows[0].UserID = 2 }, func(s *taskReadStub) { s.rows[0].CurrentStep = task.MaxSteps + 1 }, func(s *taskReadStub) { s.events[2].Type = "fake-sensitive-marker" }, func(s *taskReadStub) { s.rows[0].CurrentStep = 1; s.events[1].Step = 1 },
	} {
		s, st := eventReadFixture(t)
		mutate(st)
		if out, err := s.Events(context.Background(), 1, 9, 0, 20); err == nil || !reflect.DeepEqual(out, dto.AITaskEventPage{}) {
			t.Fatal(out, err)
		}
	}
	fault := errors.New("fake read failure")
	for _, where := range []string{"task", "events", "commit"} {
		s, st := eventReadFixture(t)
		switch where {
		case "task":
			st.err = fault
		case "events":
			st.eventErr = fault
		case "commit":
			st.commit = fault
		}
		if out, err := s.Events(context.Background(), 1, 9, 0, 20); !errors.Is(err, fault) || !reflect.DeepEqual(out, dto.AITaskEventPage{}) {
			t.Fatal(out, err)
		}
	}
	s, _ := eventReadFixture(t)
	for _, args := range [][4]int64{{0, 9, 0, 20}, {1, 0, 0, 20}, {1, 9, -1, 20}, {1, 9, 0, 0}, {1, 9, 0, 101}, {1, 9, 4, 20}} {
		if _, err := s.Events(context.Background(), args[0], args[1], args[2], int(args[3])); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Events(ctx, 1, 9, 0, 20); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var nilSvc *AITaskReadService
	if _, err := nilSvc.Events(context.Background(), 1, 9, 0, 20); err == nil {
		t.Fatal("空服务")
	}
	if _, err := (&AITaskReadService{}).Events(context.Background(), 1, 9, 0, 20); err == nil {
		t.Fatal("空存储")
	}
}
