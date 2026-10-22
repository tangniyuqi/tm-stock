//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

var intentMigration = sync.OnceValues(func() (string, error) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261021_create_tm_ai_conversations_tasks.sql"))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(raw), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;"), nil
})

type intentSQLStore struct {
	*repository.AIConversationRepo
	failState string
	unknown   bool
	once      atomic.Bool
}
type intentSQLTx struct {
	conversation.IntentTx
	store   *intentSQLStore
	changed bool
}

func (t *intentSQLTx) SetIntent(ctx context.Context, old, next conversation.Intent) error {
	if next.State == t.store.failState {
		t.changed = true
		if !t.store.unknown && t.store.once.CompareAndSwap(false, true) {
			return errors.New("测试绑定事务回滚")
		}
	}
	return t.IntentTx.SetIntent(ctx, old, next)
}
func (s *intentSQLStore) WithinIntents(ctx context.Context, write bool, fn func(conversation.IntentTx) error) error {
	changed := false
	err := s.AIConversationRepo.WithinIntents(ctx, write, func(base conversation.IntentTx) error {
		tx := &intentSQLTx{IntentTx: base, store: s}
		err := fn(tx)
		changed = tx.changed
		return err
	})
	if err == nil && changed && s.unknown && s.once.CompareAndSwap(false, true) {
		return errors.New("测试已提交但返回未知")
	}
	return err
}

type countedIntentCreator struct {
	*service.AITaskCreateService
	calls atomic.Int64
}

func (c *countedIntentCreator) CreateQuoted(ctx context.Context, uid int64, key string, in dto.AITaskCreateInput, expected int64) (dto.AITaskCreation, error) {
	c.calls.Add(1)
	return c.AITaskCreateService.CreateQuoted(ctx, uid, key, in, expected)
}

type intentSQLEnv struct {
	*submissionHTTPEnv
	repo        *intentSQLStore
	svc         *service.AIConversationTaskService
	convs       *service.AIConversationService
	creator     *countedIntentCreator
	cid         int64
	path, token string
}

