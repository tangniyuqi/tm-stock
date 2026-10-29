//go:build integration

package points_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func localWorkerEnvironment(dsn, bearer string) []string {
	var env []string
	for _, key := range []string{"SystemRoot", "WINDIR", "PATH", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP", "HOME", "GOCACHE", "GOPATH", "GOMODCACHE", "CGO_ENABLED", "GOFLAGS", "GOMAXPROCS"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return append(env, "TM_DB_DSN="+dsn, "TM_AI_LOCAL_AGENT_BEARER="+bearer)
}

func localWorkerDSN(t *testing.T, e *workflowEnv) string {
	t.Helper()
	return localProcessDSN(t, e, []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_admissions", "tm_ai_report_call_audits", "tm_ai_report_commits", "tm_ai_agent_profiles", "tm_ai_agent_runs", "tm_ai_agent_events"}, false)
}

// 权限名单仅由测试代码提供；每个进程独立账号，不继承原生实例管理权限。
func localProcessDSN(t *testing.T, e *workflowEnv, tables []string, memberAPI bool) string {
	t.Helper()
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	user, password := "local_run_"+hex.EncodeToString(seed[:8]), hex.EncodeToString(seed[:])
	if _, err := e.db.ExecContext(e.ctx, `CREATE USER '`+user+`'@'127.0.0.1' IDENTIFIED BY '`+password+`'`); err != nil {
		t.Fatal("创建本轮执行账号失败")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := e.db.ExecContext(ctx, `DROP USER '`+user+`'@'127.0.0.1'`); err != nil {
			t.Error("清理本轮执行账号失败")
		}
	})
	for _, table := range tables {
		if _, err := e.db.ExecContext(e.ctx, `GRANT SELECT,INSERT,UPDATE,DELETE ON `+e.database+`.`+table+` TO '`+user+`'@'127.0.0.1'`); err != nil {
			t.Fatal("授予本轮执行表权限失败")
		}
	}
	if memberAPI {
		if _, err := e.db.ExecContext(e.ctx, `GRANT SELECT(id,status,deleted_at) ON `+e.database+`.addon_member TO '`+user+`'@'127.0.0.1'`); err != nil {
			t.Fatal("授予会员状态权限失败")
		}
		for _, proc := range []string{"tm_ai_local_conversation_trigger_metadata", "tm_ai_local_intent_trigger_metadata"} {
			if _, err := e.db.ExecContext(e.ctx, `GRANT EXECUTE ON PROCEDURE `+e.database+`.`+proc+` TO '`+user+`'@'127.0.0.1'`); err != nil {
				t.Fatal("授予固定元数据读取权限失败")
			}
		}
	}
	for _, table := range []string{"tm_ai_local_instance", "addon_quant_theme", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies"} {
		if _, err := e.db.ExecContext(e.ctx, `GRANT SELECT ON `+e.database+`.`+table+` TO '`+user+`'@'127.0.0.1'`); err != nil {
			t.Fatal("授予本轮只读资料权限失败")
		}
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试DSN无效")
	}
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("执行账号连接失败")
	}
	defer db.Close()
	denials := []string{`CREATE TABLE local_run_forbidden(id int)`, `UPDATE tm_ai_sources SET enabled=enabled`, `UPDATE tm_ai_local_instance SET contains_real_users=contains_real_users`}
	if memberAPI {
		denials = append(denials, `SELECT password FROM addon_member`, `SELECT * FROM tm_ai_agent_runs`, `INSERT INTO tm_ai_report_call_audits(id) VALUES(1)`)
	}
	for _, q := range denials {
		_, err := db.ExecContext(e.ctx, q)
		var denied *mysql.MySQLError
		if !errors.As(err, &denied) || (denied.Number != 1044 && denied.Number != 1142 && denied.Number != 1143) {
			t.Fatal("本地执行账号越权未明确拒绝")
		}
	}
	return cfg.FormatDSN()
}

type localWorkerChild struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	ready  chan struct{}
	done   chan error
	output bytes.Buffer
	stderr bytes.Buffer
	waited bool
}

