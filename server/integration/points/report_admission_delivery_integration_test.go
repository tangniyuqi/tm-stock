//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅在每例独立可丢弃库中制造特权破坏，不能作为产品修复或放行方法。
func breakTestAdmission(ctx context.Context, e *submissionHTTPEnv, mode string) error {
	switch mode {
	case "payload":
		if _, err := e.db.ExecContext(ctx, `DROP TRIGGER tm_ai_admission_no_update`); err != nil {
			return err
		}
		if _, err := e.db.ExecContext(ctx, `ALTER TABLE tm_ai_task_admissions DROP CHECK chk_tm_ai_admission`); err != nil {
			return err
		}
		_, err := e.db.ExecContext(ctx, `UPDATE tm_ai_task_admissions SET payload='{}' WHERE task_id=?`, e.v.ID)
		return err
	case "storage":
		_, err := e.db.ExecContext(ctx, `RENAME TABLE tm_ai_task_admissions TO test_only_delivery_admission_hidden`)
		return err
	default:
		if _, err := e.db.ExecContext(ctx, `DROP TRIGGER tm_ai_admission_no_delete`); err != nil {
			return err
		}
		_, err := e.db.ExecContext(ctx, `DELETE FROM tm_ai_task_admissions WHERE task_id=?`, e.v.ID)
		return err
	}
}

func TestIntegrationAIReportAdmissionReads(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"same", "missing", "payload", "storage", "between", "unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, "")
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID)
					if out, err := admissionWorker(t, e).Run(e.ctx, 1, e.v.ID); err != nil || out.Commit.Status != task.Succeeded {
						t.Fatal("测试报告未真实提交", out, err)
					}
					path := "/api/v1/ai/tasks/" + jsonNumber(e.v.ID) + "/report"
					assertPublicReport(t, e.mustRequest(t, "GET", path, "", "", token, 200), e.reportEnv)
					txns := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
					events := e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_events`)
					reservations := e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_reservations`)
					want := 503
					switch mode {
					case "same":
						want = 200
					case "between":
						var once sync.Once
						var faultErr error
						e.web.control.hook = func(ctx context.Context) error {
							once.Do(func() { faultErr = breakTestAdmission(ctx, e, "missing") })
							return faultErr
						}
					case "unknown":
						base, err := service.NewAIReportService(&readSQLUnknownStore{AITaskRepo: e.repo}, e.ledger, e.policy, false)
						if err != nil {
							t.Fatal(err)
						}
						reader, err := service.NewAIReportReadService(base, e.web.control, true)
						if err != nil {
							t.Fatal(err)
						}
						out, err := reader.Read(e.ctx, 1, e.v.ID)
						if err == nil || out.TaskID != 0 || len(out.Sections) != 0 {
							t.Fatal("读取提交未知仍出正文", out, err)
						}
						want = 200 // 依赖恢复后仍读取同一原报告，不补执行。
					default:
						if err := breakTestAdmission(e.ctx, e, mode); err != nil {
							t.Fatal("测试故障未写入", err)
						}
					}
					body := e.mustRequest(t, "GET", path, "", "", token, want)
					if want != 200 && (strings.Contains(string(body), "sections") || strings.Contains(string(body), e.snap.Sentences[0].Text)) {
						t.Fatal("准入故障响应泄漏正文")
					}
					if want != 200 {
						e.mustRequest(t, "GET", strings.TrimSuffix(path, "/report"), "", "", token, 503)
					}
					if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != txns || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_events`) != events || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_reservations`) != reservations || e.submits.Load() != 1 || len(e.server.Requests()) != 3 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 {
						t.Fatal("历史报告查询补调用/写账或改变成功现场")
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}

type admissionFinalFaultStore struct {
	*repository.AITaskRepo
	e       *submissionHTTPEnv
	mode    string
	visits  atomic.Int32
	enabled atomic.Bool
	hit     atomic.Bool
	wrote   atomic.Bool
}

type admissionFinalFaultTx struct {
	finalreport.Tx
	store  *admissionFinalFaultStore
	broken bool
}

func (tx *admissionFinalFaultTx) InsertReportCommit(ctx context.Context, c finalreport.Commit) error {
	if err := tx.Tx.InsertReportCommit(ctx, c); err != nil {
		return err
	}
	tx.store.wrote.Store(true)
	tx.broken = true
	return nil
}
func (tx *admissionFinalFaultTx) ReadAdmission(ctx context.Context, v task.Task) (task.AdmissionRecord, bool, error) {
	if tx.broken {
		return task.AdmissionRecord{}, false, errCheckTest
	}
	return tx.Tx.ReadAdmission(ctx, v)
}
func (s *admissionFinalFaultStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	var step int
	if err := s.e.db.QueryRowContext(ctx, `SELECT current_step FROM tm_ai_tasks WHERE id=?`, s.e.v.ID).Scan(&step); err != nil {
		return err
	}
	if step == 5 && s.enabled.Load() {
		at := int32(3) // 初读、来源读之后才进入最终写事务。
		if s.mode == "before" {
			at = 1
		}
		if s.visits.Add(1) == at {
			s.hit.Store(true)
			if s.mode == "confirmation" {
				return s.AITaskRepo.WithinReportTx(ctx, func(tx finalreport.Tx) error {
					return fn(&admissionFinalFaultTx{Tx: tx, store: s})
				})
			}
			if err := breakTestAdmission(ctx, s.e, s.mode); err != nil {
				return err
			}
		}
	}
	return s.AITaskRepo.WithinReportTx(ctx, fn)
}

func TestIntegrationAIReportAdmissionFinalize(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"before", "between", "payload", "storage", "confirmation"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, "")
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID)
					store := &admissionFinalFaultStore{AITaskRepo: e.repo, e: e, mode: mode}
					store.enabled.Store(true)
					worker, err := service.NewAIL1ReportWorker(e.tasks, store, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
					if err != nil {
						t.Fatal(err)
					}
					out, err := worker.Run(e.ctx, 1, e.v.ID)
					if err == nil || out != (service.AIL1ReportReceipt{}) || !store.hit.Load() || mode == "confirmation" && !store.wrote.Load() {
						t.Fatal("终检准入故障未命中或仍成功", out, err)
					}
					if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_events WHERE task_id=? AND type='succeeded'`, e.v.ID) != 0 || e.submits.Load() != 1 || len(e.server.Requests()) != 3 {
						t.Fatal("报告/成功/结算未整体回滚或额外调用")
					}
					store.enabled.Store(false)
					if mode == "storage" {
						e.mustExec(`RENAME TABLE test_only_delivery_admission_hidden TO tm_ai_task_admissions`)
					}
					if mode == "storage" || mode == "confirmation" {
						got, err := worker.Run(e.ctx, 1, e.v.ID)
						if err != nil || got.Commit.Status != task.Succeeded || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.submits.Load() != 1 || len(e.server.Requests()) != 3 {
							t.Fatal("原任务恢复补调用或重复结算", got, err)
						}
						e.mustRequest(t, "GET", "/api/v1/ai/tasks/"+jsonNumber(e.v.ID)+"/report", "", "", token, 200)
					} else {
						current, err := e.tasks.Inspect(e.ctx, 1, e.v.ID)
						if err != nil || current.Status != task.Running || current.CurrentStep != 5 {
							t.Fatal("未保留原五步现场", err)
						}
						if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
							t.Fatal(err)
						}
						if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, e.owner, current.Fence, "internal_failure", 0); err != nil {
							t.Fatal(err)
						}
						if e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
							t.Fatal("准入损坏后不能取消释放")
						}
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
