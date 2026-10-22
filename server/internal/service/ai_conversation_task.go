package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type conversationTaskCreator interface {
	Quote(context.Context, int64, dto.AITaskQuoteInput) (dto.AITaskQuote, error)
	CreateQuoted(context.Context, int64, string, dto.AITaskCreateInput, int64) (dto.AITaskCreation, error)
	Receipt(context.Context, int64, string) (dto.AITaskCreation, error)
}
type AIConversationTaskService struct {
	store   conversation.IntentStore
	creator conversationTaskCreator
	writes  bool
}

func NewAIConversationTaskService(store conversation.IntentStore, creator conversationTaskCreator, writes bool) (*AIConversationTaskService, error) {
	if store == nil || creator == nil {
		return nil, conversation.ErrInvalid
	}
	return &AIConversationTaskService{store, creator, writes}, nil
}
func (s *AIConversationTaskService) within(parent context.Context, write bool, fn func(context.Context, conversation.IntentTx) error) error {
	if s == nil || s.store == nil || parent == nil || fn == nil {
		return conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	return s.store.WithinIntents(ctx, write, func(tx conversation.IntentTx) error { return fn(ctx, tx) })
}
func ownedIntent(ctx context.Context, tx conversation.IntentTx, uid, cid, id int64, lock bool) (conversation.Intent, error) {
	i, err := tx.IntentByID(ctx, uid, cid, id, lock)
	if err != nil {
		return conversation.Intent{}, err
	}
	if !i.Valid() || i.ID != id || i.UserID != uid || i.ConversationID != cid {
		return conversation.Intent{}, conversation.ErrInvariant
	}
	return i, nil
}
func validateIntentLink(ctx context.Context, tx conversation.IntentTx, i conversation.Intent) error {
	if i.State != conversation.Linked {
		return nil
	}
	v, err := tx.IntentTask(ctx, i.UserID, i.TaskKey, false)
	if err != nil {
		return err
	}
	if v.ID != i.TaskID || !v.Matches(i) {
		return conversation.ErrInvariant
	}
	rows, err := tx.Messages(ctx, i.UserID, i.ConversationID, i.MessageSeq-1, i.MessageSeq, 1)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return conversation.ErrInvariant
	}
	m := rows[0]
	want := conversation.MessageInput{Kind: "task", SubjectType: i.Input.SubjectType, SubjectID: i.Input.SubjectID, Skill: i.Input.AnalysisType, TaskID: i.TaskID}
	if !m.Valid() || m.ConversationID != i.ConversationID || m.UserID != i.UserID || m.Seq != i.MessageSeq || m.Key != i.TaskKey || m.Input != want {
		return conversation.ErrInvariant
	}
	return nil
}
func (s *AIConversationTaskService) Detail(ctx context.Context, uid, cid, id int64) (dto.AIConversationTask, error) {
	if !conversation.ID(uid) || !conversation.ID(cid) || !conversation.ID(id) {
		return dto.AIConversationTask{}, conversation.ErrInvalid
	}
	var out dto.AIConversationTask
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.IntentTx) error {
		if _, err := ownedConversation(ctx, tx, uid, cid, false); err != nil {
			return err
		}
		i, err := ownedIntent(ctx, tx, uid, cid, id, false)
		if err != nil {
			return err
		}
		if err := validateIntentLink(ctx, tx, i); err != nil {
			return err
		}
		out = dto.NewAIConversationTask(i)
		return nil
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	return out, nil
}
func (s *AIConversationTaskService) Receipt(ctx context.Context, uid, cid int64, key string) (dto.AIConversationTask, error) {
	if !conversation.ID(uid) || !conversation.ID(cid) || !points.ValidKey(key) {
		return dto.AIConversationTask{}, conversation.ErrInvalid
	}
	var out dto.AIConversationTask
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.IntentTx) error {
		if _, err := ownedConversation(ctx, tx, uid, cid, false); err != nil {
			return err
		}
		i, err := tx.IntentByKey(ctx, uid, cid, key)
		if err != nil {
			return err
		}
		if !i.Valid() || i.UserID != uid || i.ConversationID != cid || i.Key != key {
			return conversation.ErrInvariant
		}
		if err := validateIntentLink(ctx, tx, i); err != nil {
			return err
		}
		out = dto.NewAIConversationTask(i)
		return nil
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	return out, nil
}
func (s *AIConversationTaskService) List(ctx context.Context, uid, cid, before int64, limit int) (dto.AIConversationTasks, error) {
	if !conversation.ID(uid) || !conversation.ID(cid) || before < 0 || before > conversation.MaxID || limit < 1 || limit > 100 {
		return dto.AIConversationTasks{}, conversation.ErrInvalid
	}
	upper := before
	if upper == 0 {
		upper = conversation.MaxID + 1
	}
	out := dto.AIConversationTasks{Items: []dto.AIConversationTaskSummary{}}
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.IntentTx) error {
		if _, err := ownedConversation(ctx, tx, uid, cid, false); err != nil {
			return err
		}
		rows, err := tx.Intents(ctx, uid, cid, upper, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit+1 {
			return conversation.ErrInvariant
		}
		previous := upper
		for n, i := range rows {
			if !i.Valid() || i.UserID != uid || i.ConversationID != cid || i.ID >= previous {
				return conversation.ErrInvariant
			}
			previous = i.ID
			if n < limit {
				out.Items = append(out.Items, dto.AIConversationTaskSummary{ID: i.ID, State: i.State, AnalysisType: i.Input.AnalysisType, SubjectType: i.Input.SubjectType, SubjectID: i.Input.SubjectID, UpdatedAt: i.UpdatedAt.UnixMilli()})
			}
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			out.NextBeforeID = out.Items[len(out.Items)-1].ID
		}
		return nil
	})
	if err != nil {
		return dto.AIConversationTasks{}, err
	}
	return out, nil
}
func (s *AIConversationTaskService) Prepare(parent context.Context, uid, cid int64, key string, input conversation.IntentInput) (dto.AIConversationTask, error) {
	if s == nil || !s.writes {
		return dto.AIConversationTask{}, task.ErrCreationDisabled
	}
	hash, err := input.Digest(uid, cid)
	if parent == nil || err != nil || !points.ValidKey(key) {
		return dto.AIConversationTask{}, conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	var existing conversation.Intent
	var found bool
	err = s.within(ctx, false, func(ctx context.Context, tx conversation.IntentTx) error {
		r, err := ownedConversation(ctx, tx, uid, cid, false)
		if err != nil {
			return err
		}
		i, err := tx.IntentByKey(ctx, uid, cid, key)
		if errors.Is(err, conversation.ErrNotFound) {
			if r.Archived {
				return conversation.ErrConflict
			}
			return nil
		}
		if err != nil {
			return err
		}
		if !i.Valid() || i.UserID != uid || i.ConversationID != cid || i.Key != key {
			return conversation.ErrInvariant
		}
		if i.RequestHash != hash {
			return conversation.ErrConflict
		}
		if err := validateIntentLink(ctx, tx, i); err != nil {
			return err
		}
		existing = i
		found = true
		return nil
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	if found {
		return dto.NewAIConversationTask(existing), nil
	}
	q, err := s.creator.Quote(ctx, uid, dto.AITaskQuoteInput{AnalysisType: input.AnalysisType, SubjectType: input.SubjectType, SubjectID: input.SubjectID})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	if q.AnalysisType != input.AnalysisType || q.SubjectType != input.SubjectType || q.SubjectID != input.SubjectID || !conversation.ID(q.PricePoints) {
		return dto.AIConversationTask{}, conversation.ErrInvariant
	}
	if q.PriceVersion != input.PriceVersion {
		return dto.AIConversationTask{}, points.ErrPriceChanged
	}
	taskKey, err := conversation.NewTaskKey()
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	var out dto.AIConversationTask
	err = s.within(ctx, true, func(ctx context.Context, tx conversation.IntentTx) error {
		r, err := ownedConversation(ctx, tx, uid, cid, true)
		if err != nil {
			return err
		}
		if r.Archived {
			return conversation.ErrConflict
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		i, err := tx.InsertIntent(ctx, conversation.Intent{ConversationID: cid, UserID: uid, Key: key, RequestHash: hash, TaskKey: taskKey, Input: input, PricePoints: q.PricePoints, State: conversation.Prepared, Version: 1, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		if !i.Valid() || i.UserID != uid || i.ConversationID != cid || i.Key != key {
			return conversation.ErrInvariant
		}
		if i.RequestHash != hash {
			return conversation.ErrConflict
		}
		if err := validateIntentLink(ctx, tx, i); err != nil {
			return err
		}
		out = dto.NewAIConversationTask(i)
		return nil
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	return out, nil
}

func intentNow(ctx context.Context, tx conversation.IntentTx, i conversation.Intent, r conversation.Record) (time.Time, error) {
	now, err := tx.Now(ctx)
	if err != nil {
		return time.Time{}, err
	}
	if !conversation.Time(now) {
		return time.Time{}, conversation.ErrInvariant
	}
	if now.Before(i.UpdatedAt) {
		now = i.UpdatedAt
	}
	if now.Before(r.UpdatedAt) {
		now = r.UpdatedAt
	}
	return now, nil
}

func (s *AIConversationTaskService) Confirm(parent context.Context, uid, cid, id int64) (dto.AIConversationTask, error) {
	if s == nil || !s.writes {
		return dto.AIConversationTask{}, task.ErrCreationDisabled
	}
	if parent == nil || !conversation.ID(uid) || !conversation.ID(cid) || !conversation.ID(id) {
		return dto.AIConversationTask{}, conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	var i conversation.Intent
	won := false
	err := s.within(ctx, true, func(ctx context.Context, tx conversation.IntentTx) error {
		r, err := ownedConversation(ctx, tx, uid, cid, true)
		if err != nil {
			return err
		}
		i, err = ownedIntent(ctx, tx, uid, cid, id, true)
		if err != nil {
			return err
		}
		if i.State != conversation.Prepared {
			return validateIntentLink(ctx, tx, i)
		}
		if r.Archived || r.Version == conversation.MaxID || r.LastSeq >= conversation.MaxID-1 {
			return conversation.ErrConflict
		}
		now, err := intentNow(ctx, tx, i, r)
		if err != nil {
			return err
		}
		next := i
		next.State = conversation.Unconfirmed
		next.Version = 2
		next.UpdatedAt = now
		if err := tx.SetIntent(ctx, i, next); err != nil {
			return err
		}
		i = next
		won = true
		return nil
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	if i.State == conversation.Linked || i.State == conversation.NotCreated {
		return dto.NewAIConversationTask(i), nil
	}
	if !won {
		return s.resolve(ctx, i, nil)
	}
	// 此权限的事务已经确认提交；其它请求/进程只查原键，绝不重新执行这一行。
	_, cause := s.creator.CreateQuoted(ctx, uid, i.TaskKey, dto.AITaskCreateInput{AnalysisType: i.Input.AnalysisType, SubjectType: i.Input.SubjectType, SubjectID: i.Input.SubjectID, PriceVersion: i.Input.PriceVersion}, i.PricePoints)
	return s.resolve(ctx, i, cause)
}
func (s *AIConversationTaskService) Reconcile(parent context.Context, uid, cid, id int64) (dto.AIConversationTask, error) {
	if parent == nil || !conversation.ID(uid) || !conversation.ID(cid) || !conversation.ID(id) {
		return dto.AIConversationTask{}, conversation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	var i conversation.Intent
	err := s.within(ctx, false, func(ctx context.Context, tx conversation.IntentTx) error {
		if _, err := ownedConversation(ctx, tx, uid, cid, false); err != nil {
			return err
		}
		var err error
		i, err = ownedIntent(ctx, tx, uid, cid, id, false)
		if err != nil {
			return err
		}
		return validateIntentLink(ctx, tx, i)
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	if i.State != conversation.Unconfirmed {
		return dto.NewAIConversationTask(i), nil
	}
	return s.resolve(ctx, i, nil)
}
func intentRejection(err error) string {
	switch {
	case errors.Is(err, points.ErrPriceChanged):
		return "price_changed"
	case errors.Is(err, points.ErrInsufficient):
		return "insufficient_points"
	case errors.Is(err, task.ErrCreationDisabled):
		return "creation_disabled"
	case errors.Is(err, task.ErrAdmissionUnavailable):
		return "admission_unavailable"
	case errors.Is(err, points.ErrInvalid):
		return "invalid_request"
	}
	return ""
}

func (s *AIConversationTaskService) resolve(ctx context.Context, i conversation.Intent, cause error) (dto.AIConversationTask, error) {
	receipt, err := s.creator.Receipt(ctx, i.UserID, i.TaskKey)
	if errors.Is(err, points.ErrNotFound) {
		code := intentRejection(cause)
		if code != "" {
			return s.finish(ctx, i, dto.AITaskCreation{}, code)
		}
		if cause != nil {
			return dto.AIConversationTask{}, conversation.ErrUnconfirmed
		}
		return dto.NewAIConversationTask(i), nil
	}
	if err != nil {
		return dto.AIConversationTask{}, conversation.ErrUnconfirmed
	}
	if !conversation.ID(receipt.TaskID) || receipt.PricePoints != i.PricePoints || receipt.PriceVersion != i.Input.PriceVersion {
		return dto.AIConversationTask{}, conversation.ErrInvariant
	}
	return s.finish(ctx, i, receipt, "")
}
func (s *AIConversationTaskService) finish(ctx context.Context, old conversation.Intent, receipt dto.AITaskCreation, rejection string) (dto.AIConversationTask, error) {
	var out dto.AIConversationTask
	err := s.within(ctx, true, func(ctx context.Context, tx conversation.IntentTx) error {
		r, err := ownedConversation(ctx, tx, old.UserID, old.ConversationID, true)
		if err != nil {
			return err
		}
		i, err := ownedIntent(ctx, tx, old.UserID, old.ConversationID, old.ID, true)
		if err != nil {
			return err
		}
		if i.State == conversation.Linked || i.State == conversation.NotCreated {
			if err := validateIntentLink(ctx, tx, i); err != nil {
				return err
			}
			out = dto.NewAIConversationTask(i)
			return nil
		}
		if i.State != conversation.Unconfirmed || i.TaskKey != old.TaskKey || i.RequestHash != old.RequestHash {
			return conversation.ErrInvariant
		}
		v, err := tx.IntentTask(ctx, i.UserID, i.TaskKey, true)
		now, clockErr := intentNow(ctx, tx, i, r)
		if clockErr != nil {
			return clockErr
		}
		next := i
		next.Version = 3
		next.UpdatedAt = now
		if rejection != "" {
			if !errors.Is(err, conversation.ErrNotFound) {
				return conversation.ErrInvariant
			}
			next.State = conversation.NotCreated
			next.RejectionCode = rejection
		} else {
			if err != nil {
				return err
			}
			if r.Archived || v.ID != receipt.TaskID || !v.Matches(i) || r.LastSeq >= conversation.MaxID-1 || r.Version == conversation.MaxID {
				return conversation.ErrInvariant
			}
			messageInput := conversation.MessageInput{Kind: "task", SubjectType: i.Input.SubjectType, SubjectID: i.Input.SubjectID, Skill: i.Input.AnalysisType, TaskID: v.ID}
			hash, err := messageInput.Digest(i.UserID, i.ConversationID)
			if err != nil {
				return err
			}
			updated := r
			updated.Version++
			updated.LastSeq++
			updated.UpdatedAt = now
			if err := tx.Update(ctx, r, updated); err != nil {
				return err
			}
			if err := tx.Append(ctx, conversation.Message{ConversationID: i.ConversationID, UserID: i.UserID, Seq: updated.LastSeq, Key: i.TaskKey, RequestHash: hash, Input: messageInput, CreatedAt: now}); err != nil {
				return err
			}
			next.State = conversation.Linked
			next.TaskID = v.ID
			next.MessageSeq = updated.LastSeq
		}
		if err := tx.SetIntent(ctx, i, next); err != nil {
			return err
		}
		out = dto.NewAIConversationTask(next)
		return nil
	})
	if err != nil {
		return dto.AIConversationTask{}, err
	}
	return out, nil
}
