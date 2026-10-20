package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type taskReadStub struct {
	rows        []task.Task
	err, commit error
	uid, before int64
	limit       int
	events      []task.EventRecord
	eventErr    error
	through     int64
}

func (s *taskReadStub) Events(_ context.Context, uid, id, after, through int64, limit int) ([]task.EventRecord, error) {
	s.uid, s.before, s.through, s.limit = uid, after, through, limit
	return s.events, s.eventErr
}

func (s *taskReadStub) WithinTaskRead(ctx context.Context, fn func(task.ReadTx) error) error {
	d, ok := ctx.Deadline()
	if !ok || time.Until(d) > 3*time.Second {
		return points.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fn(s); err != nil {
		return err
	}
	return s.commit
}
func (s *taskReadStub) Task(_ context.Context, uid, id int64) (task.Task, error) {
	s.uid = uid
	if s.err != nil {
		return task.Task{}, s.err
	}
	if len(s.rows) == 0 {
		return task.Task{}, points.ErrNotFound
	}
	return s.rows[0], nil
}
func (s *taskReadStub) History(_ context.Context, uid, before int64, limit int) ([]task.Task, error) {
	s.uid, s.before, s.limit = uid, before, limit
	return s.rows, s.err
}
func readTask(id int64) task.Task {
	return task.Task{ID: id, UserID: 1, SubjectID: 2, SubjectType: task.Theme, AnalysisType: 1, Status: task.Queued, EventSeq: 1, Deadline: time.UnixMilli(123456789)}
}

func TestAITaskReadsProjectionAndPagination(t *testing.T) {
	st := &taskReadStub{rows: []task.Task{readTask(9), readTask(7), readTask(5)}}
	s, err := NewAITaskReadService(st)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.History(context.Background(), 1, 10, 2)
	if err != nil || len(out.Items) != 2 || !out.HasMore || out.NextBeforeID != 7 || st.uid != 1 || st.before != 10 || st.limit != 3 {
		t.Fatal(out, err)
	}
	st.rows = st.rows[2:]
	out, err = s.History(context.Background(), 1, 7, 2)
	if err != nil || len(out.Items) != 1 || out.HasMore || out.NextBeforeID != 0 {
		t.Fatal(out, err)
	}
	v, err := s.Progress(context.Background(), 1, 5)
	if err != nil || v.DeadlineAt != 123456789 || v.ReportAvailable {
		t.Fatal(v, err)
	}
	st.rows[0].Status = task.Succeeded
	v, err = s.Progress(context.Background(), 1, 5)
	if err != nil || v.ReportAvailable {
		t.Fatal("仅状态不能证明报告可用", v, err)
	}
	st.rows = nil
	out, err = s.History(context.Background(), 1, 0, 20)
	if err != nil || out.Items == nil || len(out.Items) != 0 || out.HasMore {
		t.Fatal(out, err)
	}
	if _, err = s.Progress(context.Background(), 1, 5); !errors.Is(err, points.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestAITaskReadsRejectCorruptionAndUnknownCommit(t *testing.T) {
	for _, change := range []func(*task.Task){func(v *task.Task) { v.UserID = 2 }, func(v *task.Task) { v.ID = 0 }, func(v *task.Task) { v.SubjectID = 0 }, func(v *task.Task) { v.SubjectType = "stock" }, func(v *task.Task) { v.AnalysisType = 3 }, func(v *task.Task) { v.Status = 7 }, func(v *task.Task) { v.CurrentStep = -1 }, func(v *task.Task) { v.EventSeq = 0 }, func(v *task.Task) { v.Deadline = time.Time{} }, func(v *task.Task) { v.ErrorCode = "模拟供应商原文" }, func(v *task.Task) { v.Status = task.Failed }, func(v *task.Task) { v.Status = task.Cancelled }, func(v *task.Task) { v.Status = task.Expired }} {
		v := readTask(9)
		change(&v)
		if _, err := taskProgress(v, 1); !errors.Is(err, points.ErrInvariant) {
			t.Fatal(v, err)
		}
	}
	for _, v := range []task.Task{{Status: task.Failed, ErrorCode: "guard_rejected"}, {Status: task.Cancelled, ErrorCode: "cancelled"}, {Status: task.Expired, ErrorCode: "expired"}} {
		row := readTask(9)
		row.Status, row.ErrorCode = v.Status, v.ErrorCode
		if _, err := taskProgress(row, 1); err != nil {
			t.Fatal(err)
		}
	}
	fault := errors.New("测试只读事务故障")
	wrongOwner := readTask(9)
	wrongOwner.UserID = 2
	for _, st := range []*taskReadStub{{err: fault}, {rows: []task.Task{wrongOwner}}, {rows: []task.Task{readTask(9)}, commit: fault}, {rows: []task.Task{readTask(9), readTask(9)}}, {rows: []task.Task{readTask(10)}}, {rows: []task.Task{readTask(9), readTask(8), readTask(7)}}} {
		s, _ := NewAITaskReadService(st)
		out, err := s.History(context.Background(), 1, 10, 1)
		if err == nil || !reflect.DeepEqual(out, dto.AITaskHistory{}) {
			t.Fatal(out, err)
		}
	}
	st := &taskReadStub{rows: []task.Task{readTask(9)}, commit: fault}
	s, _ := NewAITaskReadService(st)
	if out, err := s.Progress(context.Background(), 1, 9); !errors.Is(err, fault) || out != (dto.AITaskProgress{}) {
		t.Fatal(out, err)
	}
	st.commit = nil
	if _, err := s.Progress(context.Background(), 1, 8); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	st.err = fault
	if _, err := s.Progress(context.Background(), 1, 9); !errors.Is(err, fault) {
		t.Fatal(err)
	}
	for _, args := range [][3]int64{{0, 0, 20}, {1, -1, 20}, {1, 0, 0}, {1, 0, 101}} {
		if _, err := s.History(context.Background(), args[0], args[1], int(args[2])); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, args := range [][2]int64{{0, 9}, {1, 0}} {
		if _, err := s.Progress(context.Background(), args[0], args[1]); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Progress(ctx, 1, 9); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewAITaskReadService(nil); err == nil {
		t.Fatal("空仓储")
	}
	var absent *AITaskReadService
	if _, err := absent.Progress(ctx, 1, 1); err == nil {
		t.Fatal("空服务")
	}
	if _, err := absent.History(ctx, 1, 0, 20); err == nil {
		t.Fatal("空服务")
	}
}
