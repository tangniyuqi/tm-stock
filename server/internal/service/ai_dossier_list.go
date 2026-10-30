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

// borrowedDossierRead在外层只读事务中复用原单版读取校验，不开启/提交嵌套事务。
// 仅ListVersions内部使用，外层提交未知时仍返回空响应。
type borrowedDossierRead struct{ tx dossier.Tx }

func (s borrowedDossierRead) Within(ctx context.Context, write bool, fn func(dossier.Tx) error) error {
	if write || ctx == nil || fn == nil || s.tx == nil {
		return dossier.ErrInvalidDraft
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(s.tx)
}

// ListVersions不返回未经当前原文/许可重验的版本元数据或数量。分页游标来自最后一个可读版本。
func (s *AIDossierService) ListVersions(parent context.Context, token string, user, theme, before int64) (dto.AIDossierVersionList, error) {
	zero := dto.AIDossierVersionList{}
	if s == nil || s.store == nil || s.auth == nil || parent == nil || token == "" || !conversation.ID(user) || !conversation.ID(theme) || before < 0 || before > 9007199254740991 {
		return zero, dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	checkAuth := func() error {
		actual, err := s.auth.Authenticate(ctx, token)
		if err != nil {
			return err
		}
		if actual != user {
			return memberauth.ErrUnauthorized
		}
		return nil
	}
	if err := checkAuth(); err != nil {
		return zero, err
	}
	out := dto.AIDossierVersionList{ThemeID: theme, Items: []dto.AIDossierVersionItem{}}
	err := s.store.Within(ctx, false, func(tx dossier.Tx) error {
		if err := tx.Member(ctx, user); err != nil {
			return err
		}
		if err := tx.Theme(ctx, theme); err != nil {
			return err
		}
		reader := &AIDossierService{store: borrowedDossierRead{tx}, auth: s.auth}
		cursor := before
		scanned := 0
		finished := false
		var expires time.Time
		keepExpiry := func(t time.Time) {
			if !t.IsZero() && (expires.IsZero() || t.Before(expires)) {
				expires = t
			}
		}
		for scanned < 32 && len(out.Items) < 11 {
			ids, err := tx.Versions(ctx, user, theme, cursor, 8)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				finished = true
				break
			}
			for _, id := range ids {
				if !conversation.ID(id) || cursor != 0 && id >= cursor {
					return dossier.ErrInvalidDraft
				}
				cursor = id
				scanned++
				rows, err := reader.ReadVersions(ctx, token, user, theme, []int64{id})
				if errors.Is(err, dossier.ErrNotFound) {
					continue
				}
				if err != nil {
					return err
				}
				bound := rows[0]
				identity := bound.Identity()
				pub, err := tx.Publication(ctx, theme, id)
				if err != nil {
					return err
				}
				grant, err := tx.Grant(ctx, user, theme, id)
				if err != nil {
					return err
				}
				keepExpiry(pub.Until)
				keepExpiry(grant.Until)
				for _, o := range bound.Origins() {
					source, err := tx.Source(ctx, o.SourceID)
					if err != nil {
						return err
					}
					if source.LicenseExpires != nil {
						keepExpiry(*source.LicenseExpires)
					}
				}
				out.Items = append(out.Items, dto.AIDossierVersionItem{Version: id, CompiledAt: identity.CreatedAt})
				if len(out.Items) == 11 {
					break
				}
			}
			if len(ids) < 8 {
				finished = true
				break
			}
		}
		if !finished && len(out.Items) < 11 {
			remaining, err := tx.Versions(ctx, user, theme, cursor, 1)
			if err != nil {
				return err
			}
			if len(remaining) > 0 {
				return memberauth.ErrUnavailable
			}
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if !expires.IsZero() && !expires.After(now) {
			return dossier.ErrNotFound
		}
		out.HasMore = len(out.Items) > 10
		if out.HasMore {
			out.Items = out.Items[:10]
			out.NextBeforeVersion = out.Items[len(out.Items)-1].Version
		}
		return ctx.Err()
	})
	if err != nil {
		return zero, err
	}
	if err = checkAuth(); err != nil {
		return zero, err
	}
	return out, nil
}
