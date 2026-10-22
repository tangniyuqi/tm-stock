package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type AIConversationService struct{ store conversation.Store }

func NewAIConversationService(store conversation.Store) (*AIConversationService, error) {
	if store == nil {
		return nil, conversation.ErrInvalid
	}
	return &AIConversationService{store}, nil
}
func (s *AIConversationService) within(parent context.Context, write bool, fn func(context.Context, conversation.Tx) error) error {
	if s == nil || s.store == nil || parent == nil || fn == nil {
		return conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	return s.store.Within(ctx, write, func(tx conversation.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
}
func ownedConversation(ctx context.Context, tx conversation.Tx, uid, id int64, lock bool) (conversation.Record, error) {
	r, err := tx.ByID(ctx, uid, id, lock)
	if err != nil {
		return conversation.Record{}, err
	}
	if !r.Valid() || r.UserID != uid || r.ID != id {
		return conversation.Record{}, conversation.ErrInvariant
	}
	return r, nil
}

func (s *AIConversationService) Create(ctx context.Context, uid int64, key, title string) (dto.AIConversationReceipt, error) {
	if !conversation.ID(uid) || !points.ValidKey(key) || !conversation.Text(title, 40) {
		return dto.AIConversationReceipt{}, conversation.ErrInvalid
	}
	hash := conversation.CreationHash(title)
	var out dto.AIConversationReceipt
	err := s.within(ctx, true, func(ctx context.Context, tx conversation.Tx) error {
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if !conversation.Time(now) {
			return conversation.ErrInvariant
		}
		r, err := tx.CreateOrGet(ctx, conversation.Record{UserID: uid, Key: key, RequestHash: hash, Title: title, Version: 1, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		if !r.Valid() || r.UserID != uid || r.Key != key {
			return conversation.ErrInvariant
		}
		if r.RequestHash != hash {
			return conversation.ErrConflict
		}
		out = dto.AIConversationReceipt{ID: r.ID, CreatedAt: r.CreatedAt.UnixMilli()}
		return nil
	})
	if err != nil {
		return dto.AIConversationReceipt{}, err
	}
	return out, nil
}
func (s *AIConversationService) Receipt(ctx context.Context, uid int64, key string) (dto.AIConversationReceipt, error) {
	if !conversation.ID(uid) || !points.ValidKey(key) {
		return dto.AIConversationReceipt{}, conversation.ErrInvalid
	}
	var out dto.AIConversationReceipt
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.Tx) error {
		r, err := tx.ByKey(ctx, uid, key)
		if err != nil {
			return err
		}
		if !r.Valid() || r.UserID != uid || r.Key != key {
			return conversation.ErrInvariant
		}
		out = dto.AIConversationReceipt{ID: r.ID, CreatedAt: r.CreatedAt.UnixMilli()}
		return nil
	})
	if err != nil {
		return dto.AIConversationReceipt{}, err
	}
	return out, nil
}
func (s *AIConversationService) Detail(ctx context.Context, uid, id int64) (dto.AIConversation, error) {
	if !conversation.ID(uid) || !conversation.ID(id) {
		return dto.AIConversation{}, conversation.ErrInvalid
	}
	var out dto.AIConversation
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.Tx) error {
		r, err := ownedConversation(ctx, tx, uid, id, false)
		if err != nil {
			return err
		}
		out = dto.NewAIConversation(r)
		return nil
	})
	if err != nil {
		return dto.AIConversation{}, err
	}
	return out, nil
}
func (s *AIConversationService) List(ctx context.Context, uid, before int64, archived bool, limit int) (dto.AIConversationPage, error) {
	if !conversation.ID(uid) || before < 0 || before > conversation.MaxID || limit < 1 || limit > 100 {
		return dto.AIConversationPage{}, conversation.ErrInvalid
	}
	upper := before
	if upper == 0 {
		upper = conversation.MaxID + 1
	}
	out := dto.AIConversationPage{Items: []dto.AIConversationSummary{}}
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.Tx) error {
		rows, err := tx.List(ctx, uid, upper, archived, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit+1 {
			return conversation.ErrInvariant
		}
		previous := upper
		for i, r := range rows {
			if !r.Valid() || r.UserID != uid || r.ID >= previous || r.Archived != archived {
				return conversation.ErrInvariant
			}
			previous = r.ID
			if i < limit {
				out.Items = append(out.Items, dto.AIConversationSummary{ID: r.ID, Title: r.Title, Archived: r.Archived, Version: r.Version, UpdatedAt: r.UpdatedAt.UnixMilli()})
			}
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			out.NextBeforeID = out.Items[len(out.Items)-1].ID
		}
		return nil
	})
	if err != nil {
		return dto.AIConversationPage{}, err
	}
	return out, nil
}

