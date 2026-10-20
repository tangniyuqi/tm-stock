package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

type agentMemoryStore struct {
	mu         sync.Mutex
	records    map[string]agents.Record
	counts     map[string]int
	failSave   bool
	loseCommit bool
}
type agentMemoryTx struct {
	store   *agentMemoryStore
	records map[string]agents.Record
	counts  map[string]int
}

func agentTestStore() *agentMemoryStore {
	return &agentMemoryStore{records: map[string]agents.Record{}, counts: map[string]int{}}
}
func (s *agentMemoryStore) WithinAgentTx(ctx context.Context, fn func(agents.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := map[string]agents.Record{}
	for k, v := range s.records {
		r[k] = v
	}
	c := map[string]int{}
	for k, v := range s.counts {
		c[k] = v
	}
	if err := fn(&agentMemoryTx{store: s, records: r, counts: c}); err != nil {
		return err
	}
	s.records = r
	s.counts = c
	if s.loseCommit {
		s.loseCommit = false
		return errors.New("测试：提交回执丢失")
	}
	return nil
}
func (s *agentMemoryStore) ByRequest(_ context.Context, id string) (agents.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return r, agents.ErrNotFound
	}
	return r, nil
}
func (t *agentMemoryTx) Reserve(_ context.Context, p agents.Profile, req agents.Request, actor int64) (agents.Record, bool, error) {
	if r, ok := t.records[req.RequestID]; ok {
		if r.ActorID != actor || r.Request != req || r.ProfileDigest != p.Digest {
			return agents.Record{}, false, agents.ErrConflict
		}
		return r, false, nil
	}
	if t.counts[p.Digest] >= p.MaxSubmissions {
		return agents.Record{}, false, agents.ErrLimit
	}
	r := agents.Record{ID: int64(len(t.records) + 1), ActorID: actor, Request: req, ProfileDigest: p.Digest, State: agents.Submitting}
	t.records[req.RequestID] = r
	t.counts[p.Digest]++
	return r, true, nil
}
func (t *agentMemoryTx) Lock(_ context.Context, id int64) (agents.Record, error) {
	for _, r := range t.records {
		if r.ID == id {
			return r, nil
		}
	}
	return agents.Record{}, agents.ErrNotFound
}
func (t *agentMemoryTx) Save(_ context.Context, r agents.Record, event string, result *agents.Result) error {
	if t.store.failSave {
		return errors.New("测试：审计写失败")
	}
	if !agents.ValidChange(t.records[r.Request.RequestID], r, event, result) {
		return agents.ErrConflict
	}
	t.records[r.Request.RequestID] = r
	return nil
}

type agentExecutorStub struct {
	submits atomic.Int32
	cancels atomic.Int32
	submit  func(context.Context, string) (agents.Acceptance, error)
	status  func(context.Context, string) (agents.Observation, error)
	result  func(context.Context, string) (agents.Result, error)
	cancel  func(context.Context, string) error
}

