package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestAIL1ReportKnownSourceRejectionReleases(t *testing.T) {
	s, st, _ := reportFixture(t)
	tasks, err := NewAITaskService(st, s.core.ledger)
	if err != nil {
		t.Fatal(err)
	}
	// 只检验规划失败边界；外部依赖故意缺失，任何外呼都会导致测试失败。
	w := &AIL1ReportWorker{tasks: tasks, store: st, ledger: s.core.ledger, planner: l1TestPlanner{err: sources.ErrUnauthorized}, owner: st.v.LeaseOwner, opts: agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 5 * time.Second}}}
	out, err := w.Run(context.Background(), st.v.UserID, st.v.ID)
	if err != nil || out.Task.TaskStatus != task.Failed || out.Task.Outcome != "terminal" || st.p.lastAccount.Frozen != 0 || st.pt.hold.State != points.Released || st.v.ErrorCode != "guard_rejected" {
		t.Fatal("来源已知拒绝没有原子终结并释放", out, err)
	}
	if st.commit != nil || st.reviewCalls != 0 {
		t.Fatal("失败释放伪造报告或调用终检模型")
	}
}

func TestAIL1ReportKnownRejectionBeforeFirstCall(t *testing.T) {
	for _, rejection := range []error{sources.ErrUnauthorized, sources.ErrSourceUnavailable, sources.ErrUnsafeURL, finalreport.ErrRejected, task.ErrStageRejected} {
		t.Run(rejection.Error(), func(t *testing.T) {
			s, st, _ := reportFixture(t)
			st.v.Status, st.v.Fence, st.v.Attempt, st.v.CurrentStep = task.Queued, 0, 0, 0
			st.v.LeaseOwner, st.v.LeaseUntil = "", nil
			st.exists, st.rows = false, nil
			tasks, err := NewAITaskService(st, s.core.ledger)
			if err != nil {
				t.Fatal(err)
			}
			w := &AIL1ReportWorker{tasks: tasks, store: st, ledger: s.core.ledger, planner: l1TestPlanner{err: fmt.Errorf("测试已知拒绝: %w", rejection)}, owner: "worker_reject_test", opts: agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 5 * time.Second}}}
			out, err := w.Run(context.Background(), st.v.UserID, st.v.ID)
			if err != nil || out.Task.TaskStatus != task.Failed || st.v.Fence != 1 || st.pt.hold.State != points.Released || st.p.lastAccount.Frozen != 0 {
				t.Fatal("未使用首次领取的新围栏原子释放", out, err)
			}
			events := len(st.events)
			if _, err := w.Run(context.Background(), st.v.UserID, st.v.ID); err != nil || len(st.events) != events {
				t.Fatal("终态重放重复写入", err)
			}
		})
	}
}

func TestAIL1ReportRejectionPreservesUnknownAndDependencies(t *testing.T) {
	for _, mode := range []string{"unknown", "unknown_exhausted", "dependency", "not_found", "configuration", "identity", "canceled_context", "admission", "lease", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, st, _ := reportFixture(t)
			tasks, err := NewAITaskService(st, s.core.ledger)
			if err != nil {
				t.Fatal(err)
			}
			rejection := error(sources.ErrUnauthorized)
			ctx := context.Background()
			switch mode {
			case "unknown", "unknown_exhausted":
				st.v.CurrentStep = 0
				if mode == "unknown_exhausted" {
					until := st.p.now.Add(-time.Millisecond)
					st.v.LeaseUntil = &until
					st.v.Attempt = st.v.Budget.MaxAttempts
				}
			case "dependency":
				rejection = errPointsInjected
			case "not_found":
				rejection = points.ErrNotFound
			case "configuration":
				rejection = task.ErrAdmissionUnavailable
			case "identity":
				rejection = points.ErrIdempotency
			case "canceled_context":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "admission":
				st.admissionError = errPointsInjected
			case "lease":
				st.v.LeaseOwner = "worker_someone_else"
			case "commit":
				st.failCommitAt = st.txCount + 1
			}
			w := &AIL1ReportWorker{tasks: tasks, store: st, ledger: s.core.ledger, planner: l1TestPlanner{err: rejection}, owner: "worker_test_00001", opts: agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 5 * time.Second}}}
			out, err := w.Run(ctx, st.v.UserID, st.v.ID)
			if mode == "unknown" || mode == "unknown_exhausted" {
				if err != nil || out.Task.Outcome != "response_unknown" || out.Task.TaskStatus != task.Running || st.pt.hold.State != points.Holding || st.v.ErrorCode != "" {
					t.Fatal("未知预留被已知拒绝覆盖", out, err)
				}
			} else if err == nil || out != (AIL1ReportReceipt{}) {
				t.Fatal("依赖/提交异常返回候选收据", mode, out, err)
			}
			// 脚本夹具不模拟提交回滚，commit情形仅验证未知提交不能返回收据。
			if mode != "commit" && st.pt.hold.State != points.Holding {
				t.Fatal("未知依赖释放冻结", mode)
			}
		})
	}
}

