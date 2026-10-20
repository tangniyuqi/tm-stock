//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

var taskProbeTables = []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps"}

func taskProbeCounts(e *pointsTestEnv) []int {
	out := make([]int, len(taskProbeTables))
	for i, table := range taskProbeTables {
		out[i] = e.queryInt("SELECT COUNT(*) FROM " + table)
	}
	return out
}

func TestIntegrationAITaskSchemaProbe(t *testing.T) {
	e, db, _, s, ledger, now := persistentTaskFixture(t, sql.LevelReadCommitted)
	if _, err := ledger.Grant(e.ctx, pointGrant(now, 20)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(e.ctx, taskRequest(1), points.Price{Version: "v1", Points: 7}, taskBudget()); err != nil {
		t.Fatal(err)
	}
	before := taskProbeCounts(e)
	for i := 0; i < 2; i++ {
		if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
			t.Fatal("任务自检健康基线失败", err)
		}
		if !reflect.DeepEqual(before, taskProbeCounts(e)) || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 13 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
			t.Fatal("任务探针遗留数据或改变既有余额")
		}
	}
	requirePointAudit(t, e, ledger)
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	if err := repository.NewAITaskSchemaVerifier(db).Verify(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("取消上下文未失败关闭", err)
	}
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("隔离连接解析失败")
	}
	cfg.DBName = e.database
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	for _, zone := range []string{"'+08:00'", "'-12:00'", "'+00:00'"} {
		cfg.Params["time_zone"] = zone
		zoned, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal("时区测试连接失败")
		}
		verifyErr := repository.NewAITaskSchemaVerifier(zoned).Verify(e.ctx)
		closeErr := zoned.Close()
		if verifyErr != nil || closeErr != nil {
			t.Fatal("会话时区改变探针语义", verifyErr, closeErr)
		}
	}
	if !reflect.DeepEqual(before, taskProbeCounts(e)) {
		t.Fatal("时区探针遗留数据")
	}
	delete(cfg.Params, "time_zone")
	for _, setting := range []struct{ name, value string }{{"foreign_key_checks", "0"}, {"sql_mode", "''"}} {
		cfg.Params[setting.name] = setting.value
		unsafe, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal("不安全会话连接装配失败")
		}
		verifyErr := repository.NewAITaskSchemaVerifier(unsafe).Verify(e.ctx)
		closeErr := unsafe.Close()
		delete(cfg.Params, setting.name)
		if verifyErr == nil || closeErr != nil {
			t.Fatal("关闭外键或严格模式仍通过")
		}
	}
}

func TestIntegrationAITaskSchemaCorruption(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"任务检查恒真", "ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task, ADD CONSTRAINT chk_tm_ai_task CHECK (TRUE)"},
		{"预算检查恒真", "ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task_budget, ADD CONSTRAINT chk_tm_ai_task_budget CHECK (TRUE)"},
		{"事件检查恒真", "ALTER TABLE tm_ai_task_events DROP CHECK chk_tm_ai_task_event, ADD CONSTRAINT chk_tm_ai_task_event CHECK (TRUE)"},
		{"缺检查", "ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task_budget"},
		{"身份空保护", "DROP TRIGGER tm_ai_tasks_identity; CREATE TRIGGER tm_ai_tasks_identity BEFORE UPDATE ON tm_ai_tasks FOR EACH ROW SET @task_probe_noop=1"},
		{"任务删除空保护", "DROP TRIGGER tm_ai_tasks_no_delete; CREATE TRIGGER tm_ai_tasks_no_delete BEFORE DELETE ON tm_ai_tasks FOR EACH ROW SET @task_probe_noop=1"},
		{"事件修改空保护", "DROP TRIGGER tm_ai_task_events_no_update; CREATE TRIGGER tm_ai_task_events_no_update BEFORE UPDATE ON tm_ai_task_events FOR EACH ROW SET @task_probe_noop=1"},
		{"事件删除空保护", "DROP TRIGGER tm_ai_task_events_no_delete; CREATE TRIGGER tm_ai_task_events_no_delete BEFORE DELETE ON tm_ai_task_events FOR EACH ROW SET @task_probe_noop=1"},
		{"非整数金额", "ALTER TABLE tm_ai_tasks MODIFY price_points DOUBLE NOT NULL"},
		{"非二进制幂等键", "ALTER TABLE tm_ai_tasks MODIFY idem_key VARCHAR(80) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL"},
		{"旧主体列", "DROP TRIGGER tm_ai_tasks_identity; ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task, RENAME COLUMN subject_id TO theme_id; ALTER TABLE tm_ai_tasks ADD CONSTRAINT chk_tm_ai_task CHECK(TRUE)"},
		{"错误尝试默认值", "ALTER TABLE tm_ai_tasks ALTER COLUMN attempt SET DEFAULT 1"},
		{"错误取消默认值", "ALTER TABLE tm_ai_tasks ALTER COLUMN cancel_req SET DEFAULT 1"},
		{"缺幂等唯一", "ALTER TABLE tm_ai_tasks DROP INDEX uk_tm_ai_task_idem"},
		{"缺冻结唯一", "ALTER TABLE tm_ai_tasks DROP INDEX uk_tm_ai_task_hold"},
		{"不可见候选索引", "ALTER TABLE tm_ai_tasks ALTER INDEX idx_tm_ai_task_claim INVISIBLE"},
		{"缺期限索引", "ALTER TABLE tm_ai_tasks DROP INDEX idx_tm_ai_task_deadline"},
		{"历史索引错序", "ALTER TABLE tm_ai_tasks DROP INDEX idx_tm_ai_task_history, ADD INDEX idx_tm_ai_task_history(id,user_id)"},
		{"索引额外前缀列", "ALTER TABLE tm_ai_tasks DROP INDEX idx_tm_ai_task_history, ADD INDEX idx_tm_ai_task_history(user_id,id,idem_key(2))"},
		{"缺冻结外键", "ALTER TABLE tm_ai_tasks DROP FOREIGN KEY fk_tm_ai_task_hold"},
		{"缺事件外键", "ALTER TABLE tm_ai_task_events DROP FOREIGN KEY fk_tm_ai_event_task"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, db, _, _, _, _ := persistentTaskFixture(t, sql.LevelReadCommitted)
			verifier := repository.NewAITaskSchemaVerifier(db)
			if err := verifier.Verify(e.ctx); err != nil {
				t.Fatal("破坏前健康基线失败", err)
			}
			e.mustExec(tc.change)
			if err := verifier.Verify(e.ctx); err == nil {
				t.Fatal("结构或保护被削弱仍通过")
			} else if tc.name == "旧主体列" && !strings.Contains(err.Error(), "subject_id") {
				t.Fatal("旧主体结构未由缺列判据拒绝", err)
			} else if strings.Contains(tc.name, "默认值") && !strings.Contains(err.Error(), "默认状态不兼容") {
				t.Fatal("错误默认值未由真实创建状态判据拒绝", err)
			}
			for _, count := range taskProbeCounts(e) {
				if count != 0 {
					t.Fatal("失败探针未整体回滚")
				}
			}
		})
	}
}

