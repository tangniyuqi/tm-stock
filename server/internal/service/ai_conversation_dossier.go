package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type AIConversationDossierService struct {
	store dossier.ConversationStore
	auth  AIDossierAuthenticator
}

func NewAIConversationDossierService(store dossier.ConversationStore, auth AIDossierAuthenticator) (*AIConversationDossierService, error) {
	if store == nil || auth == nil {
		return nil, dossier.ErrInvalidDraft
	}
	return &AIConversationDossierService{store, auth}, nil
}

func (s *AIConversationDossierService) within(parent context.Context, token string, uid, cid int64, write bool, fn func(context.Context, dossier.ConversationTx, conversation.Record, time.Time) error) error {
	if s == nil || s.store == nil || s.auth == nil || parent == nil || token == "" || !conversation.ID(uid) || !conversation.ID(cid) || fn == nil {
		return dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	check := func() error {
		actual, err := s.auth.Authenticate(ctx, token)
		if err != nil {
			return err
		}
		if actual != uid {
			return memberauth.ErrUnauthorized
		}
		return ctx.Err()
	}
	if err := check(); err != nil {
		return err
	}
	err := s.store.WithinReferences(ctx, write, func(tx dossier.ConversationTx) error {
		if tx == nil || tx.Dossiers() == nil {
			return dossier.ErrInvalidDraft
		}
		if err := tx.Dossiers().Member(ctx, uid); err != nil {
			return err
		}
		r, err := tx.Conversation(ctx, uid, cid, write)
		if err != nil {
			return err
		}
		if !r.Valid() || r.UserID != uid || r.ID != cid {
			return conversation.ErrInvariant
		}
		now, err := tx.Dossiers().Now(ctx)
		if err != nil {
			return err
		}
		if !conversation.Time(now) || now.Before(r.UpdatedAt) {
			return conversation.ErrInvariant
		}
		if err = fn(ctx, tx, r, now); err != nil {
			return err
		}
		return ctx.Err()
	})
	if err != nil {
		return err
	}
	return check()
}

func conversationDossierDTO(r dossier.ConversationReference) dto.AIConversationDossier {
	return dto.AIConversationDossier{ID: r.ID, ConversationID: r.ConversationID, ThemeID: r.ThemeID, Version: r.Version, CompareVersion: r.CompareVersion, CreatedAt: r.CreatedAt.UnixMilli()}
}
func checkConversationDossier(r dossier.ConversationReference, uid, cid int64, owner conversation.Record, now time.Time) error {
	if !r.Valid() || r.UserID != uid || r.ConversationID != cid || r.CreatedAt.Before(owner.CreatedAt) || r.CreatedAt.After(now) {
		return dossier.ErrInvalidDraft
	}
	return nil
}

// Save只保存入口，复用原版本读取；提交未知时不返回已构造的收据。
func (s *AIConversationDossierService) Save(parent context.Context, token string, uid, cid, expected int64, selected dossier.Selection) (dto.AIConversationDossier, error) {
	if !selected.Valid() || !conversation.ID(expected) {
		return dto.AIConversationDossier{}, dossier.ErrInvalidDraft
	}
	var out dto.AIConversationDossier
	err := s.within(parent, token, uid, cid, true, func(ctx context.Context, tx dossier.ConversationTx, owner conversation.Record, now time.Time) error {
		old, err := tx.ReferenceBySelection(ctx, uid, cid, selected)
		if err == nil {
			if err = checkConversationDossier(old, uid, cid, owner, now); err != nil || old.Selection != selected {
				return dossier.ErrInvalidDraft
			}
			// 原组合仅恢复本人保存收据，不赋予正文权限，不因归档/撤权另建引用。
			out = conversationDossierDTO(old)
			return nil
		}
		if !errors.Is(err, dossier.ErrNotFound) {
			return err
		}
		if owner.Archived || owner.Version != expected {
			return conversation.ErrConflict
		}
		count, err := tx.ReferenceCount(ctx, uid, cid)
		if err != nil {
			return err
		}
		if count < 0 || count > dossier.MaxConversationReferences {
			return dossier.ErrInvalidDraft
		}
		if count == dossier.MaxConversationReferences {
			return dossier.ErrReferenceLimit
		}
		r, err := tx.InsertReference(ctx, dossier.ConversationReference{UserID: uid, ConversationID: cid, Selection: selected, CreatedAt: now})
		if err != nil {
			return err
		}
		if err = checkConversationDossier(r, uid, cid, owner, now); err != nil || r.Selection != selected {
			return dossier.ErrInvalidDraft
		}
		// 插入尚未提交。原服务在同一SQL事务核全部证据/许可，失败则关系也回滚。
		reader := &AIDossierService{store: borrowedDossierRead{tx.Dossiers()}, auth: s.auth}
		if _, err = reader.ReadVersions(ctx, token, uid, selected.ThemeID, selected.Versions()); err != nil {
			return err
		}
		out = conversationDossierDTO(r)
		return nil
	})
	if err != nil {
		return dto.AIConversationDossier{}, err
	}
	return out, nil
}

func (s *AIConversationDossierService) Receipt(parent context.Context, token string, uid, cid int64, selected dossier.Selection) (dto.AIConversationDossier, error) {
	if !selected.Valid() {
		return dto.AIConversationDossier{}, dossier.ErrInvalidDraft
	}
	var out dto.AIConversationDossier
	err := s.within(parent, token, uid, cid, false, func(ctx context.Context, tx dossier.ConversationTx, owner conversation.Record, now time.Time) error {
		r, err := tx.ReferenceBySelection(ctx, uid, cid, selected)
		if err != nil {
			return err
		}
		if err = checkConversationDossier(r, uid, cid, owner, now); err != nil || r.Selection != selected {
			return dossier.ErrInvalidDraft
		}
		out = conversationDossierDTO(r)
		return nil
	})
	if err != nil {
		return dto.AIConversationDossier{}, err
	}
	return out, nil
}

func (s *AIConversationDossierService) List(parent context.Context, token string, uid, cid, before int64) (dto.AIConversationDossierPage, error) {
	if before < 0 || before > conversation.MaxID {
		return dto.AIConversationDossierPage{}, dossier.ErrInvalidDraft
	}
	out := dto.AIConversationDossierPage{ConversationID: cid, Notice: dto.AIConversationDossierNotice, Items: []dto.AIConversationDossier{}}
	err := s.within(parent, token, uid, cid, false, func(ctx context.Context, tx dossier.ConversationTx, owner conversation.Record, now time.Time) error {
		rows, err := tx.References(ctx, uid, cid, before, 11)
		if err != nil {
			return err
		}
		if len(rows) > 11 {
			return dossier.ErrInvalidDraft
		}
		cursor := before
		for i, r := range rows {
			if err = checkConversationDossier(r, uid, cid, owner, now); err != nil || cursor != 0 && r.ID >= cursor {
				return dossier.ErrInvalidDraft
			}
			cursor = r.ID
			if i < 10 {
				out.Items = append(out.Items, conversationDossierDTO(r))
			}
		}
		out.HasMore = len(rows) > 10
		if out.HasMore {
			out.NextBeforeID = out.Items[len(out.Items)-1].ID
		}
		return nil
	})
	if err != nil {
		return dto.AIConversationDossierPage{}, err
	}
	return out, nil
}

func (s *AIConversationDossierService) Read(parent context.Context, token string, uid, cid, id int64) (dto.AIConversationDossierView, error) {
	if !conversation.ID(id) {
		return dto.AIConversationDossierView{}, dossier.ErrInvalidDraft
	}
	var out dto.AIConversationDossierView
	err := s.within(parent, token, uid, cid, false, func(ctx context.Context, tx dossier.ConversationTx, owner conversation.Record, now time.Time) error {
		r, err := tx.Reference(ctx, uid, cid, id)
		if err != nil {
			return err
		}
		if err = checkConversationDossier(r, uid, cid, owner, now); err != nil || r.ID != id {
			return dossier.ErrInvalidDraft
		}
		reader := &AIDossierService{store: borrowedDossierRead{tx.Dossiers()}, auth: s.auth}
		view, err := reader.ViewVersions(ctx, token, uid, r.ThemeID, r.Selection.Versions())
		if err != nil {
			return err
		}
		out = dto.AIConversationDossierView{Reference: conversationDossierDTO(r), Dossier: view}
		return nil
	})
	if err != nil {
		return dto.AIConversationDossierView{}, err
	}
	return out, nil
}
