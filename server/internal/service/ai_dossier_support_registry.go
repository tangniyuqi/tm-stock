package service

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

type AIDossierSupportRegistryService struct {
	store dossier.SupportRegistry
	stage bool
}

func NewAIDossierSupportRegistryService(store dossier.SupportRegistry, stage bool) (*AIDossierSupportRegistryService, error) {
	if store == nil {
		return nil, dossier.ErrInvalidDraft
	}
	return &AIDossierSupportRegistryService{store: store, stage: stage}, nil
}
func (AIDossierSupportRegistryService) String() string {
	return "专题核验登记服务（内部配置已隐藏）"
}
func (s AIDossierSupportRegistryService) GoString() string { return s.String() }

// RegisterInternal只登记可重建计划，不写AI用途批准、会员许可、结果发布或账务。
func (s *AIDossierSupportRegistryService) RegisterInternal(parent context.Context, m *dossier.SupportManifest) (string, error) {
	if s == nil || s.store == nil || !s.stage || parent == nil || !m.Valid() {
		return "", dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	err := s.store.WithinSupport(ctx, true, func(tx dossier.SupportRegistryTx) error {
		if tx == nil || tx.Dossiers() == nil {
			return dossier.ErrInvalidDraft
		}
		if err := tx.LockSupportTheme(ctx, m.Identity().ThemeID); err != nil {
			return err
		}
		db := tx.Dossiers()
		row, err := db.Version(ctx, m.Identity().ThemeID, m.Identity().Version)
		if err != nil {
			return err
		}
		if row.Identity != m.Identity() || row.Digest != m.DraftDigest() || len(row.Evidence) < 1 || len(row.Evidence) > dossier.MaxDraftSnapshots {
			return dossier.ErrSupportReview
		}
		ids := make([]int64, len(row.Evidence))
		for i, ref := range row.Evidence {
			if !conversation.ID(ref.ID) || i > 0 && ref.ID <= row.Evidence[i-1].ID {
				return dossier.ErrSupportReview
			}
			ids[i] = ref.ID
		}
		snapshots, err := dossierSnapshots(ctx, db, ids)
		if err != nil {
			return err
		}
		registered, err := dossierSources(ctx, db, snapshots)
		if err != nil {
			return err
		}
		bound, err := dossier.RestoreDraft(row.Canonical, row.Digest, snapshots)
		if err != nil || bound.Identity() != row.Identity || !slices.Equal(bound.Stored().Evidence, row.Evidence) {
			return dossier.ErrSupportReview
		}
		if _, err = m.Rebuild(ctx, bound, snapshots); err != nil {
			return err
		}
		now, err := db.Now(ctx)
		if err != nil {
			return err
		}
		if !dossierSourcesValid(registered, snapshots, now) {
			return dossier.ErrNotFound
		}
		old, err := tx.SupportManifest(ctx, m.ComponentDigest())
		if err == nil {
			if _, err = old.Manifest(); err != nil || !bytes.Equal(old.Canonical, m.Canonical()) || old.CreatedAt.After(now) {
				return dossier.ErrConflict
			}
		} else if !errors.Is(err, dossier.ErrNotFound) {
			return err
		} else {
			r, err := m.Registration(now)
			if err != nil {
				return err
			}
			if err = tx.InsertSupportManifest(ctx, r); err != nil {
				return err
			}
		}
		finished, err := db.Now(ctx)
		if err != nil {
			return err
		}
		if !dossierSourcesValid(registered, snapshots, finished) {
			return dossier.ErrNotFound
		}
		return ctx.Err()
	})
	if err != nil {
		return "", err
	}
	return m.ComponentDigest(), nil
}

// loadForTask仅由原准入核定身份后的执行控制器调用；清单格式/只读许可不能代替AI用途。
func (s *AIDossierSupportRegistryService) loadForTask(parent context.Context, v task.Task, a task.AdmissionPlan) (*dossier.SupportPlan, error) {
	if s == nil || s.store == nil || parent == nil || a.Validate(a.Scope) != nil || a.DossierSupportDigest == "" {
		return nil, dossier.ErrSupportReview
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	var out *dossier.SupportPlan
	err := s.store.WithinSupport(ctx, false, func(tx dossier.SupportRegistryTx) error {
		if tx == nil || tx.Dossiers() == nil {
			return dossier.ErrInvalidDraft
		}
		r, err := tx.SupportManifest(ctx, a.DossierSupportDigest)
		if err != nil {
			return err
		}
		m, err := r.Manifest()
		if err != nil || m.Identity().ThemeID != v.SubjectID || m.AnalysisType() != v.AnalysisType {
			return dossier.ErrSupportReview
		}
		db := tx.Dossiers()
		use, err := tx.SupportUse(ctx, m.ComponentDigest())
		if err != nil {
			return err
		}
		now, err := db.Now(ctx)
		if err != nil {
			return err
		}
		validUse := func(at time.Time) bool {
			return use.Digest == m.ComponentDigest() && use.Enabled && conversation.Text(use.Reference, 128) && use.Until.After(at) && !r.CreatedAt.After(at)
		}
		if !validUse(now) {
			return dossier.ErrNotFound
		}
		bound, err := readDossierVersions(ctx, db, v.UserID, v.SubjectID, []int64{m.Identity().Version})
		if err != nil {
			return err
		}
		ids := make([]int64, len(bound[0].Stored().Evidence))
		for i, ref := range bound[0].Stored().Evidence {
			ids[i] = ref.ID
		}
		snapshots, err := dossierSnapshots(ctx, db, ids)
		if err != nil {
			return err
		}
		registered, err := dossierSources(ctx, db, snapshots)
		if err != nil {
			return err
		}
		prepared, err := m.Rebuild(ctx, bound[0], snapshots)
		if err != nil {
			return err
		}
		plan, err := prepared.BindTask(ctx, v)
		if err != nil || !plan.MatchesAdmission(a) {
			return dossier.ErrSupportReview
		}
		finished, err := db.Now(ctx)
		if err != nil {
			return err
		}
		if !validUse(finished) || !dossierSourcesValid(registered, snapshots, finished) {
			return dossier.ErrNotFound
		}
		pub, err := db.Publication(ctx, v.SubjectID, m.Identity().Version)
		if err != nil {
			return err
		}
		grant, err := db.Grant(ctx, v.UserID, v.SubjectID, m.Identity().Version)
		if err != nil {
			return err
		}
		for _, approval := range []dossier.Approval{pub, grant} {
			if !approval.Enabled || approval.Digest != m.DraftDigest() || !approval.Until.After(finished) {
				return dossier.ErrNotFound
			}
		}
		out = plan
		return ctx.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