func (e *agentExecutorStub) Submit(c context.Context, s string) (agents.Acceptance, error) {
	e.submits.Add(1)
	if e.submit != nil {
		return e.submit(c, s)
	}
	return agents.Acceptance{RemoteID: "remote-" + s}, nil
}
func (e *agentExecutorStub) Status(c context.Context, id string) (agents.Observation, error) {
	if e.status != nil {
		return e.status(c, id)
	}
	return agents.Observation{RemoteID: id, State: agents.Completed}, nil
}
func (e *agentExecutorStub) Result(c context.Context, id string) (agents.Result, error) {
	if e.result != nil {
		return e.result(c, id)
	}
	return agents.Result{RemoteID: id, Payload: []byte(`{"analysis_id":"` + id + `","reports":{"news":"测试未核验正文"}}`)}, nil
}
func (e *agentExecutorStub) Cancel(c context.Context, id string) error {
	e.cancels.Add(1)
	if e.cancel != nil {
		return e.cancel(c, id)
	}
	return nil
}
func agentService(t *testing.T, s *agentMemoryStore, e *agentExecutorStub) *AIAgentService {
	t.Helper()
	v, err := NewAIAgentService(s, e, agents.Profile{Digest: strings.Repeat("a", 64), MaxSubmissions: 2})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAgentConcurrentSubmissionAndReplay(t *testing.T) {
	s := agentTestStore()
	e := &agentExecutorStub{}
	svc := agentService(t, s, e)
	req := agents.Request{RequestID: "test_agent_001", Symbol: "600001"}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Submit(context.Background(), 7, req)
			if err != nil && !errors.Is(err, agents.ErrUncertain) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if e.submits.Load() != 1 {
		t.Fatal("同键并发重复执行")
	}
	if _, err := agentService(t, s, e).Submit(context.Background(), 7, req); err != nil || e.submits.Load() != 1 {
		t.Fatal("新服务实例重放错误")
	}
	if _, err := svc.Submit(context.Background(), 8, req); !errors.Is(err, agents.ErrConflict) {
		t.Fatal("跨身份重放放行")
	}
	req.Symbol = "600002"
	if _, err := svc.Submit(context.Background(), 7, req); !errors.Is(err, agents.ErrConflict) {
		t.Fatal("载荷变更放行")
	}
	req.RequestID = "test_agent_002"
	if _, err := svc.Submit(context.Background(), 7, req); err != nil {
		t.Fatal(err)
	}
	req.RequestID = "test_agent_003"
	if _, err := svc.Submit(context.Background(), 7, req); !errors.Is(err, agents.ErrLimit) {
		t.Fatal("超限放行")
	}
}

func TestAgentUnknownNeverResubmits(t *testing.T) {
	for _, mode := range []string{"传输未知", "空任务号", "提交后审计失败", "数据库提交回执丢失"} {
		t.Run(mode, func(t *testing.T) {
			s := agentTestStore()
			e := &agentExecutorStub{}
			svc := agentService(t, s, e)
			req := agents.Request{RequestID: "test_unknown_01", Symbol: "600001"}
			switch mode {
			case "传输未知":
				e.submit = func(context.Context, string) (agents.Acceptance, error) {
					return agents.Acceptance{}, context.DeadlineExceeded
				}
			case "空任务号":
				e.submit = func(context.Context, string) (agents.Acceptance, error) { return agents.Acceptance{}, nil }
			case "提交后审计失败":
				s.failSave = true
			case "数据库提交回执丢失":
				s.loseCommit = true
			}
			if _, err := svc.Submit(context.Background(), 7, req); err == nil {
				t.Fatal("未知状态假报成功")
			}
			s.failSave = false
			before := e.submits.Load()
			for i := 0; i < 3; i++ {
				if _, err := svc.Submit(context.Background(), 7, req); !errors.Is(err, agents.ErrUncertain) {
					t.Fatal(err)
				}
			}
			if e.submits.Load() != before || mode == "数据库提交回执丢失" && before != 0 {
				t.Fatal("重复或无登记调用")
			}
			if _, err := svc.Status(context.Background(), 7, req.RequestID); !errors.Is(err, agents.ErrUncertain) {
				t.Fatal("没有映射仍查询上游")
			}
		})
	}
}

func TestAgentLifecycleQuarantinesAndNeverBills(t *testing.T) {
	s := agentTestStore()
	e := &agentExecutorStub{}
	svc := agentService(t, s, e)
	ctx := context.Background()
	req := agents.Request{RequestID: "test_lifecycle_01", Symbol: "600001"}
	r, err := svc.Submit(ctx, 7, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Result(ctx, 7, req.RequestID); !errors.Is(err, agents.ErrConflict) {
		t.Fatal("未完成任务读取结果")
	}
	r, err = svc.Status(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Completed {
		t.Fatal(r, err)
	}
	r, err = svc.Result(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Quarantined || !agents.ValidDigest(r.ResultDigest) {
		t.Fatal(r, err)
	}
	if _, err := svc.Result(ctx, 7, req.RequestID); err != nil {
		t.Fatal("隔离结果不能幂等读取")
	}
	if _, err := svc.Cancel(ctx, 7, req.RequestID); err != nil || e.cancels.Load() != 0 {
		t.Fatal("已完成仍触发取消")
	}
	if _, err := svc.Show(ctx, 8, req.RequestID); !errors.Is(err, agents.ErrNotFound) {
		t.Fatal("跨身份可查")
	}
}

func TestAgentCancelOnlyAcknowledgesRequest(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "受理", true: "回执丢失"}[failed], func(t *testing.T) {
			s := agentTestStore()
			e := &agentExecutorStub{}
			if failed {
				e.cancel = func(context.Context, string) error { return context.DeadlineExceeded }
			}
			svc := agentService(t, s, e)
			ctx := context.Background()
			req := agents.Request{RequestID: "test_cancel_01", Symbol: "600001"}
			if _, err := svc.Submit(ctx, 7, req); err != nil {
				t.Fatal(err)
			}
			r, err := svc.Cancel(ctx, 7, req.RequestID)
			if (err != nil) != failed || r.State != agents.Accepted || !r.CancelRequested {
				t.Fatal("取消回执被当成远端终态", r, err)
			}
			_, _ = svc.Cancel(ctx, 7, req.RequestID)
			if e.cancels.Load() != 1 {
				t.Fatal("取消被重发")
			}
			e.status = func(_ context.Context, id string) (agents.Observation, error) {
				return agents.Observation{RemoteID: id, State: agents.Cancelled}, nil
			}
			r, err = svc.Status(ctx, 7, req.RequestID)
			if err != nil || r.State != agents.Cancelled {
				t.Fatal(r, err)
			}
		})
	}
}

func TestAgentConcurrentCancelCompletionAndQuarantine(t *testing.T) {
	s := agentTestStore()
	entered, release := make(chan struct{}), make(chan struct{})
	e := &agentExecutorStub{cancel: func(context.Context, string) error { close(entered); <-release; return nil }}
	svc := agentService(t, s, e)
	ctx := context.Background()
	req := agents.Request{RequestID: "test_cancel_race", Symbol: "600001"}
	if _, err := svc.Submit(ctx, 7, req); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := svc.Cancel(ctx, 7, req.RequestID); done <- err }()
	<-entered
	if _, err := svc.Status(ctx, 7, req.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Result(ctx, 7, req.RequestID); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r, err := svc.Show(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Quarantined || r.ResultDigest == "" || r.ErrorCode != "" {
		t.Fatal("取消回执覆盖隔离结果", r, err)
	}
}

func TestAgentValidationAndFailedObservations(t *testing.T) {
	if _, err := NewAIAgentService(nil, nil, agents.Profile{}); err == nil {
		t.Fatal("空仓储")
	}
	if _, err := NewAIAgentService(agentTestStore(), &agentExecutorStub{}, agents.Profile{}); err == nil {
		t.Fatal("空配置")
	}
	s := agentTestStore()
	e := &agentExecutorStub{}
	svc := agentService(t, s, e)
	ctx := context.Background()
	req := agents.Request{RequestID: "test_validate_01", Symbol: "600001"}
	if _, err := svc.Submit(ctx, 0, req); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := svc.Show(ctx, 7, "bad"); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal(err)
	}
	disabled, _ := NewAIAgentService(s, nil, agents.Profile{})
	if _, err := disabled.Submit(ctx, 7, req); !errors.Is(err, agents.ErrDisabled) {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, 7, req); err != nil {
		t.Fatal(err)
	}
	if _, err := disabled.Status(ctx, 7, req.RequestID); !errors.Is(err, agents.ErrDisabled) {
		t.Fatal(err)
	}
	other, _ := NewAIAgentService(s, e, agents.Profile{Digest: strings.Repeat("b", 64), MaxSubmissions: 2})
	if _, err := other.Status(ctx, 7, req.RequestID); !errors.Is(err, agents.ErrConflict) {
		t.Fatal(err)
	}
	e.status = func(context.Context, string) (agents.Observation, error) {
		return agents.Observation{}, context.DeadlineExceeded
	}
	if _, err := svc.Status(ctx, 7, req.RequestID); err == nil {
		t.Fatal("读取错误忽略")
	}
	e.status = func(context.Context, string) (agents.Observation, error) {
		return agents.Observation{RemoteID: "other", State: agents.Completed}, nil
	}
	if _, err := svc.Status(ctx, 7, req.RequestID); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal(err)
	}
	e.status = nil
	s.failSave = true
	if _, err := svc.Status(ctx, 7, req.RequestID); err == nil {
		t.Fatal("审计失败假报完成")
	}
	s.failSave = false
	if _, err := svc.Status(ctx, 7, req.RequestID); err != nil {
		t.Fatal(err)
	}
	e.result = func(context.Context, string) (agents.Result, error) {
		return agents.Result{RemoteID: "other", Payload: []byte(`{}`)}, nil
	}
	if _, err := svc.Result(ctx, 7, req.RequestID); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal(err)
	}
	e.result = func(context.Context, string) (agents.Result, error) { return agents.Result{}, context.DeadlineExceeded }
	if _, err := svc.Result(ctx, 7, req.RequestID); err == nil {
		t.Fatal("结果错误忽略")
	}
	e.result = nil
	s.failSave = true
	if _, err := svc.Result(ctx, 7, req.RequestID); err == nil {
		t.Fatal("隔离审计失败仍成功")
	}
}

