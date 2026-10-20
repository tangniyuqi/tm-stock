package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 只用于测试的准入规划器；不接入产品配置，不能证明真实远端预算或授权。
type agentTaskTestPlanner struct {
	p                   agenttask.Plan
	planErr, checkErr   error
	checks, failCheckAt int
}

func (p *agentTaskTestPlanner) Plan(context.Context, task.Task) (agenttask.Plan, error) {
	return p.p, p.planErr
}
func (p *agentTaskTestPlanner) Recheck(ctx context.Context, _ task.Task, _ agenttask.Plan) error {
	p.checks++
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.failCheckAt == 0 || p.checks >= p.failCheckAt {
		return p.checkErr
	}
	return nil
}

func taskAgentFixture(t *testing.T) (*AITaskAgentWorker, *recoveryControl, *stageScript, *agentMemoryStore, *agentExecutorStub, *agentTaskTestPlanner) {
	t.Helper()
	_, st, r, _ := stageFixture(t)
	st.exists = false
	st.rows = nil
	st.state = task.BudgetState{}
	st.p.calls = 0
	f := &recoveryControl{st: st}
	mem, exec := agentTestStore(), &agentExecutorStub{}
	svc := agentService(t, mem, exec)
	planner := &agentTaskTestPlanner{p: agenttask.Plan{UserID: st.v.UserID, TaskID: st.v.ID, TaskNo: st.v.No, SubjectType: st.v.SubjectType, SubjectID: st.v.SubjectID, ActorID: 7, OperationKey: r.OperationKey, Symbol: "600001", ProfileDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 100}}}
	planner.p.OperationKey = planner.p.RequestID()
	w, err := NewAITaskAgentWorker(f, st, svc, planner, "worker_agent_test_01", agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Second, Heartbeat: 50 * time.Millisecond, Timeout: time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w, f, st, mem, exec, planner
}

func TestAITaskAgentBridgeLifecycleAndReplay(t *testing.T) {
	w, f, st, _, exec, p := taskAgentFixture(t)
	for i := 0; i < 2; i++ {
		v, err := w.Run(context.Background(), 1, 40)
		if err != nil || v.Outcome != "evidence_pending" || v.CurrentStep != 1 || v.TaskStatus != task.Running || v.ReportAvailable || exec.submits.Load() != 1 || f.finishes != 0 || st.p.account.Frozen != 7 || len(st.rows) != 1 || len(st.results) != 1 || len(st.steps) != 1 {
			t.Fatal(v, err)
		}
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), p.p.OperationKey) || strings.Contains(string(raw), "600001") || strings.Contains(string(st.results[1].Payload), "未核验正文") {
			t.Fatal("回执泄漏上游正文或配置")
		}
	}
	oldKey := p.p.RequestID()
	p.p.Symbol = "600002"
	if _, err := w.Run(context.Background(), 1, 40); !errors.Is(err, points.ErrIdempotency) || p.p.RequestID() != oldKey || exec.submits.Load() != 1 {
		t.Fatal("计划变更绕过原预留", err)
	}
}

func TestAITaskAgentTerminalConfirmationCanFailTask(t *testing.T) {
	for _, state := range []string{agents.Failed, agents.Cancelled} {
		t.Run(state, func(t *testing.T) {
			w, _, st, _, exec, _ := taskAgentFixture(t)
			exec.status = func(_ context.Context, id string) (agents.Observation, error) {
				return agents.Observation{RemoteID: id, State: state}, nil
			}
			out, err := w.Run(context.Background(), st.v.UserID, st.v.ID)
			if err != nil || out.Outcome != "upstream_terminal" || out.ReportAvailable || len(st.results) != 1 || len(st.steps) != 1 || st.v.CurrentStep != 1 {
				t.Fatal("已知上游失败没有可复核的阶段回执", out, err)
			}
			ledger, _, _, _, _ := scriptedPoints(t)
			core, err := NewAITaskService(st, ledger)
			if err != nil {
				t.Fatal(err)
			}
			st.p.lastAccount = st.p.account
			done, err := core.FailOrRetry(context.Background(), st.v.UserID, st.v.ID, st.v.LeaseOwner, st.v.Fence, "internal_failure", 0)
			if err != nil || done.Status != task.Failed || st.p.lastAccount.Frozen != 0 || exec.submits.Load() != 1 || strings.Contains(string(st.results[1].Payload), "未核验正文") {
				t.Fatal("已知上游失败无法原子终结或泄漏研究原文", done, err)
			}
		})
	}
}

