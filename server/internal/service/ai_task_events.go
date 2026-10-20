package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func taskEventProjection(row task.EventRecord, v task.Task) (dto.AITaskEvent, error) {
	if row.TaskID != v.ID || row.Seq < 1 || row.Seq > v.EventSeq || row.Step < 0 || row.Step > task.MaxSteps || row.Step > v.CurrentStep || row.CreatedAt.IsZero() || row.CreatedAt.UnixMilli() <= 0 || row.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 {
		return dto.AITaskEvent{}, points.ErrInvariant
	}
	expected := 0
	switch row.Type {
	case "created":
		expected = task.Queued
		if row.Seq != 1 || row.Step != 0 {
			return dto.AITaskEvent{}, points.ErrInvariant
		}
	case "retried":
		expected = task.Queued
	case "claimed", "cancel_requested":
		expected = task.Running
	case "step_completed":
		expected = task.Running
		if row.Step < 1 {
			return dto.AITaskEvent{}, points.ErrInvariant
		}
	case "succeeded":
		expected = task.Succeeded
	case "failed":
		expected = task.Failed
	case "cancelled":
		expected = task.Cancelled
	case "expired":
		expected = task.Expired
	default:
		return dto.AITaskEvent{}, points.ErrInvariant
	}
	if row.Status != expected || (row.Seq == 1 && row.Type != "created") {
		return dto.AITaskEvent{}, points.ErrInvariant
	}
	validError := row.ErrorCode == ""
	switch row.Status {
	case task.Failed:
		validError = task.ValidFailure(row.ErrorCode)
	case task.Cancelled:
		validError = row.ErrorCode == "cancelled"
	case task.Expired:
		validError = row.ErrorCode == "expired"
	}
	if !validError || (row.Status >= task.Succeeded && row.Seq != v.EventSeq) {
		return dto.AITaskEvent{}, points.ErrInvariant
	}
	if row.Seq == v.EventSeq && (row.Status != v.Status || row.Step != v.CurrentStep || row.ErrorCode != v.ErrorCode) {
		return dto.AITaskEvent{}, points.ErrInvariant
	}
	return dto.AITaskEvent{Seq: row.Seq, Type: row.Type, Status: row.Status, Step: row.Step, ErrorCode: row.ErrorCode, At: row.CreatedAt.UnixMilli()}, nil
}

func (s *AITaskReadService) Events(ctx context.Context, uid, id, after int64, limit int) (dto.AITaskEventPage, error) {
	if s == nil || s.store == nil || uid <= 0 || id <= 0 || after < 0 || limit < 1 || limit > 100 {
		return dto.AITaskEventPage{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out := dto.AITaskEventPage{Items: make([]dto.AITaskEvent, 0), NextAfterSeq: after}
	err := s.store.WithinTaskRead(ctx, func(tx task.ReadTx) error {
		v, err := tx.Task(ctx, uid, id)
		if err != nil {
			return err
		}
		if v.ID != id || v.CurrentStep > task.MaxSteps {
			return points.ErrInvariant
		}
		if _, err := taskProgress(v, uid); err != nil {
			return err
		}
		if after > v.EventSeq {
			return points.ErrInvalid
		}
		out.UpperSeq = v.EventSeq
		rows, err := tx.Events(ctx, uid, id, after, v.EventSeq, limit+1)
		if err != nil {
			return err
		}
		if int64(len(rows)) != min(v.EventSeq-after, int64(limit+1)) {
			return points.ErrInvariant
		}
		previous, lastStep := after, -1
		for index, row := range rows {
			if row.Seq != previous+1 || row.Step < lastStep {
				return points.ErrInvariant
			}
			item, err := taskEventProjection(row, v)
			if err != nil {
				return err
			}
			previous, lastStep = row.Seq, row.Step
			if index < limit {
				out.Items = append(out.Items, item)
				out.NextAfterSeq = row.Seq
			}
		}
		out.HasMore = out.NextAfterSeq < v.EventSeq
		return nil
	})
	if err != nil {
		return dto.AITaskEventPage{}, err
	}
	return out, nil
}
