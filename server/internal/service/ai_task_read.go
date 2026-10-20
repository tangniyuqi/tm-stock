package service

import (
	"context"
	"math"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type reportAvailability interface {
	Available(context.Context, int64, int64) (bool, error)
}
type AITaskReadService struct {
	store   task.ReadStore
	reports reportAvailability
}

func NewAITaskReadService(store task.ReadStore) (*AITaskReadService, error) {
	if store == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskReadService{store: store}, nil
}

// NewAITaskReadServiceWithReports 只增强详情可用性；列表仍不联表、不推导报告状态。
func NewAITaskReadServiceWithReports(store task.ReadStore, reports reportAvailability) (*AITaskReadService, error) {
	if reports == nil {
		return nil, points.ErrInvalid
	}
	s, err := NewAITaskReadService(store)
	if err != nil {
		return nil, err
	}
	s.reports = reports
	return s, nil
}

func taskProgress(v task.Task, uid int64) (dto.AITaskProgress, error) {
	if v.ID <= 0 || v.UserID != uid || v.SubjectID <= 0 || (v.SubjectType != task.Theme && v.SubjectType != task.Topic) || (v.AnalysisType != 1 && v.AnalysisType != 2) || v.Status < task.Queued || v.Status > task.Expired || v.CurrentStep < 0 || v.EventSeq < 1 || v.Deadline.IsZero() {
		return dto.AITaskProgress{}, points.ErrInvariant
	}
	switch v.Status {
	case task.Failed:
		if !task.ValidFailure(v.ErrorCode) {
			return dto.AITaskProgress{}, points.ErrInvariant
		}
	case task.Cancelled:
		if v.ErrorCode != "cancelled" {
			return dto.AITaskProgress{}, points.ErrInvariant
		}
	case task.Expired:
		if v.ErrorCode != "expired" {
			return dto.AITaskProgress{}, points.ErrInvariant
		}
	default:
		if v.ErrorCode != "" {
			return dto.AITaskProgress{}, points.ErrInvariant
		}
	}
	// 成功状态不证明当前可见；只有详情的独立终检/来源/发布核验可以返回true。
	return dto.AITaskProgress{ID: v.ID, AnalysisType: v.AnalysisType, SubjectType: v.SubjectType, SubjectID: v.SubjectID, Status: v.Status, CurrentStep: v.CurrentStep, EventSeq: v.EventSeq, CancelRequested: v.CancelRequested, ErrorCode: v.ErrorCode, DeadlineAt: v.Deadline.UnixMilli(), ReportAvailable: false}, nil
}

func (s *AITaskReadService) Progress(ctx context.Context, uid, id int64) (dto.AITaskProgress, error) {
	if s == nil || uid <= 0 || id <= 0 {
		return dto.AITaskProgress{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var out dto.AITaskProgress
	err := s.store.WithinTaskRead(ctx, func(tx task.ReadTx) error {
		v, err := tx.Task(ctx, uid, id)
		if err != nil {
			return err
		}
		if v.ID != id {
			return points.ErrInvariant
		}
		out, err = taskProgress(v, uid)
		return err
	})
	if err != nil {
		return dto.AITaskProgress{}, err
	}
	if out.Status == task.Succeeded && s.reports != nil {
		out.ReportAvailable, err = s.reports.Available(ctx, uid, id)
		if err != nil {
			return dto.AITaskProgress{}, err
		}
	}
	return out, nil
}

func (s *AITaskReadService) History(ctx context.Context, uid, before int64, limit int) (dto.AITaskHistory, error) {
	if s == nil || uid <= 0 || before < 0 || limit < 1 || limit > 100 {
		return dto.AITaskHistory{}, points.ErrInvalid
	}
	if before == 0 {
		before = math.MaxInt64
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out := dto.AITaskHistory{Items: make([]dto.AITaskSummary, 0)}
	err := s.store.WithinTaskRead(ctx, func(tx task.ReadTx) error {
		rows, err := tx.History(ctx, uid, before, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit+1 {
			return points.ErrInvariant
		}
		previous := before
		for i, row := range rows {
			if row.ID >= previous {
				return points.ErrInvariant
			}
			previous = row.ID
			v, err := taskProgress(row, uid)
			if err != nil {
				return err
			}
			if i < limit {
				out.Items = append(out.Items, dto.AITaskSummary{ID: v.ID, AnalysisType: v.AnalysisType, SubjectType: v.SubjectType, SubjectID: v.SubjectID, Status: v.Status, DeadlineAt: v.DeadlineAt, ReportAvailable: v.ReportAvailable})
			}
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			out.NextBeforeID = out.Items[len(out.Items)-1].ID
		}
		return nil
	})
	if err != nil {
		return dto.AITaskHistory{}, err
	}
	return out, nil
}
