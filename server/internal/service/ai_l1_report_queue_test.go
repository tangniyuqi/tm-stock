package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type queueScanStub struct {
	window    task.MemberQueueRange
	page      []task.MemberQueueCandidate
	err       error
	pageErr   error
	queries   int
	deadline  time.Time
	rangeHook func()
}

func (s *queueScanStub) MemberQueueRange(ctx context.Context) (task.MemberQueueRange, error) {
	s.queries++
	s.deadline, _ = ctx.Deadline()
	if s.rangeHook != nil {
		s.rangeHook()
	}
	return s.window, s.err
}
func (s *queueScanStub) MemberQueuePage(_ context.Context, after, upper int64, limit int) ([]task.MemberQueueCandidate, error) {
	s.queries++
	return s.page, s.pageErr
}

type queueRunStub struct {
	calls int
	fn    func(context.Context, int64, int64) (AIL1ReportReceipt, error)
}

func (s *queueRunStub) Run(ctx context.Context, uid, id int64) (AIL1ReportReceipt, error) {
	s.calls++
	if s.fn != nil {
		return s.fn(ctx, uid, id)
	}
	return AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: id, TaskStatus: task.Succeeded, Outcome: "terminal"}}, nil
}
func queueFixture(t *testing.T) (*AIL1ReportQueue, *queueScanStub, *queueRunStub, *bindingTestStore) {
	t.Helper()
	st, _, v := bindingFixture(t)
	at := st.record.CreatedAt
	v.Status, v.NotBefore, v.LeaseUntil = task.Queued, at, nil
	st.v, st.p.now = v, at
	scan := &queueScanStub{window: task.MemberQueueRange{UpperID: v.ID, AsOf: at}, page: []task.MemberQueueCandidate{{ID: v.ID, UserID: v.UserID, AnalysisType: 1, SubjectType: task.Theme, Status: task.Queued, NotBefore: at, Deadline: v.Deadline, HasAdmission: true}}}
	runner := &queueRunStub{}
	return &AIL1ReportQueue{runner: runner, store: st, scan: scan, enabled: true}, scan, runner, st
}
func TestAIL1ReportQueueConstructionAndClosed(t *testing.T) {
	s, st, _ := reportFixture(t)
	opts := task.RecoveryOptions{Lease: time.Minute, Heartbeat: time.Second, Timeout: time.Second}
	w, err := NewAIL1ReportWorker(&recoveryControl{st: st.stageScript}, st, s.core.ledger, nil, nil, nil, nil, nil, nil, st.v.LeaseOwner, agenttask.Options{Lease: opts, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, scan, _, _ := queueFixture(t)
	for _, p := range []struct {
		w    *AIL1ReportWorker
		scan task.MemberQueueScanner
		on   bool
	}{{nil, scan, false}, {w, nil, false}, {w, scan, true}} {
		if _, err := NewAIL1ReportQueue(p.w, p.scan, p.on); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	q, err := NewAIL1ReportQueue(w, scan, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); !errors.Is(err, finalreport.ErrDisabled) || scan.queries != 0 {
		t.Fatal(err)
	}
	deep, err := NewAIL2ReportQueue(w, scan, false)
	if err != nil || deep == nil || !deep.deep {
		t.Fatal("深度队列构造失败", err)
	}
	if _, err := deep.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); !errors.Is(err, finalreport.ErrDisabled) || scan.queries != 0 {
		t.Fatal("深度队列默认关闭失效", err)
	}
	if _, err := NewAIL2ReportQueue(w, scan, true); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("关闭的worker被深度队列开启", err)
	}
	for _, q := range []*AIL1ReportQueue{nil, {}} {
		if _, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
}

func TestAIL1ReportQueuePaginationAndRecovery(t *testing.T) {
	q, scan, runner, _ := queueFixture(t)
	member := scan.page[0]
	legacy := member
	legacy.ID, legacy.HasAdmission = member.ID-1, false
	// 夹具编号可能为1；把已准入原任务置于下一编号并同步绑定身份。
	if legacy.ID == 0 {
		st := q.store.(*bindingTestStore)
		st.v.ID, st.record.TaskID, member.ID, legacy.ID = 2, 2, 2, 1
	}
	scan.window.UpperID, scan.page = member.ID, []task.MemberQueueCandidate{legacy, member}
	a, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1})
	if err != nil || a.Complete || a.Visited != 1 || a.Skipped != 1 || runner.calls != 0 || a.LastID != legacy.ID {
		t.Fatal(a, err)
	}
	scan.page = []task.MemberQueueCandidate{member}
	b, err := q.RunBatch(context.Background(), task.MemberQueueOptions{AfterID: a.LastID, MaxTasks: 1})
	if err != nil || !b.Complete || b.TerminalObserved != 1 || runner.calls != 1 || len(b.Items) != 1 || time.Until(scan.deadline) > 5*time.Minute {
		t.Fatal(b, err)
	}
	if c, err := q.RunBatch(context.Background(), task.MemberQueueOptions{AfterID: b.LastID, MaxTasks: 1}); err != nil || !c.Complete || c.Visited != 0 {
		t.Fatal(c, err)
	}
	scan.window.UpperID, scan.page = 0, nil
	if c, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); err != nil || !c.Complete {
		t.Fatal(c, err)
	}
}