func startLocalWorkerChild(t *testing.T, e *workflowEnv, binary, dsn, runtimeFile, plansFile, executionFile string) *localWorkerChild {
	t.Helper()
	c := &localWorkerChild{ready: make(chan struct{}, 1), done: make(chan error, 1)}
	c.cmd = exec.CommandContext(e.ctx, binary, "run-local", "-config", runtimeFile, "-plans", plansFile, "-execution", executionFile, "-stop-on-stdin")
	c.cmd.Env = localWorkerEnvironment(dsn, e.agentConfig.Bearer)
	var err error
	c.input, err = c.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	c.cmd.Stderr = &c.stderr
	if err := c.cmd.Start(); err != nil {
		t.Fatal("启动实际worker失败")
	}
	scanned := make(chan struct{})
	go func() {
		defer close(scanned)
		s := bufio.NewScanner(out)
		for s.Scan() {
			line := s.Text()
			c.output.WriteString(line + "\n")
			if strings.Contains(line, "LOCAL_REPORT_WORKER_READY") {
				select {
				case c.ready <- struct{}{}:
				default:
				}
			}
		}
	}()
	go func() { <-scanned; c.done <- c.cmd.Wait() }()
	t.Cleanup(func() {
		if !c.waited {
			c.input.Close()
			select {
			case <-c.done:
			case <-time.After(10 * time.Second):
				c.cmd.Process.Kill()
				<-c.done
			}
			c.waited = true
		}
	})
	return c
}
func (c *localWorkerChild) wait(t *testing.T, want bool) {
	t.Helper()
	select {
	case err := <-c.done:
		c.waited = true
		if (err == nil) != want {
			t.Fatal("实际worker退出结果不符", c.output.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("实际worker未按期限退出")
	}
}
func (c *localWorkerChild) waitReady(t *testing.T) {
	t.Helper()
	select {
	case <-c.ready:
	case err := <-c.done:
		c.waited = true
		t.Fatal("实际worker未就绪", err, c.output.String())
	case <-time.After(35 * time.Second):
		t.Fatal("实际worker启动超时")
	}
}

func TestIntegrationAIReportLocalWorkerProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	binary := filepath.Join(t.TempDir(), "local-ai-worker")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/ai-worker")
	build.Dir = filepath.Join("..", "..")
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("构建实际worker失败")
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"light", "deep", "unknown", "stop_running", "revoked", "mismatch"} {
				t.Run(mode, func(t *testing.T) {
					kind, workflow := 1, "local"
					if mode == "deep" {
						kind, workflow = 2, "local_deep"
					}
					if mode == "unknown" {
						workflow = "local_unknown"
					}
					if mode == "stop_running" {
						workflow = "local_running"
					}
					f := newLocalPublicationFixture(t, level, kind, workflow)
					e := f.env
					dsn := localWorkerDSN(t, e)
					dir := t.TempDir()
					runtimeFile, executionFile := filepath.Join(dir, "runtime.json"), filepath.Join(dir, "execution.json")
					write := func(path string, v any) {
						b, err := json.Marshal(v)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, b, 0600); err != nil {
							t.Fatal(err)
						}
					}
					write(runtimeFile, f.runtime)
					cc, mc := e.agentConfig, workflowModelConfig(e.server.URL())
					at := time.Now().UnixMilli()
					cfg := config.AILocalExecution{Version: 1, InstanceID: f.runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: at - 1000, ExpiresAtMS: f.file.ExpiresAtMS, Model: config.AILocalExecutionModel{Name: mc.Name, BaseURL: mc.BaseURL, Model: mc.Model, TimeoutMS: mc.TimeoutMS}, Agent: config.AILocalExecutionAgent{BaseURL: cc.BaseURL, ApprovalRef: cc.ApprovalRef, LicenseRef: cc.LicenseRef, QuickModel: cc.QuickModel, DeepModel: cc.DeepModel, TimeoutMS: int(cc.Timeout / time.Millisecond), MaxSubmissions: cc.MaxSubmissions}, LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 30000, AgentPollMS: 50}
					if mode == "mismatch" {
						cfg.Model.Model = "仅测试:其它模型"
					}
					write(executionFile, cfg)
					child := startLocalWorkerChild(t, e, binary, dsn, runtimeFile, f.path, executionFile)
					if mode == "mismatch" {
						child.wait(t, false)
						if len(e.server.Requests()) != 0 || e.submits.Load() != 0 {
							t.Fatal("错绑仍发起请求")
						}
						return
					}
					child.waitReady(t)
					if mode == "revoked" {
						cfg.Revision = "withdrawn"
						write(executionFile, cfg)
						child.wait(t, false)
						if len(e.server.Requests()) != 0 || e.submits.Load() != 0 {
							t.Fatal("空队列撤回仍调用")
						}
						return
					}
					analysis := "light"
					if kind == 2 {
						analysis = "deep"
					}
					created, err := f.creates.Create(e.ctx, 1, "local_process_task_key", dto.AITaskCreateInput{AnalysisType: analysis, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "local-v1"})
					if err != nil {
						t.Fatal(err)
					}
					if mode == "unknown" {
						child.wait(t, false)
					} else {
						deadline := time.NewTimer(30 * time.Second)
						defer deadline.Stop()
						tick := time.NewTicker(50 * time.Millisecond)
						defer tick.Stop()
						for {
							v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
							if err != nil {
								t.Fatal(err)
							}
							if mode == "stop_running" && e.agentPolls.Load() > 0 && e.queryInt(`SELECT COUNT(*) FROM tm_ai_agent_runs WHERE state='running' AND remote_id IS NOT NULL`) == 1 || mode != "stop_running" && v.Status == task.Succeeded {
								break
							}
							select {
							case <-tick.C:
							case <-deadline.C:
								t.Fatal("实际worker未完成预期阶段")
							case err := <-child.done:
								child.waited = true
								t.Fatal("实际worker提前退出", err, child.output.String())
							}
						}
						if mode != "stop_running" {
							// 等待两个原扫描间隔；保持同一进程运行，检查重扫没有重复执行。
							timer := time.NewTimer(2200 * time.Millisecond)
							select {
							case <-timer.C:
							case err := <-child.done:
								timer.Stop()
								child.waited = true
								t.Fatal("成功后循环提前退出", err)
							}
						}
						if _, err := io.WriteString(child.input, "stop\n"); err != nil {
							t.Fatal(err)
						}
						child.wait(t, true)
					}
					v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "unknown" || mode == "stop_running" {
						models := 0
						if mode == "unknown" {
							models = 1
						}
						if v.Status != task.Running || len(e.server.Requests()) != models || e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
							t.Fatal("未知或停止路径被重发/误结算/释放")
						}
						if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 2*models {
							t.Fatal("未知或停止的审计记录不符")
						}
					} else {
						if v.Status != task.Succeeded || v.CurrentStep != 5 || len(e.server.Requests()) != 3 || e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
							t.Fatal("独立worker成功闭环不符")
						}
						base, err := service.NewAIReportService(e.repo, e.ledger, nil, false)
						if err != nil {
							t.Fatal(err)
						}
						control, err := service.NewAILocalReportReadControl(f.admission)
						if err != nil {
							t.Fatal(err)
						}
						reader, err := service.NewAIReportReadServiceWithPolicySource(base, control, control, true)
						if err != nil {
							t.Fatal(err)
						}
						for i := 0; i < 2; i++ {
							report, err := reader.Read(e.ctx, 1, created.TaskID)
							if err != nil || len(report.Sections) != 4 {
								t.Fatal("实际worker报告不可读", err)
							}
							if kind == 2 {
								n := 0
								for _, section := range report.Sections {
									n += len(section.Quotes)
								}
								if n < 6 {
									t.Fatal("实际深度报告缺少六条引文")
								}
							}
						}
					}
					requirePointAudit(t, e.pointsTestEnv, e.ledger)
				})
			}
		})
	}
}
