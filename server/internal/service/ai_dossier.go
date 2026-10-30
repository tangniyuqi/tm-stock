package service

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type AIDossierAuthenticator interface {
	Authenticate(context.Context, string) (int64, error)
}
type AIDossierService struct {
	store dossier.Store
	auth  AIDossierAuthenticator
	stage bool
}

// stage只允许显式内部候选导入；构造器不登记路由、批准或默认放行。
func NewAIDossierService(store dossier.Store, auth AIDossierAuthenticator, stage bool) (*AIDossierService, error) {
	if store == nil || auth == nil {
		return nil, dossier.ErrInvalidDraft
	}
	return &AIDossierService{store, auth, stage}, nil
}

// StageInternal保存未发布候选；不能由会员输入直接调用，不创建发布/读取许可。
func (s *AIDossierService) StageInternal(parent context.Context, d dossier.Draft) (string, error) {
	if s == nil || s.store == nil || !s.stage || parent == nil || !conversation.ID(d.ThemeID) || !conversation.ID(d.Version) {
		return "", dossier.ErrInvalidDraft
	}
	ids, err := d.SnapshotIDs()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	var digest string
	err = s.store.Within(ctx, true, func(tx dossier.Tx) error {
		snaps, err := dossierSnapshots(ctx, tx, ids)
		if err != nil {
			return err
		}
		registered, err := dossierSources(ctx, tx, snaps)
		if err != nil {
			return err
		}
		if err = tx.Theme(ctx, d.ThemeID); err != nil {
			return err
		}
		bound, err := dossier.BindDraft(d, snaps)
		if err != nil {
			return err
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if d.CreatedAt > now.UnixMilli() || !dossierSourcesValid(registered, snaps, now) {
			return dossier.ErrNotFound
		}
		old, err := tx.Version(ctx, d.ThemeID, d.Version)
		if err == nil {
			restored, e := dossier.RestoreDraft(old.Canonical, old.Digest, snaps)
			if e != nil || old.Identity != bound.Identity() || old.Digest != bound.Digest() || !slices.Equal(old.Evidence, bound.Stored().Evidence) || restored.Digest() != bound.Digest() {
				return dossier.ErrConflict
			}
		} else if !errors.Is(err, dossier.ErrNotFound) {
			return err
		} else {
			last, e := tx.Latest(ctx, d.ThemeID)
			if e != nil && !errors.Is(e, dossier.ErrNotFound) {
				return e
			}
			if e == nil && (last.Version >= d.Version || last.CreatedAt >= d.CreatedAt) {
				return dossier.ErrConflict
			}
			if e = tx.Insert(ctx, bound.Stored()); e != nil {
				return e
			}
		}
		// 来源锁阻止撤回写入，但不能阻止时间自然过期；提交前重新取数据库时钟。
		finished, e := tx.Now(ctx)
		if e != nil {
			return e
		}
		if !dossierSourcesValid(registered, snaps, finished) {
			return dossier.ErrNotFound
		}
		digest = bound.Digest()
		return ctx.Err()
	})
	if err != nil {
		return "", err
	}
	return digest, nil
}

// ReadVersions在一个事务中核两版全部权限；任一失败返回空集合，不泄露部分数量/正文。
// 返回内部绑定结果供后续固定模板转换，不是可直接序列化的公开响应。
func (s *AIDossierService) ReadVersions(parent context.Context, token string, user, theme int64, versions []int64) ([]*dossier.BoundDraft, error) {
	if s == nil || s.store == nil || s.auth == nil || parent == nil || token == "" || !conversation.ID(user) || !conversation.ID(theme) || len(versions) < 1 || len(versions) > 2 {
		return nil, dossier.ErrInvalidDraft
	}
	versions = slices.Clone(versions)
	for i, v := range versions {
		if !conversation.ID(v) || i > 0 && v <= versions[i-1] {
			return nil, dossier.ErrInvalidDraft
		}
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if actual, err := s.auth.Authenticate(ctx, token); err != nil {
		return nil, err
	} else if actual != user {
		return nil, memberauth.ErrUnauthorized
	}
	var out []*dossier.BoundDraft
	err := s.store.Within(ctx, false, func(tx dossier.Tx) error {
		var e error
		out, e = readDossierVersions(ctx, tx, user, theme, versions)
		return e
	})
	if err != nil {
		return nil, err
	}
	if actual, err := s.auth.Authenticate(ctx, token); err != nil {
		return nil, err
	} else if actual != user {
		return nil, memberauth.ErrUnauthorized
	}
	return out, nil
}

// readDossierVersions复用当前会员/发布/读取许可及原文校验；后台执行身份先由原任务准入核定。
func readDossierVersions(ctx context.Context, tx dossier.Tx, user, theme int64, versions []int64) ([]*dossier.BoundDraft, error) {
	var out []*dossier.BoundDraft
	if e := tx.Member(ctx, user); e != nil {
		return nil, e
	}
	started, e := tx.Now(ctx)
	if e != nil {
		return nil, e
	}
	rows := []dossier.StoredVersion{}
	approvals := []dossier.Approval{}
	allIDs := []int64{}
	for _, version := range versions {
		pub, e := tx.Publication(ctx, theme, version)
		if e != nil {
			return nil, e
		}
		grant, e := tx.Grant(ctx, user, theme, version)
		if e != nil {
			return nil, e
		}
		for _, a := range []dossier.Approval{pub, grant} {
			if !a.Enabled || !conversation.Text(a.Reference, 128) || !a.Until.After(started) {
				return nil, dossier.ErrNotFound
			}
		}
		row, e := tx.Version(ctx, theme, version)
		if e != nil {
			return nil, e
		}
		if row.Identity.ThemeID != theme || row.Identity.Version != version || len(row.Evidence) > dossier.MaxDraftSnapshots || pub.Digest != row.Digest || grant.Digest != row.Digest {
			return nil, dossier.ErrNotFound
		}
		for i, ref := range row.Evidence {
			if !conversation.ID(ref.ID) || i > 0 && ref.ID <= row.Evidence[i-1].ID {
				return nil, dossier.ErrNotFound
			}
			allIDs = append(allIDs, ref.ID)
		}
		rows = append(rows, row)
		approvals = append(approvals, pub, grant)
	}
	slices.Sort(allIDs)
	allIDs = slices.Compact(allIDs)
	snaps, e := dossierSnapshots(ctx, tx, allIDs)
	if e != nil {
		return nil, e
	}
	registered, e := dossierSources(ctx, tx, snaps)
	if e != nil {
		return nil, e
	}
	if e = tx.Theme(ctx, theme); e != nil {
		return nil, e
	}
	byID := map[int64]sources.Snapshot{}
	for _, snap := range snaps {
		byID[snap.ID] = snap
	}
	for _, row := range rows {
		selected := make([]sources.Snapshot, 0, len(row.Evidence))
		for _, ref := range row.Evidence {
			selected = append(selected, byID[ref.ID])
		}
		bound, e := dossier.RestoreDraft(row.Canonical, row.Digest, selected)
		if e != nil || bound.Identity() != row.Identity || !slices.Equal(bound.Stored().Evidence, row.Evidence) {
			return nil, dossier.ErrNotFound
		}
		out = append(out, bound)
	}
	now, e := tx.Now(ctx)
	if e != nil {
		return nil, e
	}
	for _, a := range approvals {
		if !a.Enabled || !conversation.Text(a.Reference, 128) || !a.Until.After(now) {
			return nil, dossier.ErrNotFound
		}
	}
	for i, row := range rows {
		if row.Identity.CreatedAt > now.UnixMilli() || i > 0 && row.Identity.CreatedAt <= rows[i-1].Identity.CreatedAt {
			return nil, dossier.ErrNotFound
		}
	}
	if !dossierSourcesValid(registered, snaps, now) {
		return nil, dossier.ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func dossierSnapshots(ctx context.Context, tx dossier.Tx, ids []int64) ([]sources.Snapshot, error) {
	if len(ids) > 2*dossier.MaxDraftSnapshots {
		return nil, dossier.ErrInvalidDraft
	}
	out := make([]sources.Snapshot, 0, len(ids))
	total := 0
	for _, id := range ids {
		snap, e := tx.Snapshot(ctx, id)
		if e != nil {
			return nil, e
		}
		total += len(snap.Body)
		if snap.ID != id || len(snap.Body) > dossier.MaxBodyBytes || total > 2*dossier.MaxDraftBodyBytes {
			return nil, dossier.ErrNotFound
		}
		out = append(out, snap)
	}
	return out, nil
}
func dossierSources(ctx context.Context, tx dossier.Tx, snaps []sources.Snapshot) (map[int64]sources.Source, error) {
	ids := []int64{}
	for _, s := range snaps {
		ids = append(ids, s.SourceID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	out := map[int64]sources.Source{}
	for _, id := range ids {
		v, e := tx.Source(ctx, id)
		if e != nil {
			return nil, e
		}
		if v.ID != id {
			return nil, dossier.ErrNotFound
		}
		out[id] = v
	}
	return out, nil
}
func dossierSourcesValid(current map[int64]sources.Source, snaps []sources.Snapshot, now time.Time) bool {
	for _, s := range snaps {
		c, ok := current[s.SourceID]
		if !ok || !c.HasValidAuthorization(now) || c.Category != s.SourceCategory || c.LicenseTag != s.LicenseTag || c.LicenseRef != s.LicenseRef || s.FetchedAt.After(now) {
			return false
		}
		if (c.LicenseExpires == nil) != (s.LicenseExpires == nil) || c.LicenseExpires != nil && !c.LicenseExpires.Equal(*s.LicenseExpires) {
			return false
		}
		url, e := sources.ValidateURL(c.Domain, s.URL)
		if e != nil || url != s.URL {
			return false
		}
	}
	return true
}
