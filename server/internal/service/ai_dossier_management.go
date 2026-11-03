package service

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type AIDossierManagementService struct {
	store  dossier.ManagementStore
	actors map[int64]bool
}

func NewAIDossierManagementService(store dossier.ManagementStore, actors []int64) (*AIDossierManagementService, error) {
	if store == nil || len(actors) < 1 || len(actors) > 100 {
		return nil, dossier.ErrInvalidManagement
	}
	s := &AIDossierManagementService{store: store, actors: map[int64]bool{}}
	for _, id := range actors {
		if !conversation.ID(id) || s.actors[id] {
			return nil, dossier.ErrInvalidManagement
		}
		s.actors[id] = true
	}
	return s, nil
}
func (s *AIDossierManagementService) authorized(ctx context.Context, actor int64) error {
	if s == nil || s.store == nil || ctx == nil {
		return dossier.ErrInvalidManagement
	}
	if !s.actors[actor] {
		return memberauth.ErrUnauthorized
	}
	return ctx.Err()
}
func managementReceipt(e dossier.ManagementEvent) dto.AIDossierManagementReceipt {
	return dto.AIDossierManagementReceipt{RequestID: e.Request.RequestID, Object: e.Request.Object, ActorID: e.ActorID, Revision: e.Revision, Enabled: e.After.Enabled, RecordedAt: e.CreatedAt}
}
func managementCurrent(ctx context.Context, tx dossier.ManagementTx, key dossier.ManagementObject, h dossier.ManagementHead, before *dossier.ManagementState) error {
	if h.Revision == 0 {
		if before != nil && dossier.IsManagedApproval(before.Reference) {
			return dossier.ErrManagementEvidence
		}
		return nil
	}
	e, err := tx.ManagementEvent(ctx, h.RequestID)
	if err != nil {
		return err
	}
	if e.Revision != h.Revision || e.Request.Object != key || before == nil || e.After != *before {
		return dossier.ErrManagementEvidence
	}
	return nil
}
func managementVersion(ctx context.Context, tx dossier.Tx, key dossier.ManagementObject) (*dossier.BoundDraft, map[int64]sources.Source, []sources.Snapshot, error) {
	row, err := tx.Version(ctx, key.ThemeID, key.Version)
	if err != nil {
		return nil, nil, nil, err
	}
	ids := []int64{}
	for i, ref := range row.Evidence {
		if !conversation.ID(ref.ID) || i > 0 && ref.ID <= row.Evidence[i-1].ID {
			return nil, nil, nil, dossier.ErrInvalidDraft
		}
		ids = append(ids, ref.ID)
	}
	snapshots, err := dossierSnapshots(ctx, tx, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	registered, err := dossierSources(ctx, tx, snapshots)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = tx.Theme(ctx, key.ThemeID); err != nil {
		return nil, nil, nil, err
	}
	bound, err := dossier.RestoreDraft(row.Canonical, row.Digest, snapshots)
	if err != nil || bound.Identity() != row.Identity || !slices.Equal(bound.Stored().Evidence, row.Evidence) {
		return nil, nil, nil, dossier.ErrInvalidDraft
	}
	now, err := tx.Now(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if row.Identity.ThemeID != key.ThemeID || row.Identity.Version != key.Version || row.Identity.CreatedAt > now.UnixMilli() || !dossierSourcesValid(registered, snapshots, now) {
		return nil, nil, nil, dossier.ErrNotFound
	}
	return bound, registered, snapshots, nil
}
func (s *AIDossierManagementService) Inspect(parent context.Context, actor int64, key dossier.ManagementObject) (dto.AIDossierManagementInspect, error) {
	if err := s.authorized(parent, actor); err != nil {
		return dto.AIDossierManagementInspect{}, err
	}
	if !key.Valid() {
		return dto.AIDossierManagementInspect{}, dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierManagementInspect
	err := s.store.WithinManagement(ctx, func(tx dossier.ManagementTx) error {
		row, err := tx.Version(ctx, key.ThemeID, key.Version)
		if err != nil {
			return err
		}
		current, err := tx.CurrentPermission(ctx, key)
		if err != nil {
			return err
		}
		h, err := tx.ManagementHead(ctx, key, false)
		if err != nil {
			return err
		}
		if err = managementCurrent(ctx, tx, key, h, current); err != nil {
			return err
		}
		out = dto.AIDossierManagementInspect{Notice: dto.AIDossierManagementNotice, Object: key, VersionDigest: row.Digest, Revision: h.Revision, Managed: h.Revision > 0, SourceState: "unavailable"}
		if current != nil {
			out.Permission = &dto.AIDossierPermissionState{Enabled: current.Enabled, ValidUntil: current.ValidUntil}
		}
		bound, registered, snapshots, err := managementVersion(ctx, tx, key)
		if errors.Is(err, dossier.ErrNotFound) || errors.Is(err, dossier.ErrInvalidDraft) {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		view, err := dossierView([]*dossier.BoundDraft{bound})
		if err != nil {
			return err
		}
		finished, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if !dossierSourcesValid(registered, snapshots, finished) {
			return ctx.Err()
		}
		out.SourceState = "available"
		out.Preview = &view
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierManagementInspect{}, err
	}
	return out, nil
}
func (s *AIDossierManagementService) Request(parent context.Context, actor int64, id string) (dto.AIDossierManagementReceipt, error) {
	if err := s.authorized(parent, actor); err != nil {
		return dto.AIDossierManagementReceipt{}, err
	}
	if !dossier.ValidManagementRequestID(id) {
		return dto.AIDossierManagementReceipt{}, dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierManagementReceipt
	err := s.store.WithinManagement(ctx, func(tx dossier.ManagementTx) error {
		e, err := tx.ManagementEvent(ctx, id)
		if err != nil {
			return err
		}
		if e.ActorID != actor {
			return dossier.ErrNotFound
		}
		out = managementReceipt(e)
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierManagementReceipt{}, err
	}
	return out, nil
}
func (s *AIDossierManagementService) Apply(parent context.Context, actor int64, r dossier.ManagementRequest) (dto.AIDossierManagementReceipt, error) {
	if err := s.authorized(parent, actor); err != nil {
		return dto.AIDossierManagementReceipt{}, err
	}
	if err := r.Validate(); err != nil {
		return dto.AIDossierManagementReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierManagementReceipt
	err := s.store.WithinManagement(ctx, func(tx dossier.ManagementTx) error {
		replay := func() (bool, error) {
			e, err := tx.ManagementEvent(ctx, r.RequestID)
			if errors.Is(err, dossier.ErrNotFound) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !e.Matches(actor, r) {
				return false, dossier.ErrManagementConflict
			}
			out = managementReceipt(e)
			return true, nil
		}
		if found, err := replay(); err != nil || found {
			return err
		}
		if r.Enabled && r.Object.Kind == "member_read" {
			if err := tx.Member(ctx, r.Object.UserID); err != nil {
				return err
			}
		}
		row, err := tx.Version(ctx, r.Object.ThemeID, r.Object.Version)
		if err != nil {
			return err
		}
		if row.Digest != r.VersionDigest {
			return dossier.ErrManagementConflict
		}
		h, err := tx.ManagementHead(ctx, r.Object, true)
		if err != nil {
			return err
		}
		if found, err := replay(); err != nil || found {
			return err
		}
		if h.Revision != r.ExpectedRevision {
			return dossier.ErrManagementConflict
		}
		var publication dossier.Approval
		if r.Enabled && r.Object.Kind == "member_read" {
			publication, err = tx.Publication(ctx, r.Object.ThemeID, r.Object.Version)
			if err != nil {
				return err
			}
		}
		before, err := tx.CurrentPermission(ctx, r.Object)
		if err != nil {
			return err
		}
		if err = managementCurrent(ctx, tx, r.Object, h, before); err != nil {
			return err
		}
		if before != nil && before.Digest != r.VersionDigest {
			return dossier.ErrManagementConflict
		}
		if !r.Enabled && before == nil {
			return dossier.ErrNotFound
		}
		var registered map[int64]sources.Source
		var snapshots []sources.Snapshot
		if r.Enabled {
			var bound *dossier.BoundDraft
			bound, registered, snapshots, err = managementVersion(ctx, tx, r.Object)
			if err != nil {
				return err
			}
			if bound.Digest() != r.VersionDigest {
				return dossier.ErrManagementConflict
			}
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if r.Enabled && (r.ValidUntil <= now.UnixMilli() || r.Object.Kind == "member_read" && (!publication.Enabled || publication.Digest != r.VersionDigest || !publication.Until.After(now) || r.ValidUntil > publication.Until.UnixMilli())) {
			return dossier.ErrNotFound
		}
		until := r.ValidUntil
		if !r.Enabled {
			until = before.ValidUntil
		}
		e := dossier.ManagementEvent{Protocol: dossier.ManagementProtocol, ActorID: actor, Request: r, Before: before, After: dossier.ManagementState{Digest: r.VersionDigest, Reference: dossier.ManagedApprovalPrefix + r.RequestID, Enabled: r.Enabled, ValidUntil: until}, Revision: h.Revision + 1, CreatedAt: now.UnixMilli()}
		if _, _, err = e.Canonical(); err != nil {
			return err
		}
		if err = tx.AppendManagementEvent(ctx, e); err != nil {
			return err
		}
		if err = tx.AdvanceManagementHead(ctx, r.Object, h, e); err != nil {
			return err
		}
		if err = tx.WritePermission(ctx, r.Object, before, e.After, e.CreatedAt); err != nil {
			return err
		}
		finished, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if r.Enabled && (!dossierSourcesValid(registered, snapshots, finished) || r.ValidUntil <= finished.UnixMilli() || r.Object.Kind == "member_read" && !publication.Until.After(finished)) {
			return dossier.ErrNotFound
		}
		out = managementReceipt(e)
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierManagementReceipt{}, err
	}
	return out, nil
}
