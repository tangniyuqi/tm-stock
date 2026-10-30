//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func mysqlErrorNumber(err error, number uint16) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == number
}

type referenceUnknownCommit struct{ inner dossier.ConversationStore }

type referenceBeforeConversation struct {
	dossier.ConversationStore
	entered chan struct{}
}
type referenceBeforeConversationTx struct {
	dossier.ConversationTx
	entered chan struct{}
}

func (s referenceBeforeConversation) WithinReferences(ctx context.Context, w bool, fn func(dossier.ConversationTx) error) error {
	return s.ConversationStore.WithinReferences(ctx, w, func(tx dossier.ConversationTx) error { return fn(referenceBeforeConversationTx{tx, s.entered}) })
}
func (t referenceBeforeConversationTx) Conversation(ctx context.Context, uid, cid int64, lock bool) (conversation.Record, error) {
	close(t.entered)
	return t.ConversationTx.Conversation(ctx, uid, cid, lock)
}

func (s referenceUnknownCommit) WithinReferences(ctx context.Context, w bool, fn func(dossier.ConversationTx) error) error {
	if err := s.inner.WithinReferences(ctx, w, fn); err != nil {
		return err
	}
	return errors.New("仅测试：已提交但确认丢失")
}

type referenceLateClockStore struct{ inner dossier.ConversationStore }
type referenceLateClockTx struct {
	dossier.ConversationTx
	core *referenceLateDossierClock
}
type referenceLateDossierClock struct {
	dossier.Tx
	calls int
}

func (t *referenceLateClockTx) Dossiers() dossier.Tx { return t.core }
func (t *referenceLateDossierClock) Now(ctx context.Context) (time.Time, error) {
	now, e := t.Tx.Now(ctx)
	t.calls++
	if t.calls > 2 {
		now = now.Add(2 * time.Hour)
	}
	return now, e
}
func (s referenceLateClockStore) WithinReferences(ctx context.Context, w bool, fn func(dossier.ConversationTx) error) error {
	return s.inner.WithinReferences(ctx, w, func(tx dossier.ConversationTx) error {
		return fn(&referenceLateClockTx{tx, &referenceLateDossierClock{Tx: tx.Dossiers()}})
	})
}

