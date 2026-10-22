//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 全部计划/批准仅为本文件测试：示例来源、虚构价格、回环CN/模型，不是生产控制器。
type submissionSQLControl struct {
	x             *reportEnv
	p             task.AdmissionPlan
	allowed       atomic.Bool
	allowDeep     atomic.Bool
	probeAccount  atomic.Bool
	plans, checks atomic.Int32
}

func (c *submissionSQLControl) Ready(ctx context.Context) error { return ctx.Err() }
func (c *submissionSQLControl) Plan(ctx context.Context, scope task.AdmissionScope) (task.AdmissionPlan, error) {
	c.plans.Add(1)
	if scope.SubjectID != c.p.Scope.SubjectID || scope.SubjectType != task.Theme || (scope.AnalysisType != 1 && !(scope.AnalysisType == task.DeepAnalysisType && c.allowDeep.Load())) || scope.UserID < 1 || scope.UserID > 2 {
		return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
	}
	p := c.p.Clone()
	p.Scope = scope
	return p, ctx.Err()
}
func (c *submissionSQLControl) Recheck(ctx context.Context, p task.AdmissionPlan) error {
	c.checks.Add(1)
	if !c.allowed.Load() {
		return task.ErrAdmissionUnavailable
	}
	want := c.p.Clone()
	want.Scope = p.Scope
	a, err := points.RequestHash("test-only-admission", want)
	if err != nil {
		return err
	}
	b, err := points.RequestHash("test-only-admission", p)
	if err != nil {
		return err
	}
	if a != b {
		return points.ErrInvariant
	}
	// 单请求测试用独立连接证明未持账户锁；并发时别的请求合法持锁，不能据此误判本请求。
	if c.probeAccount.Load() {
		tx, err := c.x.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var uid int64
		err = tx.QueryRowContext(ctx, "SELECT user_id FROM tm_points_accounts WHERE user_id=? FOR UPDATE NOWAIT", p.Scope.UserID).Scan(&uid)
		rollback := tx.Rollback()
		if err != nil {
			return err
		}
		if rollback != nil {
			return rollback
		}
		if uid != p.Scope.UserID {
			return points.ErrInvariant
		}
	}
	err = c.x.repo.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		ids := make([]int64, len(p.Materials))
		for i, m := range p.Materials {
			ids[i] = m.SnapshotID
		}
		snaps, name, err := tx.ReportEvidence(ctx, ids, p.Scope.SubjectID)
		if err != nil {
			return err
		}
		if len(snaps) != len(p.Materials) || name != p.SubjectName {
			return points.ErrInvariant
		}
		for i, s := range snaps {
			if s.ID != p.Materials[i].SnapshotID || s.Hash != p.Materials[i].Digest || s.URL != p.Materials[i].URL || !s.FetchedAt.Equal(p.Materials[i].CollectedAt) {
				return points.ErrInvariant
			}
		}
		return nil
	})
	if err != nil {
		return task.ErrAdmissionUnavailable
	}
	return nil
}

type submissionSQLStore struct {
	*repository.AITaskRepo
	x     *reportEnv
	key   string
	phase atomic.Int32
	once  atomic.Bool
}

func (s *submissionSQLStore) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	err := s.AITaskRepo.WithinTaskTx(ctx, fn)
	if err != nil || s.phase.Load() == 0 {
		return err
	}
	var count int
	if err := s.x.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tm_ai_tasks WHERE user_id=1 AND idem_key=?", s.key).Scan(&count); err != nil {
		return err
	}
	if count == 1 && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return nil
}

type submissionHTTPEnv struct {
	*workflowEnv
	web        *reportHTTPEnv
	ctl        *submissionSQLControl
	store      *submissionSQLStore
	key, owner string
}

func newSubmissionHTTPEnv(t *testing.T, level sql.IsolationLevel, mode string) *submissionHTTPEnv {
	return newSubmissionHTTPEnvWithPlan(t, level, mode, nil)
}

