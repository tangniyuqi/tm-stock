//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"strings"
	"testing"
	"time"
)

// 仅用于证明完整绑定不能被内部单Agent入口绕过，不能作为生产批准。
type testOnlySingleAgentPlanner struct{ p agenttask.Plan }

func (p testOnlySingleAgentPlanner) Plan(ctx context.Context, _ task.Task) (agenttask.Plan, error) {
	return p.p, ctx.Err()
}
func (testOnlySingleAgentPlanner) Recheck(ctx context.Context, _ task.Task, _ agenttask.Plan) error {
	return ctx.Err()
}

func admissionWorker(t *testing.T, e *submissionHTTPEnv) *service.AIL1ReportWorker {
	t.Helper()
	w, err := service.NewAIL1ReportWorker(e.tasks, e.repo, e.ledger, e.agent, e.payloads, e.model, e.planner, workflowSelectionControl{e.control, e.planner}, e.control, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func TestIntegrationAIReportAdmissionBinding(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"same", "multi_source", "direct_agent", "owned_agent", "actor", "symbol", "agent_profile", "agent_units", "model", "approval", "quality", "budget", "policy", "theme", "license", "missing", "payload", "withdrawn"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, mode)
					token := e.web.token(t, 1, "client")
					created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
					e.bindNewTask(t, created.TaskID)
					var payload []byte
					var digest, marker string
					if err := e.db.QueryRowContext(e.ctx, `SELECT a.payload,a.plan_digest,t.admission_digest FROM tm_ai_task_admissions a JOIN tm_ai_tasks t ON t.id=a.task_id WHERE a.task_id=?`, e.v.ID).Scan(&payload, &digest, &marker); err != nil {
						t.Fatal(err)
					}
					p, err := task.DecodeAdmission(payload)
					if err != nil || digest != marker || digest != task.StageDigest(payload) || strings.Contains(string(payload), e.snap.Body) || strings.Contains(string(payload), token) || strings.Contains(string(payload), e.key) {
						t.Fatal("准入记录损坏或含原文/凭据", err)
					}
					want, err := e.planner.p.AdmissionDigest()
					if err != nil || p.ExecutionDigest != want {
						t.Fatal("未固化完整执行身份", err)
					}
					switch mode {
					case "actor":
						e.planner.p.Agent.ActorID++
					case "symbol":
						e.planner.p.Agent.Symbol = "600002"
					case "agent_profile":
						e.planner.p.Agent.ProfileDigest = strings.Repeat("f", 64)
					case "agent_units":
						e.planner.p.Agent.Units.Tokens++
					case "model":
						e.planner.p.Model.ProfileDigest = strings.Repeat("d", 64)
					case "approval":
						e.planner.p.Model.ApprovalRef = "changed-test-approval"
					case "quality":
						e.planner.p.Model.QualityRef = "changed-test-quality"
					case "budget":
						e.planner.p.Model.HardBudgetRef = "changed-test-budget"
					case "policy":
						e.planner.p.Policy.Ref = "changed-test-policy"
					case "theme":
						e.planner.p.ThemeName = "其他示例题材"
					case "license":
						e.planner.p.Snapshots[0].LicenseRef = "changed-test-license"
					case "missing":
						e.mustExec(`DROP TRIGGER tm_ai_admission_no_delete`)
						e.mustExec(`DELETE FROM tm_ai_task_admissions`)
					case "payload":
						e.mustExec(`DROP TRIGGER tm_ai_admission_no_update`)
						e.mustExec(`ALTER TABLE tm_ai_task_admissions DROP CHECK chk_tm_ai_admission`)
						e.mustExec(`UPDATE tm_ai_task_admissions SET payload='{}'`)
					case "withdrawn":
						e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
					}
					if mode == "approval" || mode == "quality" || mode == "budget" {
						e.planner.p.Policy.EntailmentVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckEntailment, e.planner.p.Model)
						if err != nil {
							t.Fatal(err)
						}
						e.planner.p.Policy.SemanticVersion, err = finalreport.CheckPolicyVersion(finalreport.CheckSemantic, e.planner.p.Model)
						if err != nil {
							t.Fatal(err)
						}
					}
					if mode == "direct_agent" || mode == "owned_agent" {
						w, err := service.NewAITaskAgentWorker(e.tasks, e.repo, e.agent, testOnlySingleAgentPlanner{e.planner.p.Agent}, e.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
						if err != nil {
							t.Fatal(err)
						}
						if mode == "direct_agent" {
							_, err = w.Run(e.ctx, 1, e.v.ID)
						} else {
							v, claimErr := e.tasks.Claim(e.ctx, 1, e.v.ID, e.owner, time.Minute)
							if claimErr != nil {
								t.Fatal(claimErr)
							}
							_, err = w.RunOwned(e.ctx, v, e.planner.p.Agent)
						}
						if !errors.Is(err, task.ErrAdmissionUnavailable) || e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
							t.Fatal("单Agent绕过完整绑定", err)
						}
					}
					out, err := admissionWorker(t, e).Run(e.ctx, 1, e.v.ID)
					if mode == "same" || mode == "multi_source" || mode == "direct_agent" || mode == "owned_agent" {
						if err != nil || out.Commit.Status != task.Succeeded || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 {
							t.Fatal("绑定正例主链失败", out, err)
						}
						e.mustRequest(t, "GET", "/api/v1/ai/tasks/"+jsonNumber(e.v.ID)+"/report", "", "", token, 200)
						requirePointAudit(t, e.pointsTestEnv, e.ledger)
						return
					}
					if mode == "withdrawn" {
						if err != nil || out.Task.TaskStatus != task.Failed || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.submits.Load() != 0 || e.server.Count() != 0 {
							t.Fatal("首次来源撤回没有原子失败释放", out, err)
						}
						requirePointAudit(t, e.pointsTestEnv, e.ledger)
						return
					}
					if err == nil || out != (service.AIL1ReportReceipt{}) || e.submits.Load() != 0 || len(e.server.Requests()) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
						t.Fatal("改变计划或损坏记录产生调用/报告/账务", out, err)
					}
					for _, mustIdem := range []string{"actor", "symbol", "agent_profile", "agent_units", "approval", "quality", "budget", "policy", "license"} {
						if mode == mustIdem && !errors.Is(err, points.ErrIdempotency) {
							t.Fatal("不是持久绑定拒绝", err)
						}
					}
					status := 200
					if mode == "missing" || mode == "payload" {
						status = 503
					}
					e.ctl.allowed.Store(false)
					e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, status)
					e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, status)
					if e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
						t.Fatal("原键核查补执行")
					}
					if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
						t.Fatal(err)
					}
					if e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
						t.Fatal("绑定失败现场不能正常取消释放")
					}
				})
			}
		})
	}
}
func jsonNumber(id int64) string { b, _ := json.Marshal(id); return string(b) }