func TestIntegrationAITaskSchemaRestrictedCLI(t *testing.T) {
	e, _, _, _, _, _ := persistentTaskFixture(t, sql.LevelReadCommitted)
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试 DSN 解析失败")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	user := "task_probe_" + hex.EncodeToString(random[:6])
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, hex.EncodeToString(random[:])
	// CREATE USER 不支持密码参数占位；只使用本地随机十六进制，不输出凭据。
	if _, err := e.conn.ExecContext(e.ctx, "CREATE USER '"+user+"'@'%' IDENTIFIED BY '"+cfg.Passwd+"'"); err != nil {
		t.Fatal("创建受限探针账号失败")
	}
	t.Cleanup(func() { e.mustExec("DROP USER '" + user + "'@'%'") })
	for _, table := range taskProbeTables {
		e.mustExec("GRANT SELECT, INSERT, UPDATE, DELETE ON `" + e.database + "`." + table + " TO '" + user + "'@'%'")
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("受限连接装配失败")
	}
	defer db.Close()
	var triggers int
	if err := db.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE()").Scan(&triggers); err != nil || triggers != 0 {
		t.Fatal("业务账号不应看见或管理触发器", triggers)
	}
	if _, err := db.ExecContext(e.ctx, "CREATE TABLE task_probe_forbidden_ddl(id INT)"); err == nil {
		t.Fatal("业务账号拥有 DDL 权限")
	} else {
		var permission *mysqldrv.MySQLError
		if !errors.As(err, &permission) || permission.Number != 1142 {
			t.Fatal("DDL 负例不是权限拒绝")
		}
	}
	if err := repository.NewAITaskSchemaVerifier(db).Verify(e.ctx); err != nil {
		t.Fatal("无 DDL 权限自检失败", err)
	}
	executable := filepath.Join(t.TempDir(), "ai-task-check.exe")
	build := exec.CommandContext(e.ctx, "go", "build", "-p", "1", "-o", executable, "../../cmd/ai-task-check")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("构建任务自检失败: %v；%s", err, output)
	}
	command := func() ([]byte, error) {
		cmd := exec.CommandContext(e.ctx, executable)
		for _, item := range os.Environ() {
			if !strings.HasPrefix(strings.ToUpper(item), "TM_DB_DSN=") && !strings.HasPrefix(strings.ToUpper(item), "TM_QUOTE_ENABLED=") {
				cmd.Env = append(cmd.Env, item)
			}
		}
		cmd.Env = append(cmd.Env, "TM_DB_DSN="+cfg.FormatDSN(), "TM_QUOTE_ENABLED=false")
		return cmd.CombinedOutput()
	}
	output, err := command()
	if err != nil || !strings.Contains(string(output), "探针数据已回滚") || strings.Contains(string(output), cfg.Passwd) {
		t.Fatal("真实任务命令未成功或泄漏凭据")
	}
	e.mustExec("ALTER TABLE tm_ai_tasks DROP CHECK chk_tm_ai_task_budget")
	output, err = command()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || !strings.Contains(string(output), "驱动详情已隐藏") || strings.Contains(string(output), cfg.Passwd) || strings.Contains(string(output), "chk_tm_ai_task_budget") {
		t.Fatal("真实命令失败未关闭或诊断泄漏")
	}
	for _, count := range taskProbeCounts(e) {
		if count != 0 {
			t.Fatal("受限账号/命令探针遗留数据")
		}
	}
}
