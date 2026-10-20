package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type memberTaskCanceller interface {
	Cancel(context.Context, int64, int64) (task.Task, error)
}

type AITaskCancelService struct{ tasks memberTaskCanceller }

func NewAITaskCancelService(tasks memberTaskCanceller) (*AITaskCancelService, error) {
	if tasks == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskCancelService{tasks: tasks}, nil
}

func (s *AITaskCancelService) Cancel(ctx context.Context, uid, id int64) (dto.AITaskCancellation, error) {
	if s == nil || s.tasks == nil || uid <= 0 || id <= 0 {
		return dto.AITaskCancellation{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	v, err := s.tasks.Cancel(ctx, uid, id)
	if err != nil {
		return dto.AITaskCancellation{}, err
	}
	// 取消可能已提交，但父请求已失效时不能把候选收据当成已确认成功。
	if err := ctx.Err(); err != nil {
		return dto.AITaskCancellation{}, err
	}
	if v.ID != id || (!v.Terminal() && (v.Status != task.Running || !v.CancelRequested)) {
		return dto.AITaskCancellation{}, points.ErrInvariant
	}
	p, err := taskProgress(v, uid)
	if err != nil {
		return dto.AITaskCancellation{}, err
	}
	return dto.AITaskCancellation{ID: p.ID, Status: p.Status, EventSeq: p.EventSeq, CancelRequested: p.CancelRequested, ErrorCode: p.ErrorCode, ReportAvailable: false}, nil
}