func TestAIL2ReportQueueTypeIsolationAndAdmission(t *testing.T) {
	for _, mode := range []string{"deep", "light", "missing_admission", "changed_type", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			q, scan, runner, st := queueFixture(t)
			q.deep = true
			st.v.AnalysisType = task.DeepAnalysisType
			st.record.Plan.Scope.AnalysisType = task.DeepAnalysisType
			_, digest, err := st.record.Plan.Encode()
			if err != nil {
				t.Fatal(err)
			}
			st.record.Digest = digest
			scan.page[0].AnalysisType = task.DeepAnalysisType
			switch mode {
			case "light":
				scan.page[0].AnalysisType = task.LightAnalysisType
			case "missing_admission":
				st.marked = false
			case "changed_type":
				st.v.AnalysisType = task.LightAnalysisType
			case "disabled":
				q.enabled = false
			}
			out, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1})
			if mode == "deep" {
				if err != nil || runner.calls != 1 || out.TerminalObserved != 1 {
					t.Fatal(out, err, runner.calls)
				}
			} else {
				if runner.calls != 0 {
					t.Fatal("深度队列执行了范围外或未经复核的任务")
				}
				if mode == "light" {
					if err != nil || out.Skipped != 1 {
						t.Fatal(out, err)
					}
				} else if err == nil {
					t.Fatal("失效准入或关闭队列仍成功")
				}
			}
		})
	}
}

func TestAIL1ReportQueueEligibility(t *testing.T) {
	for _, kind := range []string{"legacy", "deep", "topic", "future", "active", "terminal", "due", "lease_expired", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			q, scan, runner, st := queueFixture(t)
			v := &scan.page[0]
			switch kind {
			case "legacy":
				v.HasAdmission = false
			case "deep":
				v.AnalysisType = 2
			case "topic":
				v.SubjectType = task.Topic
			case "future":
				v.NotBefore = v.NotBefore.Add(time.Hour)
			case "terminal":
				v.Status = task.Succeeded
			case "active", "lease_expired", "deadline":
				v.Status = task.Running
				at := scan.window.AsOf.Add(time.Hour)
				if kind == "lease_expired" {
					at = scan.window.AsOf
				}
				v.LeaseUntil = &at
				st.v.Status, st.v.LeaseUntil = v.Status, v.LeaseUntil
				if kind == "deadline" {
					scan.window.AsOf = v.Deadline
					st.p.now = v.Deadline
				}
			}
			out, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1})
			want := kind == "due" || kind == "lease_expired" || kind == "deadline"
			if err != nil || !out.Complete || (runner.calls == 1) != want || out.Skipped+out.TerminalObserved != 1 {
				t.Fatal(out, err, runner.calls)
			}
		})
	}
}

