//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func eventPath(id int64, after int64, limit int) string {
	return "/api/v1/ai/tasks/" + strconv.FormatInt(id, 10) + "/events?after_seq=" + strconv.FormatInt(after, 10) + "&limit=" + strconv.Itoa(limit)
}
func decodeEventPage(t *testing.T, raw []byte) dto.AITaskEventPage {
	t.Helper()
	var out struct {
		Data dto.AITaskEventPage `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	var top struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &top); err != nil || len(top.Data) != 4 {
		t.Fatal("事件页 DTO 越界", err)
	}
	for _, key := range []string{"items", "nextAfterSeq", "hasMore", "upperSeq"} {
		if _, ok := top.Data[key]; !ok {
			t.Fatal("事件页字段缺失", key)
		}
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(top.Data["items"], &items); err != nil || items == nil {
		t.Fatal("事件空数组契约错误", err)
	}
	for _, item := range items {
		if len(item) != 6 {
			t.Fatal("事件详情字段越界")
		}
		for _, key := range []string{"seq", "type", "status", "step", "errorCode", "at"} {
			if _, ok := item[key]; !ok {
				t.Fatal("事件字段缺失", key)
			}
		}
	}
	return out.Data
}

func eventReadOnlyDB(t *testing.T, e *cancelHTTPEnv) *sql.DB {
	t.Helper()
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("读取测试连接失败")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	user := "event_read_" + hex.EncodeToString(random[:6])
	pass := hex.EncodeToString(random[:])
	e.mustExec("CREATE USER '" + user + "'@'%' IDENTIFIED BY '" + pass + "'")
	t.Cleanup(func() { e.mustExec("DROP USER '" + user + "'@'%'") })
	for _, table := range []string{"addon_member", "tm_ai_tasks", "tm_ai_task_events"} {
		e.mustExec("GRANT SELECT ON `" + e.database + "`." + table + " TO '" + user + "'@'%'")
	}
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, pass
	cfg.Loc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal("测试 IANA 时区不可用")
	}
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+08:00'"
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("装配受限只读连接失败")
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.ExecContext(e.ctx, "UPDATE tm_ai_task_events SET seq=seq WHERE task_id=1"); err == nil {
		t.Fatal("只读账号拥有写权限")
	} else {
		var permission *mysqldrv.MySQLError
		if !errors.As(err, &permission) || permission.Number != 1142 {
			t.Fatal("不是预期的写权限拒绝")
		}
	}
	return db
}

func TestIntegrationAITaskMemberEventsLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCancelHTTPEnv(t, level)
			v := e.create(t, 1, 1, taskBudget())
			e.create(t, 2, 2, taskBudget())
			token := e.token(t, 1, "client", e.key)
			lease, err := e.tasks.Claim(e.ctx, 1, v.TaskID, "worker_event_read_0001", 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			retry, err := e.tasks.FailOrRetry(e.ctx, 1, v.TaskID, lease.LeaseOwner, lease.Fence, "provider_unavailable", time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, e.pointsTestEnv, retry.NotBefore)
			lease, err = e.tasks.Claim(e.ctx, 1, v.TaskID, "worker_event_read_0002", 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			budget, err := service.NewAITaskBudgetService(e.repo)
			if err != nil {
				t.Fatal(err)
			}
			key, digest := "test_event_extract_operation", strings.Repeat("a", 64)
			if _, err := budget.Reserve(e.ctx, task.ReserveRequest{UserID: 1, TaskID: v.TaskID, Fence: lease.Fence, Owner: lease.LeaseOwner, Key: key, InputDigest: digest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
				t.Fatal(err)
			}
			stages, err := service.NewAITaskStageService(e.repo, testStagePolicies())
			if err != nil {
				t.Fatal(err)
			}
			r := task.StageRequest{UserID: 1, TaskID: v.TaskID, Fence: lease.Fence, Owner: lease.LeaseOwner, OperationKey: key, Stage: "extract", InputDigest: digest, StepNo: 1}
			if _, err := stages.Capture(e.ctx, r, testStageBody); err != nil {
				t.Fatal(err)
			}
			if _, err := stages.Complete(e.ctx, r); err != nil {
				t.Fatal(err)
			}
			if _, err := e.tasks.Cancel(e.ctx, 1, v.TaskID); err != nil {
				t.Fatal(err)
			}
			if _, err := e.tasks.FailOrRetry(e.ctx, 1, v.TaskID, lease.LeaseOwner, lease.Fence, "internal_failure", 0); err != nil {
				t.Fatal(err)
			}
			before := taskProbeCounts(e.pointsTestEnv)
			after := int64(0)
			var all []dto.AITaskEvent
			for round := 0; round < 4; round++ {
				path := eventPath(v.TaskID, after, 2)
				raw := e.mustRequest(t, "GET", path, token, "", 200)
				page := decodeEventPage(t, raw)
				same := decodeEventPage(t, e.mustRequest(t, "GET", path, token, "", 200))
				if !reflect.DeepEqual(page, same) || page.UpperSeq != 7 || len(page.Items) == 0 {
					t.Fatal("事件重放/上界不稳定", page)
				}
				for _, item := range page.Items {
					if item.Seq != after+1 || item.At <= 0 {
						t.Fatal("事件次序/时点错误", item)
					}
					after = item.Seq
					all = append(all, item)
				}
				if page.NextAfterSeq != after || page.HasMore != (after < 7) {
					t.Fatal("事件游标不符", page)
				}
				if strings.Contains(string(raw), "隔离测试数据") {
					t.Fatal("原始响应进入事件")
				}
			}
			want := []string{"created", "claimed", "retried", "claimed", "step_completed", "cancel_requested", "cancelled"}
			for i, item := range all {
				if item.Type != want[i] {
					t.Fatal(item)
				}
			}
			empty := decodeEventPage(t, e.mustRequest(t, "GET", eventPath(v.TaskID, 7, 20), token, "", 200))
			if len(empty.Items) != 0 || empty.NextAfterSeq != 7 || empty.HasMore {
				t.Fatal("空页重置游标", empty)
			}
			e.mustRequest(t, "GET", eventPath(v.TaskID, 8, 20), token, "", 400)
			cross := e.mustRequest(t, "GET", eventPath(v.TaskID, 0, 20), e.token(t, 2, "client", e.key), "", 404)
			missing := e.mustRequest(t, "GET", eventPath(99999999, 0, 20), e.token(t, 2, "client", e.key), "", 404)
			if string(cross) != string(missing) {
				t.Fatal("事件泄漏任务存在性")
			}
			for _, tc := range []struct {
				uid  int64
				kind string
			}{{1, "admin"}, {1, ""}, {3, "client"}, {4, "client"}, {99, "client"}} {
				e.mustRequest(t, "GET", eventPath(v.TaskID, 0, 20), e.token(t, tc.uid, tc.kind, e.key), "", 401)
			}
			e.mustExec("UPDATE addon_member SET status=0 WHERE id=1")
			e.mustRequest(t, "GET", eventPath(v.TaskID, 0, 20), token, "", 401)
			e.mustExec("UPDATE addon_member SET status=1 WHERE id=1")
			limited := eventReadOnlyDB(t, e)
			verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: e.key, PreviousKey: e.old, Issuer: "GVA", Audience: "GVA"})
			if err != nil {
				t.Fatal(err)
			}
			sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(limited))
			if err != nil {
				t.Fatal(err)
			}
			auth, err := middleware.NewMemberAuth(sessions)
			if err != nil {
				t.Fatal(err)
			}
			reads, err := service.NewAITaskReadService(repository.NewAITaskRepo(limited))
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			handler.NewAITaskReadHandler(reads).Register(mux, auth)
			srv := httptest.NewServer(mux)
			defer srv.Close()
			original := e.server
			e.server = srv
			page := decodeEventPage(t, e.mustRequest(t, "GET", eventPath(v.TaskID, 0, 20), e.token(t, 1, "client", e.old), "", 200))
			e.server = original
			if !reflect.DeepEqual(page.Items, all) {
				t.Fatal("驱动或会话 +08 时区改变 UTC 时点", page.Items, all)
			}
			if !reflect.DeepEqual(before, taskProbeCounts(e.pointsTestEnv)) {
				t.Fatal("事件 GET 写入账本/任务/预留")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

type eventSnapshotStore struct {
	base task.ReadStore
	hook func()
}
type eventSnapshotTx struct {
	task.ReadTx
	hook func()
}

func (s eventSnapshotStore) WithinTaskRead(ctx context.Context, fn func(task.ReadTx) error) error {
	return s.base.WithinTaskRead(ctx, func(tx task.ReadTx) error { return fn(eventSnapshotTx{ReadTx: tx, hook: s.hook}) })
}
func (tx eventSnapshotTx) Task(ctx context.Context, uid, id int64) (task.Task, error) {
	v, err := tx.ReadTx.Task(ctx, uid, id)
	if err == nil && tx.hook != nil {
		tx.hook()
	}
	return v, err
}

func TestIntegrationAITaskMemberEventsSnapshot(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCancelHTTPEnv(t, level)
			v := e.create(t, 1, 1, taskBudget())
			reads, err := service.NewAITaskReadService(eventSnapshotStore{base: e.repo, hook: func() {
				if _, err := e.tasks.Cancel(e.ctx, 1, v.TaskID); err != nil {
					t.Fatal(err)
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			old, err := reads.Events(e.ctx, 1, v.TaskID, 0, 20)
			if err != nil || old.UpperSeq != 1 || len(old.Items) != 1 || old.Items[0].Type != "created" {
				t.Fatal("新提交混入既有 RR 快照", old, err)
			}
			fresh, err := service.NewAITaskReadService(e.repo)
			if err != nil {
				t.Fatal(err)
			}
			page, err := fresh.Events(e.ctx, 1, v.TaskID, old.NextAfterSeq, 20)
			if err != nil || page.UpperSeq != 2 || len(page.Items) != 1 || page.Items[0].Type != "cancelled" || page.NextAfterSeq != 2 {
				t.Fatal("下一轮漏读新提交", page, err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'") != 1 {
				t.Fatal("快照读取重复取消")
			}
		})
	}
}

func TestIntegrationAITaskMemberEventsCorruption(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"缺首事件", "DELETE FROM tm_ai_task_events WHERE seq=1"}, {"缺中间事件", "DELETE FROM tm_ai_task_events WHERE seq=2"}, {"缺末事件", "DELETE FROM tm_ai_task_events WHERE seq=3"},
		{"未知类型", "UPDATE tm_ai_task_events SET type='fake-sensitive-marker' WHERE seq=2"}, {"自由错误码", "UPDATE tm_ai_task_events SET error_code='fake-sensitive-marker' WHERE seq=2"},
		{"末状态不符", "UPDATE tm_ai_task_events SET type='claimed',status=2 WHERE seq=3"}, {"无效时点", "UPDATE tm_ai_task_events SET created_at='1900-01-01 00:00:00.000' WHERE seq=2"},
		{"事件表缺失", "RENAME TABLE tm_ai_task_events TO test_events_unavailable"},
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					e := newCancelHTTPEnv(t, level)
					v := e.create(t, 1, 1, taskBudget())
					lease, err := e.tasks.Claim(e.ctx, 1, v.TaskID, "worker_event_corrupt_01", 10*time.Second)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := e.tasks.FailOrRetry(e.ctx, 1, v.TaskID, lease.LeaseOwner, lease.Fence, "provider_unavailable", time.Millisecond); err != nil {
						t.Fatal(err)
					}
					token := e.token(t, 1, "client", e.key)
					healthy := decodeEventPage(t, e.mustRequest(t, "GET", eventPath(v.TaskID, 0, 20), token, "", 200))
					if healthy.UpperSeq != 3 || len(healthy.Items) != 3 {
						t.Fatal("破坏前事件页基线不符")
					}
					if err := repository.NewAITaskSchemaVerifier(e.db).Verify(e.ctx); err != nil {
						t.Fatal("破坏前保护基线失败", err)
					}
					if tc.name != "事件表缺失" {
						e.mustExec("DROP TRIGGER tm_ai_task_events_no_update")
						e.mustExec("DROP TRIGGER tm_ai_task_events_no_delete")
						e.mustExec("ALTER TABLE tm_ai_task_events DROP CHECK chk_tm_ai_task_event")
					}
					e.mustExec(tc.sql)
					raw := e.mustRequest(t, "GET", eventPath(v.TaskID, 0, 20), token, "", 503)
					var env map[string]json.RawMessage
					if err := json.Unmarshal(raw, &env); err != nil {
						t.Fatal(err)
					}
					if _, ok := env["data"]; ok {
						t.Fatal("损坏事件返回部分数据")
					}
					if strings.Contains(string(raw), "tm_ai_task_events") || strings.Contains(string(raw), "test_events_unavailable") {
						t.Fatal("泄漏数据库详情")
					}
					if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
						t.Fatal("读取故障释放冻结")
					}
				})
			}
		})
	}
}