func (s *AIConversationService) mutate(ctx context.Context, uid, id, version int64, title string, archive bool) (dto.AIConversation, error) {
	if !conversation.ID(uid) || !conversation.ID(id) || !conversation.ID(version) || !archive && !conversation.Text(title, 40) {
		return dto.AIConversation{}, conversation.ErrInvalid
	}
	var out dto.AIConversation
	err := s.within(ctx, true, func(ctx context.Context, tx conversation.Tx) error {
		old, err := ownedConversation(ctx, tx, uid, id, true)
		if err != nil {
			return err
		}
		if old.Version != version || old.Archived || old.Version == conversation.MaxID {
			return conversation.ErrConflict
		}
		next := old
		if archive {
			next.Archived = true
		} else {
			next.Title = title
		}
		if next.Title == old.Title && next.Archived == old.Archived {
			out = dto.NewAIConversation(old)
			return nil
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if !conversation.Time(now) {
			return conversation.ErrInvariant
		}
		if now.Before(old.UpdatedAt) {
			now = old.UpdatedAt
		}
		next.Version++
		next.UpdatedAt = now
		if err := tx.Update(ctx, old, next); err != nil {
			return err
		}
		out = dto.NewAIConversation(next)
		return nil
	})
	if err != nil {
		return dto.AIConversation{}, err
	}
	return out, nil
}
func (s *AIConversationService) Rename(ctx context.Context, uid, id, version int64, title string) (dto.AIConversation, error) {
	return s.mutate(ctx, uid, id, version, title, false)
}
func (s *AIConversationService) Archive(ctx context.Context, uid, id, version int64) (dto.AIConversation, error) {
	return s.mutate(ctx, uid, id, version, "", true)
}

func (s *AIConversationService) Messages(ctx context.Context, uid, id, after int64, limit int) (dto.AIConversationMessages, error) {
	if !conversation.ID(uid) || !conversation.ID(id) || after < 0 || after > conversation.MaxID || limit < 1 || limit > 100 {
		return dto.AIConversationMessages{}, conversation.ErrInvalid
	}
	out := dto.AIConversationMessages{ConversationID: id, Items: []dto.AIConversationMessage{}, NextAfterSeq: after}
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.Tx) error {
		r, err := ownedConversation(ctx, tx, uid, id, false)
		if err != nil {
			return err
		}
		if after > r.LastSeq {
			return conversation.ErrInvalid
		}
		out.UpperSeq = r.LastSeq
		rows, err := tx.Messages(ctx, uid, id, after, r.LastSeq, limit+1)
		if err != nil {
			return err
		}
		if int64(len(rows)) != min(r.LastSeq-after, int64(limit+1)) {
			return conversation.ErrInvariant
		}
		for i, m := range rows {
			if !m.Valid() || m.UserID != uid || m.ConversationID != id || m.Seq != after+int64(i)+1 || m.CreatedAt.Before(r.CreatedAt) || m.CreatedAt.After(r.UpdatedAt) {
				return conversation.ErrInvariant
			}
			if i < limit {
				v := m.Input
				out.Items = append(out.Items, dto.AIConversationMessage{Seq: m.Seq, Kind: v.Kind, SubjectType: v.SubjectType, SubjectID: v.SubjectID, Skill: v.Skill, Question: v.Question, TaskID: v.TaskID, CreatedAt: m.CreatedAt.UnixMilli()})
				out.NextAfterSeq = m.Seq
			}
		}
		out.HasMore = out.NextAfterSeq < out.UpperSeq
		return nil
	})
	if err != nil {
		return dto.AIConversationMessages{}, err
	}
	return out, nil
}