// 仅测试：在注册HTTP读取和首次创建前完成资料/政策装配，不能改写已经准入的任务。
func newSubmissionHTTPEnvWithPlan(t *testing.T, level sql.IsolationLevel, mode string, prepare func(*submissionHTTPEnv)) *submissionHTTPEnv {
	t.Helper()
	workerMode := "pass"
	if mode == "reject" {
		workerMode = "ent_block"
	}
	if mode == "multi_source" {
		workerMode = "multi_source"
	}
	if mode == "agent_failed" || mode == "agent_cancelled" || mode == "agent_running" {
		workerMode = mode
	}
	e := newWorkflowEnv(t, level, workerMode)
	owner := e.v.LeaseOwner
	// 复用完整示例环境，但先原子取消并释放夹具任务，不让它充当HTTP新建成果。
	if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, owner, e.v.Fence, "internal_failure", 0); err != nil {
		t.Fatal(err)
	}
	if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
		t.Fatal("夹具冻结未清理")
	}
	key := "test_member_submission_01"
	store := &submissionSQLStore{AITaskRepo: e.repo, x: e.reportEnv, key: key}
	if mode == "unknown" {
		store.phase.Store(1)
	}
	core, err := service.NewAITaskService(store, e.ledger)
	if err != nil {
		t.Fatal(err)
	}
	ctl := &submissionSQLControl{x: e.reportEnv, p: task.AdmissionPlan{Scope: task.AdmissionScope{UserID: 1, AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1}, Price: points.Price{Version: "v1", Points: 7}, Budget: e.v.Budget, SubjectName: "示例题材", ModelName: "测试模型", FilingNo: "test-only-filing", ProfileDigest: e.model.DirectProfileDigest(), ApprovalRef: "test-only-model", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", PriceRef: "test-only-price", SourceRef: "test-only-source", Materials: []task.AdmissionMaterial{{SnapshotID: e.snap.ID, Digest: e.snap.Hash, Domain: e.source.Domain, Category: e.snap.SourceCategory, URL: e.snap.URL, CollectedAt: e.snap.FetchedAt}}}}
	ctl.p.ExecutionDigest, err = e.planner.p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	ctl.p.ApprovalRef = e.planner.p.Model.ApprovalRef
	ctl.p.Materials = nil
	for _, s := range e.planner.p.Snapshots {
		ctl.p.Materials = append(ctl.p.Materials, task.AdmissionMaterial{SnapshotID: s.ID, Digest: s.Hash, Domain: e.source.Domain, Category: s.SourceCategory, URL: s.URL, CollectedAt: s.FetchedAt})
	}
	slices.SortFunc(ctl.p.Materials, func(a, b task.AdmissionMaterial) int {
		if a.SnapshotID < b.SnapshotID {
			return -1
		}
		if a.SnapshotID > b.SnapshotID {
			return 1
		}
		return 0
	})
	env := &submissionHTTPEnv{workflowEnv: e, ctl: ctl, store: store, key: key, owner: owner}
	if prepare != nil {
		prepare(env)
	}
	web := reportHTTPFixtureWithModel(t, e.reportEnv, "", true, e.planner.p.Model.Model, e.planner.p.Model.FilingNo)
	env.web = web
	ctl.allowed.Store(true)
	ctl.probeAccount.Store(true)
	on := mode != "off" && mode != "read_only"
	svc, err := service.NewAITaskCreateService(core, ctl, on)
	if err != nil {
		t.Fatal(err)
	}
	v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: web.key, Issuer: "GVA", Audience: "GVA"})
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
	handler.NewAITaskCreateHandler(svc).Register(web.server.Config.Handler.(*http.ServeMux), auth, mode != "read_only")
	return env
}

func (e *submissionHTTPEnv) request(ctx context.Context, method, path, key, body, bearer string) (int, []byte, error) {
	r, err := http.NewRequestWithContext(ctx, method, e.web.server.URL+path, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Authorization", "Bearer "+bearer)
	r.Header.Set("Idempotency-Key", key)
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
	}
	res, err := e.web.server.Client().Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	if res.Header.Get("Cache-Control") != "no-store" && res.StatusCode != 404 && res.StatusCode != 405 {
		return 0, nil, errCheckTest
	}
	return res.StatusCode, b, nil
}
func (e *submissionHTTPEnv) mustRequest(t *testing.T, method, path, key, body, bearer string, status int) []byte {
	t.Helper()
	got, b, err := e.request(e.ctx, method, path, key, body, bearer)
	if err != nil || got != status {
		t.Fatal("HTTP不符", got, string(b), err)
	}
	if status != 200 && strings.Contains(string(b), "taskId") {
		t.Fatal("失败返回创建候选收据")
	}
	return b
}

const sqlSubmissionBody = `{"analysis_type":"light","subject_type":"theme","subject_id":1,"price_version":"v1"}`
const sqlSubmissionQuote = `{"analysis_type":"light","subject_type":"theme","subject_id":1}`

