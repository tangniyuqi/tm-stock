//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	mysql "github.com/go-sql-driver/mysql"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type dossierTestAuth struct{}
type dossierAuthFunc func(context.Context, string) (int64, error)

func (f dossierAuthFunc) Authenticate(ctx context.Context, token string) (int64, error) {
	return f(ctx, token)
}

type dossierTimeJumpStore struct{ inner dossier.Store }

func (s dossierTimeJumpStore) Within(ctx context.Context, w bool, fn func(dossier.Tx) error) error {
	return s.inner.Within(ctx, w, func(tx dossier.Tx) error { return fn(&dossierTimeJumpTx{Tx: tx}) })
}

type dossierTimeJumpTx struct {
	dossier.Tx
	calls int
}

func (t *dossierTimeJumpTx) Now(ctx context.Context) (time.Time, error) {
	now, e := t.Tx.Now(ctx)
	t.calls++
	if t.calls > 1 {
		now = now.Add(2 * time.Hour)
	}
	return now, e
}

type dossierUnknownStore struct{ inner dossier.Store }

func (s dossierUnknownStore) Within(ctx context.Context, w bool, fn func(dossier.Tx) error) error {
	if err := s.inner.Within(ctx, w, fn); err != nil {
		return err
	}
	return errors.New("仅测试：提交确认丢失")
}

type dossierHeldStore struct {
	inner   dossier.Store
	entered chan struct{}
	release <-chan struct{}
}

func (s dossierHeldStore) Within(ctx context.Context, w bool, fn func(dossier.Tx) error) error {
	return s.inner.Within(ctx, w, func(tx dossier.Tx) error { return fn(dossierHeldTx{tx, s.entered, s.release}) })
}

type dossierHeldTx struct {
	dossier.Tx
	entered chan struct{}
	release <-chan struct{}
}

func (t dossierHeldTx) Source(ctx context.Context, id int64) (sources.Source, error) {
	s, e := t.Tx.Source(ctx, id)
	if e != nil {
		return s, e
	}
	close(t.entered)
	select {
	case <-t.release:
		return s, nil
	case <-ctx.Done():
		return sources.Source{}, ctx.Err()
	}
}

func (dossierTestAuth) Authenticate(ctx context.Context, token string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	switch token {
	case "test-member-1":
		return 1, nil
	case "test-member-2":
		return 2, nil
	}
	return 0, dossier.ErrNotFound
}

