package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// RunLoop 串行执行原有限批次。每轮扫描结束后从零复核，游标不是永久消费位置。
// 未知结果或错误立即交还调用者；不在循环内重试外部执行、不修改任务或账务。
func (q *AIL1ReportQueue) RunLoop(ctx context.Context, maxTasks int, interval time.Duration) error {
	return runReportQueueLoop(ctx, maxTasks, interval, q.RunBatch, func(ctx context.Context, d time.Duration) error {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	})
}

func runReportQueueLoop(ctx context.Context, maxTasks int, interval time.Duration, batch func(context.Context, task.MemberQueueOptions) (task.MemberQueueReceipt, error), wait func(context.Context, time.Duration) error) error {
	opts := task.MemberQueueOptions{MaxTasks: maxTasks}
	if ctx == nil || opts.Validate() != nil || interval < time.Second || interval > time.Minute || batch == nil || wait == nil {
		return points.ErrInvalid
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, err := batch(ctx, opts)
		if err != nil {
			return err
		}
		if out.Pending > 0 {
			return task.ErrNotReady
		}
		if out.LastID < opts.AfterID || out.LastID > out.UpperID || out.Complete != (out.LastID == out.UpperID) || !out.Complete && out.LastID == opts.AfterID {
			return points.ErrInvariant
		}
		opts.AfterID = out.LastID
		if out.Complete {
			opts.AfterID = 0
		}
		// 每批均等待，防止大队列或空队列占满数据库；等待可被进程取消打断。
		if err := wait(ctx, interval); err != nil {
			return err
		}
	}
}