func (e *submissionHTTPEnv) newTaskCount(t *testing.T) int {
	t.Helper()
	var count int
	if err := e.db.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM tm_ai_tasks WHERE user_id=1 AND idem_key=?", e.key).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
func decodeCreation(t *testing.T, b []byte) dto.AITaskCreation {
	t.Helper()
	var out struct {
		Code int                `json:"code"`
		Data dto.AITaskCreation `json:"data"`
	}
	if json.Unmarshal(b, &out) != nil || out.Code != 0 || out.Data.TaskID <= 0 || out.Data.PricePoints != 7 || out.Data.PriceVersion != "v1" {
		t.Fatal("创建收据无效", string(b))
	}
	var fields struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(b, &fields) != nil || len(fields.Data) != 3 {
		t.Fatal("收据泄漏或缺失字段")
	}
	return out.Data
}

func (e *submissionHTTPEnv) bindNewTask(t *testing.T, id int64) {
	t.Helper()
	v, err := e.tasks.Inspect(e.ctx, 1, id)
	if err != nil {
		t.Fatal(err)
	}
	e.v = v
	a := e.planner.p.Agent
	a.TaskID = v.ID
	a.TaskNo = v.No
	a.OperationKey = ""
	a.OperationKey = a.RequestID()
	e.planner.p.Agent = a
}
func (e *submissionHTTPEnv) newWorker(t *testing.T) *service.AIL1ReportWorker {
	t.Helper()
	w, err := service.NewAIL1ReportWorker(e.tasks, e.store.AITaskRepo, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func (e *submissionHTTPEnv) run(t *testing.T) {
	t.Helper()
	w := e.newWorker(t)
	if _, err := w.Run(e.ctx, 1, e.v.ID); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationAIReportMemberSubmissionLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"success", "unknown", "reject"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, mode)
					token := e.web.token(t, 1, "client")
					before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
					quote := e.mustRequest(t, "POST", "/api/v1/ai/tasks/quote", e.key, sqlSubmissionQuote, token, 200)
					var q struct {
						Data dto.AITaskQuote `json:"data"`
					}
					if json.Unmarshal(quote, &q) != nil || q.Data.AvailablePoints != 20 || q.Data.QuotedAt <= 0 || len(q.Data.MaterialScope) != 1 {
						t.Fatal(string(quote))
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before || e.newTaskCount(t) != 0 || e.submits.Load() != 0 {
						t.Fatal("报价产生副作用")
					}
					status := 200
					if mode == "unknown" {
						status = 503
					}
					raw := e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, status)
					if status != 200 {
						raw = e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)
					}
					created := decodeCreation(t, raw)
					if e.newTaskCount(t) != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
						t.Fatal("创建未单次冻结")
					}
					e.bindNewTask(t, created.TaskID)
					e.run(t)
					reportStatus := 200
					wantStatus := task.Succeeded
					modelCalls := 3
					if mode == "reject" {
						reportStatus = 422
						wantStatus = task.Failed
						modelCalls = 2
					}
					var actualStatus int
					if err := e.db.QueryRowContext(e.ctx, "SELECT status FROM tm_ai_tasks WHERE id=? AND user_id=1", created.TaskID).Scan(&actualStatus); err != nil {
						t.Fatal(err)
					}
					if actualStatus != wantStatus || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.submits.Load() != 1 || len(e.server.Requests()) != modelCalls {
						t.Fatal("创建后主链未闭合")
					}
					body := e.web.mustGet(t, e.web.path(), token, reportStatus)
					if reportStatus == 200 {
						assertPublicReport(t, body, e.reportEnv)
					}
					txns := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
					events := e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events")
					e.ctl.allowed.Store(false)
					if replay := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200)); replay != created {
						t.Fatal("终态创建回执不一致")
					}
					if receipt := decodeCreation(t, e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)); receipt != created {
						t.Fatal("原键回执不一致")
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != txns || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != events || e.submits.Load() != 1 || len(e.server.Requests()) != modelCalls {
						t.Fatal("重放改变资金、事件或调用")
					}
					if reportStatus == 200 {
						if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
							t.Fatal(err)
						}
						e.web.mustGet(t, e.web.path(), token, 422)
						e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportMemberSubmissionBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"off", "read_only", "admin", "disabled_member", "foreign", "price", "balance", "proof", "source", "unsupported", "bad_body", "event", "receipt_storage", "receipt_unknown", "duplicate_body", "scope"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, mode)
					token := e.web.token(t, 1, "client")
					body := sqlSubmissionBody
					want := 422
					if mode == "foreign" || mode == "receipt_storage" || mode == "receipt_unknown" {
						e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, body, token, 200)
						switch mode {
						case "foreign":
							other := e.web.token(t, 2, "client")
							a := e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", other, 404)
							b := e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", "test_request_missing_01", "", other, 404)
							if string(a) != string(b) {
								t.Fatal("外人可枚举原键")
							}
						case "receipt_storage":
							e.mustExec("RENAME TABLE tm_points_holds TO test_only_receipt_hold_outage")
							e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 503)
						case "receipt_unknown":
							e.store.phase.Store(2)
							e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 503)
							e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)
						}
						if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.submits.Load() != 0 {
							t.Fatal("核查改变了冻结或执行")
						}
						return
					}
					switch mode {
					case "off":
						want = 503
					case "read_only":
						want = 405
					case "admin":
						token = e.web.token(t, 1, "admin")
						want = 401
					case "disabled_member":
						token = e.web.token(t, 3, "client")
						want = 401
					case "price":
						body = strings.Replace(body, `"v1"`, `"old"`, 1)
						want = 409
					case "balance":
						e.ctl.p.Price.Points = 21
					case "proof":
						e.ctl.allowed.Store(false)
					case "source":
						if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
							t.Fatal(err)
						}
					case "unsupported":
						body = strings.Replace(body, `"light"`, `"deep"`, 1)
					case "bad_body":
						body = strings.TrimSuffix(body, "}") + `,"price_points":0}`
						want = 400
					case "duplicate_body":
						body = strings.TrimSuffix(body, "}") + `,"subject_id":1}`
						want = 400
					case "scope":
						body = strings.Replace(body, `"subject_id":1`, `"subject_id":2`, 1)
					case "event":
						e.mustExec("CREATE TRIGGER test_only_create_event BEFORE INSERT ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only event rollback'")
						want = 503
					}
					e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, body, token, want)
					if e.newTaskCount(t) != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.submits.Load() != 0 {
						t.Fatal("前置拒绝仍创建或冻结")
					}
					if mode == "event" {
						e.mustExec("DROP TRIGGER test_only_create_event")
						e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200)
						if e.newTaskCount(t) != 1 {
							t.Fatal("回滚后原键不能正常创建")
						}
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportMemberSubmissionConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "pass")
			e.ctl.probeAccount.Store(false)
			token := e.web.token(t, 1, "client")
			var wg sync.WaitGroup
			failures := make(chan error, 40)
			ids := make(chan int64, 40)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					status, b, err := e.request(e.ctx, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token)
					if err != nil {
						failures <- err
						return
					}
					var out struct {
						Data dto.AITaskCreation `json:"data"`
					}
					if status != 200 || json.Unmarshal(b, &out) != nil || out.Data.TaskID <= 0 {
						failures <- fmt.Errorf("测试并发创建HTTP=%d，响应=%s", status, string(b))
						return
					}
					ids <- out.Data.TaskID
				}()
			}
			wg.Wait()
			close(failures)
			close(ids)
			for err := range failures {
				t.Error(err)
			}
			if t.Failed() {
				return
			}
			var id int64
			for value := range ids {
				if id != 0 && id != value {
					t.Fatal("同键创建了不同任务")
				}
				id = value
			}
			if e.newTaskCount(t) != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.submits.Load() != 0 {
				t.Fatal("并发重复冻结或提前执行")
			}
			e.bindNewTask(t, id)
			e.run(t)
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

