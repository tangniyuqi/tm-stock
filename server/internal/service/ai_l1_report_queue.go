package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type l1QueueRunner interface {
	Run(context.Context, int64, int64) (AIL1ReportReceipt, error)
}

// AIL1ReportQueue 复用原执行核；构造不启动服务，不创建任务，不开启真实调用。
type AIL1ReportQueue struct {
	runner  l1QueueRunner
	store   task.AdmissionStore
	scan    task.MemberQueueScanner
	enabled bool
	deep    bool
	mu      sync.Mutex
}

func NewAIL1ReportQueue(worker *AIL1ReportWorker, scan task.MemberQueueScanner, enabled bool) (*AIL1ReportQueue, error) {
	if worker == nil || worker.store == nil || scan == nil || enabled && !worker.opts.Enabled {
		return nil, points.ErrInvalid
	}
	return &AIL1ReportQueue{runner: worker, store: worker.store, scan: scan, enabled: enabled}, nil
}

func queueTimeValid(at time.Time) bool {
	return at.UnixMilli() > 0 && at.Nanosecond()%int(time.Millisecond) == 0
}

// NewAIL2ReportQueue 显式选择深度任务，沿用原准入复核、租约和执行核。
func NewAIL2ReportQueue(worker *AIL1ReportWorker, scan task.MemberQueueScanner, enabled bool) (*AIL1ReportQueue, error) {
	q, err := NewAIL1ReportQueue(worker, scan, enabled)
	if err != nil {
		return nil, err
	}
	q.deep = true
	return q, nil
}

func (q *AIL1ReportQueue) accepts(analysisType int) bool {
	return !q.deep && analysisType == task.LightAnalysisType || q.deep && analysisType == task.DeepAnalysisType
}

func (q *AIL1ReportQueue) memberQueueDue(v task.MemberQueueCandidate, at time.Time) bool {
	if !v.HasAdmission || !q.accepts(v.AnalysisType) || v.SubjectType != task.Theme || v.Status != task.Queued && v.Status != task.Running {
		return false
	}
	return !v.Deadline.After(at) || v.Status == task.Queued && !v.NotBefore.After(at) || v.Status == task.Running && v.LeaseUntil != nil && !v.LeaseUntil.After(at)
}

func validateQueuePage(page []task.MemberQueueCandidate, after, upper int64, limit int) error {
	if len(page) == 0 || len(page) > limit {
		return points.ErrInvariant
	}
	previous := after
	for _, v := range page {
		if v.ID <= previous || v.ID > upper || v.UserID <= 0 || v.AnalysisType != 1 && v.AnalysisType != 2 || v.SubjectType != task.Theme && v.SubjectType != task.Topic || v.Status < task.Queued || v.Status > task.Expired || !queueTimeValid(v.NotBefore) || !queueTimeValid(v.Deadline) || v.LeaseUntil != nil && !queueTimeValid(*v.LeaseUntil) || v.Status == task.Running && v.LeaseUntil == nil {
			return points.ErrInvariant
		}
		previous = v.ID
	}
	// 不可删除任务表中的最大编号必须在最后一页，不能把截断读当扫描完成。
	if len(page) < limit && previous != upper {
		return points.ErrInvariant
	}
	return nil
}

func (q *AIL1ReportQueue) currentDue(ctx context.Context, hint task.MemberQueueCandidate) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var due bool
	err := q.store.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
		v, err := lockOwnedTask(ctx, tx, hint.UserID, hint.ID)
		if err != nil {
			return err
		}
		if v.ID != hint.ID || v.UserID != hint.UserID || v.AnalysisType != hint.AnalysisType || !q.accepts(v.AnalysisType) || v.SubjectType != task.Theme {
			return points.ErrInvariant
		}
		record, found, err := tx.ReadAdmission(ctx, v)
		if err != nil {
			return err
		}
		if !found {
			return task.ErrAdmissionUnavailable
		}
		if err := record.Bind(v); err != nil {
			return err
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if !queueTimeValid(now) {
			return points.ErrInvariant
		}
		due = q.memberQueueDue(task.MemberQueueCandidate{AnalysisType: v.AnalysisType, SubjectType: v.SubjectType, Status: v.Status, NotBefore: v.NotBefore, Deadline: v.Deadline, LeaseUntil: v.LeaseUntil, HasAdmission: true}, now)
		return nil
	})
	if err != nil {
		return false, err
	}
	return due, ctx.Err()
}

