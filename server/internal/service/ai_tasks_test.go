package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 复用逐访问边界故障脚本；它不模拟 SQL 回滚或隔离，相关证据由真库用例提供。
type scriptedTaskStore struct {
	p           *scriptedPointsTx
	v           task.Task
	exists      bool
	events      []task.Event
	commitError error
}

func (s *scriptedTaskStore) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 30*time.Second {
		return points.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fn(s); err != nil {
		return err
	}
	return s.commitError
}
func (s *scriptedTaskStore) Points() points.Tx { return s.p }

// 该基础夹具没有任何执行预算或响应；实际阶段夹具覆写这些读取方法。
func (s *scriptedTaskStore) LockBudget(ctx context.Context, _, _ int64) (task.BudgetState, error) {
	if err := s.p.step(ctx); err != nil {
		return task.BudgetState{}, err
	}
	return task.BudgetState{}, points.ErrNotFound
}
func (s *scriptedTaskStore) Reservations(ctx context.Context, _, _ int64) ([]task.Reservation, error) {
	return nil, s.p.step(ctx)
}
func (s *scriptedTaskStore) StageResult(ctx context.Context, _, _ int64, _ int) (task.StageResult, error) {
	if err := s.p.step(ctx); err != nil {
		return task.StageResult{}, err
	}
	return task.StageResult{}, points.ErrNotFound
}
func (s *scriptedTaskStore) FindByKey(ctx context.Context, uid int64, key string) (task.Task, error) {
	if err := s.p.step(ctx); err != nil {
		return task.Task{}, err
	}
	if !s.exists || s.v.UserID != uid || s.v.Key != key {
		return task.Task{}, points.ErrNotFound
	}
	return s.v, nil
}
func (s *scriptedTaskStore) LockTask(ctx context.Context, uid, id int64) (task.Task, error) {
	if err := s.p.step(ctx); err != nil {
		return task.Task{}, err
	}
	if s.v.UserID != uid || s.v.ID != id {
		return task.Task{}, points.ErrNotFound
	}
	return s.v, nil
}
func (s *scriptedTaskStore) InsertTask(ctx context.Context, v task.Task) (int64, error) {
	if err := s.p.step(ctx); err != nil {
		return 0, err
	}
	v.ID = 40
	s.v = v
	s.exists = true
	return v.ID, nil
}
func (s *scriptedTaskStore) SetTask(ctx context.Context, old, next task.Task) error {
	if err := s.p.step(ctx); err != nil {
		return err
	}
	if !reflect.DeepEqual(old, s.v) {
		return points.ErrInvariant
	}
	s.v = next
	return nil
}
func (s *scriptedTaskStore) InsertEvent(ctx context.Context, e task.Event) error {
	if err := s.p.step(ctx); err != nil {
		return err
	}
	s.events = append(s.events, e)
	return nil
}

func taskFixture(t *testing.T) (*AITaskService, *scriptedTaskStore, task.CreateRequest, points.Price, task.Budget) {
	t.Helper()
	ledger, ps, _, _, _ := scriptedPoints(t)
	st := &scriptedTaskStore{p: ps.tx}
	s, err := NewAITaskService(st, ledger)
	if err != nil {
		t.Fatal(err)
	}
	s.entropy = strings.NewReader(strings.Repeat("a", 16))
	return s, st, task.CreateRequest{UserID: 1, Key: "test_task_create_0001", AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 2, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 3600000}
}
func existingTask(t *testing.T) (*AITaskService, *scriptedTaskStore) {
	t.Helper()
	s, st, r, p, b := taskFixture(t)
	hash, err := points.RequestHash("ai_task_create", r)
	if err != nil {
		t.Fatal(err)
	}
	st.exists = true
	st.v = task.Task{ID: 40, UserID: 1, No: strings.Repeat("a", 32), Key: r.Key, Hash: hash, AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, HoldID: 30, PriceVersion: p.Version, PricePoints: p.Points, Budget: b, Status: task.Queued, NotBefore: st.p.now, Deadline: st.p.now.Add(time.Hour), EventSeq: 1}
	st.p.account.Avail = 13
	st.p.account.Frozen = 7
	st.p.lot.Frozen = 7
	return s, st
}

