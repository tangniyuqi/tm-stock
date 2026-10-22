//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

var conversationMigration = sync.OnceValues(func() (string, error) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261021_create_tm_ai_conversations.sql"))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(raw), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;"), nil
})

func conversationEnvironment(t *testing.T, level sql.IsolationLevel) (*cancelHTTPEnv, *repository.AIConversationRepo, *service.AIConversationService) {
	t.Helper()
	e := newCancelHTTPEnv(t, level)
	ddl, err := conversationMigration()
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(ddl)
	e.mustExec(ddl)
	if err := repository.NewAIConversationSchemaVerifier(e.db).Verify(e.ctx); err != nil {
		t.Fatal("会话健康保护基线失败", err)
	}
	repo := repository.NewAIConversationRepoWithIsolation(e.db, level)
	svc, err := service.NewAIConversationService(repo)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.NewAIConversationHandler(svc).Register(mux, e.auth)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	e.server = srv
	return e, repo, svc
}
func conversationRequest(t *testing.T, e *cancelHTTPEnv, method, path, key, token, body string, want int) []byte {
	t.Helper()
	r, err := http.NewRequestWithContext(e.ctx, method, e.server.URL+"/api/v1/ai"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	res, err := e.server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil {
		t.Fatal(err)
	}
	// 未注册写方法的405由标准路由器产生，不含私有DTO；真正会话响应必须no-store。
	if len(raw) > 65536 || res.StatusCode != want || want != http.StatusMethodNotAllowed && res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("会话响应不符", res.StatusCode, string(raw))
	}
	for _, hidden := range []string{e.key, e.old, "req_hash", "idem_key", "user_id", "RequestHash"} {
		if strings.Contains(string(raw), hidden) {
			t.Fatal("会话泄漏内部字段")
		}
	}
	return raw
}
func conversationData[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v.Data
}

// 仅由测试夹具追加明确标注的用户输入/任务引用，不调用问答或伪造模型答案。
func appendConversationFixture(ctx context.Context, repo *repository.AIConversationRepo, uid, id int64, key string, input conversation.MessageInput) error {
	return repo.Within(ctx, true, func(tx conversation.Tx) error {
		r, err := tx.ByID(ctx, uid, id, true)
		if err != nil {
			return err
		}
		next := r
		next.LastSeq++
		next.Version++
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		if now.Before(r.UpdatedAt) {
			now = r.UpdatedAt
		}
		next.UpdatedAt = now
		if err := tx.Update(ctx, r, next); err != nil {
			return err
		}
		hash, err := input.Digest(uid, id)
		if err != nil {
			return err
		}
		return tx.Append(ctx, conversation.Message{ConversationID: id, UserID: uid, Seq: next.LastSeq, Key: key, RequestHash: hash, Input: input, CreatedAt: now})
	})
}

func TestIntegrationAIConversationsLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, repo, svc := conversationEnvironment(t, level)
			token := e.token(t, 1, "client", e.key)
			key := "conversation_integration_create"
			ownTask := e.create(t, 1, 1, taskBudget())
			foreignTask := e.create(t, 2, 2, taskBudget())
			before := taskProbeCounts(e.pointsTestEnv)
			created := conversationData[dto.AIConversationReceipt](t, conversationRequest(t, e, "POST", "/conversations", key, token, `{"title":"测试会话"}`, 200))
			if !conversation.ID(created.ID) || created.CreatedAt <= 0 {
				t.Fatal(created)
			}
			path := "/conversations/" + strconv.FormatInt(created.ID, 10)
			if again := conversationData[dto.AIConversationReceipt](t, conversationRequest(t, e, "POST", "/conversations", key, token, `{"title":"测试会话"}`, 200)); again != created {
				t.Fatal("原键重放变化")
			}
			conversationRequest(t, e, "POST", "/conversations", key, token, `{"title":"不同标题"}`, 409)
			conversationRequest(t, e, "GET", path, "", e.token(t, 2, "client", e.key), "", 404)
			conversationRequest(t, e, "GET", "/conversation-receipt", key, e.token(t, 2, "client", e.key), "", 404)
			conversationRequest(t, e, "GET", path, "", e.token(t, 1, "admin", e.key), "", 401)
			r := conversationData[dto.AIConversation](t, conversationRequest(t, e, "PATCH", path, "", token, `{"title":"测试改名","expectedVersion":1}`, 200))
			if r.Version != 2 {
				t.Fatal(r)
			}
			conversationRequest(t, e, "PATCH", path, "", token, `{"title":"测试旧版本","expectedVersion":1}`, 409)
			if again := conversationData[dto.AIConversationReceipt](t, conversationRequest(t, e, "POST", "/conversations", key, token, `{"title":"测试会话"}`, 200)); again != created {
				t.Fatal("改名后收据变化")
			}
			q := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "这是测试用户问题，非模型回答"}
			if err := appendConversationFixture(e.ctx, repo, 1, r.ID, "conversation_fixture_question", q); err != nil {
				t.Fatal(err)
			}
			ref := conversation.MessageInput{Kind: "task", SubjectType: "theme", SubjectID: 1, Skill: "light", TaskID: ownTask.TaskID}
			if err := appendConversationFixture(e.ctx, repo, 1, r.ID, "conversation_fixture_task_ref", ref); err != nil {
				t.Fatal(err)
			}
			ref.TaskID = foreignTask.TaskID
			if err := appendConversationFixture(e.ctx, repo, 1, r.ID, "conversation_foreign_task_ref", ref); err == nil {
				t.Fatal("跨用户任务引用被接受")
			}
			msgs := conversationData[dto.AIConversationMessages](t, conversationRequest(t, e, "GET", path+"/messages?after_seq=0&limit=1", "", token, "", 200))
			if len(msgs.Items) != 1 || !msgs.HasMore || msgs.UpperSeq != 2 || msgs.NextAfterSeq != 1 || msgs.Items[0].Question != q.Question {
				t.Fatal(msgs)
			}
			second := conversationData[dto.AIConversationMessages](t, conversationRequest(t, e, "GET", path+"/messages?after_seq=1&limit=20", "", token, "", 200))
			if len(second.Items) != 1 || second.HasMore || second.Items[0].TaskID != ownTask.TaskID {
				t.Fatal(second)
			}
			conversationRequest(t, e, "GET", path+"/messages?after_seq=3", "", token, "", 400)
			conversationRequest(t, e, "POST", path+"/messages", "", token, `{}`, 405)
			arch := conversationData[dto.AIConversation](t, conversationRequest(t, e, "POST", path+"/archive", "", token, `{"expectedVersion":4}`, 200))
			if !arch.Archived || arch.Version != 5 {
				t.Fatal(arch)
			}
			if err := appendConversationFixture(e.ctx, repo, 1, r.ID, "conversation_archive_append", q); err == nil {
				t.Fatal("归档后追加成功")
			}
			conversationRequest(t, e, "PATCH", path, "", token, `{"title":"归档后标题","expectedVersion":5}`, 409)
			active, err := svc.List(e.ctx, 1, 0, false, 20)
			if err != nil || len(active.Items) != 0 {
				t.Fatal(active, err)
			}
			archived, err := svc.List(e.ctx, 1, 0, true, 20)
			if err != nil || len(archived.Items) != 1 {
				t.Fatal(archived, err)
			}
			e.mustExec("UPDATE addon_member SET status=0 WHERE id=1")
			conversationRequest(t, e, "GET", path, "", token, "", 401)
			e.mustExec("UPDATE addon_member SET status=1 WHERE id=1")
			if !reflect.DeepEqual(before, taskProbeCounts(e.pointsTestEnv)) {
				t.Fatal("会话操作改变任务/账本")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

type conversationUnknownCommit struct {
	base conversation.Store
	once atomic.Bool
}

func (s *conversationUnknownCommit) Within(ctx context.Context, write bool, fn func(conversation.Tx) error) error {
	if err := s.base.Within(ctx, write, fn); err != nil {
		return err
	}
	if write && s.once.Swap(false) {
		return fmt.Errorf("测试：实际提交后回执未知")
	}
	return nil
}
func TestIntegrationAIConversationsConcurrencyAndUnknown(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, repo, svc := conversationEnvironment(t, level)
			before := taskProbeCounts(e.pointsTestEnv)
			var wg sync.WaitGroup
			var failures atomic.Int32
			var id atomic.Int64
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					r, err := svc.Create(e.ctx, 1, "conversation_parallel_create", "测试并发会话")
					if err != nil {
						failures.Add(1)
						return
					}
					previous := id.Load()
					if previous != 0 && previous != r.ID {
						failures.Add(1)
					}
					id.CompareAndSwap(0, r.ID)
				}()
			}
			wg.Wait()
			if failures.Load() != 0 {
				t.Fatal("并发原键创建失败", failures.Load())
			}
			var count int
			if err := e.db.QueryRowContext(e.ctx, `SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=1 AND idem_key='conversation_parallel_create'`).Scan(&count); err != nil || count != 1 {
				t.Fatal(count, err)
			}
			var winners atomic.Int32
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, err := svc.Rename(e.ctx, 1, id.Load(), 1, fmt.Sprintf("测试并发标题%d", i))
					if err == nil {
						winners.Add(1)
					} else if !errors.Is(err, conversation.ErrConflict) {
						failures.Add(1)
					}
				}(i)
			}
			wg.Wait()
			if winners.Load() != 1 || failures.Load() != 0 {
				t.Fatal("CAS不唯一", winners.Load(), failures.Load())
			}
			unknown := &conversationUnknownCommit{base: repo}
			unknown.once.Store(true)
			unconfirmed, err := service.NewAIConversationService(unknown)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			handler.NewAIConversationHandler(unconfirmed).Register(mux, e.auth)
			srv := httptest.NewServer(mux)
			defer srv.Close()
			e.server = srv
			token := e.token(t, 1, "client", e.key)
			key := "conversation_unknown_response"
			conversationRequest(t, e, "POST", "/conversations", key, token, `{"title":"测试未知提交"}`, 503)
			receipt := conversationData[dto.AIConversationReceipt](t, conversationRequest(t, e, "GET", "/conversation-receipt", key, token, "", 200))
			again := conversationData[dto.AIConversationReceipt](t, conversationRequest(t, e, "POST", "/conversations", key, token, `{"title":"测试未知提交"}`, 200))
			if receipt != again {
				t.Fatal("未知提交生成第二会话")
			}
			if !reflect.DeepEqual(before, taskProbeCounts(e.pointsTestEnv)) {
				t.Fatal("未知提交改变账本")
			}
		})
	}
}

