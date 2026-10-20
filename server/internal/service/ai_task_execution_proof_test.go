package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type hiddenExecutionTx struct{ task.Tx }

func TestAITaskAttemptLimitCallsProof(t *testing.T) {
	for _, mode := range []string{"complete", "captured", "unknown", "empty", "budget_empty", "missing_reader", "budget_missing", "orphan", "counter", "payload", "stage", "units", "identity", "steps", "negative_step", "overflow_step"} {
		t.Run(mode, func(t *testing.T) {
			_, st, _ := reportFixture(t)
			var tx task.Tx = st
			var want error
			switch mode {
			case "captured":
				st.v.CurrentStep = 0
			case "unknown":
				delete(st.results, 1)
				st.v.CurrentStep = 0
				want = task.ErrUnconfirmed
			case "empty":
				st.v.CurrentStep, st.exists, st.rows = 0, false, nil
			case "budget_empty":
				st.v.CurrentStep, st.rows = 0, nil
				st.state.Count, st.state.Reserved = 0, task.Units{}
			case "missing_reader":
				tx, want = hiddenExecutionTx{Tx: st}, points.ErrInvariant
			case "budget_missing":
				st.exists, want = false, points.ErrInvariant
			case "orphan":
				st.exists, st.v.CurrentStep, want = false, 0, points.ErrInvariant
			case "counter":
				st.state.Reserved.Tokens++
				want = points.ErrInvariant
			case "payload", "stage", "units", "identity":
				r := st.results[1]
				if mode == "payload" {
					r.Payload = []byte(`{"test_only":"不同测试响应"}`)
				}
				if mode == "stage" {
					r.Stage = "not_a_stage"
				}
				if mode == "units" {
					r.Stage = "collect" // 原预留只有模型调用，没有采集页数。
				}
				if mode == "identity" {
					r.UserID++
				}
				st.results[1], want = r, points.ErrInvariant
			case "steps":
				st.v.CurrentStep, want = 2, points.ErrInvariant
			case "negative_step":
				st.v.CurrentStep, want = -1, points.ErrInvariant
			case "overflow_step":
				st.v.CurrentStep, want = task.MaxSteps+1, points.ErrInvariant
			}
			before := st.v
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := checkTaskCallsKnown(ctx, tx, st.v); !errors.Is(err, want) {
				t.Fatal("调用确认门禁错误", mode, err)
			}
			if st.v != before || st.commit != nil || st.reviewCalls != 0 {
				t.Fatal("只读核查改变任务或执行模型")
			}
		})
	}
}

func TestAITaskAttemptLimitEveryReadBoundary(t *testing.T) {
	// 预算、预留明细和原始响应任一读取失败，都不能被当成“没有未知调用”。
	for fail := 1; fail <= 3; fail++ {
		_, st, _ := reportFixture(t)
		st.p.calls, st.p.failAt = 0, fail
		before, account, events := st.v, st.p.account, len(st.events)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := checkTaskCallsKnown(ctx, st, st.v)
		cancel()
		if !errors.Is(err, errPointsInjected) || st.p.calls != fail || st.v != before || st.p.account != account || len(st.events) != events || st.commit != nil {
			t.Fatal("调用证明读取失败未关闭", fail, err)
		}
	}
}

func TestAITaskClaimPreservesUnconfirmedAtAttemptLimit(t *testing.T) {
	for _, captured := range []bool{false, true} {
		s, st, _ := reportFixture(t)
		st.v.CurrentStep, st.v.EventSeq = 0, 1
		st.v.Attempt = st.v.Budget.MaxAttempts
		until := st.p.now.Add(-time.Millisecond)
		st.v.LeaseUntil = &until
		if !captured {
			delete(st.results, 1)
		}
		st.p.lastAccount = st.p.account
		before, account, events := st.v, st.p.lastAccount, len(st.events)
		core, err := NewAITaskService(st, s.core.ledger)
		if err != nil {
			t.Fatal(err)
		}
		out, err := core.Claim(context.Background(), st.v.UserID, st.v.ID, st.v.LeaseOwner, time.Minute)
		if captured {
			if err != nil || out.Status != task.Failed || out.ErrorCode != "attempts_exhausted" || st.p.lastAccount.Frozen != 0 {
				t.Fatal("已确认响应没有保留正常尝试上限", out, err)
			}
		} else if !errors.Is(err, task.ErrUnconfirmed) || out != (task.Task{}) || st.v != before || st.p.lastAccount != account || len(st.events) != events {
			t.Fatal("未知调用被尝试耗尽覆盖", out, err)
		}
	}
}