func TestAITaskEveryStorageBoundaryAndCommitFailure(t *testing.T) {
	for _, kind := range []string{"create", "claim", "heartbeat", "cancel_queued", "cancel_running", "retry", "fail", "expire"} {
		prepare := func() (*scriptedTaskStore, func() error) {
			ctx := context.Background()
			if kind == "create" {
				s, st, r, p, b := taskFixture(t)
				return st, func() error {
					v, err := s.Create(ctx, r, p, b)
					if err != nil && v != (task.Created{}) {
						t.Fatal("失败返回成功收据")
					}
					return err
				}
			}
			s, st := existingTask(t)
			if kind == "heartbeat" || kind == "cancel_running" || kind == "retry" || kind == "fail" {
				st.v.Status = task.Running
				st.v.LeaseOwner = "worker_test_00001"
				st.v.Fence = 1
				st.v.Attempt = 1
				until := st.p.now.Add(30 * time.Second)
				st.v.LeaseUntil = &until
			}
			if kind == "expire" {
				st.p.now = st.v.Deadline
			}
			return st, func() error {
				var v task.Task
				var err error
				switch kind {
				case "claim":
					v, err = s.Claim(ctx, 1, 40, "worker_test_00001", time.Minute)
				case "heartbeat":
					v, err = s.Heartbeat(ctx, 1, 40, "worker_test_00001", 1, time.Minute)
				case "cancel_queued", "cancel_running":
					v, err = s.Cancel(ctx, 1, 40)
				case "retry":
					v, err = s.FailOrRetry(ctx, 1, 40, "worker_test_00001", 1, "provider_unavailable", time.Second)
				case "fail":
					v, err = s.FailOrRetry(ctx, 1, 40, "worker_test_00001", 1, "guard_rejected", 0)
				case "expire":
					v, err = s.Expire(ctx, 1, 40)
				}
				if err != nil && !reflect.DeepEqual(v, task.Task{}) {
					t.Fatal("失败返回成功任务")
				}
				return err
			}
		}
		base, run := prepare()
		if err := run(); err != nil {
			t.Fatal(kind, err)
		}
		for fail := 1; fail <= base.p.calls; fail++ {
			st, run := prepare()
			st.p.failAt = fail
			if err := run(); !errors.Is(err, errPointsInjected) {
				t.Fatalf("%s 第 %d 个访问故障被吞：%v", kind, fail, err)
			}
		}
		st, run := prepare()
		st.commitError = errPointsInjected
		if err := run(); !errors.Is(err, errPointsInjected) {
			t.Fatal("提交未知结果误报成功", kind, err)
		}
	}
}