func intentSQLEnvironment(t *testing.T, level sql.IsolationLevel, mode string) *intentSQLEnv {
	t.Helper()
	e := newSubmissionHTTPEnv(t, level, mode)
	for _, load := range []func() (string, error){conversationMigration, intentMigration} {
		ddl, err := load()
		if err != nil {
			t.Fatal(err)
		}
		e.mustExec(ddl)
		e.mustExec(ddl)
	}
	if err := repository.NewAIConversationTaskSchemaVerifier(e.db).Verify(e.ctx); err != nil {
		t.Fatal("意图健康基线", err)
	}
	repo := &intentSQLStore{AIConversationRepo: repository.NewAIConversationRepoWithIsolation(e.db, level)}
	convs, err := service.NewAIConversationService(repo.AIConversationRepo)
	if err != nil {
		t.Fatal(err)
	}
	r, err := convs.Create(e.ctx, 1, "conversation_sql_intent", "隔离任务会话")
	if err != nil {
		t.Fatal(err)
	}
	core, err := service.NewAITaskService(e.store, e.ledger)
	if err != nil {
		t.Fatal(err)
	}
	original, err := service.NewAITaskCreateService(core, e.ctl, true)
	if err != nil {
		t.Fatal(err)
	}
	creator := &countedIntentCreator{AITaskCreateService: original}
	svc, err := service.NewAIConversationTaskService(repo, creator, true)
	if err != nil {
		t.Fatal(err)
	}
	v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: e.web.key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(e.db))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		t.Fatal(err)
	}
	mux := e.web.server.Config.Handler.(*http.ServeMux)
	handler.NewAIConversationTaskHandler(svc).Register(mux, auth, true)
	handler.NewAIConversationHandler(convs).Register(mux, auth)
	return &intentSQLEnv{e, repo, svc, convs, creator, r.ID, fmt.Sprintf("/api/v1/ai/conversations/%d", r.ID), e.web.token(t, 1, "client")}
}
func (e *intentSQLEnv) prepare(t *testing.T) dto.AIConversationTask {
	t.Helper()
	before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
	v := conversationData[dto.AIConversationTask](t, e.mustRequest(t, "POST", e.path+"/task-intents", "conversation_sql_task", sqlSubmissionBody, e.token, 200))
	if v.State != conversation.Prepared || v.PricePoints != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
		t.Fatal("意图报价冻结积分", v)
	}
	return v
}
func TestIntegrationAIConversationsTasksLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"pass", "reject"} {
				t.Run(mode, func(t *testing.T) {
					e := intentSQLEnvironment(t, level, mode)
					i := e.prepare(t)
					path := fmt.Sprintf("%s/task-intents/%d", e.path, i.ID)
					e.ctl.probeAccount.Store(false)
					var wg sync.WaitGroup
					errs := make(chan error, 20)
					for n := 0; n < 20; n++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							status, body, err := e.request(e.ctx, "POST", path+"/confirm", "", `{"confirmed":true}`, e.token)
							if err != nil || status != 200 {
								errs <- fmt.Errorf("测试并发确认=%d %s: %w", status, body, err)
							}
						}()
					}
					wg.Wait()
					close(errs)
					for err := range errs {
						t.Fatal(err)
					}
					v := conversationData[dto.AIConversationTask](t, e.mustRequest(t, "GET", path, "", "", e.token, 200))
					if v.State != conversation.Linked || v.MessageSeq != 1 || e.creator.calls.Load() != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages") != 1 {
						t.Fatal("关联/单次创建冻结失败", v)
					}
					if receipt := conversationData[dto.AIConversationTask](t, e.mustRequest(t, "GET", e.path+"/task-intent-receipt", "conversation_sql_task", "", e.token, 200)); receipt != v {
						t.Fatal("历史原键收据不一致")
					}
					// 主表列表游标必须真走SQL；额外准备报价不创建第二个任务或冻结。
					e.mustRequest(t, "POST", e.path+"/task-intents", "conversation_sql_task_next", sqlSubmissionBody, e.token, 200)
					pageRaw := e.mustRequest(t, "GET", e.path+"/task-intents?limit=1", "", "", e.token, 200)
					page := conversationData[dto.AIConversationTasks](t, pageRaw)
					if len(page.Items) != 1 || !page.HasMore || page.NextBeforeID == 0 || strings.Contains(string(pageRaw), "taskId") {
						t.Fatal("列表分页或摘要字段错误", string(pageRaw))
					}
					tail := conversationData[dto.AIConversationTasks](t, e.mustRequest(t, "GET", fmt.Sprintf("%s/task-intents?before_id=%d&limit=1", e.path, page.NextBeforeID), "", "", e.token, 200))
					if len(tail.Items) != 1 || tail.Items[0].ID != i.ID || tail.HasMore {
						t.Fatal("意图分页丢失或重复", tail)
					}
					messages := conversationData[dto.AIConversationMessages](t, e.mustRequest(t, "GET", e.path+"/messages", "", "", e.token, 200))
					if len(messages.Items) != 1 || messages.Items[0].TaskID != v.TaskID {
						t.Fatal("关联没有进入会话历史", messages)
					}
					e.mustRequest(t, "GET", path, "", "", e.web.token(t, 2, "client"), 404)
					e.mustRequest(t, "POST", path+"/confirm", "", `{"confirmed":true}`, e.web.token(t, 2, "client"), 404)
					e.bindNewTask(t, v.TaskID)
					e.run(t)
					want := task.Succeeded
					status := 200
					if mode == "reject" {
						want = task.Failed
						status = 422
					}
					actual, err := e.tasks.Inspect(e.ctx, 1, v.TaskID)
					if err != nil || actual.Status != want {
						t.Fatal(actual, err)
					}
					e.web.mustGet(t, e.web.path(), e.token, status)
					before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
					e.mustRequest(t, "POST", path+"/confirm", "", `{"confirmed":true}`, e.token, 200)
					e.mustRequest(t, "POST", path+"/reconcile", "", `{}`, e.token, 200)
					if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before || e.creator.calls.Load() != 1 || e.submits.Load() != 1 {
						t.Fatal("重放重复执行或改变账务")
					}
					if mode == "pass" && e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 {
						t.Fatal("结算不是恰好一次")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
func TestIntegrationAIConversationsTasksRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"claim_unknown", "bind_rollback", "bind_unknown", "original_unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := intentSQLEnvironment(t, level, "pass")
					i := e.prepare(t)
					e.repo.failState = conversation.Linked
					e.repo.unknown = mode == "bind_unknown"
					if mode == "claim_unknown" {
						e.repo.failState = conversation.Unconfirmed
						e.repo.unknown = true
					}
					if mode == "original_unknown" {
						e.repo.failState = ""
						if err := e.db.QueryRowContext(e.ctx, `SELECT task_key FROM tm_ai_conversation_tasks WHERE id=?`, i.ID).Scan(&e.store.key); err != nil {
							t.Fatal(err)
						}
						e.store.phase.Store(1)
					}
					if _, err := e.svc.Confirm(e.ctx, 1, e.cid, i.ID); mode != "original_unknown" && err == nil || mode == "original_unknown" && err != nil {
						t.Fatal("故障未生效")
					}
					// 创建开关关闭后，未完成的关联仍只能凭原创建收据恢复。
					core, err := service.NewAITaskService(e.store, e.ledger)
					if err != nil {
						t.Fatal(err)
					}
					originalClosed, err := service.NewAITaskCreateService(core, nil, false)
					if err != nil {
						t.Fatal(err)
					}
					closed, err := service.NewAIConversationTaskService(e.repo, originalClosed, false)
					if err != nil {
						t.Fatal(err)
					}
					for n := 0; n < 3; n++ {
						v, err := closed.Reconcile(e.ctx, 1, e.cid, i.ID)
						want := conversation.Linked
						if mode == "claim_unknown" {
							want = conversation.Unconfirmed
						}
						if err != nil || v.State != want {
							t.Fatal(v, err)
						}
					}
					if mode == "claim_unknown" {
						e.mustRequest(t, "POST", e.path+"/archive", "", `{"expectedVersion":1}`, e.token, 409)
						if e.creator.calls.Load() != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
							t.Fatal("未知权限创建了任务")
						}
					} else {
						if e.creator.calls.Load() != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
							t.Fatal("恢复重复创建或冻结")
						}
					}
				})
			}
		})
	}
}
func TestIntegrationAIConversationsTasksPriceAndArchive(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"price", "archive", "disabled"} {
				t.Run(mode, func(t *testing.T) {
					e := intentSQLEnvironment(t, level, "pass")
					i := e.prepare(t)
					if mode == "price" {
						e.ctl.p.Price.Points = 9
						v, err := e.svc.Confirm(e.ctx, 1, e.cid, i.ID)
						if err != nil || v.State != conversation.NotCreated || v.RejectionCode != "price_changed" {
							t.Fatal(v, err)
						}
					}
					if mode == "archive" {
						if _, err := e.convs.Archive(e.ctx, 1, e.cid, 1); err != nil {
							t.Fatal(err)
						}
						if _, err := e.svc.Confirm(e.ctx, 1, e.cid, i.ID); !errors.Is(err, conversation.ErrConflict) {
							t.Fatal(err)
						}
					}
					if mode == "disabled" {
						closed, err := service.NewAIConversationTaskService(e.repo, e.creator, false)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := closed.Confirm(e.ctx, 1, e.cid, i.ID); !errors.Is(err, task.ErrCreationDisabled) {
							t.Fatal(err)
						}
						if v, err := closed.Reconcile(e.ctx, 1, e.cid, i.ID); err != nil || v.State != conversation.Prepared {
							t.Fatal(v, err)
						}
					}
					if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages") != 0 || e.submits.Load() != 0 {
						t.Fatal("边界错误触发冻结/执行")
					}
				})
			}
		})
	}
}
func TestIntegrationAIConversationsTasksGuards(t *testing.T) {
	e := intentSQLEnvironment(t, sql.LevelRepeatableRead, "pass")
	ddl, err := intentMigration()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"missing_archive_guard", `DROP TRIGGER tm_ai_conversation_intent_archive_guard`},
		{"noop_archive_guard", `DROP TRIGGER tm_ai_conversation_intent_archive_guard; CREATE TRIGGER tm_ai_conversation_intent_archive_guard BEFORE UPDATE ON tm_ai_conversations FOR EACH ROW SET @tm_test_noop=1`},
		{"noop_update", `DROP TRIGGER tm_ai_intent_update; CREATE TRIGGER tm_ai_intent_update BEFORE UPDATE ON tm_ai_conversation_tasks FOR EACH ROW SET @tm_test_noop=1`},
		{"noop_delete", `DROP TRIGGER tm_ai_intent_no_delete; CREATE TRIGGER tm_ai_intent_no_delete BEFORE DELETE ON tm_ai_conversation_tasks FOR EACH ROW SET @tm_test_noop=1`},
		{"noop_insert", `DROP TRIGGER tm_ai_intent_insert; CREATE TRIGGER tm_ai_intent_insert BEFORE INSERT ON tm_ai_conversation_tasks FOR EACH ROW SET @tm_test_noop=1`},
		{"unenforced_check", `ALTER TABLE tm_ai_conversation_tasks ALTER CHECK chk_tm_ai_intent NOT ENFORCED`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e.mustExec(tc.sql)
			if err := repository.NewAIConversationTaskSchemaVerifier(e.db).Verify(e.ctx); err == nil {
				t.Fatal("损坏保护仍通过")
			}
			if tc.name == "unenforced_check" {
				e.mustExec(`ALTER TABLE tm_ai_conversation_tasks ALTER CHECK chk_tm_ai_intent ENFORCED`)
			}
			e.mustExec(ddl)
			if err := repository.NewAIConversationTaskSchemaVerifier(e.db).Verify(e.ctx); err != nil {
				t.Fatal("还原后失败", err)
			}
		})
	}
}
