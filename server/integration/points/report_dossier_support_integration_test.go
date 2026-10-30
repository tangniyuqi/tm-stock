//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 全部题材、批准及模型回执均为合成夹具。HTTP创建使用测试签发会员令牌，
// 前置步骤明确合成；本组不证明密码登录、真实多Agent、语义质量或生产批准。
type supportSQLStore struct {
	*repository.AITaskRepo
	db       *sql.DB
	id       int64
	step     int
	phase    string
	injected atomic.Bool
}

func (s *supportSQLStore) lostConfirmation(ctx context.Context, phase, query string, args ...any) error {
	if s.phase != phase || s.injected.Load() {
		return nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return err
	}
	if count == 1 && s.injected.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return nil
}
func (s *supportSQLStore) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	if err := s.AITaskRepo.WithinBudgetTx(ctx, fn); err != nil {
		return err
	}
	return s.lostConfirmation(ctx, "budget", `SELECT COUNT(*) FROM tm_ai_task_reservations WHERE task_id=? AND seq=?`, s.id, s.step)
}
func (s *supportSQLStore) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	if err := s.AITaskRepo.WithinStageTx(ctx, fn); err != nil {
		return err
	}
	if err := s.lostConfirmation(ctx, "capture", `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=?`, s.id, s.step); err != nil {
		return err
	}
	return s.lostConfirmation(ctx, "checkpoint", `SELECT COUNT(*) FROM tm_ai_task_steps WHERE task_id=? AND step_no=?`, s.id, s.step)
}
func (s *supportSQLStore) WithinCallAuditTx(ctx context.Context, fn func(finalreport.CallAuditTx) error) error {
	if err := s.AITaskRepo.WithinCallAuditTx(ctx, fn); err != nil {
		return err
	}
	for _, phase := range []string{"start", "finish"} {
		if err := s.lostConfirmation(ctx, phase, `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=? AND step_no=? AND phase=?`, s.id, s.step, phase); err != nil {
			return err
		}
	}
	return nil
}

type supportSQLControl struct {
	e        *submissionHTTPEnv
	dossiers *service.AIDossierService
	audit    *service.AIReportCallAuditService
	cfg      dossier.SupportConfig
	aiUse    atomic.Bool // 测试范围AI用途批准；不是生产许可管理实现。
	after    func(context.Context) error
	start    func(context.Context) error
}

func (c *supportSQLControl) Recheck(ctx context.Context, v task.Task, p *dossier.SupportPlan) error {
	if !c.aiUse.Load() || p.Config() != c.cfg || !p.MatchesTask(v) {
		return sources.ErrUnauthorized
	}
	var record task.AdmissionRecord
	if err := c.e.repo.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
		current, err := tx.LockTask(ctx, v.UserID, v.ID)
		if err != nil {
			return err
		}
		r, found, err := tx.ReadAdmission(ctx, current)
		if err != nil {
			return err
		}
		if !found || r.Bind(current) != nil || !p.MatchesAdmission(r.Plan) {
			return points.ErrInvariant
		}
		record = r
		return nil
	}); err != nil {
		return err
	}
	// 原测试准入控制器重新核完整执行方案、模型/质量/硬预算引用和实际资料，
	// 不能只核DossierSupportDigest便认为整套执行获准。
	if err := c.e.ctl.Recheck(ctx, record.Plan); err != nil {
		return err
	}
	versions, err := c.dossiers.ReadVersions(ctx, "test-member-1", v.UserID, v.SubjectID, []int64{1})
	if err != nil {
		return err
	}
	var snapshots []sources.Snapshot
	err = c.e.repo.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		var e error
		snapshots, _, e = tx.ReportEvidence(ctx, []int64{c.e.snap.ID}, v.SubjectID)
		return e
	})
	if err != nil {
		return err
	}
	prepared, err := dossier.PrepareSupport(ctx, c.cfg, record.Plan.SubjectName, versions[0], snapshots, []string{"test_card"})
	if err != nil {
		return err
	}
	current, err := prepared.BindTask(ctx, v)
	if err != nil || current.InputDigest() != p.InputDigest() || current.ComponentDigest() != record.Plan.DossierSupportDigest {
		return points.ErrIdempotency
	}
	return nil
}
func (c *supportSQLControl) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	if err := c.audit.Start(ctx, r, req); err != nil {
		return err
	}
	if c.start != nil {
		return c.start(ctx)
	}
	return nil
}
func (c *supportSQLControl) Finish(ctx context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	if err := c.audit.Finish(ctx, r, out, callErr); err != nil {
		return err
	}
	if c.after != nil {
		return c.after(ctx)
	}
	return nil
}

