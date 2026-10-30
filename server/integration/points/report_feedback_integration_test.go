//go:build integration

package points_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func feedbackFixture(t *testing.T, level sql.IsolationLevel) (*workflowEnv, *service.AIFeedbackService) {
	t.Helper()
	e := newWorkflowEnv(t, level, "pass")
	if _, err := e.worker(t, true).Run(e.ctx, e.v.UserID, e.v.ID); err != nil {
		t.Fatal(err)
	}
	reportDDL(t, e.pointsTestEnv, "20261020_create_tm_ai_report_feedbacks.sql")
	s, err := service.NewAIFeedbackService(repository.NewAIFeedbackRepoWithIsolation(e.db, level))
	if err != nil {
		t.Fatal(err)
	}
	return e, s
}

type feedbackUnknownStore struct{ feedback.Store }

func (s feedbackUnknownStore) Within(ctx context.Context, f func(feedback.Tx) error) error {
	err := s.Store.Within(ctx, f)
	if err == nil {
		return errCheckTest
	}
	return err
}

func TestIntegrationAIReportFeedbackLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, s := feedbackFixture(t, level)
			before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			if err := repository.NewAIFeedbackSchemaVerifier(e.db).Verify(e.ctx); err != nil {
				t.Fatal("反馈启动探针", err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedbacks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedback_events") != 0 {
				t.Fatal("探针残留")
			}
			if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
				t.Fatal(err)
			}
			const n = 24
			ids := make(chan int64, n)
			errs := make(chan error, n)
			var wg sync.WaitGroup
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					r, err := s.Submit(e.ctx, e.v.UserID, e.v.ID, "fact")
					ids <- r.ID
					errs <- err
				}()
			}
			wg.Wait()
			close(ids)
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal("并发受理", err)
				}
			}
			var id int64
			for v := range ids {
				if id == 0 {
					id = v
				}
				if id != v || v == 0 {
					t.Fatal("重复受理")
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedbacks") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedback_events") != 1 {
				t.Fatal("受理与审计未唯一")
			}
			var won, conflicts, bad atomic.Int32
			var winner atomic.Int64
			for i := 0; i < n; i++ {
				actor := int64(i + 7)
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := s.Decide(e.ctx, actor, id, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
					if err == nil {
						won.Add(1)
						winner.Store(actor)
					} else if errors.Is(err, feedback.ErrConflict) {
						conflicts.Add(1)
					} else {
						bad.Add(1)
					}
				}()
			}
			wg.Wait()
			if won.Load() != 1 || conflicts.Load() != n-1 || bad.Load() != 0 {
				t.Fatal("并发处理未CAS", won.Load(), conflicts.Load(), bad.Load())
			}
			final, err := s.Decide(e.ctx, winner.Load(), id, feedback.Decision{ExpectedVersion: 2, Status: "resolved", Resolution: "reviewed"})
			if err != nil || final.Version != 3 {
				t.Fatal(final, err)
			}
			replay, err := s.Decide(e.ctx, winner.Load(), id, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
			if err != nil || replay != final {
				t.Fatal("已完成后的同操作重放", replay, err)
			}
			list, err := s.ForReport(e.ctx, e.v.UserID, e.v.ID)
			if err != nil || len(list.Items) != 1 || list.Items[0] != final {
				t.Fatal("会员未查询到终态", list, err)
			}
			detail, err := s.Detail(e.ctx, 7, id)
			if err != nil || len(detail.Events) != 3 || detail.Events[0].ActorID != 0 || detail.Events[1].ActorID != winner.Load() {
				t.Fatal("审计未闭环", detail, err)
			}
			if _, err = s.ForReport(e.ctx, e.v.UserID+1, e.v.ID); !errors.Is(err, feedback.ErrNotFound) {
				t.Fatal("跨会员可读", err)
			}
			if _, err = s.Submit(e.ctx, e.v.UserID+1, e.v.ID, "fact"); !errors.Is(err, feedback.ErrNotFound) {
				t.Fatal("跨会员可写", err)
			}
			if before != e.queryInt("SELECT COUNT(*) FROM tm_points_txns") || e.submits.Load() != 1 || e.server.Count() != 3 {
				t.Fatal("反馈引起扣费或外呼")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

func TestIntegrationAIReportFeedbackRollback(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, s := feedbackFixture(t, level)
			e.mustExec(`CREATE TRIGGER test_only_feedback_fail BEFORE INSERT ON tm_ai_report_feedback_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only-audit-failure'`)
			r, err := s.Submit(e.ctx, e.v.UserID, e.v.ID, "source")
			if err == nil || r.ID != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedbacks") != 0 {
				t.Fatal("失败受理未原子回滚", r, err)
			}
			e.mustExec(`DROP TRIGGER test_only_feedback_fail`)
			u, err := service.NewAIFeedbackService(feedbackUnknownStore{repository.NewAIFeedbackRepoWithIsolation(e.db, level)})
			if err != nil {
				t.Fatal(err)
			}
			r, err = u.Submit(e.ctx, e.v.UserID, e.v.ID, "source")
			if err == nil || r.ID != 0 {
				t.Fatal("未知提交返回成功DTO")
			}
			list, err := s.ForReport(e.ctx, e.v.UserID, e.v.ID)
			if err != nil || len(list.Items) != 1 {
				t.Fatal("未知提交不能恢复查询", err)
			}
			id := list.Items[0].ID
			e.mustExec(`CREATE TRIGGER test_only_feedback_fail BEFORE INSERT ON tm_ai_report_feedback_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only-audit-failure'`)
			r, err = s.Decide(e.ctx, 7, id, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
			if err == nil || r.ID != 0 {
				t.Fatal("失败处理返回成功")
			}
			e.mustExec(`DROP TRIGGER test_only_feedback_fail`)
			d, err := s.Detail(e.ctx, 7, id)
			if err != nil || d.Item.Version != 1 || len(d.Events) != 1 {
				t.Fatal("状态与审计未一起回滚", d, err)
			}
			r, err = u.Decide(e.ctx, 7, id, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
			if err == nil || r.ID != 0 {
				t.Fatal("未知处理泄漏DTO")
			}
			d, err = s.Detail(e.ctx, 7, id)
			if err != nil || d.Item.Version != 2 || len(d.Events) != 2 {
				t.Fatal("未知处理不能查询", d, err)
			}
			if _, err = s.Decide(e.ctx, 7, id, feedback.Decision{ExpectedVersion: 2, Status: "dismissed", Resolution: "unconfirmed"}); err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{`DELETE FROM tm_ai_report_feedbacks`, `DELETE FROM tm_ai_report_feedback_events`, `UPDATE tm_ai_report_feedback_events SET actor_id=99`, `UPDATE tm_ai_report_feedbacks SET category='other'`} {
				if _, err := e.db.ExecContext(e.ctx, q); err == nil {
					t.Fatal("不可变保护失效", q)
				}
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

func TestIntegrationAIReportFeedbackHTTP(t *testing.T) {
	e, s := feedbackFixture(t, sql.LevelReadCommitted)
	h := reportHTTPFixture(t, e.reportEnv, "pass")
	v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: h.key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(e.db))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(ms)
	if err != nil {
		t.Fatal(err)
	}
	mux := h.server.Config.Handler.(*http.ServeMux)
	handler.NewAIFeedbackHandler(s).RegisterMember(mux, auth)
	a := http.NewServeMux()
	handler.NewAIFeedbackHandler(s).RegisterInternal(a)
	token := strings.Repeat("j", 40)
	aa, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: token}})
	if err != nil {
		t.Fatal(err)
	}
	admin := httptest.NewServer(aa(a))
	defer admin.Close()
	request := func(base, method, path, b, key string, want int) []byte {
		t.Helper()
		r, err := http.NewRequestWithContext(e.ctx, method, base+path, strings.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+key)
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("反馈HTTP", res.StatusCode, string(raw))
		}
		return raw
	}
	path := "/api/v1/ai/tasks/" + strconv.FormatInt(e.v.ID, 10) + "/feedback"
	memberToken := h.token(t, e.v.UserID, "client")
	raw := request(h.server.URL, "POST", path, `{"category":"display"}`, memberToken, 200)
	var receipt struct {
		Data dto.AIFeedback `json:"data"`
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.Data.ID == 0 {
		t.Fatal("无受理单")
	}
	request(h.server.URL, "GET", path, "", h.token(t, 2, "client"), 404)
	request(h.server.URL, "GET", path, "", h.token(t, 1, "admin"), 401)
	request(admin.URL, "GET", "/internal/v1/ai-feedback", "", memberToken, 401)
	internalPath := "/internal/v1/ai-feedback/" + strconv.FormatInt(receipt.Data.ID, 10)
	page := request(admin.URL, "GET", "/internal/v1/ai-feedback?limit=1", "", token, 200)
	var queue struct {
		Data dto.AIFeedbackPage `json:"data"`
	}
	if json.Unmarshal(page, &queue) != nil || len(queue.Data.Items) != 1 || queue.Data.Items[0].ID != receipt.Data.ID || queue.Data.HasMore || queue.Data.NextAfterID != 0 {
		t.Fatal("内部待处理列表未查到原受理单")
	}
	request(admin.URL, "GET", internalPath, "", token, 200)
	request(admin.URL, "GET", "/internal/v1/ai-feedback?after_id="+strconv.FormatInt(receipt.Data.ID, 10)+"&limit=1", "", token, 200)
	request(h.server.URL, "GET", path, "", token, 401)
	request(admin.URL, "POST", internalPath+"/decisions", `{"expectedVersion":1,"status":"processing","resolution":""}`, token, 200)
	request(admin.URL, "POST", internalPath+"/decisions", `{"expectedVersion":2,"status":"resolved","resolution":"referred"}`, token, 200)
	raw = request(h.server.URL, "GET", path, "", memberToken, 200)
	if !strings.Contains(string(raw), `"status":"resolved"`) || strings.Contains(string(raw), "actorId") || strings.Contains(string(raw), "reportDigest") || strings.Contains(string(raw), "userId") {
		t.Fatal("会员查询未脱敏闭环", string(raw))
	}
	if os.Getenv("TM_TEST_FEEDBACK_ADMIN_RUNNER") != "" {
		request(h.server.URL, "POST", path, `{"category":"source"}`, memberToken, 200)
	}
	e.mustExec(`UPDATE addon_member SET status=0 WHERE id=1`)
	request(h.server.URL, "GET", path, "", memberToken, 401)
	if runner := os.Getenv("TM_TEST_FEEDBACK_ADMIN_RUNNER"); runner != "" {
		feedbackConsoleBrowser(t, e, s, admin.URL, token, runner)
	}
}

func feedbackConsoleBrowser(t *testing.T, e *workflowEnv, s *service.AIFeedbackService, upstream, credential, runner string, targetTask ...int64) {
	t.Helper()
	if !filepath.IsAbs(runner) || !strings.HasSuffix(strings.ToLower(runner), ".exe") {
		t.Fatal("测试运行器必须为显式编译的绝对路径测试程序")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()
	previous := e.ctx
	e.pointsTestEnv.ctx = ctx
	defer func() { e.pointsTestEnv.ctx = previous }()
	id := e.v.ID
	if len(targetTask) > 1 || len(targetTask) == 1 && targetTask[0] <= 0 {
		t.Fatal("每轮只允许一个明确报告")
	}
	if len(targetTask) == 1 {
		id = targetTask[0]
	}
	r, err := s.Submit(ctx, e.v.UserID, id, "source")
	if err != nil {
		t.Fatal(err)
	}
	before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
	cmd := exec.CommandContext(ctx, runner, "-test.run=^TestFeedbackConsoleBrowser$", "-test.v")
	// 测试子进程只继承系统/编译环境，不继承宿主供应商密钥或生产配置。
	cmd.Env = append(localWorkerEnvironment("", ""), "TM_TEST_FEEDBACK_UPSTREAM="+upstream, "TM_TEST_FEEDBACK_CREDENTIAL="+credential, "TM_TEST_FEEDBACK_UI_ORIGIN="+os.Getenv("TM_TEST_FEEDBACK_UI_ORIGIN"))
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal("测试运行器管道失败")
	}
	if err := cmd.Start(); err != nil {
		t.Fatal("测试运行器启动失败")
	}
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "FEEDBACK_ADMIN_BROWSER_READY ") {
			t.Log(line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Error("测试运行器输出读取失败")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal("后台反馈页面会话未通过（运行器详情未公开）")
	}
	d, err := s.Detail(ctx, 7, r.ID)
	if err != nil || d.Item.Version != 3 || len(d.Events) != 3 || d.Events[1].ActorID != 7 || d.Events[2].ActorID != 7 {
		t.Fatal("后台页面未写入真实SQL处理历史", err)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before || e.submits.Load() != 1 || e.server.Count() != 3 {
		t.Fatal("后台反馈操作影响账务或增加模型调用")
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
}

func TestIntegrationAIReportFeedbackSchemaGuards(t *testing.T) {
	for _, name := range []string{"tm_ai_feedback_insert", "tm_ai_feedback_update", "tm_ai_feedback_no_delete", "tm_ai_feedback_event_insert", "tm_ai_feedback_event_no_update", "tm_ai_feedback_event_no_delete", "case_check", "event_check", "unique"} {
		t.Run(name, func(t *testing.T) {
			e, _ := feedbackFixture(t, sql.LevelReadCommitted)
			switch name {
			case "case_check":
				e.mustExec(`ALTER TABLE tm_ai_report_feedbacks ALTER CHECK chk_tm_ai_feedback NOT ENFORCED`)
			case "event_check":
				e.mustExec(`ALTER TABLE tm_ai_report_feedback_events ALTER CHECK chk_tm_ai_feedback_event NOT ENFORCED`)
			case "unique":
				e.mustExec(`ALTER TABLE tm_ai_report_feedbacks DROP INDEX uq_tm_ai_feedback_category`)
			default:
				e.mustExec("DROP TRIGGER " + name)
			}
			if err := repository.NewAIFeedbackSchemaVerifier(e.db).Verify(e.ctx); err == nil {
				t.Fatal("启动探针未拒绝缺失保护", name)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedbacks") != 0 {
				t.Fatal("失败探针残留")
			}
		})
	}
}