func TestAIL1ReportCompletedCallsProof(t *testing.T) {
	for _, mode := range []string{"complete", "empty_budget", "pending", "missing_budget", "orphan_row", "too_few", "bad_budget", "bad_result", "bad_checkpoint", "bad_event", "missing_event", "missing_result", "missing_checkpoint", "bad_step", "bad_limits"} {
		t.Run(mode, func(t *testing.T) {
			_, st, _ := reportFixture(t)
			switch mode {
			case "empty_budget":
				st.v.CurrentStep, st.state.Count, st.rows = 0, 0, nil
				st.state.Reserved = task.Units{}
			case "pending":
				st.v.CurrentStep = 0
			case "missing_budget":
				st.exists = false
			case "orphan_row":
				st.exists, st.v.CurrentStep = false, 0
			case "too_few":
				st.v.CurrentStep = 2
			case "bad_budget":
				st.state.Reserved.Tokens++
			case "bad_result":
				r := st.results[1]
				r.Payload = []byte(`{"test":"different"}`)
				st.results[1] = r
			case "bad_checkpoint":
				c := st.steps[1]
				c.PolicyDigest = "bad"
				st.steps[1] = c
			case "bad_event":
				e := st.stageEvents[st.steps[1].EventSeq]
				e.Step++
				st.stageEvents[st.steps[1].EventSeq] = e
			case "missing_event":
				delete(st.stageEvents, st.steps[1].EventSeq)
			case "missing_result":
				delete(st.results, 1)
			case "missing_checkpoint":
				delete(st.steps, 1)
			case "bad_step":
				st.v.CurrentStep = -1
			case "bad_limits":
				st.v.Budget.MaxCalls = 0
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var complete bool
			err := st.WithinReportTx(ctx, func(tx finalreport.Tx) error {
				var err error
				complete, err = l1CallsCompleted(ctx, tx, st.v)
				return err
			})
			want := mode == "complete" || mode == "empty_budget"
			if complete != want || (mode != "pending" && !want) != (err != nil) {
				t.Fatal("完成证明错误", mode, complete, err)
			}
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("证明检查意外超时")
			}
		})
	}
}

type rejectionRaceScript struct {
	*reportScript
	hook func()
}

func (s *rejectionRaceScript) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	if s.hook != nil {
		s.hook()
	}
	return s.reportScript.WithinReportTx(ctx, fn)
}

func TestAIL1ReportRejectionCurrentTransactionGates(t *testing.T) {
	for _, mode := range []string{"fence", "terminal", "report", "hold", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s, st, _ := reportFixture(t)
			tasks, err := NewAITaskService(st, s.core.ledger)
			if err != nil {
				t.Fatal(err)
			}
			store := &rejectionRaceScript{reportScript: st}
			switch mode {
			case "fence":
				store.hook = func() { st.v.Fence++ }
			case "terminal":
				store.hook = func() { st.v.Status = task.Failed }
			case "report":
				st.commit = &finalreport.Commit{}
			case "hold":
				st.pt.hold.BizID = "task:other_test_task"
			case "cancel":
				st.v.CancelRequested = true
			}
			w := &AIL1ReportWorker{tasks: tasks, store: store, ledger: s.core.ledger, owner: st.v.LeaseOwner, opts: agenttask.Options{Lease: task.RecoveryOptions{Lease: time.Minute}}}
			out, err := w.rejectKnown(context.Background(), st.v)
			if mode == "terminal" {
				if err != nil || out.Task.TaskStatus != task.Failed || st.pt.hold.State != points.Holding {
					t.Fatal("竞争终态被覆盖", out, err)
				}
			} else if mode == "cancel" {
				if err != nil || out.Task.TaskStatus != task.Cancelled || st.pt.hold.State != points.Released {
					t.Fatal("取消优先级丢失", out, err)
				}
			} else if err == nil || out != (AIL1ReportReceipt{}) || st.pt.hold.State != points.Holding {
				t.Fatal("围栏/报告/冻结身份异常仍释放", mode, out, err)
			}
		})
	}
}

func TestAIL1ReportRejectionEveryAccessFailure(t *testing.T) {
	run := func(failAt int) (AIL1ReportReceipt, error, int) {
		s, st, _ := reportFixture(t)
		tasks, err := NewAITaskService(st, s.core.ledger)
		if err != nil {
			t.Fatal(err)
		}
		st.p.calls, st.p.failAt = 0, failAt
		w := &AIL1ReportWorker{tasks: tasks, store: st, ledger: s.core.ledger, owner: st.v.LeaseOwner, opts: agenttask.Options{Lease: task.RecoveryOptions{Lease: time.Minute}}}
		out, err := w.rejectKnown(context.Background(), st.v)
		return out, err, st.p.calls
	}
	if out, err, _ := run(0); err != nil || out.Task.TaskStatus != task.Failed {
		t.Fatal("访问故障基线失败", out, err)
	}
	_, _, accesses := run(0)
	for i := 1; i <= accesses; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			out, err, _ := run(i)
			if err == nil || out != (AIL1ReportReceipt{}) {
				t.Fatal("访问故障返回候选收据", i, out, err)
			}
		})
	}
}
