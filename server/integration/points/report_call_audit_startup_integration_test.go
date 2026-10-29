//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func callAuditProbeCounts(t *testing.T, e *pointsTestEnv, audit bool) map[string]int {
	t.Helper()
	tables := []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_admissions"}
	if audit {
		tables = append(tables, "tm_ai_report_call_audits")
	}
	out := map[string]int{}
	for _, table := range tables {
		out[table] = e.queryInt(`SELECT COUNT(*) FROM ` + table)
	}
	return out
}

func TestIntegrationAIReportCallAuditStartup(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"valid", "missing_table", "missing_identity_check", "missing_step_check", "missing_phase_check", "missing_primary", "missing_fk", "wrong_collation", "missing_insert", "missing_update", "missing_delete", "ineffective_insert", "ineffective_update", "ineffective_delete", "ineffective_identity_check", "ineffective_step_check", "ineffective_phase_check"} {
				t.Run(mode, func(t *testing.T) {
					e, db, _, _, _, _ := persistentTaskFixtureWithLocal(t, level, true)
					reportDDL(t, e, "20261018_create_tm_ai_task_admissions.sql")
					reportDDL(t, e, "20261029_create_tm_ai_report_call_audits.sql")
					switch mode {
					case "missing_table":
						e.mustExec(`DROP TABLE tm_ai_report_call_audits`)
					case "missing_primary":
						e.mustExec(`ALTER TABLE tm_ai_report_call_audits DROP PRIMARY KEY`)
					case "missing_fk":
						e.mustExec(`ALTER TABLE tm_ai_report_call_audits DROP FOREIGN KEY fk_tm_ai_report_call_task`)
					case "wrong_collation":
						e.mustExec(`ALTER TABLE tm_ai_report_call_audits MODIFY owner varchar(64) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL`)
					default:
						if strings.Contains(mode, "_check") {
							name := "chk_tm_ai_report_call_" + strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(mode, "missing_"), "ineffective_"), "_check")
							e.mustExec(`ALTER TABLE tm_ai_report_call_audits DROP CHECK ` + name)
							if strings.HasPrefix(mode, "ineffective_") {
								e.mustExec(`ALTER TABLE tm_ai_report_call_audits ADD CONSTRAINT ` + name + ` CHECK (user_id>0)`)
							}
						} else if mode != "valid" {
							action := strings.TrimPrefix(strings.TrimPrefix(mode, "missing_"), "ineffective_")
							name := "tm_ai_report_call_" + action
							if action != "insert" {
								name = "tm_ai_report_call_no_" + action
							}
							e.mustExec(`DROP TRIGGER ` + name)
							if strings.HasPrefix(mode, "ineffective_") {
								e.mustExec(`CREATE TRIGGER ` + name + ` BEFORE ` + strings.ToUpper(action) + ` ON tm_ai_report_call_audits FOR EACH ROW SET @tm_audit_probe_noop=1`)
							}
						}
					}
					before := callAuditProbeCounts(t, e, mode != "missing_table")
					err := repository.NewAIReportCallAuditSchemaVerifier(db).Verify(e.ctx)
					if (err == nil) != (mode == "valid") {
						t.Fatal("调用审计启动自检边界不符", err)
					}
					after := callAuditProbeCounts(t, e, mode != "missing_table")
					for table, want := range before {
						if got := after[table]; got != want {
							t.Fatal("自检留下业务行", table)
						}
					}
					if mode == "valid" {
						verifyLocalAuditWorkerCommand(t, e, db)
					}
				})
			}
		})
	}
}

func verifyLocalAuditWorkerCommand(t *testing.T, e *pointsTestEnv, owner *sql.DB) {
	t.Helper()
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	user := "audit_" + hex.EncodeToString(seed[:8])
	password := hex.EncodeToString(seed[:])
	if _, err := owner.ExecContext(e.ctx, `CREATE USER '`+user+`'@'127.0.0.1' IDENTIFIED BY '`+password+`'`); err != nil {
		t.Fatal("创建本轮审计账号失败")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := owner.ExecContext(ctx, `DROP USER '`+user+`'@'127.0.0.1'`); err != nil {
			t.Error("清理本轮审计账号失败")
		}
	})
	for _, table := range []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_admissions", "tm_ai_report_call_audits"} {
		if _, err := owner.ExecContext(e.ctx, `GRANT SELECT,INSERT,UPDATE,DELETE ON `+e.database+`.`+table+` TO '`+user+`'@'127.0.0.1'`); err != nil {
			t.Fatal("授予本轮回滚探针权限失败")
		}
	}
	if _, err := owner.ExecContext(e.ctx, `GRANT SELECT ON `+e.database+`.tm_ai_local_instance TO '`+user+`'@'127.0.0.1'`); err != nil {
		t.Fatal(err)
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试DSN无效")
	}
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("审计账号连接失败")
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{`CREATE TABLE forbidden_audit_ddl(id int)`, `SELECT COUNT(*) FROM mysql.user`} {
		_, err := db.ExecContext(e.ctx, q)
		var denied *mysql.MySQLError
		if !errors.As(err, &denied) || (denied.Number != 1044 && denied.Number != 1142 && denied.Number != 1143) {
			t.Fatal("受限账号权限未明确拒绝")
		}
	}
	var visible int
	if err := db.QueryRowContext(e.ctx, `SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND event_object_table='tm_ai_report_call_audits'`).Scan(&visible); err != nil || visible != 0 {
		t.Fatal("审计账号意外持有触发器权限")
	}
	id := strings.TrimPrefix(e.database, "tm_ai_local_")
	file := filepath.Join(t.TempDir(), "runtime.json")
	b, err := json.Marshal(config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: e.database, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0600); err != nil {
		t.Fatal(err)
	}
	command := func(want bool) {
		cmd := exec.CommandContext(e.ctx, "go", "run", "./cmd/ai-worker", "check-local-audit", "-config", file)
		cmd.Dir = filepath.Join("..", "..")
		for _, key := range []string{"SystemRoot", "WINDIR", "PATH", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP", "HOME", "GOCACHE", "GOPATH", "GOMODCACHE", "CGO_ENABLED", "GOFLAGS", "GOMAXPROCS"} {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
		cmd.Env = append(cmd.Env, "TM_DB_DSN="+cfg.FormatDSN())
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatal("实际worker审计预检结果不符")
		}
		if want && !strings.Contains(string(out), "保护核验通过") {
			t.Fatal("没有实际worker自检回执")
		}
		if !want && !strings.Contains(string(out), "阶段4") {
			t.Fatal("没有实际worker保护拒绝回执")
		}
	}
	before := callAuditProbeCounts(t, e, true)
	command(true)
	e.mustExec(`DROP TRIGGER tm_ai_report_call_no_delete`)
	e.mustExec(`CREATE TRIGGER tm_ai_report_call_no_delete BEFORE DELETE ON tm_ai_report_call_audits FOR EACH ROW SET @tm_audit_probe_noop=1`)
	command(false)
	reportDDL(t, e, "20261029_create_tm_ai_report_call_audits.sql")
	command(true)
	after := callAuditProbeCounts(t, e, true)
	for table, want := range before {
		if got := after[table]; got != want {
			t.Fatal("实际worker留下业务行", table)
		}
	}
}
