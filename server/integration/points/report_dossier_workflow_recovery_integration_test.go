//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type dossierWorkflowFaultStore struct {
	*repository.AITaskRepo
	f    *dossierWorkflowFixture
	mode string
	once atomic.Bool
}

func (s *dossierWorkflowFaultStore) inject(ctx context.Context, phase, query string, args ...any) error {
	if s.mode != phase || s.once.Load() {
		return nil
	}
	var n int
	if err := s.f.env.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return err
	}
	if n == 1 && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return nil
}
func (s *dossierWorkflowFaultStore) WithinBudgetTx(ctx context.Context, fn func(task.BudgetTx) error) error {
	if err := s.AITaskRepo.WithinBudgetTx(ctx, fn); err != nil {
		return err
	}
	return s.inject(ctx, "budget4", `SELECT COUNT(*) FROM tm_ai_task_reservations WHERE task_id=? AND seq=4`, s.f.id)
}
func (s *dossierWorkflowFaultStore) WithinStageTx(ctx context.Context, fn func(task.StageTx) error) error {
	if err := s.AITaskRepo.WithinStageTx(ctx, fn); err != nil {
		return err
	}
	for _, entry := range []struct{ phase, query string }{{"capture4", `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=4`}, {"checkpoint4", `SELECT COUNT(*) FROM tm_ai_task_steps WHERE task_id=? AND step_no=4`}, {"capture6", `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=6`}} {
		if err := s.inject(ctx, entry.phase, entry.query, s.f.id); err != nil {
			return err
		}
	}
	return nil
}
func (s *dossierWorkflowFaultStore) WithinCallAuditTx(ctx context.Context, fn func(finalreport.CallAuditTx) error) error {
	if err := s.AITaskRepo.WithinCallAuditTx(ctx, fn); err != nil {
		return err
	}
	for _, phase := range []string{"start", "finish"} {
		if err := s.inject(ctx, phase+"4", `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=? AND step_no=4 AND phase=?`, s.f.id, phase); err != nil {
			return err
		}
	}
	return nil
}
func (s *dossierWorkflowFaultStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	if err := s.AITaskRepo.WithinReportTx(ctx, fn); err != nil {
		return err
	}
	return s.inject(ctx, "commit6", `SELECT COUNT(*) FROM tm_ai_report_commits WHERE task_id=?`, s.f.id)
}

type dossierWorkflowAudit struct {
	audit  *service.AIReportCallAuditService
	db     *sql.DB
	revoke bool
}

func (a dossierWorkflowAudit) Start(ctx context.Context, r task.StageRequest, p provider.Request) error {
	return a.audit.Start(ctx, r, p)
}
func (a dossierWorkflowAudit) Finish(ctx context.Context, r task.StageRequest, p provider.Response, e error) error {
	if err := a.audit.Finish(ctx, r, p, e); err != nil {
		return err
	}
	if a.revoke && r.StepNo == 4 {
		_, err := a.db.ExecContext(ctx, `UPDATE tm_ai_dossier_support_uses SET enabled=0`)
		return err
	}
	return nil
}

type dossierWorkflowSupport struct {
	base  *service.AIDossierSupportExecutionControl
	audit dossierWorkflowAudit
}

func (c dossierWorkflowSupport) Recheck(ctx context.Context, v task.Task, p *dossier.SupportPlan) error {
	return c.base.Recheck(ctx, v, p)
}
func (c dossierWorkflowSupport) Start(ctx context.Context, r task.StageRequest, p provider.Request) error {
	return c.audit.Start(ctx, r, p)
}
func (c dossierWorkflowSupport) Finish(ctx context.Context, r task.StageRequest, p provider.Response, e error) error {
	return c.audit.Finish(ctx, r, p, e)
}

