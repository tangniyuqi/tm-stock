package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AITaskExpiryService 仅清扫已有任务的期限终态，不抢占、不重发调用、不清预算。
type AITaskExpiryService struct {
	tasks task.Expirer
	scan  task.ExpiryScanner
}

func NewAITaskExpiryService(tasks task.Expirer, scan task.ExpiryScanner) (*AITaskExpiryService, error) {
	if tasks == nil || scan == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskExpiryService{tasks: tasks, scan: scan}, nil
}

func (s *AITaskExpiryService) Sweep(ctx context.Context, opts task.ExpiryOptions) (task.ExpirySweep, error) {
	var out task.ExpirySweep
	if s == nil || s.tasks == nil || s.scan == nil || opts.Validate() != nil {
		return out, points.ErrInvalid
	}
	// 即便调用方漏设期限，也不能无界占用数据库或持续释放。
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	rangeHint, err := s.scan.ExpiryRange(ctx)
	if err != nil {
		return out, fmt.Errorf("读取过期清扫范围失败: %w", err)
	}
	if rangeHint.UpperID < opts.AfterID || rangeHint.AsOf.UnixMilli() <= 0 || rangeHint.AsOf.Nanosecond()%int(time.Millisecond) != 0 {
		return out, points.ErrInvariant
	}
	out.UpperID, out.AsOf, out.LastID = rangeHint.UpperID, rangeHint.AsOf.UnixMilli(), opts.AfterID
	for out.LastID < out.UpperID && out.Visited < opts.MaxTasks {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		limit := min(opts.PageSize, opts.MaxTasks-out.Visited)
		page, err := s.scan.ExpiryPage(ctx, out.LastID, out.UpperID, limit)
		if err != nil {
			return out, fmt.Errorf("读取过期清扫分页失败: %w", err)
		}
		if len(page) == 0 || len(page) > limit {
			return out, points.ErrInvariant
		}
		previous := out.LastID
		for _, v := range page {
			if v.ID <= previous || v.ID > out.UpperID || v.UserID <= 0 || v.Status < task.Queued || v.Status > task.Expired || v.Deadline.UnixMilli() <= 0 || v.Deadline.Nanosecond()%int(time.Millisecond) != 0 {
				return out, points.ErrInvariant
			}
			previous = v.ID
		}
		for _, v := range page {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			expired := false
			if (v.Status == task.Queued || v.Status == task.Running) && !v.Deadline.After(rangeHint.AsOf) {
				// Expire 自行按账户→任务→冻结分摊顺序锁定，并重读数据库 UTC。
				confirmed, err := s.tasks.Expire(ctx, v.UserID, v.ID)
				if err != nil && !errors.Is(err, task.ErrNotReady) {
					return out, fmt.Errorf("终结过期任务失败 task=%d: %w", v.ID, err)
				}
				if err == nil {
					if confirmed.ID != v.ID || confirmed.UserID != v.UserID || !confirmed.Terminal() {
						return out, points.ErrInvariant
					}
					expired = confirmed.Status == task.Expired
				}
				// 时钟回拨导致尚未到期时跳过；绝不提前释放，下一次从零扫描再查。
			}
			if expired {
				out.ExpiredObserved++
			} else {
				out.Skipped++
			}
			out.Visited++
			out.LastID = v.ID
		}
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Complete = out.LastID == out.UpperID
	return out, nil
}