func TestAITaskAgentBridgeUnknownAndCaptureGap(t *testing.T) {
	for _, mode := range []string{"提交未知", "预算提交未知", "旧围栏无回执", "上游失败", "上游取消"} {
		t.Run(mode, func(t *testing.T) {
			w, _, st, mem, exec, p := taskAgentFixture(t)
			want := "response_unknown"
			switch mode {
			case "提交未知":
				exec.submit = func(context.Context, string) (agents.Acceptance, error) {
					return agents.Acceptance{}, context.DeadlineExceeded
				}
			case "预算提交未知":
				// 定点模拟预算已提交但回执未知；只读准入事务不能提前消费该故障。
				st.budgetCommitError = errPointsInjected
			case "旧围栏无回执":
				if _, err := w.Run(context.Background(), 1, 40); err != nil {
					t.Fatal(err)
				}
				st.results = map[int]task.StageResult{}
				st.steps = map[int]task.StageCheckpoint{}
				st.stageEvents = map[int64]task.Event{}
				st.v.CurrentStep = 0
				want = "capture_pending"
			case "上游失败", "上游取消":
				state := agents.Failed
				if mode == "上游取消" {
					state = agents.Cancelled
				}
				exec.status = func(_ context.Context, id string) (agents.Observation, error) {
					return agents.Observation{RemoteID: id, State: state}, nil
				}
				want = "upstream_terminal"
			}
			v, err := w.Run(context.Background(), 1, 40)
			if mode == "预算提交未知" {
				if err == nil || v != (task.RecoveryResult{}) || exec.submits.Load() != 0 {
					t.Fatal(v, err)
				}
				st.budgetCommitError = nil
				v, err = w.Run(context.Background(), 1, 40)
			}
			if err != nil || v.Outcome != want || v.ReportAvailable || st.p.account.Frozen != 7 {
				t.Fatal(v, err)
			}
			if mode == "旧围栏无回执" {
				if len(st.results) != 0 || len(st.steps) != 0 {
					t.Fatal("补造旧租约回执")
				}
			}
			before := exec.submits.Load()
			if v, err := w.Run(context.Background(), 1, 40); err != nil || v.Outcome != want || exec.submits.Load() != before {
				t.Fatal("重放重新提交", v, err)
			}
			if mode == "提交未知" && mem.records[p.p.RequestID()].State != agents.SubmissionUnknown {
				t.Fatal("未知记录丢失")
			}
		})
	}
}

func TestAITaskAgentBridgeGuardsAndFaults(t *testing.T) {
	for _, mode := range []string{"disabled", "inspect", "plan", "bad_plan", "recheck", "claim", "terminal", "claimed_terminal", "after_claim", "post_submit", "store", "agent_show", "agent_status", "agent_result", "wrong_identity", "bad_result_state"} {
		t.Run(mode, func(t *testing.T) {
			w, f, st, _, exec, p := taskAgentFixture(t)
			fault := errors.New("fake bridge failure")
			switch mode {
			case "disabled":
				w.opts.Enabled = false
			case "inspect":
				f.inspectError = fault
			case "plan":
				p.planErr = fault
			case "bad_plan":
				p.p.UserID++
			case "recheck":
				p.checkErr = fault
			case "claim":
				f.claimError = fault
			case "terminal":
				st.v.Status = task.Cancelled
			case "claimed_terminal":
				st.v.Attempt = st.v.Budget.MaxAttempts
			case "after_claim":
				p.checkErr = fault
				p.failCheckAt = 2
			case "post_submit":
				p.checkErr = fault
				p.failCheckAt = 4
			case "store":
				w.store = nil
			case "agent_show":
				w.agent = agentTaskClientFault{taskAgentClient: w.agent, showErr: fault}
			case "agent_status":
				exec.status = func(context.Context, string) (agents.Observation, error) { return agents.Observation{}, fault }
			case "agent_result":
				exec.result = func(context.Context, string) (agents.Result, error) { return agents.Result{}, fault }
			case "wrong_identity":
				w.agent = agentTaskClientFault{taskAgentClient: w.agent, wrong: true}
			case "bad_result_state":
				w.agent = agentTaskClientFault{taskAgentClient: w.agent, resultPending: true}
			}
			v, err := w.Run(context.Background(), 1, 40)
			if mode == "terminal" || mode == "claimed_terminal" {
				if err != nil || v.Outcome != "terminal" {
					t.Fatal(v, err)
				}
				return
			}
			if err == nil || v != (task.RecoveryResult{}) || f.finishes != 0 {
				t.Fatal(v, err)
			}
			if mode == "disabled" && (f.claims != 0 || p.checks != 0 || len(st.rows) != 0) {
				t.Fatal("关闭仍有副作用")
			}
		})
	}
	w, _, st, _, _, _ := taskAgentFixture(t)
	if _, err := w.Run(context.Background(), 1, 40); err != nil {
		t.Fatal(err)
	}
	boundaries := st.p.calls
	for n := 1; n <= boundaries; n++ {
		w, _, st, _, _, _ := taskAgentFixture(t)
		st.p.failAt = n
		v, err := w.Run(context.Background(), 1, 40)
		if err == nil || !reflect.DeepEqual(v, task.RecoveryResult{}) {
			t.Fatal("存储故障返回候选成功", n, v, err)
		}
	}
}

