//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

// 仅可丢弃库与回环 HTTP：真实签名→会员查询→用户隔离 SQL→DTO，不运行模型。
func TestIntegrationAITaskMemberReads(t *testing.T) {
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, sql.LevelReadCommitted)
	// 仅创建 GVA 会员身份查询所需的同名最小表，不迁移/修改实际 GVA 数据。
	e.mustExec(`CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3) NULL); INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL),(3,0,NULL),(4,1,UTC_TIMESTAMP(3))`)
	for _, uid := range []int64{1, 2} {
		r := pointGrant(now, 100)
		r.UserID = uid
		r.Key = "test_member_grant_" + strconv.FormatInt(uid, 10)
		r.BizID = "test-member-grant-" + strconv.FormatInt(uid, 10)
		if _, err := ledger.Grant(e.ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	var ids []int64
	for i := 1; i <= 4; i++ {
		r := taskRequest(i)
		if i == 4 {
			r.UserID = 2
		}
		created, err := tasks.Create(e.ctx, r, points.Price{Version: "v1", Points: 7}, taskBudget())
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, created.TaskID)
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
	reads, err := service.NewAITaskReadService(repo)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.NewAITaskReadHandler(reads).Register(mux, auth)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	token := func(uid int64, kind, secret string) string {
		c := jwt.MapClaims{"ID": uid, "UserType": kind, "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	get := func(path, bearer string, want int) []byte {
		t.Helper()
		req, err := http.NewRequestWithContext(e.ctx, "GET", srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+bearer)
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal(path, res.StatusCode, string(body))
		}
		for _, s := range []string{key, old, "leaseOwner", "fence", "req_hash", "idem_key", "budget_json", "reports", "provider_payload"} {
			if strings.Contains(string(body), s) {
				t.Fatal("只读响应包含内部数据", s)
			}
		}
		return body
	}
	path := "/api/v1/ai/tasks/" + strconv.FormatInt(ids[0], 10)
	startTxns := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
	startEvents := e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events")
	get(path, token(1, "client", old), 200)
	for _, tc := range []struct {
		uid  int64
		kind string
	}{{1, "admin"}, {1, ""}, {3, "client"}, {4, "client"}, {99, "client"}} {
		get(path, token(tc.uid, tc.kind, key), 401)
	}
	get(path, token(1, "client", strings.Repeat("c", 40)), 401)
	other := get(path, token(2, "client", key), 404)
	missing := get("/api/v1/ai/tasks/999999999", token(2, "client", key), 404)
	if string(other) != string(missing) {
		t.Fatal("跨用户查询泄漏任务存在性")
	}
	get(path+"?user_id=1", token(2, "client", key), 400)
	var page struct {
		Data dto.AITaskHistory `json:"data"`
	}
	if err := json.Unmarshal(get("/api/v1/ai/tasks?limit=2", token(1, "client", key), 200), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Items) != 2 || page.Data.Items[0].ID != ids[2] || page.Data.Items[1].ID != ids[1] || !page.Data.HasMore || page.Data.NextBeforeID != ids[1] {
		t.Fatal(page)
	}
	if err := json.Unmarshal(get("/api/v1/ai/tasks?limit=2&before_id="+strconv.FormatInt(page.Data.NextBeforeID, 10), token(1, "client", key), 200), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Items) != 1 || page.Data.Items[0].ID != ids[0] || page.Data.HasMore || page.Data.NextBeforeID != 0 {
		t.Fatal(page)
	}
	if err := json.Unmarshal(get("/api/v1/ai/tasks", token(2, "client", key), 200), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Items) != 1 || page.Data.Items[0].ID != ids[3] {
		t.Fatal("历史越权", page)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != startTxns || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != startEvents || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 21 {
		t.Fatal("GET 修改任务或积分")
	}
	e.mustExec(`UPDATE addon_member SET status=0 WHERE id=1`)
	get(path, token(1, "client", key), 401)
	e.mustExec(`UPDATE addon_member SET status=1 WHERE id=1`)
	if _, err := tasks.Cancel(e.ctx, 1, ids[0]); err != nil {
		t.Fatal(err)
	}
	var detail struct {
		Data dto.AITaskProgress `json:"data"`
	}
	if err := json.Unmarshal(get(path, token(1, "client", key), 200), &detail); err != nil || detail.Data.Status != task.Cancelled || detail.Data.ReportAvailable || detail.Data.ErrorCode != "cancelled" {
		t.Fatal(detail, err)
	}
	requirePointAudit(t, e, ledger)
	e.mustExec(`RENAME TABLE addon_member TO test_member_unavailable`)
	get(path, token(1, "client", key), 503)
	get(path, token(1, "admin", key), 401)
}
