//go:build integration

package points_test

import (
	"bufio"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 本组是真JWT/真库/仅SELECT账号的实际HTTP流，不调用模型；沿用事件专项选择器。
func TestIntegrationAITaskMemberEventsStream(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCancelHTTPEnv(t, level)
			created := e.create(t, 1, 1, taskBudget())
			if _, err := e.tasks.Cancel(e.ctx, 1, created.TaskID); err != nil {
				t.Fatal(err)
			}
			limited := eventReadOnlyDB(t, e)
			if err := repository.VerifyAITaskStreamReadSchema(e.ctx, limited); err != nil {
				t.Fatal("受限账号读取结构检查失败", err)
			}
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
			streams, err := service.NewAITaskStreamService(reads, sessions)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			handler.NewAITaskStreamHandler(streams).Register(mux, auth)
			srv := httptest.NewServer(mux)
			defer srv.Close()
			path := func(id int64, after string) string {
				return srv.URL + "/api/v1/ai/tasks/" + strconv.FormatInt(id, 10) + "/stream?after_seq=" + after
			}
			get := func(url, token string) (int, string) {
				t.Helper()
				ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
				defer cancel()
				r, err := http.NewRequestWithContext(ctx, "GET", url, nil)
				if err != nil {
					t.Fatal(err)
				}
				r.Header.Set("Authorization", "Bearer "+token)
				res, err := srv.Client().Do(r)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				raw, err := io.ReadAll(io.LimitReader(res.Body, 8193))
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) > 8192 || res.Header.Get("Cache-Control") != "no-store" {
					t.Fatal("流缓存/大小约束失效")
				}
				return res.StatusCode, string(raw)
			}
			before := taskProbeCounts(e.pointsTestEnv)
			token := e.token(t, 1, "client", e.key)
			code, body := get(path(created.TaskID, "0"), token)
			if code != 200 || strings.Count(body, "event: progress") != 2 || !strings.Contains(body, "\"type\":\"cancelled\"") {
				t.Fatal(code, body)
			}
			code, resumed := get(path(created.TaskID, "1"), e.token(t, 1, "client", e.old))
			if code != 200 || strings.Count(resumed, "event: progress") != 1 || strings.Contains(resumed, "id: 1\n") {
				t.Fatal(code, resumed)
			}
			code, resumed = get(path(created.TaskID, "2"), token)
			if code != 200 || strings.Contains(resumed, "event:") {
				t.Fatal("终态末断点仍发送/等待", code, resumed)
			}
			code, cross := get(path(created.TaskID, "0"), e.token(t, 2, "client", e.key))
			if code != 404 {
				t.Fatal(code)
			}
			code, missing := get(path(99999999, "0"), e.token(t, 2, "client", e.key))
			if code != 404 || cross != missing {
				t.Fatal("越权存在性泄漏", code)
			}
			code, _ = get(path(created.TaskID, "0"), e.token(t, 1, "admin", e.key))
			if code != 401 {
				t.Fatal(code)
			}
			code, _ = get(path(created.TaskID, "3"), token)
			if code != 400 {
				t.Fatal(code)
			}
			if !reflect.DeepEqual(before, taskProbeCounts(e.pointsTestEnv)) {
				t.Fatal("流读取改写账本/事件")
			}

			// 新任务在连接期间真实推进；一个独立事务取消不是流读取的副作用。
			live := e.create(t, 2, 1, taskBudget())
			ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
			defer cancel()
			r, err := http.NewRequestWithContext(ctx, "GET", path(live.TaskID, "0"), nil)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Authorization", "Bearer "+token)
			res, err := srv.Client().Do(r)
			if err != nil {
				t.Fatal(err)
			}
			buf := bufio.NewReader(res.Body)
			if line, err := buf.ReadString('\n'); err != nil || line != ": connected\n" {
				res.Body.Close()
				t.Fatal("未实际开始流", err)
			}
			if _, err := e.tasks.Cancel(e.ctx, 1, live.TaskID); err != nil {
				res.Body.Close()
				t.Fatal(err)
			}
			raw, err := io.ReadAll(buf)
			res.Body.Close()
			if err != nil || !strings.Contains(string(raw), "\"type\":\"cancelled\"") {
				t.Fatal(string(raw), err)
			}

			// 连接已建立后撤权，下一轮不能再读取/推送任务数据。
			live = e.create(t, 3, 1, taskBudget())
			r, err = http.NewRequestWithContext(ctx, "GET", path(live.TaskID, "0"), nil)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Authorization", "Bearer "+token)
			res, err = srv.Client().Do(r)
			if err != nil {
				t.Fatal(err)
			}
			buf = bufio.NewReader(res.Body)
			if _, err = buf.ReadString('\n'); err != nil {
				res.Body.Close()
				t.Fatal(err)
			}
			e.mustExec("UPDATE addon_member SET status=0 WHERE id=1")
			raw, err = io.ReadAll(buf)
			res.Body.Close()
			if err != nil || !strings.Contains(string(raw), "event: error\ndata: {\"code\":40100}") {
				t.Fatal("撤权后未关闭", string(raw), err)
			}
			e.mustExec("UPDATE addon_member SET status=1 WHERE id=1")
			if _, err := e.tasks.Cancel(e.ctx, 1, live.TaskID); err != nil {
				t.Fatal(err)
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
			e.mustExec("ALTER TABLE tm_ai_task_events DROP CHECK chk_tm_ai_task_event")
			if err := repository.VerifyAITaskStreamReadSchema(e.ctx, limited); err == nil {
				t.Fatal("缺少事件约束仍通过启动检查")
			}
		})
	}
}
