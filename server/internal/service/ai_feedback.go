package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

type AIFeedbackService struct{ store feedback.Store }

func NewAIFeedbackService(store feedback.Store) (*AIFeedbackService, error) {
	if store == nil {
		return nil, feedback.ErrInvalid
	}
	return &AIFeedbackService{store}, nil
}
func validateFeedback(ctx context.Context, tx feedback.Tx, r feedback.Record) error {
	es, err := tx.History(ctx, r.ID)
	if err != nil {
		return err
	}
	return feedback.ValidateHistory(r, es)
}
func (s *AIFeedbackService) Submit(parent context.Context, uid, id int64, category string) (dto.AIFeedback, error) {
	if s == nil || s.store == nil || uid <= 0 || !feedback.ID(id) || !feedback.Category(category) {
		return dto.AIFeedback{}, feedback.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	var out dto.AIFeedback
	err := s.store.Within(ctx, func(tx feedback.Tx) error {
		digest, err := tx.OwnedReport(ctx, uid, id)
		if err != nil {
			return err
		}
		r, err := tx.ByCategory(ctx, uid, id, category)
		if errors.Is(err, feedback.ErrNotFound) {
			now, err := tx.Now(ctx)
			if err != nil {
				return err
			}
			r, err = tx.Insert(ctx, feedback.Record{TaskID: id, UserID: uid, Category: category, ReportDigest: digest, Status: "received", Version: 1, CreatedAt: now, UpdatedAt: now})
			if err != nil {
				return err
			}
			if err = tx.Append(ctx, feedback.Event{FeedbackID: r.ID, Version: 1, Status: "received", At: now}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if r.UserID != uid || r.TaskID != id || r.Category != category || r.ReportDigest != digest {
			return feedback.ErrInvariant
		}
		if err = validateFeedback(ctx, tx, r); err != nil {
			return err
		}
		out = dto.NewAIFeedback(r)
		return nil
	})
	if err != nil {
		return dto.AIFeedback{}, err
	}
	return out, nil
}
func (s *AIFeedbackService) ForReport(parent context.Context, uid, id int64) (dto.AIFeedbackList, error) {
	if s == nil || s.store == nil || uid <= 0 || !feedback.ID(id) {
		return dto.AIFeedbackList{}, feedback.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	out := dto.AIFeedbackList{TaskID: id, Items: []dto.AIFeedback{}}
	err := s.store.Within(ctx, func(tx feedback.Tx) error {
		digest, err := tx.OwnedReport(ctx, uid, id)
		if err != nil {
			return err
		}
		rows, err := tx.ForReport(ctx, uid, id)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		var previous int64
		for _, r := range rows {
			if !r.Valid() || r.UserID != uid || r.TaskID != id || r.ReportDigest != digest || seen[r.Category] || r.ID <= previous || len(rows) > 4 {
				return feedback.ErrInvariant
			}
			if err = validateFeedback(ctx, tx, r); err != nil {
				return err
			}
			seen[r.Category] = true
			previous = r.ID
			out.Items = append(out.Items, dto.NewAIFeedback(r))
		}
		return nil
	})
	if err != nil {
		return dto.AIFeedbackList{}, err
	}
	return out, nil
}
func (s *AIFeedbackService) List(parent context.Context, actor, after int64, limit int) (dto.AIFeedbackPage, error) {
	if s == nil || s.store == nil || actor <= 0 || after < 0 || after > feedback.MaxID || limit < 1 || limit > 100 {
		return dto.AIFeedbackPage{}, feedback.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	out := dto.AIFeedbackPage{Items: []dto.AIFeedback{}}
	err := s.store.Within(ctx, func(tx feedback.Tx) error {
		rows, err := tx.List(ctx, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit+1 {
			return feedback.ErrInvariant
		}
		previous := after
		for i, r := range rows {
			if !r.Valid() || r.ID <= previous {
				return feedback.ErrInvariant
			}
			previous = r.ID
			if err = validateFeedback(ctx, tx, r); err != nil {
				return err
			}
			if i < limit {
				out.Items = append(out.Items, dto.NewAIFeedback(r))
			}
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			out.NextAfterID = out.Items[len(out.Items)-1].ID
		}
		return nil
	})
	if err != nil {
		return dto.AIFeedbackPage{}, err
	}
	return out, nil
}
func (s *AIFeedbackService) Detail(parent context.Context, actor, id int64) (dto.AIInternalFeedback, error) {
	if s == nil || s.store == nil || actor <= 0 || !feedback.ID(id) {
		return dto.AIInternalFeedback{}, feedback.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	var out dto.AIInternalFeedback
	err := s.store.Within(ctx, func(tx feedback.Tx) error {
		r, err := tx.ByID(ctx, id)
		if err != nil {
			return err
		}
		es, err := tx.History(ctx, id)
		if err != nil {
			return err
		}
		if err = feedback.ValidateHistory(r, es); err != nil {
			return err
		}
		out.Item = dto.NewAIFeedback(r)
		out.Events = []dto.AIFeedbackEvent{}
		for _, e := range es {
			out.Events = append(out.Events, dto.AIFeedbackEvent{Version: e.Version, ActorID: e.ActorID, Status: e.Status, Resolution: e.Resolution, At: e.At.UnixMilli()})
		}
		return nil
	})
	if err != nil {
		return dto.AIInternalFeedback{}, err
	}
	return out, nil
}
func (s *AIFeedbackService) Decide(parent context.Context, actor, id int64, d feedback.Decision) (dto.AIFeedback, error) {
	if s == nil || s.store == nil || actor <= 0 || !feedback.ID(id) || !d.Valid() {
		return dto.AIFeedback{}, feedback.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	var out dto.AIFeedback
	err := s.store.Within(ctx, func(tx feedback.Tx) error {
		r, err := tx.ByID(ctx, id)
		if err != nil {
			return err
		}
		es, err := tx.History(ctx, id)
		if err != nil {
			return err
		}
		if err = feedback.ValidateHistory(r, es); err != nil {
			return err
		}
		if r.Version > d.ExpectedVersion {
			e := es[d.ExpectedVersion]
			if e.ActorID != actor || e.Status != d.Status || e.Resolution != d.Resolution {
				return feedback.ErrConflict
			}
			out = dto.NewAIFeedback(r)
			return nil
		}
		if r.Version != d.ExpectedVersion || !feedback.Transition(r.Status, d.Status) {
			return feedback.ErrConflict
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		next := r
		next.Status = d.Status
		next.Resolution = d.Resolution
		next.Version++
		next.UpdatedAt = now
		if !next.Valid() {
			return feedback.ErrInvariant
		}
		if err = tx.Update(ctx, r, next); err != nil {
			return err
		}
		if err = tx.Append(ctx, feedback.Event{FeedbackID: id, Version: next.Version, ActorID: actor, Status: next.Status, Resolution: next.Resolution, At: now}); err != nil {
			return err
		}
		if err = validateFeedback(ctx, tx, next); err != nil {
			return err
		}
		out = dto.NewAIFeedback(next)
		return nil
	})
	if err != nil {
		return dto.AIFeedback{}, err
	}
	return out, nil
}