func TestIntegrationAIConversationsSchemaGuards(t *testing.T) {
	for _, mode := range []string{"session_check", "message_check", "owner_fk", "task_fk", "insert_guard", "update_guard", "message_guard", "delete_guard", "message_update", "message_delete", "unique_key", "history_index", "column_precision"} {
		t.Run(mode, func(t *testing.T) {
			e, _, _ := conversationEnvironment(t, sql.LevelReadCommitted)
			switch mode {
			case "session_check":
				e.mustExec("ALTER TABLE tm_ai_conversations DROP CHECK chk_tm_ai_conversation, ADD CONSTRAINT chk_tm_ai_conversation CHECK(1=1)")
			case "message_check":
				e.mustExec("ALTER TABLE tm_ai_conversation_messages DROP CHECK chk_tm_ai_conversation_message, ADD CONSTRAINT chk_tm_ai_conversation_message CHECK(1=1)")
			case "owner_fk":
				e.mustExec("ALTER TABLE tm_ai_conversation_messages DROP FOREIGN KEY fk_tm_ai_conversation_message_owner")
			case "task_fk":
				e.mustExec("ALTER TABLE tm_ai_conversation_messages DROP FOREIGN KEY fk_tm_ai_conversation_message_task")
			case "unique_key":
				e.mustExec("ALTER TABLE tm_ai_conversations DROP INDEX uq_tm_ai_conversation_key")
			case "history_index":
				e.mustExec("ALTER TABLE tm_ai_conversations DROP INDEX idx_tm_ai_conversation_list")
			case "column_precision":
				e.mustExec("ALTER TABLE tm_ai_conversations MODIFY updated_at datetime(6) NOT NULL")
			default:
				contracts := map[string][3]string{"insert_guard": {"tm_ai_conversation_insert", "INSERT", "tm_ai_conversations"}, "update_guard": {"tm_ai_conversation_update", "UPDATE", "tm_ai_conversations"}, "message_guard": {"tm_ai_conversation_message_insert", "INSERT", "tm_ai_conversation_messages"}, "delete_guard": {"tm_ai_conversation_no_delete", "DELETE", "tm_ai_conversations"}, "message_update": {"tm_ai_conversation_message_no_update", "UPDATE", "tm_ai_conversation_messages"}, "message_delete": {"tm_ai_conversation_message_no_delete", "DELETE", "tm_ai_conversation_messages"}}
				c := contracts[mode]
				e.mustExec("DROP TRIGGER " + c[0])
				e.mustExec("CREATE TRIGGER " + c[0] + " BEFORE " + c[1] + " ON " + c[2] + " FOR EACH ROW SET @conversation_probe_noop=1")
			}
			if err := repository.NewAIConversationSchemaVerifier(e.db).Verify(e.ctx); err == nil {
				t.Fatal("损坏保护被接受")
			}
			var n int
			if err := e.db.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM tm_ai_conversations").Scan(&n); err != nil || n != 0 {
				t.Fatal("自检探针未回滚", n, err)
			}
		})
	}
}