func TestIntegrationAIReportDossierStore(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db := setupPointsLedger(t)
			e.mustExec(`CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL)`)
			e.mustExec(`CREATE TABLE addon_quant_theme(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL);INSERT INTO addon_quant_theme VALUES(7,1)`)
			reportDDL(t, e, "20261010_create_ai_source_snapshots.sql")
			reportDDL(t, e, "20261030_create_tm_ai_dossiers.sql")
			reportDDL(t, e, "20261030_create_tm_ai_dossiers.sql")
			repo := repository.NewAIDossierRepo(db, level)
			svc, err := service.NewAIDossierService(repo, dossierTestAuth{}, true)
			if err != nil {
				t.Fatal(err)
			}
			sourceRepo := repository.NewAISourceRepo(db)
			until := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
			src, err := sourceRepo.RegisterSource(e.ctx, sources.Source{Domain: "example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试", LicenseRef: "test-fixture:dossier", LicenseExpires: &until, MinInterval: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if err = sourceRepo.SetSourceEnabled(e.ctx, src.ID, true); err != nil {
				t.Fatal(err)
			}
			src, err = sourceRepo.SourceByID(e.ctx, src.ID)
			if err != nil {
				t.Fatal(err)
			}
			base := time.Now().UTC().Add(-10 * time.Second).Truncate(time.Millisecond)
			makeSnapshot := func(body string, at time.Time) sources.Snapshot {
				s, er := sources.BuildSnapshot(src, "https://example.test/test-only", sources.MediaPlain, []byte(body), at, 4096)
				if er != nil {
					t.Fatal(er)
				}
				s, er = sourceRepo.SaveSnapshot(e.ctx, s)
				if er != nil {
					t.Fatal(er)
				}
				return s
			}
			a := makeSnapshot("仅测试：测试资料甲记录样品处于送样阶段。", base)
			b := makeSnapshot("仅测试：测试资料甲记录样品处于验证阶段。", base.Add(time.Second))
			draft := func(s sources.Snapshot, v int64) dossier.Draft {
				x := s.Sentences[0]
				ref := dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: x.Number, StartByte: x.StartByte, EndByte: x.EndByte}
				u := dossier.Field{State: "unknown"}
				return dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 7, Version: v, CreatedAt: base.Add(time.Duration(v+1) * time.Second).UnixMilli(), Cards: []dossier.FactCard{{Key: "progress_a", Section: "progress", Statement: ref, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
			}
			d1, d2 := draft(a, 1), draft(b, 2)
			t.Run("候选幂等与并发", func(t *testing.T) {
				var wg sync.WaitGroup
				errs := make(chan error, 6)
				for i := 0; i < 6; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); _, er := svc.StageInternal(e.ctx, d1); errs <- er }()
				}
				wg.Wait()
				close(errs)
				for er := range errs {
					if er != nil {
						t.Fatal(er)
					}
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_versions`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_evidence`) != 1 {
					t.Fatal("重复保存产生多条版本或依据")
				}
				if _, er := svc.StageInternal(e.ctx, d2); er != nil {
					t.Fatal(er)
				}
				bad := d1
				bad.Cards = append([]dossier.FactCard{}, d1.Cards...)
				bad.Cards[0].Section = "definition"
				if _, er := svc.StageInternal(e.ctx, bad); !errors.Is(er, dossier.ErrConflict) {
					t.Fatal("同版本异体未冲突", er)
				}
				reader, _ := service.NewAIDossierService(repo, dossierTestAuth{}, false)
				if _, er := reader.StageInternal(e.ctx, draft(a, 3)); er == nil {
					t.Fatal("只读装配允许导入")
				}
			})
			read := func(ids ...int64) ([]*dossier.BoundDraft, error) {
				return svc.ReadVersions(e.ctx, "test-member-1", 1, 7, ids)
			}
			deny := func(t *testing.T, ids ...int64) {
				t.Helper()
				got, er := read(ids...)
				if er == nil || got != nil {
					t.Fatal("拒读泄露部分结果")
				}
			}
			t.Run("没有批准默认拒绝", func(t *testing.T) { deny(t, 1, 2) })
			e.mustExec(`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:publication',0,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions`)
			e.mustExec(`INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:member',0,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions`)
			t.Run("批准记录仍须启用", func(t *testing.T) { deny(t, 1, 2) })
			e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1;UPDATE tm_ai_dossier_grants SET enabled=1`)
			t.Run("授权两版与跨会员拒读", func(t *testing.T) {
				got, er := read(1, 2)
				if er != nil || len(got) != 2 || got[0].Cards()[0].Statement != a.Body || got[1].Cards()[0].Statement != b.Body {
					t.Fatal("授权原文读取失败", er)
				}
				got, er = svc.ReadVersions(e.ctx, "test-member-2", 2, 7, []int64{1, 2})
				if er == nil || got != nil {
					t.Fatal("跨会员读取")
				}
				got, er = svc.ReadVersions(e.ctx, "test-member-2", 1, 7, []int64{1, 2})
				if er == nil || got != nil {
					t.Fatal("伪报身份读取")
				}
			})
			for _, tc := range []struct{ name, off, on string }{
				{"撤回旧版发布", `UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=1`, `UPDATE tm_ai_dossier_publications SET enabled=1 WHERE version_no=1`},
				{"撤回新版会员许可", `UPDATE tm_ai_dossier_grants SET enabled=0 WHERE version_no=2`, `UPDATE tm_ai_dossier_grants SET enabled=1 WHERE version_no=2`},
				{"停用来源", `UPDATE tm_ai_sources SET enabled=0`, `UPDATE tm_ai_sources SET enabled=1`},
				{"修改来源许可", `UPDATE tm_ai_sources SET license_ref='test-fixture:changed'`, `UPDATE tm_ai_sources SET license_ref='test-fixture:dossier'`},
				{"停用会员", `UPDATE addon_member SET status=0 WHERE id=1`, `UPDATE addon_member SET status=1 WHERE id=1`},
				{"停用题材", `UPDATE addon_quant_theme SET status=0 WHERE id=7`, `UPDATE addon_quant_theme SET status=1 WHERE id=7`},
				{"发布批准过期", `UPDATE tm_ai_dossier_publications SET created_at=UTC_TIMESTAMP(3)-INTERVAL 1 DAY,valid_until=UTC_TIMESTAMP(3)-INTERVAL 1 HOUR`, `UPDATE tm_ai_dossier_publications SET valid_until=UTC_TIMESTAMP(3)+INTERVAL 1 HOUR`},
				{"会员许可过期", `UPDATE tm_ai_dossier_grants SET created_at=UTC_TIMESTAMP(3)-INTERVAL 1 DAY,valid_until=UTC_TIMESTAMP(3)-INTERVAL 1 HOUR`, `UPDATE tm_ai_dossier_grants SET valid_until=UTC_TIMESTAMP(3)+INTERVAL 1 HOUR`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					e.mustExec(tc.off)
					deny(t, 1, 2)
					e.mustExec(tc.on)
					if _, er := read(1, 2); er != nil {
						t.Fatal("恢复后无法读取", er)
					}
				})
			}
			t.Run("快照复用不覆盖版本", func(t *testing.T) {
				reused := makeSnapshot(a.Body, base.Add(5*time.Second))
				if reused.ID != a.ID || !reused.FetchedAt.Equal(a.FetchedAt) {
					t.Fatal("未复现快照去重")
				}
				if _, er := svc.StageInternal(e.ctx, draft(reused, 3)); er != nil {
					t.Fatal("新版不能复用旧正文", er)
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_versions`) != 3 {
					t.Fatal("旧版被覆盖")
				}
				deny(t, 2, 3) // 新版仍须另行批准，不能继承旧版许可。
			})
			t.Run("不可变与外键", func(t *testing.T) {
				for _, q := range []string{`UPDATE tm_ai_dossier_versions SET digest=REPEAT('0',64)`, `DELETE FROM tm_ai_dossier_versions`, `UPDATE tm_ai_dossier_evidence SET content_hash=REPEAT('0',64)`, `DELETE FROM tm_ai_dossier_evidence`, `UPDATE tm_ai_dossier_grants SET digest=REPEAT('0',64)`} {
					if _, er := db.ExecContext(e.ctx, q); er == nil {
						t.Fatal("数据库未拒绝篡改")
					}
				}
			})
			t.Run("依据插入失败全部回滚", func(t *testing.T) {
				e.mustExec(`CREATE TRIGGER dossier_test_reject BEFORE INSERT ON tm_ai_dossier_evidence FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='仅测试拒绝'`)
				if digest, er := svc.StageInternal(e.ctx, draft(b, 4)); er == nil || digest != "" {
					t.Fatal("保存失败返回成功")
				}
				e.mustExec(`DROP TRIGGER dossier_test_reject`)
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_versions WHERE version_no=4`) != 0 {
					t.Fatal("版本未回滚")
				}
				if _, er := svc.StageInternal(e.ctx, draft(b, 4)); er != nil {
					t.Fatal("恢复导入失败", er)
				}
			})
			t.Run("提交确认未知不返回结果且原版本可核查", func(t *testing.T) {
				unknown, _ := service.NewAIDossierService(dossierUnknownStore{repo}, dossierTestAuth{}, true)
				d5 := draft(a, 5)
				if digest, er := unknown.StageInternal(e.ctx, d5); er == nil || digest != "" {
					t.Fatal("未知提交被当成功")
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_versions WHERE version_no=5`) != 1 {
					t.Fatal("未实际提交原版本")
				}
				if _, er := svc.StageInternal(e.ctx, d5); er != nil {
					t.Fatal("原版本重读失败", er)
				}
				if got, er := unknown.ReadVersions(e.ctx, "test-member-1", 1, 7, []int64{1, 2}); er == nil || got != nil {
					t.Fatal("读取事务未确认却返回了正文")
				}
			})
			t.Run("当前来源共享锁与撤回互斥", func(t *testing.T) {
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				defer unblock()
				held, _ := service.NewAIDossierService(dossierHeldStore{repo, entered, release}, dossierTestAuth{}, false)
				finished := make(chan error, 1)
				go func() { _, er := held.ReadVersions(e.ctx, "test-member-1", 1, 7, []int64{1, 2}); finished <- er }()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("读取未持有来源锁")
				}
				conn, er := db.Conn(e.ctx)
				if er != nil {
					t.Fatal(er)
				}
				defer conn.Close()
				if _, er = conn.ExecContext(e.ctx, `SET SESSION innodb_lock_wait_timeout=1`); er != nil {
					t.Fatal(er)
				}
				_, er = conn.ExecContext(e.ctx, `UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, src.ID)
				var me *mysql.MySQLError
				if !errors.As(er, &me) || me.Number != 1205 {
					t.Fatal("实际撤回没有被读取共享锁阻止", er)
				}
				unblock()
				select {
				case er = <-finished:
					if er != nil {
						t.Fatal("锁内原读取失败", er)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("读取未结束")
				}
				e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
				deny(t, 1, 2)
				e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
			})
			t.Run("事务末尾过期回滚与拒读", func(t *testing.T) {
				jump, _ := service.NewAIDossierService(dossierTimeJumpStore{repo}, dossierTestAuth{}, true)
				if digest, er := jump.StageInternal(e.ctx, draft(b, 6)); er == nil || digest != "" {
					t.Fatal("提交前过期仍保存")
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_versions WHERE version_no=6`) != 0 {
					t.Fatal("过期写入没有回滚")
				}
				if got, er := jump.ReadVersions(e.ctx, "test-member-1", 1, 7, []int64{1, 2}); er == nil || got != nil {
					t.Fatal("读取结束过期仍返回")
				}
			})
			t.Run("读取后身份失效清空正文", func(t *testing.T) {
				calls := 0
				auth := dossierAuthFunc(func(context.Context, string) (int64, error) {
					calls++
					if calls == 1 {
						return 1, nil
					}
					return 0, dossier.ErrNotFound
				})
				reader, _ := service.NewAIDossierService(repo, auth, false)
				if got, er := reader.ReadVersions(e.ctx, "仅测试", 1, 7, []int64{1, 2}); er == nil || got != nil || calls != 2 {
					t.Fatal("读取后身份复核未清空结果")
				}
			})
			if e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts`) != 0 {
				t.Fatal("档案操作创建了积分账户")
			}
		})
	}
}