func TestAITaskFailurePreservesUnconfirmedCalls(t *testing.T) {
	for _, mode := range []string{"failed", "retry_exhausted", "retry_past_deadline"} {
		t.Run(mode, func(t *testing.T) {
			for _, captured := range []bool{false, true} {
				s, st, _ := reportFixture(t)
				st.v.CurrentStep, st.v.EventSeq = 0, 1
				backoff := time.Duration(0)
				if mode == "retry_exhausted" {
					st.v.Attempt, backoff = st.v.Budget.MaxAttempts, time.Second
				}
				if mode == "retry_past_deadline" {
					st.v.Deadline, backoff = st.p.now.Add(time.Second), 2*time.Second
				}
				if !captured {
					delete(st.results, 1)
				}
				st.p.lastAccount = st.p.account
				before, account, events := st.v, st.p.lastAccount, len(st.events)
				core, err := NewAITaskService(st, s.core.ledger)
				if err != nil {
					t.Fatal(err)
				}
				out, err := core.FailOrRetry(context.Background(), st.v.UserID, st.v.ID, st.v.LeaseOwner, st.v.Fence, "provider_unavailable", backoff)
				if captured {
					if err != nil || out.Status != task.Failed || st.p.lastAccount.Frozen != 0 {
						t.Fatal("已确认调用无法按原政策失败释放", out, err)
					}
				} else if !errors.Is(err, task.ErrUnconfirmed) || out != (task.Task{}) || st.v != before || st.p.lastAccount != account || len(st.events) != events {
					t.Fatal("失败入口覆盖未知调用现场", mode, out, err)
				}
			}
		})
	}
}

func TestWorkersPreserveUnknownAtAttemptLimit(t *testing.T) {
	for _, kind := range []string{"l1", "recovery", "agent"} {
		t.Run(kind, func(t *testing.T) {
			p, v, model := l1PlanFixture(t)
			s, st, _ := reportFixture(t)
			st.v.Budget, st.state.Limits = v.Budget, v.Budget.Limits()
			st.v.CurrentStep, st.v.EventSeq, st.v.Attempt = 0, 1, st.v.Budget.MaxAttempts
			until := st.p.now.Add(-time.Millisecond)
			st.v.LeaseUntil = &until
			delete(st.results, 1)
			st.p.lastAccount = st.p.account
			before, account := st.v, st.p.lastAccount
			core, err := NewAITaskService(st, s.core.ledger)
			if err != nil {
				t.Fatal(err)
			}
			o := task.RecoveryOptions{Lease: time.Minute, Timeout: 5 * time.Second, Heartbeat: 100 * time.Millisecond}
			var out task.RecoveryResult
			switch kind {
			case "l1":
				w := &AIL1ReportWorker{tasks: core, store: st, ledger: s.core.ledger, model: model, planner: admittedL1Planner{base: l1TestPlanner{p: p}, store: st}, owner: st.v.LeaseOwner, opts: agenttask.Options{Enabled: true, Lease: o}}
				var result AIL1ReportReceipt
				result, err = w.Run(context.Background(), st.v.UserID, st.v.ID)
				out = result.Task
			case "recovery":
				w := &AIRecoveryWorker{tasks: core, store: st, owner: st.v.LeaseOwner, options: o}
				out, err = w.Recover(context.Background(), st.v.UserID, st.v.ID)
			case "agent":
				w := &AITaskAgentWorker{tasks: core, store: st, planner: l1AgentPlanner{planner: l1TestPlanner{p: p}, plan: p}, owner: st.v.LeaseOwner, opts: agenttask.Options{Enabled: true, Lease: o}}
				out, err = w.Run(context.Background(), st.v.UserID, st.v.ID)
			}
			if err != nil || out.Outcome != "response_unknown" || out.TaskStatus != task.Running || out.ReportAvailable || st.v != before || st.p.lastAccount != account {
				t.Fatal("worker未知恢复产生副作用", kind, out, err)
			}
		})
	}
}

func TestUnconfirmedRecoveryReadGates(t *testing.T) {
	for _, mode := range []string{"unknown", "terminal", "queued", "identity", "steps", "dependency"} {
		t.Run(mode, func(t *testing.T) {
			_, st, _ := reportFixture(t)
			f := &recoveryControl{st: st.stageScript}
			uid, id := st.v.UserID, st.v.ID
			switch mode {
			case "terminal":
				st.v.Status = task.Failed
			case "queued":
				st.v.Status = task.Queued
			case "identity":
				st.v.ID++
			case "steps":
				st.v.CurrentStep = task.MaxSteps + 1
			case "dependency":
				f.inspectError = errPointsInjected
			}
			out, err := unconfirmedRecovery(context.Background(), f, uid, id)
			if mode == "unknown" || mode == "terminal" {
				if err != nil || out.ReportAvailable || (mode == "unknown" && out.Outcome != "response_unknown") || (mode == "terminal" && out.Outcome != "terminal") {
					t.Fatal(out, err)
				}
			} else if err == nil || out != (task.RecoveryResult{}) {
				t.Fatal("不明读取返回候选结果", out, err)
			}
		})
	}
}