func TestAIL1ReportQueueFailClosed(t *testing.T) {
	for _, kind := range []string{"range", "upper", "time", "precision", "page", "empty", "oversize", "id", "user", "type", "scope", "status", "deadline", "before", "lease", "missing", "corrupt", "storage", "commit", "runner", "result", "disclosure", "outcome", "receipt", "digest"} {
		t.Run(kind, func(t *testing.T) {
			q, scan, runner, st := queueFixture(t)
			v := &scan.page[0]
			switch kind {
			case "range":
				scan.err = errPointsInjected
			case "upper":
				scan.window.UpperID = -1
			case "time":
				scan.window.AsOf = time.Time{}
			case "precision":
				scan.window.AsOf = scan.window.AsOf.Add(time.Nanosecond)
			case "page":
				scan.pageErr = errPointsInjected
			case "empty":
				scan.page = nil
			case "oversize":
				scan.page = []task.MemberQueueCandidate{*v, *v, *v}
			case "id":
				v.ID++
			case "user":
				v.UserID = 0
			case "type":
				v.AnalysisType = 0
			case "scope":
				v.SubjectType = "stock"
			case "status":
				v.Status = 99
			case "deadline":
				v.Deadline = time.Time{}
			case "before":
				v.NotBefore = v.NotBefore.Add(time.Nanosecond)
			case "lease":
				v.Status = task.Running
			case "missing":
				st.marked = false
			case "corrupt":
				st.record.Digest = "bad"
			case "storage":
				st.readError = errPointsInjected
			case "commit":
				st.commitError = errPointsInjected
			case "runner":
				runner.fn = func(context.Context, int64, int64) (AIL1ReportReceipt, error) {
					return AIL1ReportReceipt{}, errPointsInjected
				}
			default:
				runner.fn = func(_ context.Context, _ int64, id int64) (AIL1ReportReceipt, error) {
					out := AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: id, TaskStatus: task.Succeeded, Outcome: "terminal"}}
					switch kind {
					case "result":
						out.Task.TaskID++
					case "disclosure":
						out.Task.ReportAvailable = true
					case "outcome":
						out.Task.Outcome = "passed"
					case "receipt":
						out.Commit = finalreport.Receipt{TaskID: id + 1, Status: task.Succeeded, TerminalTxnID: 1}
					case "digest":
						out.Commit = finalreport.Receipt{TaskID: id, Status: task.Succeeded, TerminalTxnID: 1, ReportDigest: "bad"}
					}
					return out, nil
				}
			}
			out, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1})
			if err == nil || out.Complete || out.Visited != 0 || len(out.Items) != 0 {
				t.Fatal(out, err)
			}
		})
	}
}