type agentTaskClientFault struct {
	taskAgentClient
	showErr              error
	wrong, resultPending bool
}

func (s agentTaskClientFault) Show(c context.Context, a int64, id string) (agents.Record, error) {
	if s.showErr != nil {
		return agents.Record{}, s.showErr
	}
	return s.taskAgentClient.Show(c, a, id)
}
func (s agentTaskClientFault) Status(c context.Context, a int64, id string) (agents.Record, error) {
	r, e := s.taskAgentClient.Status(c, a, id)
	if e == nil && s.wrong {
		r.ActorID++
	}
	return r, e
}
func (s agentTaskClientFault) Result(c context.Context, a int64, id string) (agents.Record, error) {
	r, e := s.taskAgentClient.Result(c, a, id)
	if e == nil && s.resultPending {
		r.State = agents.Completed
		r.ResultDigest = ""
	}
	return r, e
}

func TestAITaskAgentBridgeHeartbeatAndCancellation(t *testing.T) {
	for _, mode := range []string{"heartbeat", "parent", "poll"} {
		t.Run(mode, func(t *testing.T) {
			w, f, st, _, exec, _ := taskAgentFixture(t)
			ctx := context.Background()
			stop := func() {}
			if mode == "parent" || mode == "poll" {
				ctx, stop = context.WithTimeout(ctx, 90*time.Millisecond)
			}
			defer stop()
			if mode == "heartbeat" {
				f.beatError = task.ErrCancelled
				f.beatFailAt = 3
			}
			exec.status = func(ctx context.Context, id string) (agents.Observation, error) {
				if mode != "poll" {
					<-ctx.Done()
					return agents.Observation{}, ctx.Err()
				}
				return agents.Observation{RemoteID: id, State: agents.Running}, nil
			}
			out, err := w.Run(ctx, 1, 40)
			if err == nil || out != (task.RecoveryResult{}) || len(st.steps) != 0 || exec.submits.Load() != 1 || st.p.account.Frozen != 7 {
				t.Fatal(out, err)
			}
			f.mu.Lock()
			before := f.beats
			f.mu.Unlock()
			time.Sleep(60 * time.Millisecond)
			f.mu.Lock()
			after := f.beats
			f.mu.Unlock()
			if before != after {
				t.Fatal("返回后仍续租")
			}
		})
	}
	w, f, st, _, _, p := taskAgentFixture(t)
	if _, err := (*AITaskAgentWorker)(nil).Run(context.Background(), 1, 40); err == nil {
		t.Fatal("空接线")
	}
	if _, err := w.Run(context.Background(), 0, 40); err == nil {
		t.Fatal("坏身份")
	}
	opts := w.opts
	for _, bad := range []string{"tasks", "store", "agent", "planner", "owner", "options"} {
		var tasks recoveryTasks = f
		var store taskAgentStore = st
		var client taskAgentClient = w.agent
		var planner agenttask.Planner = p
		owner := w.owner
		o := opts
		switch bad {
		case "tasks":
			tasks = nil
		case "store":
			store = nil
		case "agent":
			client = nil
		case "planner":
			planner = nil
		case "owner":
			owner = "bad"
		case "options":
			o.PollInterval = 0
		}
		if _, err := NewAITaskAgentWorker(tasks, store, client, planner, owner, o); err == nil {
			t.Fatal(bad)
		}
	}
	opts.Enabled = false
	if _, err := NewAITaskAgentWorker(f, st, nil, nil, w.owner, opts); err != nil {
		t.Fatal("关闭模式仍需外部凭据", err)
	}
}
