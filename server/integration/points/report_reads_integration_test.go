//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅测试当前发布批准；不能作为真实模型/质量/可见授权控制器装入生产。
type readSQLPublication struct {
	x             *reportEnv
	allowed       atomic.Bool
	calls         atomic.Int32
	denyAt        int32
	hook          func(context.Context) error
	model, filing string // 固定测试批准的实际披露；不得从待发布报告反推放行配置。
}

func (c *readSQLPublication) Recheck(ctx context.Context, p service.AIReportPublication) error {
	n := c.calls.Add(1)
	policyOK := p.PolicyDigest == c.x.policy.Digest() || c.x.deepPolicy != nil && p.PolicyDigest == c.x.deepPolicy.Digest()
	if !c.allowed.Load() || n == c.denyAt || p.UserID != c.x.v.UserID || p.TaskID != c.x.v.ID || p.SubjectID != c.x.v.SubjectID || !finalreport.ValidDigest(p.ReportDigest) || !policyOK || p.ModelName != c.model || p.FilingNo != c.filing {
		return finalreport.ErrUnavailable
	}
	if c.hook != nil {
		return c.hook(ctx)
	}
	return ctx.Err()
}

func TestReadSQLPublicationModelBinding(t *testing.T) {
	policy, err := finalreport.NewPolicy(finalreport.Config{Ref: "test-only", ModelName: "deepseek-flash", FilingNo: "test-only-filing", EntailmentVersion: "test-ent", SemanticVersion: "test-sem", AllowedDomains: []string{"evidence.example.test"}},
		func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
			return finalreport.Uncertain, nil
		}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	x := &reportEnv{policy: policy}
	x.v.ID, x.v.UserID, x.v.SubjectID = 1, 1, 1
	c := &readSQLPublication{x: x, model: "deepseek-flash", filing: "test-only-filing"}
	c.allowed.Store(true)
	p := service.AIReportPublication{UserID: 1, TaskID: 1, SubjectID: 1, ReportDigest: strings.Repeat("a", 64), PolicyDigest: policy.Digest(), ModelName: c.model, FilingNo: c.filing}
	if err := c.Recheck(context.Background(), p); err != nil {
		t.Fatal("实际批准模型被测试夹具误拒绝", err)
	}
	for _, change := range []func(*service.AIReportPublication){func(p *service.AIReportPublication) { p.ModelName = "测试模型" }, func(p *service.AIReportPublication) { p.FilingNo = "other" }, func(p *service.AIReportPublication) { p.PolicyDigest = strings.Repeat("b", 64) }} {
		bad := p
		change(&bad)
		if c.Recheck(context.Background(), bad) == nil {
			t.Fatal("模型、披露或政策错绑未拒绝")
		}
	}
}

type readSQLUnknownStore struct {
	*repository.AITaskRepo
	calls atomic.Int32
}

func (s *readSQLUnknownStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	err := s.AITaskRepo.WithinReportTx(ctx, fn)
	if err == nil && s.calls.Add(1) == 2 {
		return errCheckTest
	}
	return err
}

type reportHTTPEnv struct {
	x       *reportEnv
	reader  *service.AIReportReadService
	control *readSQLPublication
	server  *httptest.Server
	key     string
}