func TestAgentCommitUnknownDoesNotReturnUnconfirmedState(t *testing.T) {
	s := agentTestStore()
	e := &agentExecutorStub{}
	svc := agentService(t, s, e)
	ctx := context.Background()
	req := agents.Request{RequestID: "test_commit_unknown", Symbol: "600001"}
	e.submit = func(context.Context, string) (agents.Acceptance, error) {
		s.mu.Lock()
		s.loseCommit = true
		s.mu.Unlock()
		return agents.Acceptance{RemoteID: "remote-1"}, nil
	}
	r, err := svc.Submit(ctx, 7, req)
	if !errors.Is(err, agents.ErrUncertain) || r.State != agents.Submitting {
		t.Fatal("未确认提交返回新状态", r, err)
	}
	r, err = svc.Show(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Accepted {
		t.Fatal("原键不能恢复实际已提交状态", r, err)
	}
	s.loseCommit = true
	r, err = svc.Status(ctx, 7, req.RequestID)
	if err == nil || r.State != agents.Accepted {
		t.Fatal("未确认观察返回新状态", r, err)
	}
	s.loseCommit = true
	r, err = svc.Result(ctx, 7, req.RequestID)
	if err == nil || r.State != agents.Completed {
		t.Fatal("未确认隔离返回新状态", r, err)
	}
	r, err = svc.Show(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Quarantined {
		t.Fatal("隔离后原键查询失败", r, err)
	}
}