func TestIntegrationAIConversationsReadOnlyAndCorruption(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, repo, svc := conversationEnvironment(t, level)
			r, err := svc.Create(e.ctx, 1, "conversation_readonly_test", "测试只读会话")
			if err != nil {
				t.Fatal(err)
			}
			q := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "测试用户输入，不是模型输出"}
			if err := appendConversationFixture(e.ctx, repo, 1, r.ID, "conversation_readonly_message", q); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				if _, err := svc.Create(e.ctx, 1, fmt.Sprintf("conversation_page_%02d", i), "测试分页会话"); err != nil {
					t.Fatal(err)
				}
			}
			page, err := svc.List(e.ctx, 1, 0, false, 2)
			if err != nil || !page.HasMore || len(page.Items) != 2 {
				t.Fatal(page, err)
			}
			last, err := svc.List(e.ctx, 1, page.NextBeforeID, false, 2)
			if err != nil || last.HasMore || len(last.Items) != 2 {
				t.Fatal(last, err)
			}
			cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
			if err != nil {
				t.Fatal(err)
			}
			cfg.DBName = e.database
			// 此账号名仅在本次随机隔离库中存在；只授会话两表SELECT，不授账本/任务/DDL。
			user := "conv_read_" + strconv.FormatInt(r.ID, 10)
			e.mustExec("CREATE USER '" + user + "'@'%' IDENTIFIED BY 'fake-local-readonly'")
			t.Cleanup(func() { e.mustExec("DROP USER '" + user + "'@'%'") })
			for _, table := range []string{"tm_ai_conversations", "tm_ai_conversation_messages"} {
				e.mustExec("GRANT SELECT ON `" + e.database + "`." + table + " TO '" + user + "'@'%'")
			}
			cfg.User, cfg.Passwd = user, "fake-local-readonly"
			if cfg.Params == nil {
				cfg.Params = map[string]string{}
			}
			cfg.Params["time_zone"] = "'+08:00'"
			limited, err := sql.Open("mysql", cfg.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { limited.Close() })
			readSvc, err := service.NewAIConversationService(repository.NewAIConversationRepo(limited))
			if err != nil {
				t.Fatal(err)
			}
			want, err := svc.Messages(e.ctx, 1, r.ID, 0, 20)
			if err != nil {
				t.Fatal(err)
			}
			got, err := readSvc.Messages(e.ctx, 1, r.ID, 0, 20)
			if err != nil || !reflect.DeepEqual(want, got) {
				t.Fatal("只读/时区破坏历史", err)
			}
			if _, err := readSvc.Create(e.ctx, 1, "conversation_forbidden_write", "测试拒写"); err == nil {
				t.Fatal("只读账号可以写")
			}
			e.mustExec("DROP TRIGGER tm_ai_conversation_message_no_delete")
			e.mustExec("DELETE FROM tm_ai_conversation_messages")
			if out, err := svc.Messages(e.ctx, 1, r.ID, 0, 20); err == nil || out.Items != nil {
				t.Fatal("丢序仍返回部分历史")
			}
		})
	}
}