func dossierFaultWorker(t *testing.T, f *dossierWorkflowFixture, store *dossierWorkflowFaultStore, revoke bool) *service.AIL1ReportWorker {
	t.Helper()
	e := f.env
	basePlanner, err := service.NewAILocalReportPlanner(f.admission)
	if err != nil {
		t.Fatal(err)
	}
	var planner service.AIL1ReportPlanner = basePlanner
	if store.mode == "use_before_final" {
		planner = dossierFinalUsePlanner{base: basePlanner, fault: store}
	}
	audit, err := service.NewAIReportCallAuditService(store)
	if err != nil {
		t.Fatal(err)
	}
	wrapped := dossierWorkflowAudit{audit: audit, db: e.db, revoke: revoke}
	selection, checks, model, err := service.NewAILocalReportModelControls(f.admission, e.ledger, e.payloads, workflowModelConfig(e.server.URL()), wrapped)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(e.db, sql.LevelReadCommitted), false)
	if err != nil {
		t.Fatal(err)
	}
	control, err := service.NewAIDossierSupportExecutionControl(registry, e.repo, f.admission, audit)
	if err != nil {
		t.Fatal(err)
	}
	support := dossierWorkflowSupport{base: control, audit: wrapped}
	w, err := service.NewAIDossierReportWorker(e.tasks, store, e.ledger, e.agent, e.payloads, model, planner, selection, checks, support, f.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestIntegrationAIReportDossierWorkflowRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"budget4", "start4", "finish4", "capture4", "checkpoint4", "capture6", "commit6", "capture_new_fence", "use_after_call"} {
				t.Run(mode, func(t *testing.T) {
					f := newDossierWorkflowFixture(t, level, 1, "partial")
					e := f.env
					store := &dossierWorkflowFaultStore{AITaskRepo: e.repo, f: f, mode: mode}
					if mode == "capture_new_fence" {
						store.mode = "capture4"
					}
					worker := dossierFaultWorker(t, f, store, mode == "use_after_call")
					if _, err := worker.Run(e.ctx, 1, f.id); err == nil {
						t.Fatal("指定确认丢失或撤回没有发生")
					}
					if mode != "use_after_call" && !store.once.Load() {
						t.Fatal("故障未发生在实际提交之后")
					}
					before := len(e.server.Requests())
					if mode == "use_after_call" {
						e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
						worker = dossierFaultWorker(t, f, store, false)
					}
					if mode == "capture_new_fence" {
						if _, err := e.db.ExecContext(e.ctx, `UPDATE tm_ai_tasks SET lease_until=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE id=?`, f.id); err != nil {
							t.Fatal(err)
						}
					}
					got, err := worker.Run(e.ctx, 1, f.id)
					if err != nil {
						t.Fatal("原键恢复失败", err)
					}
					unknown := mode == "budget4" || mode == "start4" || mode == "finish4" || mode == "use_after_call"
					v, err := e.tasks.Inspect(e.ctx, 1, f.id)
					if err != nil {
						t.Fatal(err)
					}
					if unknown {
						if got.Task.Outcome != "response_unknown" || v.Status != task.Running || len(e.server.Requests()) != before || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 {
							t.Fatal("未知恢复补发或擅自结算")
						}
					} else if v.Status != task.Succeeded || v.CurrentStep != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || len(e.server.Requests()) != 4 {
						t.Fatal("已保存捕获未完成原报告与结算")
					}
					if mode == "capture_new_fence" && v.Fence < 2 {
						t.Fatal("没有实际更高围栏接管")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportDossierWorkflowFifthStepGuard(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			f := newDossierWorkflowFixture(t, level, 1, "partial")
			e := f.env
			store := &dossierWorkflowFaultStore{AITaskRepo: e.repo, f: f, mode: "checkpoint4"}
			if _, err := dossierFaultWorker(t, f, store, false).Run(e.ctx, 1, f.id); err == nil || !store.once.Load() {
				t.Fatal("未停在第四步完成后")
			}
			v, err := e.tasks.Inspect(e.ctx, 1, f.id)
			if err != nil {
				t.Fatal(err)
			}
			key, input := task.StageDigest([]byte("test-only-fifth-audit-key")), task.StageDigest([]byte("test-only-fifth-audit-input"))
			budget, err := service.NewAITaskBudgetService(e.repo)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = budget.ReserveStep(e.ctx, task.ReserveRequest{TaskID: f.id, UserID: 1, Owner: f.owner, Fence: v.Fence, Key: key, InputDigest: input, Units: task.Units{Calls: 1, Tokens: 100}}, 5); err != nil {
				t.Fatal(err)
			}
			var record task.AdmissionRecord
			if err = e.repo.WithinAdmissionTx(e.ctx, func(tx task.AdmissionReadTx) error {
				var found bool
				var err error
				record, found, err = tx.ReadAdmission(e.ctx, v)
				if !found {
					return errCheckTest
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			// 故障注入只移除编排版本并重算原记录/标记；其余任务、预算与第4步状态有效。
			legacy := record.Plan.Clone()
			legacy.ReportProfile = ""
			legacyRaw, legacyDigest, err := legacy.Encode()
			if err != nil {
				t.Fatal(err)
			}
			e.mustExec(`DROP TRIGGER tm_ai_admission_no_update;DROP TRIGGER tm_ai_task_admission_identity`)
			if _, err = e.db.ExecContext(e.ctx, `UPDATE tm_ai_task_admissions SET payload=?,plan_digest=? WHERE task_id=?`, legacyRaw, legacyDigest, f.id); err != nil {
				t.Fatal(err)
			}
			if _, err = e.db.ExecContext(e.ctx, `UPDATE tm_ai_tasks SET admission_digest=? WHERE id=?`, legacyDigest, f.id); err != nil {
				t.Fatal(err)
			}
			zero := 0.0
			r := task.StageRequest{TaskID: f.id, UserID: 1, Owner: f.owner, Fence: v.Fence, StepNo: 5, Stage: "judge", OperationKey: key, InputDigest: input}
			req := provider.Request{TaskID: v.No, Step: 5, Attempt: 1, Model: legacy.ModelName, System: "仅测试", Prompt: "仅测试", MaxTokens: 64, Format: provider.FormatJSONSchema, Strict: true, Schema: []byte(`{"type":"object"}`), Temperature: &zero}
			audit, err := service.NewAIReportCallAuditService(e.repo)
			if err != nil {
				t.Fatal(err)
			}
			if err = audit.Start(e.ctx, r, req); !errors.Is(err, task.ErrAdmissionUnavailable) {
				t.Fatal("旧准入在服务层使用第五步", err)
			}
			requestDigest, err := finalreport.ReportRequestDigest(r, v.No, legacy.ModelName, req)
			if err != nil {
				t.Fatal(err)
			}
			_, err = e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_report_call_audits(task_id,user_id,step_no,stage,owner,fence,operation_key,input_digest,phase,task_no,admission_digest,profile_digest,request_digest,result_digest,outcome,created_at,updated_at) VALUES(?,1,5,'judge',?,?,?,?,'start',?,?,?,?,NULL,'started',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, f.id, f.owner, v.Fence, key, input, v.No, legacyDigest, legacy.ProfileDigest, requestDigest)
			var me *mysql.MySQLError
			if !errors.As(err, &me) || me.Number != 1644 {
				t.Fatal("旧准入在SQL触发器使用第五步", err)
			}
			if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE step_no=5`) != 0 {
				t.Fatal("拒绝仍写审计")
			}
			// 还原同一已获准入版本，直接审计正例仅写start，不请求模型或推进/结算。
			raw, digest, err := record.Plan.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = e.db.ExecContext(e.ctx, `UPDATE tm_ai_task_admissions SET payload=?,plan_digest=? WHERE task_id=?`, raw, digest, f.id); err != nil {
				t.Fatal(err)
			}
			if _, err = e.db.ExecContext(e.ctx, `UPDATE tm_ai_tasks SET admission_digest=? WHERE id=?`, digest, f.id); err != nil {
				t.Fatal(err)
			}
			if err = audit.Start(e.ctx, r, req); err != nil {
				t.Fatal("新版第五步正例误拒", err)
			}
			if len(e.server.Requests()) != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE step_no=5`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
				t.Fatal("审计正反例产生模型或账务副作用")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}
