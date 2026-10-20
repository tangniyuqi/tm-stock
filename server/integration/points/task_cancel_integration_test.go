//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type cancelHTTPEnv struct {
	*pointsTestEnv
	db       *sql.DB
	repo     *repository.AITaskRepo
	tasks    *service.AITaskService
	ledger   *service.PointsService
	server   *httptest.Server
	auth     func(http.Handler) http.Handler
	key, old string
}

func newCancelHTTPEnv(t *testing.T, level sql.IsolationLevel) *cancelHTTPEnv {
	t.Helper()
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
	e.mustExec(`CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3) NULL); INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL),(3,0,NULL),(4,1,UTC_TIMESTAMP(3))`)
	for _, uid := range []int64{1, 2} {
		r := pointGrant(now, 100)
		r.UserID = uid
		r.Key = fmt.Sprintf("test_cancel_grant_%02d", uid)
		r.BizID = fmt.Sprintf("test-cancel-grant-%02d", uid)
		if _, err := ledger.Grant(e.ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	key, old := strings.Repeat("a", 40), strings.Repeat("b", 40)
	v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: key, PreviousKey: old, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(db))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewAITaskCancelService(tasks)
	if err != nil {
		t.Fatal(err)
	}
	reads, err := service.NewAITaskReadService(repo)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.NewAITaskCancelHandler(svc).Register(mux, auth)
	handler.NewAITaskReadHandler(reads).Register(mux, auth)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &cancelHTTPEnv{pointsTestEnv: e, db: db, repo: repo, tasks: tasks, ledger: ledger, server: srv, auth: auth, key: key, old: old}
}

func (e *cancelHTTPEnv) token(t *testing.T, uid int64, kind, secret string) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"ID": uid, "UserType": kind, "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func (e *cancelHTTPEnv) create(t *testing.T, number int, uid int64, b task.Budget) task.Created {
	t.Helper()
	r := taskRequest(number)
	r.UserID = uid
	v, err := e.tasks.Create(e.ctx, r, points.Price{Version: "v1", Points: 7}, b)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func cancelPath(id int64) string { return "/api/v1/ai/tasks/" + strconv.FormatInt(id, 10) + "/cancel" }
func (e *cancelHTTPEnv) request(method, path, token, body string) (int, []byte, error) {
	r, err := http.NewRequestWithContext(e.ctx, method, e.server.URL+path, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	res, err := e.server.Client().Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8193))
	if err != nil {
		return 0, nil, err
	}
	if len(raw) > 8192 || res.Header.Get("Cache-Control") != "no-store" {
		return 0, nil, points.ErrInvariant
	}
	for _, hidden := range []string{e.key, e.old, "leaseOwner", "fence", "req_hash", "idem_key", "budget_json", "provider_payload", "fake-sensitive-marker"} {
		if strings.Contains(string(raw), hidden) {
			return 0, nil, points.ErrInvariant
		}
	}
	return res.StatusCode, raw, nil
}
func (e *cancelHTTPEnv) mustRequest(t *testing.T, method, path, token, body string, want int) []byte {
	t.Helper()
	code, raw, err := e.request(method, path, token, body)
	if err != nil || code != want {
		t.Fatal("取消 HTTP 不符", code, err, string(raw))
	}
	return raw
}
func decodeCancellation(t *testing.T, raw []byte) dto.AITaskCancellation {
	t.Helper()
	var out struct {
		Data dto.AITaskCancellation `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(env["data"], &fields); err != nil || len(fields) != 6 || out.Data.ReportAvailable {
		t.Fatal("取消 DTO 越界", err)
	}
	return out.Data
}

func TestIntegrationAITaskMemberCancelLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCancelHTTPEnv(t, level)
			good := e.token(t, 1, "client", e.key)
			queued := e.create(t, 1, 1, taskBudget())
			running := e.create(t, 2, 1, taskBudget())
			other := e.create(t, 3, 2, taskBudget())
			lease, err := e.tasks.Claim(e.ctx, 1, running.TaskID, "worker_cancel_http_01", 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			beforeEvents, beforeTxns := e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events"), e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			cross := e.mustRequest(t, "POST", cancelPath(queued.TaskID), e.token(t, 2, "client", e.key), "{}", 404)
			missing := e.mustRequest(t, "POST", cancelPath(999999999), e.token(t, 2, "client", e.key), "{}", 404)
			if string(cross) != string(missing) {
				t.Fatal("取消泄漏任务存在性")
			}
			for _, tc := range []struct {
				uid  int64
				kind string
			}{{1, "admin"}, {1, ""}, {3, "client"}, {4, "client"}, {99, "client"}} {
				e.mustRequest(t, "POST", cancelPath(queued.TaskID), e.token(t, tc.uid, tc.kind, e.key), "{}", 401)
			}
			e.mustRequest(t, "POST", cancelPath(queued.TaskID), e.token(t, 1, "client", strings.Repeat("c", 40)), "{}", 401)
			e.mustRequest(t, "POST", cancelPath(queued.TaskID)+"?user_id=1", good, "{}", 400)
			e.mustRequest(t, "POST", cancelPath(queued.TaskID), good, `{"reason":"fake-sensitive-marker"}`, 400)
			e.mustExec("UPDATE addon_member SET status=0 WHERE id=1")
			e.mustRequest(t, "POST", cancelPath(queued.TaskID), good, "{}", 401)
			e.mustExec("UPDATE addon_member SET status=1 WHERE id=1")
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != beforeEvents || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != beforeTxns {
				t.Fatal("拒绝请求修改状态")
			}
			out := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(queued.TaskID), e.token(t, 1, "client", e.old), "{}", 200))
			if out.Status != task.Cancelled || out.ID != queued.TaskID || out.ErrorCode != "cancelled" || e.queryInt("SELECT state FROM tm_points_holds WHERE id=?", queued.HoldID) != points.Released {
				t.Fatal(out)
			}
			if again := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(queued.TaskID), good, "", 200)); again != out {
				t.Fatal("终态重放改变")
			}
			out = decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(running.TaskID), good, "{}", 200))
			if out.Status != task.Running || !out.CancelRequested || e.queryInt("SELECT state FROM tm_points_holds WHERE id=?", running.HoldID) != points.Holding {
				t.Fatal("运行取消伪终态", out)
			}
			if again := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(running.TaskID), good, "{}", 200)); again != out {
				t.Fatal("运行取消重复事件")
			}
			if _, err := e.tasks.Heartbeat(e.ctx, 1, running.TaskID, lease.LeaseOwner, lease.Fence, time.Second); !errors.Is(err, task.ErrCancelled) {
				t.Fatal(err)
			}
			if _, err := e.tasks.FailOrRetry(e.ctx, 1, running.TaskID, lease.LeaseOwner, lease.Fence, "internal_failure", 0); err != nil {
				t.Fatal(err)
			}
			if final := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(running.TaskID), good, "{}", 200)); final.Status != task.Cancelled {
				t.Fatal(final)
			}
			if e.queryInt("SELECT state FROM tm_points_holds WHERE id=?", other.HoldID) != points.Holding {
				t.Fatal("波及另一会员冻结")
			}
			expiringBudget := taskBudget()
			expiringBudget.MaxDurationMS = 1000
			expired := e.create(t, 4, 1, expiringBudget)
			v, err := e.tasks.Inspect(e.ctx, 1, expired.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, e.pointsTestEnv, v.Deadline)
			if _, err := e.tasks.Expire(e.ctx, 1, expired.TaskID); err != nil {
				t.Fatal(err)
			}
			if out := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(expired.TaskID), good, "{}", 200)); out.Status != task.Expired {
				t.Fatal("取消改写到期", out)
			}
			failed := e.create(t, 5, 1, taskBudget())
			failedLease, err := e.tasks.Claim(e.ctx, 1, failed.TaskID, "worker_failed_http_01", 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.tasks.FailOrRetry(e.ctx, 1, failed.TaskID, failedLease.LeaseOwner, failedLease.Fence, "guard_rejected", 0); err != nil {
				t.Fatal(err)
			}
			if out := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(failed.TaskID), good, "{}", 200)); out.Status != task.Failed || out.ErrorCode != "guard_rejected" {
				t.Fatal("取消改写失败", out)
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
			e.mustExec("RENAME TABLE addon_member TO test_member_unavailable")
			e.mustRequest(t, "POST", cancelPath(other.TaskID), good, "{}", 503)
			e.mustRequest(t, "POST", cancelPath(other.TaskID), e.token(t, 1, "admin", e.key), "{}", 401)
		})
	}
}

func TestIntegrationAITaskMemberCancelConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCancelHTTPEnv(t, level)
			token := e.token(t, 1, "client", e.key)
			for _, running := range []bool{false, true} {
				v := e.create(t, 1+e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks"), 1, taskBudget())
				var lease task.Task
				var err error
				if running {
					lease, err = e.tasks.Claim(e.ctx, 1, v.TaskID, "worker_cancel_race_01", 10*time.Second)
					if err != nil {
						t.Fatal(err)
					}
				}
				start := make(chan struct{})
				failures := make(chan error, 41)
				var wg sync.WaitGroup
				for i := 0; i < 40; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						<-start
						code, _, err := e.request("POST", cancelPath(v.TaskID), token, "{}")
						if err != nil {
							failures <- err
						} else if code != 200 {
							failures <- fmt.Errorf("取消状态=%d", code)
						}
					}()
				}
				if running {
					wg.Add(1)
					go func() {
						defer wg.Done()
						<-start
						_, err := e.tasks.FailOrRetry(e.ctx, 1, v.TaskID, lease.LeaseOwner, lease.Fence, "guard_rejected", 0)
						if err != nil && !errors.Is(err, task.ErrLease) {
							failures <- err
						}
					}()
				}
				close(start)
				wg.Wait()
				close(failures)
				for err := range failures {
					t.Fatal(err)
				}
				if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE task_id=? AND type IN ('failed','cancelled')", v.TaskID) != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE task_id=? AND type='cancel_requested'", v.TaskID) > 1 || e.queryInt("SELECT state FROM tm_points_holds WHERE id=?", v.HoldID) != points.Released {
					t.Fatal("取消竞争重复终结/释放")
				}
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

func TestIntegrationAITaskMemberCancelRollbackAndUnknown(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCancelHTTPEnv(t, level)
			token := e.token(t, 1, "client", e.key)
			v := e.create(t, 1, 1, taskBudget())
			beforeTxns, beforeEvents := e.queryInt("SELECT COUNT(*) FROM tm_points_txns"), e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events")
			e.mustExec("CREATE TRIGGER test_cancel_event_failure BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='cancelled' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fake-sensitive-marker'; END IF; END")
			e.mustRequest(t, "POST", cancelPath(v.TaskID), token, "{}", 503)
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != beforeTxns || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != beforeEvents || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", v.TaskID) != task.Queued {
				t.Fatal("取消故障未整体回滚")
			}
			e.mustExec("DROP TRIGGER test_cancel_event_failure")
			fault := errors.New("fake-sensitive-marker")
			unknown, err := service.NewAITaskService(abortTaskStore{db: e.db, level: level, commit: true, after: fault}, e.ledger)
			if err != nil {
				t.Fatal(err)
			}
			wrapper, err := service.NewAITaskCancelService(unknown)
			if err != nil {
				t.Fatal(err)
			}
			unknownMux := http.NewServeMux()
			handler.NewAITaskCancelHandler(wrapper).Register(unknownMux, e.auth)
			unknownServer := httptest.NewServer(unknownMux)
			defer unknownServer.Close()
			originalServer := e.server
			e.server = unknownServer
			e.mustRequest(t, "POST", cancelPath(v.TaskID), token, "{}", 503)
			e.server = originalServer
			if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", v.TaskID) != task.Cancelled || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("未知回执夹具未实际提交")
			}
			out := decodeCancellation(t, e.mustRequest(t, "POST", cancelPath(v.TaskID), token, "{}", 200))
			if out.Status != task.Cancelled || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != beforeTxns+1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != beforeEvents+1 {
				t.Fatal("原任务复核重复释放")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}
