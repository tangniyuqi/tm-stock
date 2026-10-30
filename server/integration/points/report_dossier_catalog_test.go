//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type dossierListExpiryStore struct{ inner dossier.Store }

func (s dossierListExpiryStore) Within(ctx context.Context, w bool, fn func(dossier.Tx) error) error {
	return s.inner.Within(ctx, w, func(tx dossier.Tx) error { return fn(&dossierListExpiryTx{Tx: tx}) })
}

type dossierListExpiryTx struct {
	dossier.Tx
	calls int
}

func (t *dossierListExpiryTx) Now(ctx context.Context) (time.Time, error) {
	now, err := t.Tx.Now(ctx)
	t.calls++
	if t.calls > 22 {
		now = now.Add(2 * time.Hour)
	}
	return now, err
}

func verifyDossierCatalog(t *testing.T, e *pointsTestEnv, db *sql.DB, repo *repository.AIDossierRepo, svc *service.AIDossierService, src sources.Source, body sources.Snapshot, draft func(sources.Snapshot, int64) dossier.Draft, base time.Time) {
	t.Helper()
	create := func(first, last int64) {
		t.Helper()
		for v := first; v <= last; v++ {
			d := draft(body, v)
			d.CreatedAt = base.Add(6*time.Second + time.Duration(v)*time.Millisecond).UnixMilli()
			if _, err := svc.StageInternal(e.ctx, d); err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{
				`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:catalog',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no=?`,
				`INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:catalog',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no=?`,
			} {
				if _, err := db.ExecContext(e.ctx, q, v); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	create(6, 18)
	first, err := svc.ListVersions(e.ctx, "test-member-1", 1, 7, 0)
	if err != nil || len(first.Items) != 10 || !first.HasMore || first.Items[0].Version != 18 || first.NextBeforeVersion != 9 {
		t.Fatal("首个可读分页错误", err, first)
	}
	second, err := svc.ListVersions(e.ctx, "test-member-1", 1, 7, first.NextBeforeVersion)
	if err != nil || len(second.Items) != 5 || second.HasMore || second.NextBeforeVersion != 0 || second.Items[3].Version != 2 || second.Items[4].Version != 1 {
		t.Fatal("分页未跳过未发布版本", err, second)
	}
	other, err := svc.ListVersions(e.ctx, "test-member-2", 2, 7, 0)
	if err != nil || len(other.Items) != 0 || other.HasMore || other.NextBeforeVersion != 0 {
		t.Fatal("跨会员列表泄露", err)
	}
	e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=18`)
	current, err := svc.ListVersions(e.ctx, "test-member-1", 1, 7, 0)
	if err != nil || current.Items[0].Version != 17 {
		t.Fatal("撤回版本仍在列表", err)
	}
	e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1 WHERE version_no=18`)
	unknown, _ := service.NewAIDossierService(dossierUnknownStore{repo}, dossierTestAuth{}, false)
	if got, err := unknown.ListVersions(e.ctx, "test-member-1", 1, 7, 0); err == nil || got.Items != nil || got.ThemeID != 0 {
		t.Fatal("提交确认未知返回版本元数据")
	}
	expiring, _ := service.NewAIDossierService(dossierListExpiryStore{repo}, dossierTestAuth{}, false)
	if got, err := expiring.ListVersions(e.ctx, "test-member-1", 1, 7, 0); err == nil || got.Items != nil || got.ThemeID != 0 {
		t.Fatal("列表结束过期仍返回元数据")
	}
	calls := 0
	auth := dossierAuthFunc(func(context.Context, string) (int64, error) {
		calls++
		if calls == 24 {
			return 0, memberauth.ErrUnauthorized
		}
		return 1, nil
	})
	changed, _ := service.NewAIDossierService(repo, auth, false)
	if got, err := changed.ListVersions(e.ctx, "仅测试", 1, 7, 0); err == nil || got.Items != nil || calls != 24 {
		t.Fatal("列表结束身份失效未清空结果", err, calls)
	}
	// 明确越过扫描预算，不能把未完成检查假装成“没有可读版本”。
	create(19, 38)
	e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
	got, err := svc.ListVersions(e.ctx, "test-member-1", 1, 7, 0)
	e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
	if !errors.Is(err, memberauth.ErrUnavailable) || got.Items != nil || got.ThemeID != 0 {
		t.Fatal("扫描未完成仍返回部分分页或伪空结果", err)
	}
}
