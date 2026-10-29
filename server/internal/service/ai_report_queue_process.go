package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// RunAIReportQueues 管理轻度和深度循环的共同生命周期，不构造批准配置或启动真实调用。
// 任一路退出即取消其余循环并等待收尾；不重启、不补发、不自行释放未知任务。
func RunAIReportQueues(ctx context.Context, light, deep *AIL1ReportQueue, maxTasks int, interval time.Duration) error {
	if ctx == nil || light == nil || deep == nil || light == deep || light.deep || !deep.deep || !light.enabled || !deep.enabled || (task.MemberQueueOptions{MaxTasks: maxTasks}).Validate() != nil || interval < time.Second || interval > time.Minute {
		return points.ErrInvalid
	}
	for _, q := range []*AIL1ReportQueue{light, deep} {
		if q.runner == nil || q.store == nil || q.scan == nil {
			return points.ErrInvalid
		}
	}
	return runAIReportQueues(ctx,
		func(ctx context.Context) error { return light.RunLoop(ctx, maxTasks, interval) },
		func(ctx context.Context) error { return deep.RunLoop(ctx, maxTasks, interval) })
}

func runAIReportQueues(parent context.Context, light, deep func(context.Context) error) error {
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	done := make(chan error, 2)
	for _, run := range []func(context.Context) error{light, deep} {
		go func(run func(context.Context) error) { done <- run(ctx) }(run)
	}
	err := <-done
	cancel()
	<-done // 必须等另一循环退出，不能在它仍可能执行时宣告进程已停止。
	if err == nil {
		return points.ErrInvariant
	}
	return err
}
