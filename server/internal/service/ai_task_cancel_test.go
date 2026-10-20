package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type memberCancelFunc func(context.Context, int64, int64) (task.Task, error)

func (f memberCancelFunc) Cancel(ctx context.Context, uid, id int64) (task.Task, error) {
	return f(ctx, uid, id)
}

func TestAITaskCancelServiceCurrentStateAndReplay(t *testing.T) {
	core, st := existingTask(t)
	s, err := NewAITaskCancelService(core)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Cancel(context.Background(), 1, 40)
	if err != nil || out.Status != task.Cancelled || out.ErrorCode != "cancelled" || out.ReportAvailable || st.p.lastAccount.UserID != 1 || st.p.lastAccount.Frozen != 0 || st.p.lastAccount.Avail != 20 {
		t.Fatal(out, err)
	}
	replayed, err := s.Cancel(context.Background(), 1, 40)
	if err != nil || replayed != out || len(st.events) != 1 {
		t.Fatal(replayed, err)
	}
	core, st = existingTask(t)
	st.v.Status = task.Running
	s, _ = NewAITaskCancelService(core)
	out, err = s.Cancel(context.Background(), 1, 40)
	if err != nil || out.Status != task.Running || !out.CancelRequested || st.p.account.Frozen != 7 || out.EventSeq != 2 {
		t.Fatal("运行取消不应承诺已释放", out, err)
	}
}

func TestAITaskCancelServiceFailureAndDeadline(t *testing.T) {
	_, st := existingTask(t)
	base := st.v
	base.Status, base.ErrorCode = task.Cancelled, "cancelled"
	for _, terminal := range []struct {
		status int
		code   string
	}{{task.Succeeded, ""}, {task.Failed, "guard_rejected"}, {task.Expired, "expired"}} {
		v := base
		v.Status, v.ErrorCode = terminal.status, terminal.code
		s, _ := NewAITaskCancelService(memberCancelFunc(func(context.Context, int64, int64) (task.Task, error) { return v, nil }))
		if out, err := s.Cancel(context.Background(), 1, 40); err != nil || out.Status != terminal.status || out.ErrorCode != terminal.code || out.ReportAvailable {
			t.Fatal("既有终态被改写", out, err)
		}
	}
	for _, mutate := range []func(*task.Task){
		func(v *task.Task) { v.ID++ }, func(v *task.Task) { v.UserID++ },
		func(v *task.Task) { v.Status, v.ErrorCode = task.Queued, "" },
		func(v *task.Task) { v.Status, v.ErrorCode, v.CancelRequested = task.Running, "", false },
		func(v *task.Task) { v.SubjectID = 0 }, func(v *task.Task) { v.ErrorCode = "fake-sensitive-marker" },
	} {
		v := base
		mutate(&v)
		s, _ := NewAITaskCancelService(memberCancelFunc(func(context.Context, int64, int64) (task.Task, error) { return v, nil }))
		if out, err := s.Cancel(context.Background(), 1, 40); !errors.Is(err, points.ErrInvariant) || out != (dto.AITaskCancellation{}) {
			t.Fatal(out, err)
		}
	}
	for _, failure := range []error{points.ErrNotFound, errPointsInjected, context.DeadlineExceeded} {
		s, _ := NewAITaskCancelService(memberCancelFunc(func(ctx context.Context, uid, id int64) (task.Task, error) {
			deadline, ok := ctx.Deadline()
			if uid != 1 || id != 40 || !ok || time.Until(deadline) > 10*time.Second {
				t.Fatal("身份或时限丢失")
			}
			return base, failure
		}))
		if out, err := s.Cancel(context.Background(), 1, 40); !errors.Is(err, failure) || out != (dto.AITaskCancellation{}) {
			t.Fatal(out, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s, _ := NewAITaskCancelService(memberCancelFunc(func(context.Context, int64, int64) (task.Task, error) { cancel(); return base, nil }))
	if out, err := s.Cancel(ctx, 1, 40); !errors.Is(err, context.Canceled) || out != (dto.AITaskCancellation{}) {
		t.Fatal("取消父上下文仍报成功", out, err)
	}
	short, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	parentDeadline, _ := short.Deadline()
	s, _ = NewAITaskCancelService(memberCancelFunc(func(ctx context.Context, _, _ int64) (task.Task, error) {
		deadline, _ := ctx.Deadline()
		if !deadline.Equal(parentDeadline) {
			t.Fatal("扩大父时限")
		}
		return base, nil
	}))
	if _, err := s.Cancel(short, 1, 40); err != nil {
		t.Fatal(err)
	}
	core, fake := existingTask(t)
	fake.commitError = errPointsInjected
	s, _ = NewAITaskCancelService(core)
	if out, err := s.Cancel(context.Background(), 1, 40); err == nil || out != (dto.AITaskCancellation{}) {
		t.Fatal("提交未知返回成功", out, err)
	} // 脚本不模拟回滚；实际已提交但回执未知另有真库验证。
	if _, err := NewAITaskCancelService(nil); err == nil {
		t.Fatal("空取消依赖")
	}
	for _, invalid := range []*AITaskCancelService{nil, {}} {
		if _, err := invalid.Cancel(context.Background(), 1, 40); err == nil {
			t.Fatal("空服务")
		}
	}
	if _, err := s.Cancel(context.Background(), 0, 40); err == nil {
		t.Fatal("无效身份")
	}
}