func TestIntegrationAIReportConversationDossiers(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			p := newDossierProcessFixture(t, 5*time.Minute)
			e := p.f.env
			reportDDL(t, e.pointsTestEnv, "20261030_link_tm_ai_conversation_dossiers.sql")
			reportDDL(t, e.pointsTestEnv, "20261030_link_tm_ai_conversation_dossiers.sql")
			_, _ = localPasswordTokens(t, e.ctx, p.login, p.client)
			conversations, er := service.NewAIConversationService(repository.NewAIConversationRepoWithIsolation(e.db, level))
			if er != nil {
				t.Fatal(er)
			}
			create := func(uid int64, key string) int64 {
				t.Helper()
				r, err := conversations.Create(e.ctx, uid, key, "仅测试：已保存档案")
				if err != nil {
					t.Fatal(err)
				}
				return r.ID
			}
			cid := create(1, "test_dossier_reference_main")
			repo := repository.NewAIConversationDossierRepo(e.db, level)
			svc, er := service.NewAIConversationDossierService(repo, dossierTestAuth{})
			if er != nil {
				t.Fatal(er)
			}
			save := func(s dossier.Selection) (dto.AIConversationDossier, error) {
				return svc.Save(e.ctx, "test-member-1", 1, cid, 1, s)
			}
			single, pair := dossier.Selection{ThemeID: 1, Version: 1}, dossier.Selection{ThemeID: 1, Version: 1, CompareVersion: 2}
			var first, comparison dto.AIConversationDossier
			t.Run("单版双版与空值幂等", func(t *testing.T) {
				var err error
				first, err = save(single)
				if err != nil {
					t.Fatal(err)
				}
				again, err := save(single)
				if err != nil || again != first {
					t.Fatal("单版重放不一致", err)
				}
				comparison, err = save(pair)
				if err != nil || comparison.ID == first.ID {
					t.Fatal("双版保存失败", err)
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 2 {
					t.Fatal("组合没有去重")
				}
				_, err = e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) SELECT conversation_id,user_id,theme_id,version_no,NULL,created_at,updated_at FROM tm_ai_conversation_dossiers WHERE id=?`, first.ID)
				if !mysqlErrorNumber(err, 1062) {
					t.Fatal("单版NULL组合未被唯一键拒绝", err)
				}
			})
			t.Run("缺版本明确拒绝且不落引用", func(t *testing.T) {
				for _, selected := range []dossier.Selection{{ThemeID: 1, Version: 999}, {ThemeID: 1, Version: 1, CompareVersion: 999}} {
					if out, err := save(selected); !errors.Is(err, dossier.ErrNotFound) || out.ID != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 2 {
						t.Fatal("缺版本被伪装为提交未知或残留引用", err)
					}
				}
			})
			t.Run("并发原组合仅一条", func(t *testing.T) {
				var wg sync.WaitGroup
				results := make(chan error, 20)
				for i := 0; i < 20; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						r, err := save(pair)
						if err == nil && r != comparison {
							err = errors.New("仅测试：并发返回不同引用")
						}
						results <- err
					}()
				}
				wg.Wait()
				close(results)
				for err := range results {
					if err != nil {
						t.Fatal(err)
					}
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 2 {
					t.Fatal("并发增添重复引用")
				}
			})
			t.Run("本人原文与跨会员隔离", func(t *testing.T) {
				view, err := svc.Read(e.ctx, "test-member-1", 1, cid, comparison.ID)
				if err != nil || len(view.Dossier.Versions) != 2 || view.Dossier.Comparison == nil {
					t.Fatal("保存引用未回取原双版", err)
				}
				if _, err = svc.List(e.ctx, "test-member-2", 2, cid, 0); !errors.Is(err, conversation.ErrNotFound) {
					t.Fatal("跨会员列出引用", err)
				}
				if _, err = svc.Read(e.ctx, "test-member-2", 2, cid, comparison.ID); !errors.Is(err, conversation.ErrNotFound) {
					t.Fatal("跨会员回取正文", err)
				}
				if _, err = svc.Read(e.ctx, "test-member-1", 2, cid, comparison.ID); !errors.Is(err, memberauth.ErrUnauthorized) {
					t.Fatal("令牌与调用身份不符未拒绝")
				}
			})
			t.Run("撤权回滚新保存但保留原收据", func(t *testing.T) {
				e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=2`)
				if out, err := svc.Read(e.ctx, "test-member-1", 1, cid, comparison.ID); err == nil || out.Reference.ID != 0 || out.Dossier.Versions != nil {
					t.Fatal("旧引用泄露撤权内容")
				}
				if r, err := svc.Receipt(e.ctx, "test-member-1", 1, cid, pair); err != nil || r != comparison {
					t.Fatal("撤权阻止本人核查保存收据", err)
				}
				if out, err := save(dossier.Selection{ThemeID: 1, Version: 2}); err == nil || out.ID != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 2 {
					t.Fatal("撤权后新引用未回滚")
				}
				e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1 WHERE version_no=2`)
				if _, err := svc.Read(e.ctx, "test-member-1", 1, cid, comparison.ID); err != nil {
					t.Fatal("恢复后原入口不可读", err)
				}
			})
			t.Run("旧版本拒绝与已提交未知找回", func(t *testing.T) {
				selected := dossier.Selection{ThemeID: 1, Version: 2}
				if _, err := svc.Save(e.ctx, "test-member-1", 1, cid, 2, selected); !errors.Is(err, conversation.ErrConflict) {
					t.Fatal("错误会话版本未拒绝")
				}
				unknown, _ := service.NewAIConversationDossierService(referenceUnknownCommit{repo}, dossierTestAuth{})
				if out, err := unknown.Save(e.ctx, "test-member-1", 1, cid, 1, selected); err == nil || out.ID != 0 {
					t.Fatal("提交确认未知仍返回收据")
				}
				r, err := svc.Receipt(e.ctx, "test-member-1", 1, cid, selected)
				if err != nil || r.ID == 0 {
					t.Fatal("原组合无法找回已提交引用", err)
				}
				if again, err := save(selected); err != nil || again != r || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 3 {
					t.Fatal("未知恢复创建了重复引用", err)
				}
			})
			// 下面增加的版本/批准和引用全部为本库合成夹具，不代表正式审核或用户操作证据。
			var snapshotID int64
			if err := e.db.QueryRowContext(e.ctx, `SELECT snapshot_id FROM tm_ai_dossier_evidence WHERE theme_id=1 AND version_no=2 LIMIT 1`).Scan(&snapshotID); err != nil {
				t.Fatal(err)
			}
			dossiers := repository.NewAIDossierRepo(e.db, level)
			stager, _ := service.NewAIDossierService(dossiers, dossierTestAuth{}, true)
			var bound dossier.StoredVersion
			var statement dossier.SentenceRef
			if err := dossiers.Within(e.ctx, false, func(tx dossier.Tx) error {
				var err error
				bound, err = tx.Version(e.ctx, 1, 2)
				if err != nil {
					return err
				}
				snap, err := tx.Snapshot(e.ctx, snapshotID)
				if err != nil {
					return err
				}
				s := snap.Sentences[0]
				statement = dossier.SentenceRef{SnapshotID: snap.ID, Hash: snap.Hash, Number: s.Number, StartByte: s.StartByte, EndByte: s.EndByte}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			u := dossier.Field{State: "unknown"}
			for v := int64(3); v <= 102; v++ {
				d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 1, Version: v, CreatedAt: bound.Identity.CreatedAt + v, Cards: []dossier.FactCard{{Key: "test_only", Section: "progress", Statement: statement, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
				if _, err := stager.StageInternal(e.ctx, d); err != nil {
					t.Fatal("合成版本准备失败", v, err)
				}
			}
			e.mustExec(`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:reference-pub',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no>=3;INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:reference-member',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no>=3`)
			t.Run("插入失败整组回滚", func(t *testing.T) {
				e.mustExec(`CREATE TRIGGER reference_test_reject BEFORE INSERT ON tm_ai_conversation_dossiers FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='仅测试引用插入失败'`)
				if out, err := save(dossier.Selection{ThemeID: 1, Version: 1, CompareVersion: 3}); err == nil || out.ID != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 3 {
					t.Fatal("插入失败泄露收据或未回滚")
				}
				e.mustExec(`DROP TRIGGER reference_test_reject`)
			})
			t.Run("末尾自然过期回滚", func(t *testing.T) {
				clock, _ := service.NewAIConversationDossierService(referenceLateClockStore{repo}, dossierTestAuth{})
				if out, err := clock.Save(e.ctx, "test-member-1", 1, cid, 1, dossier.Selection{ThemeID: 1, Version: 1, CompareVersion: 4}); err == nil || out.ID != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 3 {
					t.Fatal("末尾过期没有回滚引用")
				}
			})
			t.Run("提交后身份失效不交付元数据", func(t *testing.T) {
				calls := 0
				auth := dossierAuthFunc(func(context.Context, string) (int64, error) {
					calls++
					if calls >= 4 {
						return 0, memberauth.ErrUnauthorized
					}
					return 1, nil
				})
				expired, _ := service.NewAIConversationDossierService(repo, auth)
				selected := dossier.Selection{ThemeID: 1, Version: 1, CompareVersion: 5}
				if out, err := expired.Save(e.ctx, "test-member-1", 1, cid, 1, selected); !errors.Is(err, memberauth.ErrUnauthorized) || out.ID != 0 {
					t.Fatal("末尾身份失效泄露引用")
				}
				if _, err := svc.Receipt(e.ctx, "test-member-1", 1, cid, selected); err != nil {
					t.Fatal("有效身份不能核查原保存", err)
				}
			})
			t.Run("归档只读且原组合可核查", func(t *testing.T) {
				other := create(1, "test_dossier_reference_archive")
				r, err := svc.Save(e.ctx, "test-member-1", 1, other, 1, single)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = conversations.Archive(e.ctx, 1, other, 1); err != nil {
					t.Fatal(err)
				}
				if again, err := svc.Save(e.ctx, "test-member-1", 1, other, 1, single); err != nil || again != r {
					t.Fatal("归档不能核原收据", err)
				}
				if _, err = svc.Save(e.ctx, "test-member-1", 1, other, 2, pair); !errors.Is(err, conversation.ErrConflict) {
					t.Fatal("归档允许新增引用")
				}
				if _, err = svc.Read(e.ctx, "test-member-1", 1, other, r.ID); err != nil {
					t.Fatal("归档不允许当前授权读取", err)
				}
			})
			addFixture := func(version int64) {
				t.Helper()
				_, err := e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) VALUES(?,1,1,1,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, cid, version)
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Run("本人引用分页与数量上限", func(t *testing.T) {
				for v := int64(6); v <= 13; v++ {
					addFixture(v)
				}
				page, err := svc.List(e.ctx, "test-member-1", 1, cid, 0)
				if err != nil || len(page.Items) != 10 || !page.HasMore || page.NextBeforeID != page.Items[9].ID {
					t.Fatal("首分页不符", err)
				}
				last, err := svc.List(e.ctx, "test-member-1", 1, cid, page.NextBeforeID)
				if err != nil || len(last.Items) != 2 || last.HasMore {
					t.Fatal("末分页不符", err)
				}
				for v := int64(14); v <= 101; v++ {
					addFixture(v)
				}
				if _, err = save(dossier.Selection{ThemeID: 1, Version: 1, CompareVersion: 102}); !errors.Is(err, dossier.ErrReferenceLimit) {
					t.Fatal("超过100条未拒绝", err)
				}
				if r, err := save(single); err != nil || r != first {
					t.Fatal("上限妨碍原收据核查", err)
				}
			})
			t.Run("外键约束与不可变保护", func(t *testing.T) {
				for _, q := range []string{`UPDATE tm_ai_conversation_dossiers SET version_no=2 WHERE id=?`, `DELETE FROM tm_ai_conversation_dossiers WHERE id=?`} {
					if _, err := e.db.ExecContext(e.ctx, q, first.ID); !mysqlErrorNumber(err, 1644) {
						t.Fatal("引用不可变保护失效", err)
					}
				}
				for _, selected := range []dossier.Selection{{ThemeID: 1, Version: 1, CompareVersion: 999}, {ThemeID: 1, Version: 999}} {
					var compare any
					if selected.CompareVersion > 0 {
						compare = selected.CompareVersion
					}
					if _, err := e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) VALUES(?,1,1,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, cid, selected.Version, compare); !mysqlErrorNumber(err, 1452) {
						t.Fatal("版本外键失效", err)
					}
				}
				if _, err := e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_conversation_dossiers(conversation_id,user_id,theme_id,version_no,compare_version,created_at,updated_at) VALUES(?,2,1,2,NULL,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, cid); !mysqlErrorNumber(err, 1644) {
					t.Fatal("直接写入未拒绝不匹配会话归属", err)
				}
			})
			t.Run("不同会话并发不互相错绑", func(t *testing.T) {
				ids := []int64{}
				for i := 0; i < 8; i++ {
					ids = append(ids, create(1, "test_dossier_parallel_"+strconv.Itoa(i)))
				}
				var wg sync.WaitGroup
				results := make(chan error, len(ids))
				for _, id := range ids {
					wg.Add(1)
					go func(cid int64) {
						defer wg.Done()
						r, err := svc.Save(e.ctx, "test-member-1", 1, cid, 1, pair)
						if err == nil && (r.ConversationID != cid || r.CompareVersion != 2) {
							err = errors.New("仅测试：不同会话错绑")
						}
						results <- err
					}(id)
				}
				wg.Wait()
				close(results)
				for err := range results {
					if err != nil {
						t.Fatal(err)
					}
				}
			})
			t.Run("归档先提交时等待保存拒绝", func(t *testing.T) {
				other := create(1, "test_dossier_archive_race")
				holding, err := e.db.BeginTx(e.ctx, &sql.TxOptions{Isolation: level})
				if err != nil {
					t.Fatal(err)
				}
				defer holding.Rollback() // 测试失败时释放真实会话锁。
				var id int64
				if err = holding.QueryRowContext(e.ctx, `SELECT id FROM tm_ai_conversations WHERE id=? AND user_id=1 FOR UPDATE`, other).Scan(&id); err != nil {
					t.Fatal(err)
				}
				entered := make(chan struct{})
				waiting, _ := service.NewAIConversationDossierService(referenceBeforeConversation{repo, entered}, dossierTestAuth{})
				done := make(chan error, 1)
				go func() { _, err := waiting.Save(e.ctx, "test-member-1", 1, other, 1, single); done <- err }()
				select {
				case <-entered:
				case <-e.ctx.Done():
					t.Fatal("保存未进入原会话锁读取")
				}
				if _, err = holding.ExecContext(e.ctx, `UPDATE tm_ai_conversations SET archived=1,version=2,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND user_id=1`, other); err != nil {
					t.Fatal(err)
				}
				if err = holding.Commit(); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					if !errors.Is(err, conversation.ErrConflict) {
						t.Fatal("归档后等待保存未拒绝", err)
					}
				case <-e.ctx.Done():
					t.Fatal("等待保存未结束")
				}
				var count int
				if err = e.db.QueryRowContext(e.ctx, `SELECT COUNT(*) FROM tm_ai_conversation_dossiers WHERE conversation_id=?`, other).Scan(&count); err != nil || count != 0 {
					t.Fatal("归档后仍创建引用", err)
				}
			})
			t.Run("只读借用不能写引用或资料", func(t *testing.T) {
				if err := repo.WithinReferences(e.ctx, false, func(tx dossier.ConversationTx) error {
					if _, err := tx.InsertReference(e.ctx, dossier.ConversationReference{}); !errors.Is(err, dossier.ErrInvalidDraft) {
						return errors.New("仅测试：只读事务允许写引用")
					}
					if err := tx.Dossiers().Insert(e.ctx, dossier.StoredVersion{}); !errors.Is(err, dossier.ErrInvalidDraft) {
						return errors.New("仅测试：借用读取允许写资料")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			})
			t.Run("原消息任务账务均不增加", func(t *testing.T) {
				p.verifyNoGeneration(t)
				owner, err := conversations.Detail(e.ctx, 1, cid)
				if err != nil || owner.Version != 1 || owner.LastSeq != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 0 {
					t.Fatal("保存改写原会话或消息", err)
				}
			})
			p.login.finishAs(t, e.ctx, false)
		})
	}
}