func TestAITaskCreateReplayAndGuards(t *testing.T) {
	ctx := context.Background()
	s, st, r, p, b := taskFixture(t)
	created, err := s.Create(ctx, r, p, b)
	if err != nil || created.TaskID != 40 || created.HoldID != 30 || created.PricePoints != 7 || len(st.events) != 1 {
		t.Fatal(created, err)
	}
	st.v.Status = task.Failed
	again, err := s.Create(ctx, r, points.Price{}, task.Budget{})
	if err != nil || again != created {
		t.Fatal("配置变化后重放不稳定", again, err)
	}
	r.SubjectID++
	if _, err := s.Create(ctx, r, p, b); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*AITaskService, *scriptedTaskStore, *task.CreateRequest, *points.Price, *task.Budget)
		want   error
	}{
		{"请求无效", func(_ *AITaskService, _ *scriptedTaskStore, r *task.CreateRequest, _ *points.Price, _ *task.Budget) {
			r.UserID = 0
		}, points.ErrInvalid},
		{"报价无效", func(_ *AITaskService, _ *scriptedTaskStore, _ *task.CreateRequest, p *points.Price, _ *task.Budget) {
			p.Points = 0
		}, points.ErrInvalid},
		{"价格变化", func(_ *AITaskService, _ *scriptedTaskStore, _ *task.CreateRequest, p *points.Price, _ *task.Budget) {
			p.Version = "v2"
		}, points.ErrPriceChanged},
		{"预算无效", func(_ *AITaskService, _ *scriptedTaskStore, _ *task.CreateRequest, _ *points.Price, b *task.Budget) {
			b.MaxCalls = 0
		}, points.ErrInvalid},
		{"账户缺失", func(_ *AITaskService, st *scriptedTaskStore, _ *task.CreateRequest, _ *points.Price, _ *task.Budget) {
			st.p.account.UserID = 2
		}, points.ErrInsufficient},
		{"熵源故障", func(s *AITaskService, _ *scriptedTaskStore, _ *task.CreateRequest, _ *points.Price, _ *task.Budget) {
			s.entropy = strings.NewReader("")
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, r, p, b := taskFixture(t)
			tc.change(s, st, &r, &p, &b)
			v, err := s.Create(ctx, r, p, b)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) || v != (task.Created{}) || len(st.events) != 0 {
				t.Fatal(v, err)
			}
		})
	}
	if _, err := NewAITaskService(nil, s.ledger); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := NewAITaskService(st, nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	var absent *AITaskService
	if _, err := absent.Create(ctx, r, p, b); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Cancel(canceled, 1, 40); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAITaskLeaseRetryCancelAndExpiry(t *testing.T) {
	ctx := context.Background()
	owner := "worker_test_00001"
	s, st := existingTask(t)
	v, err := s.Claim(ctx, 1, 40, owner, time.Minute)
	if err != nil || v.Status != task.Running || v.Fence != 1 || v.Attempt != 1 || v.EventSeq != 2 {
		t.Fatal(v, err)
	}
	if _, err := s.Claim(ctx, 1, 40, "worker_test_00002", time.Minute); !errors.Is(err, task.ErrNotReady) {
		t.Fatal(err)
	}
	if _, err := s.Heartbeat(ctx, 1, 40, "worker_test_00002", 1, time.Minute); !errors.Is(err, task.ErrLease) {
		t.Fatal(err)
	}
	if _, err := s.Heartbeat(ctx, 1, 40, owner, 2, time.Minute); !errors.Is(err, task.ErrLease) {
		t.Fatal(err)
	}
	if _, err := s.Heartbeat(ctx, 1, 40, owner, 1, time.Minute); err != nil {
		t.Fatal("同毫秒心跳无变化不应失败", err)
	}
	st.p.now = st.p.now.Add(5 * time.Second)
	v, err = s.Heartbeat(ctx, 1, 40, owner, 1, time.Minute)
	if err != nil || v.EventSeq != 2 {
		t.Fatal(v, err)
	}
	v, err = s.FailOrRetry(ctx, 1, 40, owner, 1, "provider_unavailable", time.Second)
	if err != nil || v.Status != task.Queued || v.LeaseUntil != nil || v.Fence != 1 {
		t.Fatal(v, err)
	}
	if _, err := s.Claim(ctx, 1, 40, owner, time.Minute); !errors.Is(err, task.ErrNotReady) {
		t.Fatal(err)
	}
	st.p.now = st.p.now.Add(2 * time.Second)
	v, err = s.Claim(ctx, 1, 40, owner, time.Minute)
	if err != nil || v.Fence != 2 || v.Attempt != 2 {
		t.Fatal(v, err)
	}
	if _, err := s.FailOrRetry(ctx, 1, 40, owner, 1, "internal_failure", 0); !errors.Is(err, task.ErrLease) {
		t.Fatal("旧围栏写入被放行", err)
	}
	v, err = s.Cancel(ctx, 1, 40)
	if err != nil || !v.CancelRequested || v.Status != task.Running {
		t.Fatal(v, err)
	}
	if _, err := s.Heartbeat(ctx, 1, 40, owner, 2, time.Minute); !errors.Is(err, task.ErrCancelled) {
		t.Fatal(err)
	}
	v, err = s.FailOrRetry(ctx, 1, 40, owner, 2, "provider_unavailable", time.Second)
	if err != nil || v.Status != task.Cancelled || st.p.lastAccount.Frozen != 0 || st.p.lastAccount.Avail != 20 {
		t.Fatal(v, err)
	}
	before := st.p.calls
	if again, err := s.Cancel(ctx, 1, 40); err != nil || !reflect.DeepEqual(again, v) || st.p.calls != before+2 {
		t.Fatal("重复取消再次写账", err)
	}
	if _, err := s.Claim(ctx, 1, 40, owner, time.Minute); !errors.Is(err, points.ErrClosed) {
		t.Fatal(err)
	}
	s, st = existingTask(t)
	if _, err := s.Expire(ctx, 1, 40); !errors.Is(err, task.ErrNotReady) {
		t.Fatal(err)
	}
	st.p.now = st.v.Deadline
	v, err = s.Claim(ctx, 1, 40, owner, time.Minute)
	if err != nil || v.Status != task.Expired {
		t.Fatal(v, err)
	}
	if again, err := s.Expire(ctx, 1, 40); err != nil || again.Status != task.Expired {
		t.Fatal(again, err)
	}
}

func TestAITaskTakeoverCapsAndBoundaries(t *testing.T) {
	ctx := context.Background()
	owner := "worker_test_00001"
	for _, mode := range []string{"takeover", "cancel_takeover", "attempts_exhausted", "fence_overflow", "event_overflow", "wrong_state", "lease_clamped", "retry_exhausted", "retry_past_deadline"} {
		t.Run(mode, func(t *testing.T) {
			s, st := existingTask(t)
			switch mode {
			case "takeover", "cancel_takeover":
				st.v.Status = task.Running
				st.v.Fence = 1
				st.v.Attempt = 1
				st.v.LeaseOwner = "worker_test_old01"
				until := st.p.now
				st.v.LeaseUntil = &until
				st.v.CancelRequested = mode == "cancel_takeover"
			case "attempts_exhausted":
				st.v.Attempt = 2
			case "fence_overflow":
				st.v.Fence = math.MaxInt64
			case "event_overflow":
				st.v.EventSeq = math.MaxInt64
			case "wrong_state":
				st.v.Status = 0
			case "lease_clamped":
				st.v.Deadline = st.p.now.Add(time.Second)
			}
			v, err := s.Claim(ctx, 1, 40, owner, time.Minute)
			if mode == "fence_overflow" || mode == "event_overflow" {
				if !errors.Is(err, points.ErrOverflow) {
					t.Fatal(err)
				}
				return
			}
			if mode == "wrong_state" {
				if !errors.Is(err, points.ErrInvariant) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "takeover":
				if v.Fence != 2 || v.Attempt != 2 {
					t.Fatal(v)
				}
			case "cancel_takeover":
				if v.Status != task.Cancelled {
					t.Fatal(v)
				}
			case "attempts_exhausted":
				if v.Status != task.Failed || v.ErrorCode != "attempts_exhausted" {
					t.Fatal(v)
				}
			case "lease_clamped":
				if !v.LeaseUntil.Equal(v.Deadline) {
					t.Fatal(v)
				}
			case "retry_exhausted", "retry_past_deadline":
				if mode == "retry_exhausted" {
					st.v.Attempt = 2
				}
				backoff := time.Second
				if mode == "retry_past_deadline" {
					backoff = time.Hour
				}
				v, err = s.FailOrRetry(ctx, 1, 40, owner, 1, "invalid_output", backoff)
				if err != nil || v.Status != task.Failed {
					t.Fatal(v, err)
				}
			}
		})
	}
	s, _ := existingTask(t)
	for _, fn := range []func() error{
		func() error { _, err := s.Claim(ctx, 1, 40, "bad", time.Minute); return err },
		func() error { _, err := s.Claim(ctx, 1, 40, owner, 0); return err },
		func() error { _, err := s.Heartbeat(ctx, 1, 40, owner, 1, 0); return err },
		func() error { _, err := s.FailOrRetry(ctx, 1, 40, owner, 1, "供应商敏感原文", 0); return err },
		func() error {
			_, err := s.FailOrRetry(ctx, 1, 40, owner, 1, "invalid_output", -time.Second)
			return err
		},
		func() error { _, err := s.Cancel(ctx, 0, 40); return err },
	} {
		if err := fn(); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := s.Cancel(ctx, 2, 40); !errors.Is(err, points.ErrNotFound) {
		t.Fatal("越权任务可读写", err)
	}
}