func TestIntegrationAIReportAdmissionAtomicity(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"admission_insert", "event", "missing_table", "unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, mode)
					token := e.web.token(t, 1, "client")
					switch mode {
					case "admission_insert":
						e.mustExec(`CREATE TRIGGER test_only_admission_failure BEFORE INSERT ON tm_ai_task_admissions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only admission failure'`)
					case "event":
						e.mustExec(`CREATE TRIGGER test_only_admission_event_failure BEFORE INSERT ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only admission event failure'`)
					case "missing_table":
						e.mustExec(`RENAME TABLE tm_ai_task_admissions TO test_only_hidden_admissions`)
					}
					e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 503)
					if mode == "missing_table" {
						e.mustExec(`RENAME TABLE test_only_hidden_admissions TO tm_ai_task_admissions`)
					}
					if mode == "unknown" {
						if e.newTaskCount(t) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
							t.Fatal("提交未知未保留同一原子现场")
						}
						e.ctl.allowed.Store(false)
						e.mustRequest(t, "GET", "/api/v1/ai/task-receipt", e.key, "", token, 200)
						e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200)
						if e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
							t.Fatal("未知重放补记录或重复冻结")
						}
						return
					}
					if e.newTaskCount(t) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
						t.Fatal("准入写入故障留下半个创建事务")
					}
					if mode == "admission_insert" {
						e.mustExec(`DROP TRIGGER test_only_admission_failure`)
					}
					if mode == "event" {
						e.mustExec(`DROP TRIGGER test_only_admission_event_failure`)
					}
					e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200)
					if e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
						t.Fatal("原键恢复不是单次原子创建")
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportAdmissionSchema(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"same", "insert", "update", "delete", "marker", "check", "marker_check", "marker_tautology", "check_tautology", "fk", "payload_nullable", "marker_type", "missing_table"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, "")
					verifier := repository.NewAIAdmissionSchemaVerifier(e.db)
					before := e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts`)
					if err := verifier.Verify(e.ctx); err != nil {
						t.Fatal("准入自检基线失败", err)
					}
					if err := repository.NewAIReportSchemaVerifier(e.db).Verify(e.ctx); err != nil {
						t.Fatal("报告联合准入自检基线失败", err)
					}
					if e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts`) != before || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 0 {
						t.Fatal("自检留下探针")
					}
					switch mode {
					case "insert":
						e.mustExec(`DROP TRIGGER tm_ai_admission_insert`)
					case "update":
						e.mustExec(`DROP TRIGGER tm_ai_admission_no_update`)
					case "delete":
						e.mustExec(`DROP TRIGGER tm_ai_admission_no_delete`)
					case "marker":
						e.mustExec(`DROP TRIGGER tm_ai_task_admission_identity`)
					case "check":
						e.mustExec(`ALTER TABLE tm_ai_task_admissions DROP CHECK chk_tm_ai_admission`)
					case "marker_check":
						e.mustExec(`ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task_admission`)
					case "marker_tautology":
						e.mustExec(`ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task_admission,ADD CONSTRAINT chk_tm_ai_task_admission CHECK (1=1)`)
					case "check_tautology":
						e.mustExec(`ALTER TABLE tm_ai_task_admissions DROP CHECK chk_tm_ai_admission,ADD CONSTRAINT chk_tm_ai_admission CHECK(1=1)`)
					case "fk":
						e.mustExec(`ALTER TABLE tm_ai_task_admissions DROP FOREIGN KEY fk_tm_ai_admission_task`)
					case "payload_nullable":
						e.mustExec(`ALTER TABLE tm_ai_task_admissions MODIFY payload mediumblob NULL`)
					case "marker_type":
						e.mustExec(`ALTER TABLE tm_ai_tasks MODIFY admission_digest char(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL`)
					case "missing_table":
						e.mustExec(`RENAME TABLE tm_ai_task_admissions TO test_only_hidden_admissions`)
					}
					err := verifier.Verify(e.ctx)
					if mode == "same" {
						if err != nil {
							t.Fatal(err)
						}
					} else if err == nil {
						t.Fatal("缺保护或空壳约束仍放行", mode)
					}
					reportErr := repository.NewAIReportSchemaVerifier(e.db).Verify(e.ctx)
					if mode == "same" && reportErr != nil || mode != "same" && reportErr == nil {
						t.Fatal("报告入口未复核准入保护", mode, reportErr)
					}
					if e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts`) != before || e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
						t.Fatal("自检写脏或产生模型调用")
					}
				})
			}
		})
	}
}
