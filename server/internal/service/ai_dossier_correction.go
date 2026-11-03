package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type AIDossierCorrectionService struct {
	store   dossier.CorrectionStore
	manager *AIDossierManagementService
	auth    AIDossierAuthenticator
}

func NewAIDossierCorrectionService(store dossier.CorrectionStore, manager *AIDossierManagementService, auth AIDossierAuthenticator) (*AIDossierCorrectionService, error) {
	if store == nil || manager == nil && auth == nil {
		return nil, dossier.ErrInvalidManagement
	}
	return &AIDossierCorrectionService{store, manager, auth}, nil
}
func correctionReceipt(e dossier.CorrectionEvent) dto.AIDossierCorrectionReceipt {
	return dto.AIDossierCorrectionReceipt{RequestID: e.Request.RequestID, Object: e.Request.Object, ActorID: e.ActorID, Revision: e.Revision, Enabled: e.Request.Enabled, RecordedAt: e.CreatedAt}
}
func (s *AIDossierCorrectionService) admin(ctx context.Context, actor int64) error {
	if s == nil || s.manager == nil || s.store == nil {
		return memberauth.ErrUnauthorized
	}
	return s.manager.authorized(ctx, actor)
}
func (s *AIDossierCorrectionService) Request(parent context.Context, actor int64, id string) (dto.AIDossierCorrectionReceipt, error) {
	if err := s.admin(parent, actor); err != nil {
		return dto.AIDossierCorrectionReceipt{}, err
	}
	if !dossier.ValidManagementRequestID(id) {
		return dto.AIDossierCorrectionReceipt{}, dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierCorrectionReceipt
	err := s.store.WithinCorrection(ctx, false, func(tx dossier.CorrectionTx) error {
		e, err := tx.CorrectionEvent(ctx, id)
		if err != nil {
			return err
		}
		if e.ActorID != actor {
			return dossier.ErrNotFound
		}
		out = correctionReceipt(e)
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierCorrectionReceipt{}, err
	}
	return out, nil
}
func (s *AIDossierCorrectionService) Inspect(parent context.Context, actor int64, p dossier.CorrectionPair) (dto.AIDossierCorrectionInspect, error) {
	if err := s.admin(parent, actor); err != nil {
		return dto.AIDossierCorrectionInspect{}, err
	}
	if !p.Valid() {
		return dto.AIDossierCorrectionInspect{}, dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierCorrectionInspect
	err := s.store.WithinCorrection(ctx, false, func(tx dossier.CorrectionTx) error {
		locked, err := tx.CorrectionReadLock(ctx, p)
		if err != nil {
			return err
		}
		b, err := tx.Version(ctx, p.ThemeID, p.BeforeVersion)
		if err != nil {
			return err
		}
		a, err := tx.Version(ctx, p.ThemeID, p.AfterVersion)
		if err != nil {
			return err
		}
		out = dto.AIDossierCorrectionInspect{Notice: dto.AIDossierManagementNotice, Object: p, BeforeDigest: b.Digest, AfterDigest: a.Digest, SourceState: "unavailable"}
		e, _, err := tx.LatestCorrection(ctx, p)
		if err != nil && !errors.Is(err, dossier.ErrNotFound) {
			return err
		}
		if err == nil {
			if !locked {
				return dossier.ErrManagementEvidence
			}
			out.Revision = e.Revision
			out.Enabled = e.Request.Enabled
			out.ValidUntil = e.Request.ValidUntil
		}
		// 内部候选不借用会员身份；双方来源有效才提供预览，发布状态另由提交核查。
		before, beforeSources, beforeSnapshots, err := managementVersion(ctx, tx, dossier.ManagementObject{Kind: "publication", ThemeID: p.ThemeID, Version: p.BeforeVersion})
		if errors.Is(err, dossier.ErrNotFound) || errors.Is(err, dossier.ErrInvalidDraft) {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		after, afterSources, afterSnapshots, err := managementVersion(ctx, tx, dossier.ManagementObject{Kind: "publication", ThemeID: p.ThemeID, Version: p.AfterVersion})
		if errors.Is(err, dossier.ErrNotFound) || errors.Is(err, dossier.ErrInvalidDraft) {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if before.Identity().CreatedAt >= after.Identity().CreatedAt {
			return dossier.ErrManagementEvidence
		}
		view, err := dossierView([]*dossier.BoundDraft{before, after})
		if err != nil {
			return err
		}
		// 当前来源行仍持共享锁；末尾统一核两版许可期限，不交付自然到期正文。
		finished, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if !dossierSourcesValid(beforeSources, beforeSnapshots, finished) || !dossierSourcesValid(afterSources, afterSnapshots, finished) {
			return ctx.Err()
		}
		out.SourceState = "available"
		out.Preview = &view
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierCorrectionInspect{}, err
	}
	return out, nil
}
func (s *AIDossierCorrectionService) Apply(parent context.Context, actor int64, r dossier.CorrectionRequest) (dto.AIDossierCorrectionReceipt, error) {
	if err := s.admin(parent, actor); err != nil {
		return dto.AIDossierCorrectionReceipt{}, err
	}
	if err := r.Validate(); err != nil {
		return dto.AIDossierCorrectionReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierCorrectionReceipt
	err := s.store.WithinCorrection(ctx, true, func(tx dossier.CorrectionTx) error {
		replay := func() (bool, error) {
			e, err := tx.CorrectionEvent(ctx, r.RequestID)
			if errors.Is(err, dossier.ErrNotFound) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !e.Matches(actor, r) {
				return false, dossier.ErrManagementConflict
			}
			out = correctionReceipt(e)
			return true, nil
		}
		if found, err := replay(); found || err != nil {
			return err
		}
		// 复用较新版发布管理头，只锁定，不推进发布修订或接管旧许可。
		if _, err := tx.ManagementHead(ctx, dossier.ManagementObject{Kind: "publication", ThemeID: r.Object.ThemeID, Version: r.Object.AfterVersion}, true); err != nil {
			return err
		}
		if found, err := replay(); found || err != nil {
			return err
		}
		previous, previousDigest, err := tx.LatestCorrection(ctx, r.Object)
		revision := int64(0)
		if err == nil {
			revision = previous.Revision
		} else if !errors.Is(err, dossier.ErrNotFound) {
			return err
		}
		if revision != r.ExpectedRevision {
			return dossier.ErrManagementConflict
		}
		if !r.Enabled && revision == 0 {
			return dossier.ErrNotFound
		}
		for _, v := range []struct {
			version int64
			digest  string
		}{{r.Object.BeforeVersion, r.BeforeDigest}, {r.Object.AfterVersion, r.AfterDigest}} {
			row, e := tx.Version(ctx, r.Object.ThemeID, v.version)
			if e != nil {
				return e
			}
			if row.Digest != v.digest {
				return dossier.ErrManagementConflict
			}
		}
		validate := func() error {
			before, beforeSources, beforeSnapshots, err := managementVersion(ctx, tx, dossier.ManagementObject{Kind: "publication", ThemeID: r.Object.ThemeID, Version: r.Object.BeforeVersion})
			if err != nil {
				return err
			}
			after, afterSources, afterSnapshots, err := managementVersion(ctx, tx, dossier.ManagementObject{Kind: "publication", ThemeID: r.Object.ThemeID, Version: r.Object.AfterVersion})
			if err != nil {
				return err
			}
			if before.Identity().CreatedAt >= after.Identity().CreatedAt {
				return dossier.ErrManagementEvidence
			}
			if err = dossier.BindCorrections(r, before, after); err != nil {
				return err
			}
			now, err := tx.Now(ctx)
			if err != nil {
				return err
			}
			if r.ValidUntil <= now.UnixMilli() || !dossierSourcesValid(beforeSources, beforeSnapshots, now) || !dossierSourcesValid(afterSources, afterSnapshots, now) {
				return dossier.ErrNotFound
			}
			for _, v := range []struct {
				version int64
				digest  string
			}{{r.Object.BeforeVersion, r.BeforeDigest}, {r.Object.AfterVersion, r.AfterDigest}} {
				pub, e := tx.Publication(ctx, r.Object.ThemeID, v.version)
				if e != nil {
					return e
				}
				if !pub.Enabled || pub.Digest != v.digest || !pub.Until.After(now) || r.ValidUntil > pub.Until.UnixMilli() {
					return dossier.ErrNotFound
				}
			}
			return nil
		}
		if r.Enabled {
			if err = validate(); err != nil {
				return err
			}
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		e := dossier.CorrectionEvent{Protocol: dossier.CorrectionProtocol, ActorID: actor, Request: r, Revision: revision + 1, PreviousDigest: previousDigest, CreatedAt: now.UnixMilli()}
		if err = tx.AppendCorrection(ctx, e); err != nil {
			return err
		}
		if r.Enabled {
			if err = validate(); err != nil {
				return err
			}
		}
		out = correctionReceipt(e)
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierCorrectionReceipt{}, err
	}
	return out, nil
}
func correctionPublicView(e dossier.CorrectionEvent, bound []*dossier.BoundDraft) (dto.AIDossierCorrections, error) {
	p := e.Request.Object
	out := dto.AIDossierCorrections{ThemeID: p.ThemeID, BeforeVersion: p.BeforeVersion, AfterVersion: p.AfterVersion, Notice: dto.AIDossierCorrectionNotice, State: "reviewed_declaration", Items: []dto.AIDossierCorrectionItem{}}
	if len(bound) != 2 || dossier.BindCorrections(e.Request, bound[0], bound[1]) != nil {
		return dto.AIDossierCorrections{}, dossier.ErrManagementEvidence
	}
	view, err := dossierView(bound)
	if err != nil {
		return dto.AIDossierCorrections{}, err
	}
	quote := func(version int64, l dossier.CorrectionLocator) (dto.AIDossierCorrectionQuote, error) {
		for _, v := range view.Versions {
			if v.Version != version {
				continue
			}
			for _, c := range v.Cards {
				if c.Key != l.Key {
					continue
				}
				if l.Field == "" {
					return dto.AIDossierCorrectionQuote{Version: version, Key: l.Key, Field: l.Field, Quote: c.Quote}, nil
				}
				for _, f := range c.Fields {
					if f.Key == l.Field && f.State == "known" && f.Quote != nil {
						return dto.AIDossierCorrectionQuote{Version: version, Key: l.Key, Field: l.Field, Quote: *f.Quote}, nil
					}
				}
			}
		}
		return dto.AIDossierCorrectionQuote{}, dossier.ErrManagementEvidence
	}
	for _, d := range e.Request.Declarations {
		b, err := quote(p.BeforeVersion, d.Before)
		if err != nil {
			return dto.AIDossierCorrections{}, err
		}
		a, err := quote(p.AfterVersion, d.After)
		if err != nil {
			return dto.AIDossierCorrections{}, err
		}
		s, err := quote(p.AfterVersion, dossier.CorrectionLocator{Key: d.StatementKey})
		if err != nil {
			return dto.AIDossierCorrections{}, err
		}
		out.Items = append(out.Items, dto.AIDossierCorrectionItem{Before: b, After: a, Statement: s})
	}
	return out, nil
}

// 复用原读取最后一次统一授权时点；不在其后再取新时钟而脱离原来源/许可检查。
type correctionReadClock struct {
	dossier.Tx
	last time.Time
}

func (c *correctionReadClock) Now(ctx context.Context) (time.Time, error) {
	now, err := c.Tx.Now(ctx)
	if err == nil {
		c.last = now
	}
	return now, err
}
func (s *AIDossierCorrectionService) Read(parent context.Context, token string, user int64, p dossier.CorrectionPair) (dto.AIDossierCorrections, error) {
	if s == nil || s.store == nil || s.auth == nil || parent == nil || token == "" || user <= 0 || !p.Valid() {
		return dto.AIDossierCorrections{}, dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	var out dto.AIDossierCorrections
	if actual, err := s.auth.Authenticate(ctx, token); err != nil {
		return dto.AIDossierCorrections{}, err
	} else if actual != user {
		return dto.AIDossierCorrections{}, memberauth.ErrUnauthorized
	}
	err := s.store.WithinCorrection(ctx, false, func(tx dossier.CorrectionTx) error {
		locked, err := tx.CorrectionReadLock(ctx, p)
		if err != nil {
			return err
		}
		clock := &correctionReadClock{Tx: tx}
		reader := &AIDossierService{store: borrowedDossierRead{clock}, auth: s.auth}
		bound, err := reader.ReadVersions(ctx, token, user, p.ThemeID, []int64{p.BeforeVersion, p.AfterVersion})
		if err != nil {
			return err
		}
		out = dto.AIDossierCorrections{ThemeID: p.ThemeID, BeforeVersion: p.BeforeVersion, AfterVersion: p.AfterVersion, Notice: dto.AIDossierCorrectionNotice, State: "unverified", Items: []dto.AIDossierCorrectionItem{}}
		e, _, err := tx.LatestCorrection(ctx, p)
		if err != nil && !errors.Is(err, dossier.ErrNotFound) {
			return err
		}
		if err == nil {
			if !locked {
				return dossier.ErrManagementEvidence
			}
			now, e1 := tx.Now(ctx)
			if e1 != nil {
				return e1
			}
			if e.Request.Enabled && e.Request.ValidUntil > now.UnixMilli() {
				out, err = correctionPublicView(e, bound)
				if err != nil {
					return err
				}
			}
		}
		if _, err = reader.ReadVersions(ctx, token, user, p.ThemeID, []int64{p.BeforeVersion, p.AfterVersion}); err != nil {
			return err
		}
		if out.State == "reviewed_declaration" {
			if e.Request.ValidUntil <= clock.last.UnixMilli() {
				out.State = "unverified"
				out.Items = []dto.AIDossierCorrectionItem{}
			}
		}
		return ctx.Err()
	})
	if err != nil {
		return dto.AIDossierCorrections{}, err
	}
	if actual, err := s.auth.Authenticate(ctx, token); err != nil {
		return dto.AIDossierCorrections{}, err
	} else if actual != user {
		return dto.AIDossierCorrections{}, memberauth.ErrUnauthorized
	}
	return out, nil
}