func queueDigestValid(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, c := range digest {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func validateQueueResult(id int64, out AIL1ReportReceipt) error {
	r := out.Task
	if r.TaskID != id || r.ReportAvailable || r.CurrentStep < 0 || r.CurrentStep > task.MaxSteps {
		return points.ErrInvariant
	}
	switch r.Outcome {
	case "terminal":
		if r.TaskStatus < task.Succeeded || r.TaskStatus > task.Expired {
			return points.ErrInvariant
		}
	case "response_unknown":
		if r.TaskStatus != task.Running {
			return points.ErrInvariant
		}
	default:
		return points.ErrInvariant
	}
	c := out.Commit
	if c != (finalreport.Receipt{}) {
		if r.Outcome != "terminal" || c.TaskID != id || c.Status != r.TaskStatus || c.TerminalTxnID <= 0 || c.Status != task.Succeeded && c.Status != task.Failed {
			return points.ErrInvariant
		}
		if c.Status == task.Succeeded && !queueDigestValid(c.ReportDigest) || c.Status != task.Succeeded && c.ReportDigest != "" {
			return points.ErrInvariant
		}
	}
	return nil
}

// RunBatch 只进行一次有限遍历；未知事务不跨过该行，不换键/补发/释放。
func (q *AIL1ReportQueue) RunBatch(parent context.Context, opts task.MemberQueueOptions) (task.MemberQueueReceipt, error) {
	var out task.MemberQueueReceipt
	if q == nil || q.runner == nil || q.store == nil || q.scan == nil || opts.Validate() != nil {
		return out, points.ErrInvalid
	}
	if !q.enabled {
		return out, finalreport.ErrDisabled
	}
	if !q.mu.TryLock() {
		return out, task.ErrNotReady
	}
	defer q.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	window, err := q.scan.MemberQueueRange(ctx)
	if err != nil {
		return out, fmt.Errorf("读取会员编排范围失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if window.UpperID < opts.AfterID || !queueTimeValid(window.AsOf) {
		return out, points.ErrInvariant
	}
	out.UpperID, out.AsOf, out.LastID = window.UpperID, window.AsOf.UnixMilli(), opts.AfterID
	if out.LastID == out.UpperID {
		out.Complete = true
		return out, nil
	}
	page, err := q.scan.MemberQueuePage(ctx, opts.AfterID, window.UpperID, opts.MaxTasks+1)
	if err != nil {
		return out, fmt.Errorf("读取会员编排分页失败: %w", err)
	}
	if err := validateQueuePage(page, opts.AfterID, window.UpperID, opts.MaxTasks+1); err != nil {
		return out, err
	}
	for _, v := range page[:min(len(page), opts.MaxTasks)] {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if !q.memberQueueDue(v, window.AsOf) {
			out.Skipped++
		} else {
			due, err := q.currentDue(ctx, v)
			if err == nil && due {
				var result AIL1ReportReceipt
				result, err = q.runner.Run(ctx, v.UserID, v.ID)
				if err == nil {
					if err := validateQueueResult(v.ID, result); err != nil {
						return out, err
					}
					out.Items = append(out.Items, task.MemberQueueItem{TaskID: v.ID, Status: result.Task.TaskStatus, Outcome: result.Task.Outcome})
					if result.Task.Outcome == "terminal" {
						out.TerminalObserved++
					} else {
						out.Pending++
					}
				}
			} else if err == nil {
				out.Deferred++
			}
			if err != nil {
				if errors.Is(err, task.ErrNotReady) || errors.Is(err, task.ErrLease) || errors.Is(err, points.ErrClosed) {
					out.Deferred++
				} else {
					return out, fmt.Errorf("执行会员编排任务失败 task=%d: %w", v.ID, err)
				}
			}
		}
		out.Visited++
		out.LastID = v.ID
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Complete = out.LastID == out.UpperID
	return out, nil
}