func TestAIL1ReportQueueRacesUnknownAndContext(t *testing.T) {
	for _, race := range []error{task.ErrNotReady, task.ErrLease, points.ErrClosed} {
		q, _, runner, _ := queueFixture(t)
		runner.fn = func(context.Context, int64, int64) (AIL1ReportReceipt, error) { return AIL1ReportReceipt{}, race }
		out, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1})
		if err != nil || out.Deferred != 1 || out.TerminalObserved != 0 || len(out.Items) != 0 {
			t.Fatal(out, err)
		}
	}
	qMissing, _, missingRunner, _ := queueFixture(t)
	missingRunner.fn = func(context.Context, int64, int64) (AIL1ReportReceipt, error) {
		return AIL1ReportReceipt{}, points.ErrNotFound
	}
	if out, err := qMissing.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); !errors.Is(err, points.ErrNotFound) || out.Complete || out.Visited != 0 {
		t.Fatal("缺失依赖被误当竞争", out, err)
	}
	q, scan, runner, st := queueFixture(t)
	runner.fn = func(_ context.Context, _ int64, id int64) (AIL1ReportReceipt, error) {
		return AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: id, TaskStatus: task.Running, Outcome: "response_unknown"}}, nil
	}
	out, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1})
	if err != nil || out.Pending != 1 || out.TerminalObserved != 0 || runner.calls != 1 {
		t.Fatal(out, err)
	}
	st.v.NotBefore = st.v.NotBefore.Add(time.Hour)
	if out, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); err != nil || out.Deferred != 1 || runner.calls != 1 {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := scan.queries
	if _, err := q.RunBatch(ctx, task.MemberQueueOptions{MaxTasks: 1}); !errors.Is(err, context.Canceled) || scan.queries != before {
		t.Fatal(err)
	}
	q.mu.Lock()
	if _, err := q.RunBatch(context.Background(), task.MemberQueueOptions{MaxTasks: 1}); !errors.Is(err, task.ErrNotReady) {
		t.Fatal(err)
	}
	q.mu.Unlock()
	for _, o := range []task.MemberQueueOptions{{}, {AfterID: -1, MaxTasks: 1}, {MaxTasks: 101}} {
		if _, err := q.RunBatch(context.Background(), o); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
}

func TestAIL1ReportQueueCancelledEmptyWindow(t *testing.T) {
	q, scan, _, _ := queueFixture(t)
	scan.window.UpperID = 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scan.rangeHook = cancel
	out, err := q.RunBatch(ctx, task.MemberQueueOptions{MaxTasks: 1})
	if !errors.Is(err, context.Canceled) || out.Complete {
		t.Fatal("取消后的空窗口误报完成", out, err)
	}
}

func TestAIL1ReportQueueReceiptWhitelist(t *testing.T) {
	for _, status := range []int{task.Succeeded, task.Failed, task.Cancelled, task.Expired} {
		out := AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: 1, TaskStatus: status, Outcome: "terminal"}}
		if status == task.Succeeded || status == task.Failed {
			out.Commit = finalreport.Receipt{TaskID: 1, Status: status, TerminalTxnID: 9}
			if status == task.Succeeded {
				out.Commit.ReportDigest = strings.Repeat("a", 64)
			}
		}
		if err := validateQueueResult(1, out); err != nil {
			t.Fatal(out, err)
		}
	}
	for _, digest := range []string{strings.Repeat("a", 63), strings.Repeat("A", 64), strings.Repeat("g", 64)} {
		if queueDigestValid(digest) {
			t.Fatal("非法摘要被放行")
		}
	}
	for _, kind := range []string{"status", "unknown_status", "step", "commit_status", "failed_digest", "unknown_commit"} {
		out := AIL1ReportReceipt{Task: task.RecoveryResult{TaskID: 1, TaskStatus: task.Succeeded, Outcome: "terminal"}}
		switch kind {
		case "status":
			out.Task.TaskStatus = task.Queued
		case "unknown_status":
			out.Task.Outcome = "response_unknown"
		case "step":
			out.Task.CurrentStep = task.MaxSteps + 1
		case "commit_status":
			out.Commit = finalreport.Receipt{TaskID: 1, Status: task.Cancelled, TerminalTxnID: 9}
		case "failed_digest":
			out.Task.TaskStatus = task.Failed
			out.Commit = finalreport.Receipt{TaskID: 1, Status: task.Failed, TerminalTxnID: 9, ReportDigest: strings.Repeat("a", 64)}
		case "unknown_commit":
			out.Task.TaskStatus = task.Running
			out.Task.Outcome = "response_unknown"
			out.Commit = finalreport.Receipt{TaskID: 1, Status: task.Succeeded, TerminalTxnID: 9, ReportDigest: strings.Repeat("a", 64)}
		}
		if validateQueueResult(1, out) == nil {
			t.Fatal("异常回执放行", kind)
		}
	}
}