func TestIntegrationAIReportMemberQuoteEligibility(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "pass")
			at := e.snap.FetchedAt
			expired := at.Add(-time.Hour)
			for i, grant := range []points.GrantRequest{
				{UserID: 1, Amount: 10, Source: points.Gift, EffectiveAt: at.Add(-2 * time.Hour), ExpiresAt: &expired},
				{UserID: 1, Amount: 10, Source: points.Gift, EffectiveAt: at.Add(time.Hour)},
			} {
				grant.Key = "test_quote_extra_grant_" + string(rune('a'+i))
				grant.BizID = "test-quote-extra-" + string(rune('a'+i))
				if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
					t.Fatal(err)
				}
			}
			if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 40 {
				t.Fatal("未建立过期/未生效余额夹具")
			}
			txns := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			token := e.web.token(t, 1, "client")
			for i := 0; i < 2; i++ {
				b := e.mustRequest(t, "POST", "/api/v1/ai/tasks/quote", e.key, sqlSubmissionQuote, token, 200)
				var out struct {
					Data dto.AITaskQuote `json:"data"`
				}
				if json.Unmarshal(b, &out) != nil || out.Data.AvailablePoints != 20 || out.Data.QuotedAt < at.UnixMilli() || out.Data.MaterialScope[0].SourceType != e.snap.SourceCategory {
					t.Fatal("报价把账面余额当可消费额", string(b))
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != txns || e.newTaskCount(t) != 0 || e.submits.Load() != 0 {
				t.Fatal("重复报价产生副作用")
			}
			e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200)
			b := e.mustRequest(t, "POST", "/api/v1/ai/tasks/quote", e.key, sqlSubmissionQuote, token, 200)
			var out struct {
				Data dto.AITaskQuote `json:"data"`
			}
			if json.Unmarshal(b, &out) != nil || out.Data.AvailablePoints != 13 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 33 {
				t.Fatal("冻结后可消费批次计算错误")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}