func reportHTTPFixture(t *testing.T, x *reportEnv, mode string) *reportHTTPEnv {
	return reportHTTPFixtureWithMembers(t, x, mode, true)
}
func reportHTTPFixtureWithMembers(t *testing.T, x *reportEnv, mode string, createMembers bool) *reportHTTPEnv {
	return reportHTTPFixtureWithModel(t, x, mode, createMembers, "测试模型", "test-only-filing")
}
func reportHTTPFixtureWithModel(t *testing.T, x *reportEnv, mode string, createMembers bool, model, filing string) *reportHTTPEnv {
	t.Helper()
	if createMembers {
		x.mustExec(`CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL),(3,0,NULL)`)
	}
	ctl := &readSQLPublication{x: x, model: model, filing: filing}
	ctl.allowed.Store(true)
	var store finalreport.Store = x.repo
	if mode == "unknown_read" {
		store = &readSQLUnknownStore{AITaskRepo: x.repo}
	}
	base, err := service.NewAIReportService(store, x.ledger, x.policy, false)
	if err != nil {
		t.Fatal(err)
	}
	var deep *service.AIReportService
	if x.deepPolicy != nil {
		deep, err = service.NewAIReportService(store, x.ledger, x.deepPolicy, false)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader, err := service.NewAIReportReadServiceWithDeep(base, deep, ctl, mode != "off")
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("r", 40)
	verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(x.db))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		t.Fatal(err)
	}
	reads, err := service.NewAITaskReadServiceWithReports(x.repo, reader)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.NewAITaskReadHandler(reads).Register(mux, auth)
	handler.NewAIReportReadHandler(reader).Register(mux, auth)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &reportHTTPEnv{x, reader, ctl, srv, key}
}
func (e *reportHTTPEnv) token(t *testing.T, uid int64, kind string) string {
	t.Helper()
	claims := jwt.MapClaims{"ID": uid, "UserType": kind, "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(e.key))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func (e *reportHTTPEnv) get(ctx context.Context, path, token string) (int, []byte, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", e.server.URL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	res, err := e.server.Client().Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		return res.StatusCode, nil, errCheckTest
	}
	return res.StatusCode, b, nil
}
func (e *reportHTTPEnv) path() string {
	return "/api/v1/ai/tasks/" + strconv.FormatInt(e.x.v.ID, 10) + "/report"
}
func (e *reportHTTPEnv) mustGet(t *testing.T, path, token string, status int) []byte {
	t.Helper()
	got, b, err := e.get(e.x.ctx, path, token)
	if err != nil || got != status {
		t.Fatal(got, string(b), err)
	}
	return b
}
func assertPublicReport(t *testing.T, b []byte, x *reportEnv) {
	t.Helper()
	var out struct {
		Code int          `json:"code"`
		Data dto.AIReport `json:"data"`
	}
	if json.Unmarshal(b, &out) != nil || out.Code != 0 || out.Data.TaskID != x.v.ID || len(out.Data.Sections) != 4 || !out.Data.Disclosure.AIGenerated || out.Data.GeneratedAt <= 0 {
		t.Fatal("报告公开契约不符", string(b))
	}
	for i, sec := range out.Data.Sections {
		if !sec.AIGenerated || len(sec.Quotes) != 1 || sec.Quotes[0].Text != x.snap.Sentences[i].Text || sec.Quotes[0].CollectedAt != x.snap.FetchedAt.UnixMilli() {
			t.Fatal("公开事实未逐字回取")
		}
	}
	for _, private := range []string{x.v.No, x.snap.LicenseRef, "snapshot_id", "input_digest", "terminal_txn_id", "fence", "verdict", "test-only-agent-untrusted-marker"} {
		if strings.Contains(string(b), private) {
			t.Fatal("报告泄漏内部数据", private)
		}
	}
}
func TestIntegrationAIReportReadsLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newReportEnv(t, level, time.Minute)
			if _, err := x.svc.Finalize(x.ctx, x.r); err != nil {
				t.Fatal(err)
			}
			e := reportHTTPFixture(t, x, "")
			token := e.token(t, 1, "client")
			// 发布控制器不能持账户/任务锁等待其他服务；独立连接可立即取得原任务锁。
			e.control.hook = func(ctx context.Context) error {
				tx, err := x.db.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				defer tx.Rollback()
				var id int64
				return tx.QueryRowContext(ctx, `SELECT id FROM tm_ai_tasks WHERE id=? FOR UPDATE NOWAIT`, x.v.ID).Scan(&id)
			}
			txns, events, calls := x.queryInt("SELECT COUNT(*) FROM tm_points_txns"), x.queryInt("SELECT COUNT(*) FROM tm_ai_task_events"), x.calls.Load()
			first := e.mustGet(t, e.path(), token, 200)
			assertPublicReport(t, first, x)
			again := e.mustGet(t, e.path(), token, 200)
			if string(again) != string(first) {
				t.Fatal("重复查询改变报告")
			}
			var progress struct {
				Data dto.AITaskProgress `json:"data"`
			}
			if json.Unmarshal(e.mustGet(t, strings.TrimSuffix(e.path(), "/report"), token, 200), &progress) != nil || !progress.Data.ReportAvailable {
				t.Fatal("未证明报告可用")
			}
			var history struct {
				Data dto.AITaskHistory `json:"data"`
			}
			if json.Unmarshal(e.mustGet(t, "/api/v1/ai/tasks", token, 200), &history) != nil || len(history.Data.Items) != 1 || history.Data.Items[0].ReportAvailable {
				t.Fatal("列表联表或推断可用性")
			}
			if x.queryInt("SELECT COUNT(*) FROM tm_points_txns") != txns || x.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != events || x.calls.Load() != calls {
				t.Fatal("GET重新调用终检/模型或扣费")
			}
			requirePointAudit(t, x.pointsTestEnv, x.ledger)
		})
	}
}
func TestIntegrationAIReportReadsBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"off", "foreign", "admin", "disabled_member", "pending", "withdrawn", "expiry", "license_copy", "theme", "publication", "last_publication", "between_withdraw", "unknown_read", "policy", "storage", "malformed"} {
				t.Run(name, func(t *testing.T) {
					x := newReportEnv(t, level, time.Minute)
					if name != "pending" {
						if _, err := x.svc.Finalize(x.ctx, x.r); err != nil {
							t.Fatal(err)
						}
					}
					if name == "policy" {
						cfg := finalreport.Config{Ref: "test-only-changed", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{x.source.Domain}}
						p, err := finalreport.NewPolicy(cfg, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
							return finalreport.Uncertain, nil
						}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
						if err != nil {
							t.Fatal(err)
						}
						x.policy = p
					}
					e := reportHTTPFixture(t, x, name)
					uid, kind, want := int64(1), "client", 422
					switch name {
					case "off":
						want = 503
					case "foreign":
						uid = 2
						want = 404
					case "admin":
						kind = "admin"
						want = 401
					case "disabled_member":
						x.mustExec("UPDATE addon_member SET status=0 WHERE id=1")
						want = 401
					case "withdrawn":
						if err := repository.NewAISourceRepo(x.db).SetSourceEnabled(x.ctx, x.source.ID, false); err != nil {
							t.Fatal(err)
						}
					case "expiry":
						x.mustExec("UPDATE tm_ai_sources SET license_expires_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE id=?", x.source.ID)
					case "license_copy":
						x.mustExec("UPDATE tm_ai_sources SET license_ref='test-only-changed' WHERE id=?", x.source.ID)
					case "theme":
						x.mustExec("UPDATE addon_quant_theme SET name='另一示例题材' WHERE id=1")
					case "publication":
						e.control.allowed.Store(false)
					case "last_publication":
						e.control.denyAt = 2
					case "between_withdraw":
						var once sync.Once
						e.control.hook = func(ctx context.Context) error {
							var err error
							once.Do(func() { err = repository.NewAISourceRepo(x.db).SetSourceEnabled(ctx, x.source.ID, false) })
							return err
						}
					case "unknown_read":
						want = 503
					case "storage":
						x.mustExec("RENAME TABLE tm_ai_report_commits TO test_only_report_unavailable")
						want = 503
					case "malformed":
						want = 400
					}
					path := e.path()
					if name == "malformed" {
						path += "?user_id=1"
					}
					body := e.mustGet(t, path, e.token(t, uid, kind), want)
					if strings.Contains(string(body), x.snap.Sentences[0].Text) || strings.Contains(string(body), "sections") {
						t.Fatal("拒绝/未知响应泄漏正文")
					}
					if name == "foreign" {
						missing := e.mustGet(t, "/api/v1/ai/tasks/99999999/report", e.token(t, uid, kind), 404)
						if string(body) != string(missing) {
							t.Fatal("跨用户泄漏存在性")
						}
					}
				})
			}
		})
	}
}
func TestIntegrationAIReportReadsConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newReportEnv(t, level, time.Minute)
			if _, err := x.svc.Finalize(x.ctx, x.r); err != nil {
				t.Fatal(err)
			}
			e := reportHTTPFixture(t, x, "")
			token := e.token(t, 1, "client")
			before := x.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			events := x.queryInt("SELECT COUNT(*) FROM tm_ai_task_events")
			calls := x.calls.Load()
			var wg sync.WaitGroup
			errs := make(chan error, 12)
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					status, b, err := e.get(x.ctx, e.path(), token)
					if err == nil && (status != 200 || !strings.Contains(string(b), "sections")) {
						err = errCheckTest
					}
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			if x.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before || x.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != events || x.calls.Load() != calls {
				t.Fatal("并发读取写账或重跑终检")
			}
			requirePointAudit(t, x.pointsTestEnv, x.ledger)
		})
	}
}
func TestIntegrationAIReportReadsWorkflow(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newWorkflowEnv(t, level, "multi_source")
			if _, err := x.worker(t, true).Run(x.ctx, x.v.UserID, x.v.ID); err != nil {
				t.Fatal(err)
			}
			e := reportHTTPFixture(t, x.reportEnv, "")
			token := e.token(t, 1, "client")
			before := x.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			assertPublicReport(t, e.mustGet(t, e.path(), token, 200), x.reportEnv)
			if x.server.Count() != 3 || x.submits.Load() != 1 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
				t.Fatal("读取重新调用Agent/模型或扣费")
			}
			if err := repository.NewAISourceRepo(x.db).SetSourceEnabled(x.ctx, x.source.ID, false); err != nil {
				t.Fatal(err)
			}
			e.mustGet(t, e.path(), token, 422)
		})
	}
}