type supportSQLEnv struct {
	*submissionHTTPEnv
	store   *supportSQLStore
	control *supportSQLControl
	worker  *service.AIDossierSupportWorker
	server  *providertest.Server
	plan    *dossier.SupportPlan
	request task.StageRequest
	txns    int
}

func supportSQLReply(verdict, reason string) providertest.Step {
	return providertest.Success(fmt.Sprintf(`{"results":[{"id":1,"verdict":%q,"reason":%q}]}`, verdict, reason), provider.Usage{Prompt: 2, Completion: 2, Total: 4})
}

func newSupportSQLEnv(t *testing.T, level sql.IsolationLevel, step int, reply providertest.Step) *supportSQLEnv {
	t.Helper()
	srv := providertest.NewServer(t, reply)
	model, err := provider.NewDirect(provider.Config{Name: "test-only", BaseURL: srv.URL(), Model: "test-only-field-model", TimeoutMS: 2000}, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := dossier.SupportConfig{Model: "test-only-field-model", Version: "test-only-v1", FilingNo: "test-only-filing", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", ProfileDigest: model.DirectProfileDigest(), MaxInputTokens: 32, MaxOutputTokens: 128, MaxPromptBytes: 16000, ReviewStep: step}
	var prepared *dossier.SupportPreparation
	var dossiers *service.AIDossierService
	e := newSubmissionHTTPEnvWithPlan(t, level, "pass", func(e *submissionHTTPEnv) {
		reportDDL(t, e.pointsTestEnv, "20261030_create_tm_ai_dossiers.sql")
		reportDDL(t, e.pointsTestEnv, "20261029_create_tm_ai_report_call_audits.sql")
		var er error
		dossiers, er = service.NewAIDossierService(repository.NewAIDossierRepo(e.db, level), dossierTestAuth{}, true)
		if er != nil {
			t.Fatal(er)
		}
		sentence := e.snap.Sentences[0]
		u := dossier.Field{State: "unknown"}
		draft := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 1, Version: 1, CreatedAt: e.snap.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{{Key: "test_card", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: e.snap.ID, Hash: e.snap.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
		digest, er := dossiers.StageInternal(e.ctx, draft)
		if er != nil {
			t.Fatal(er)
		}
		e.mustExec(`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:publication',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions;
INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:member',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions`)
		// 这里只准备内部计划；原HTTP宿主稍后才建立会员表。
		// 真正执行时控制器必须从真库重新鉴权读取，不能把BindDraft当读取许可。
		bound, er := dossier.BindDraft(draft, []sources.Snapshot{e.snap})
		if er != nil || bound.Digest() != digest {
			t.Fatal(er)
		}
		prepared, er = dossier.PrepareSupport(e.ctx, cfg, e.ctl.p.SubjectName, bound, []sources.Snapshot{e.snap}, []string{"test_card"})
		if er != nil {
			t.Fatal(er)
		}
		e.ctl.p.DossierSupportDigest, er = prepared.ComponentDigest(1)
		if er != nil {
			t.Fatal(er)
		}
		e.ctl.p.ExecutionDigest = task.StageDigest([]byte("test-only-synthetic-predecessors-and-field-support:" + e.ctl.p.DossierSupportDigest))
		e.ctl.p.ProfileDigest, e.ctl.p.ModelName, e.ctl.p.FilingNo = cfg.ProfileDigest, cfg.Model, cfg.FilingNo
		e.ctl.p.Materials = prepared.Materials()
	})
	created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, e.web.token(t, 1, "client"), 200))
	// 创建请求的独立账户锁探针已结束；worker心跳/并发请求可以合法短时持锁。
	e.ctl.probeAccount.Store(false)
	e.bindNewTask(t, created.TaskID)
	e.v, err = e.tasks.Claim(e.ctx, 1, created.TaskID, e.owner, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	p, err := prepared.BindTask(e.ctx, e.v)
	if err != nil {
		t.Fatal(err)
	}
	store := &supportSQLStore{AITaskRepo: e.repo, db: e.db, id: e.v.ID, step: step}
	budget, err := service.NewAITaskBudgetService(e.repo)
	if err != nil {
		t.Fatal(err)
	}
	policy := task.StagePolicy{Digest: task.StageDigest([]byte("test-only-synthetic-predecessor-policy")), Check: func(ctx context.Context, _ task.StageValidationInput) error { return ctx.Err() }}
	stages, err := service.NewAITaskStageService(e.repo, map[string]task.StagePolicy{"extract": policy})
	if err != nil {
		t.Fatal(err)
	}
	for n := 1; n < step; n++ {
		key := task.StageDigest([]byte(fmt.Sprint("test-only-predecessor-", n)))
		r := task.StageRequest{TaskID: e.v.ID, UserID: 1, Owner: e.owner, Fence: e.v.Fence, StepNo: n, Stage: "extract", OperationKey: key, InputDigest: task.StageDigest([]byte(key))}
		if _, err = budget.ReserveStep(e.ctx, task.ReserveRequest{TaskID: r.TaskID, UserID: r.UserID, Owner: r.Owner, Fence: r.Fence, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 1}}, n); err != nil {
			t.Fatal(err)
		}
		if _, err = stages.Capture(e.ctx, r, []byte(`{"testOnly":true}`)); err != nil {
			t.Fatal(err)
		}
		if _, err = stages.Complete(e.ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	audit, err := service.NewAIReportCallAuditService(store)
	if err != nil {
		t.Fatal(err)
	}
	ctl := &supportSQLControl{e: e, dossiers: dossiers, audit: audit, cfg: cfg}
	ctl.aiUse.Store(true)
	w, err := service.NewAIDossierSupportWorker(e.tasks, store, model, ctl, e.owner, service.AIReportCheckOptions{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 10 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	r := task.StageRequest{TaskID: e.v.ID, UserID: 1, Owner: e.owner, Fence: e.v.Fence, StepNo: step, Stage: "judge", OperationKey: p.OperationKey(e.v, step), InputDigest: p.InputDigest()}
	return &supportSQLEnv{submissionHTTPEnv: e, store: store, control: ctl, worker: w, server: srv, plan: p, request: r, txns: e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)}
}

func (e *supportSQLEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func (e *supportSQLEnv) assertUnsettled(t *testing.T) {
	t.Helper()
	v, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
	if err != nil || v.Status != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != e.txns || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.submits.Load() != 0 || len(e.workflowEnv.server.Requests()) != 0 {
		t.Fatal("单步核验改变终态/账务或混入旧Agent调用", err)
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
}

func TestIntegrationAIReportDossierSupportLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, step := range []int{3, 4} {
				t.Run(fmt.Sprint(step), func(t *testing.T) {
					for _, state := range []struct{ verdict, reason string }{{"supported", "direct_support"}, {"partial", "partial_support"}, {"unsupported", "not_supported"}, {"uncertain", "insufficient_context"}} {
						t.Run(state.verdict, func(t *testing.T) {
							e := newSupportSQLEnv(t, level, step, supportSQLReply(state.verdict, state.reason))
							for repeat := 0; repeat < 2; repeat++ {
								got, err := e.worker.RunOwned(e.ctx, e.request, e.plan)
								if err != nil || got.Review == nil || got.Task.Outcome != "dossier_support_recorded" || got.Task.ReportAvailable || got.Review.Results()[0].Verdict != state.verdict || got.Checkpoint.StepNo != step {
									t.Fatal("真库核验或原键回放失败", err)
								}
							}
							if e.server.Count() != 1 || e.count(t, `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=?`, e.v.ID) != 2 || e.count(t, `SELECT COUNT(*) FROM tm_ai_task_reservations WHERE task_id=?`, e.v.ID) != step || e.count(t, `SELECT COUNT(*) FROM tm_ai_task_steps WHERE task_id=?`, e.v.ID) != step {
								t.Fatal("重复调用或缺持久审计/预留/检查点")
							}
							e.assertUnsettled(t)
						})
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportDossierSupportRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"budget", "capture", "checkpoint", "start", "finish", "transport", "estimated", "malformed", "capture_new_fence", "checkpoint_new_fence"} {
				t.Run(mode, func(t *testing.T) {
					reply := supportSQLReply("partial", "partial_support")
					switch mode {
					case "transport":
						reply = providertest.Abort()
					case "estimated":
						reply = providertest.SuccessNoUsage(`{"results":[{"id":1,"verdict":"partial","reason":"partial_support"}]}`)
					case "malformed":
						reply = providertest.Success(`{"results":[]}`, provider.Usage{Prompt: 2, Completion: 2, Total: 4})
					}
					e := newSupportSQLEnv(t, level, 3, reply)
					e.store.phase = mode
					if mode == "capture_new_fence" {
						e.store.phase = "capture"
					}
					if mode == "checkpoint_new_fence" {
						e.store.phase = "checkpoint"
					}
					got, err := e.worker.RunOwned(e.ctx, e.request, e.plan)
					if err == nil || got.Review != nil {
						t.Fatal("丢确认或坏回执被交付")
					}
					calls := e.server.Count()
					wantCalls, wantAudits, wantCaptured := 1, 2, 1
					switch mode {
					case "budget":
						wantCalls, wantAudits, wantCaptured = 0, 0, 0
					case "start":
						wantCalls, wantAudits, wantCaptured = 0, 1, 0
					case "finish", "transport":
						wantCaptured = 0
					}
					faultInjected := mode != "transport" && mode != "estimated" && mode != "malformed"
					if calls != wantCalls || faultInjected && !e.store.injected.Load() || e.count(t, `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=?`, e.v.ID) != wantAudits || e.count(t, `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=3`, e.v.ID) != wantCaptured || e.count(t, `SELECT COUNT(*) FROM tm_ai_task_reservations WHERE task_id=? AND seq=3`, e.v.ID) != 1 {
						t.Fatal("故障没有发生在指定持久提交边界")
					}
					if mode == "capture_new_fence" || mode == "checkpoint_new_fence" {
						if _, err := e.db.ExecContext(e.ctx, `UPDATE tm_ai_tasks SET lease_until=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE id=?`, e.v.ID); err != nil {
							t.Fatal(err)
						}
						// 原owner重新取得更高围栏；原键与捕获不因围栏变化被重建。
						e.v, err = e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, time.Minute)
						if err != nil || e.v.Fence <= e.request.Fence {
							t.Fatal("未实际接管", err)
						}
						e.request.Fence = e.v.Fence
					}
					again, err := e.worker.RunOwned(e.ctx, e.request, e.plan)
					if e.server.Count() != calls {
						t.Fatal("未知结果重发模型")
					}
					switch mode {
					case "capture", "checkpoint", "capture_new_fence", "checkpoint_new_fence":
						if err != nil || again.Review == nil || again.Task.Outcome != "dossier_support_recorded" {
							t.Fatal("真库捕获未恢复", err)
						}
					case "estimated", "malformed":
						if !errors.Is(err, task.ErrStageRejected) || again.Review != nil {
							t.Fatal("非法回执被当作语义不确定", err)
						}
					default:
						if err != nil || again.Review != nil || again.Task.Outcome != "response_unknown" {
							t.Fatal("未知调用未保留原现场", err)
						}
					}
					e.assertUnsettled(t)
				})
			}
		})
	}
}

func TestIntegrationAIReportDossierSupportRevocation(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"grant_before", "source_before", "ai_use_before", "profile_before", "source_after", "grant_after", "publication_after", "cached_grant"} {
				t.Run(mode, func(t *testing.T) {
					e := newSupportSQLEnv(t, level, 3, supportSQLReply("supported", "direct_support"))
					switch mode {
					case "grant_before":
						e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=0`)
					case "source_before":
						e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
					case "ai_use_before":
						e.control.aiUse.Store(false)
					case "profile_before":
						e.ctl.p.ExecutionDigest = task.StageDigest([]byte("test-only-changed-full-profile"))
					case "source_after", "grant_after", "publication_after":
						e.control.after = func(ctx context.Context) error {
							query := `UPDATE tm_ai_sources SET enabled=0`
							if mode == "grant_after" {
								query = `UPDATE tm_ai_dossier_grants SET enabled=0`
							}
							if mode == "publication_after" {
								query = `UPDATE tm_ai_dossier_publications SET enabled=0`
							}
							_, err := e.db.ExecContext(ctx, query)
							return err
						}
					case "cached_grant":
						if _, err := e.worker.RunOwned(e.ctx, e.request, e.plan); err != nil {
							t.Fatal(err)
						}
						e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=0`)
					}
					got, err := e.worker.RunOwned(e.ctx, e.request, e.plan)
					if err == nil || got.Review != nil {
						t.Fatal("撤回后仍交付核验")
					}
					wantCalls := 0
					if mode == "source_after" || mode == "grant_after" || mode == "publication_after" || mode == "cached_grant" {
						wantCalls = 1
						if e.count(t, `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=?`, e.v.ID) != 2 {
							t.Fatal("调用后撤回丢失审计")
						}
					}
					if e.server.Count() != wantCalls {
						t.Fatal("撤回边界调用次数不符")
					}
					wantCaptured := 0
					if mode == "cached_grant" {
						wantCaptured = 1
					}
					if e.count(t, `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=3`, e.v.ID) != wantCaptured || e.count(t, `SELECT COUNT(*) FROM tm_ai_task_steps WHERE task_id=? AND step_no=3`, e.v.ID) != wantCaptured {
						t.Fatal("撤回后仍捕获或推进步骤")
					}
					if mode == "cached_grant" {
						e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=1`)
						if got, err := e.worker.RunOwned(e.ctx, e.request, e.plan); err != nil || got.Review == nil || e.server.Count() != 1 {
							t.Fatal("恢复许可未读取原检查点", err)
						}
					}
					e.assertUnsettled(t)
				})
			}
		})
	}
}

func TestIntegrationAIReportDossierSupportConcurrent(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSupportSQLEnv(t, level, 3, supportSQLReply("partial", "partial_support"))
			entered, release := make(chan struct{}), make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			e.control.start = func(ctx context.Context) error {
				close(entered)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			type result struct {
				receipt service.AIDossierSupportReceipt
				err     error
			}
			done := make(chan result, 1)
			go func() { got, err := e.worker.RunOwned(e.ctx, e.request, e.plan); done <- result{got, err} }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("首请求未进入原持久审计")
			}
			second, err := e.worker.RunOwned(e.ctx, e.request, e.plan)
			if err != nil || second.Review != nil || second.Task.Outcome != "response_unknown" || e.server.Count() != 0 {
				t.Fatal("并发请求重发预留调用", err)
			}
			close(release)
			select {
			case first := <-done:
				if first.err != nil || first.receipt.Review == nil {
					t.Fatal("首调用未提交", first.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("首调用未结束")
			}
			if got, err := e.worker.RunOwned(e.ctx, e.request, e.plan); err != nil || got.Review == nil || e.server.Count() != 1 {
				t.Fatal("并发结束后不能原键恢复", err)
			}
			e.assertUnsettled(t)
		})
	}
}
