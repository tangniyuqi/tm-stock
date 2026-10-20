//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mysqldrv "github.com/go-sql-driver/mysql"
)

const pointsMigration = "20261011_create_points_ledger.sql"

func setupPointsProbe(t *testing.T) (*convergeEnv, *sql.DB) {
	t.Helper()
	e := newConvergeEnv(t)
	clear := func() {
		for _, name := range []string{"tm_points_debt_allocs", "tm_points_hold_allocs", "tm_points_entries", "tm_points_debts", "tm_points_holds", "tm_points_lots", "tm_points_txns", "tm_points_accounts"} {
			e.mustExec("DROP TABLE IF EXISTS " + name)
		}
	}
	clear()
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", pointsMigration))
	if err != nil {
		t.Fatal(err)
	}
	// DELIMITER 是 mysql 客户端指令，驱动将完整触发器体发给服务器，无需该指令。
	script := strings.ReplaceAll(string(migration), "DELIMITER $$", "")
	script = strings.ReplaceAll(script, "DELIMITER ;", "")
	script = strings.ReplaceAll(script, "END$$", "END;")
	e.mustExec(script)
	e.mustExec(script)
	db, err := sql.Open("mysql", os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(); _ = db.Close() })
	// 每个破坏性反例先证明同一环境的完整约束能通过，防止基础错误造成假阳性。
	if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err != nil {
		t.Fatalf("账本正向基线失败: %v", err)
	}
	assertPointsProbeRollback(t, e)
	return e, db
}

func TestIntegrationPointsProbeSessionTimeZones(t *testing.T) {
	e, _ := setupPointsProbe(t)
	for _, zone := range []string{"'+08:00'", "'-12:00'", "'+00:00'"} {
		t.Run(zone, func(t *testing.T) {
			cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
			if err != nil {
				t.Fatal("测试 DSN 解析失败")
			}
			if cfg.Params == nil {
				cfg.Params = make(map[string]string)
			}
			cfg.Params["time_zone"] = zone
			db, err := sql.Open("mysql", cfg.FormatDSN())
			if err != nil {
				t.Fatal("测试数据库装配失败")
			}
			defer db.Close()
			if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err != nil {
				t.Fatal(err)
			}
			assertPointsProbeRollback(t, e)
		})
	}
}

func assertPointsProbeRollback(t *testing.T, e *convergeEnv) {
	t.Helper()
	for _, table := range pointsTables {
		if n := e.queryInt("SELECT COUNT(*) FROM " + table.name); n != 0 {
			t.Fatalf("探针遗留 %s 行数=%d", table.name, n)
		}
	}
}

func TestIntegrationPointsSchemaProbeAndRollback(t *testing.T) {
	e, db := setupPointsProbe(t)
	check := NewPointsSchemaVerifier(db)
	if err := check.Verify(e.ctx); err != nil {
		t.Fatal(err)
	}
	if err := check.Verify(e.ctx); err != nil {
		t.Fatal(err)
	}
	assertPointsProbeRollback(t, e)
	// 既有余额不参与探针，必须原样保留；不把 DB 约束验证当作余额对账。
	e.mustExec("INSERT INTO tm_points_accounts(user_id,avail,frozen,debt) VALUES(7,123,5,2)")
	if err := check.Verify(e.ctx); err != nil {
		t.Fatal(err)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_points_accounts") != 1 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=7") != 123 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=7") != 5 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=7") != 2 {
		t.Fatal("探针改动既有账户")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := check.Verify(ctx); err == nil {
		t.Fatal("取消上下文仍通过")
	}
}

func TestIntegrationPointsCheckCLI(t *testing.T) {
	e, _ := setupPointsProbe(t)
	command := exec.CommandContext(e.ctx, "go", "run", "../../cmd/points-check")
	// 子进程单独注入隔离库，不改变测试宿主的业务 DSN。
	for _, env := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(env), "TM_DB_DSN=") {
			command.Env = append(command.Env, env)
		}
	}
	command.Env = append(command.Env, "TM_DB_DSN="+os.Getenv("TM_TEST_DSN"))
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "探针数据已回滚") {
		t.Fatalf("自检命令失败: %v；%s", err, output)
	}
	assertPointsProbeRollback(t, e)
}

func TestIntegrationPointsProbeWithoutDDLPrivileges(t *testing.T) {
	e, _ := setupPointsProbe(t)
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试 DSN 解析失败")
	}
	// CREATE USER 的口令语法不接受参数占位符；只拼入本地生成的固定长度十六进制随机值。
	// 仅用于隔离测试账号，不接受外部输入，不输出凭据或 DSN。
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal("生成隔离测试口令失败")
	}
	cfg.Passwd = hex.EncodeToString(random[:])
	if err := e.exec("CREATE USER 'tm_points_probe'@'%' IDENTIFIED BY '" + cfg.Passwd + "'"); err != nil {
		if me, ok := err.(*mysqldrv.MySQLError); ok {
			t.Fatalf("创建受限测试账号失败，MySQL 错误码=%d", me.Number)
		}
		t.Fatalf("创建受限测试账号失败，错误类型=%T", err)
	}
	t.Cleanup(func() { e.mustExec("DROP USER 'tm_points_probe'@'%'") })
	// 库名由隔离测试 DSN 提供，反引号按 MySQL 规则转义；不接受外部请求。
	database := "`" + strings.ReplaceAll(cfg.DBName, "`", "``") + "`"
	for _, table := range pointsTables {
		e.mustExec("GRANT SELECT, INSERT, UPDATE, DELETE ON " + database + "." + table.name + " TO 'tm_points_probe'@'%'")
	}
	cfg.User = "tm_points_probe"
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("受限测试连接装配失败")
	}
	defer db.Close()
	var triggerCount int
	if err := db.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE()").Scan(&triggerCount); err != nil || triggerCount != 0 {
		t.Fatal("受限账号不应拥有触发器可见/DDL 权限")
	}
	if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err != nil {
		t.Fatal(err)
	}
	assertPointsProbeRollback(t, e)
}

func TestIntegrationPointsProbeDetectsWeakenedChecks(t *testing.T) {
	for _, table := range pointsTables {
		t.Run(table.name, func(t *testing.T) {
			e, db := setupPointsProbe(t)
			e.mustExec("ALTER TABLE " + table.name + " DROP CHECK " + table.check + ", ADD CONSTRAINT " + table.check + " CHECK (TRUE)")
			if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err == nil || !strings.Contains(err.Error(), "未按预期约束拒绝") {
				t.Fatalf("同名恒真 CHECK 未被目标探针发现: %v", err)
			}
			assertPointsProbeRollback(t, e)
		})
	}
}

func TestIntegrationPointsProbeDetectsNoopTriggers(t *testing.T) {
	for _, table := range pointsTables {
		for _, operation := range []string{"UPDATE", "DELETE"} {
			t.Run(table.name+"_"+operation, func(t *testing.T) {
				e, db := setupPointsProbe(t)
				trigger := table.name + "_no_delete"
				if operation == "UPDATE" {
					trigger = table.name + "_identity"
					if table.immutable {
						trigger = table.name + "_no_update"
					}
				}
				e.mustExec("DROP TRIGGER " + trigger)
				e.mustExec("CREATE TRIGGER " + trigger + " BEFORE " + operation + " ON " + table.name + " FOR EACH ROW SET @tm_points_noop=1")
				if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err == nil || !strings.Contains(err.Error(), "保护失效") {
					t.Fatalf("同名空触发器未被目标探针发现: %v", err)
				}
				assertPointsProbeRollback(t, e)
			})
		}
	}
}

func TestIntegrationPointsProbeDetectsWeakenedMetadata(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{"缺少 CHECK", "ALTER TABLE tm_points_accounts DROP CHECK chk_tm_points_account"},
		{"CHECK 不执行", "ALTER TABLE tm_points_accounts ALTER CHECK chk_tm_points_account NOT ENFORCED"},
		{"跨用户外键移除", "ALTER TABLE tm_points_entries DROP FOREIGN KEY fk_tm_points_entry_lot"},
		{"唯一幂等键变普通索引", "ALTER TABLE tm_points_txns DROP INDEX uk_tm_points_txn_idem, ADD INDEX uk_tm_points_txn_idem(idem_key)"},
		{"业务唯一键移除", "ALTER TABLE tm_points_txns DROP INDEX uk_tm_points_txn_business"},
		{"冻结分摊用户索引移除", "ALTER TABLE tm_points_hold_allocs DROP INDEX idx_tm_points_hold_alloc_user"},
		{"欠款分摊用户索引移除", "ALTER TABLE tm_points_debt_allocs DROP INDEX idx_tm_points_debt_alloc_user"},
		{"用户索引首列不一致", "ALTER TABLE tm_points_hold_allocs DROP INDEX idx_tm_points_hold_alloc_user, ADD INDEX idx_tm_points_hold_alloc_user(hold_id,user_id,lot_id)"},
		{"用户索引隐藏", "ALTER TABLE tm_points_debt_allocs ALTER INDEX idx_tm_points_debt_alloc_user INVISIBLE"},
		{"整数金额变小数", "ALTER TABLE tm_points_accounts MODIFY avail decimal(20,2) NOT NULL DEFAULT 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, db := setupPointsProbe(t)
			e.mustExec(tc.query)
			if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err == nil {
				t.Fatal("被削弱的账本结构仍通过")
			}
			assertPointsProbeRollback(t, e)
		})
	}
	for _, param := range []string{"foreign_key_checks=0", "sql_mode=''"} {
		t.Run(param, func(t *testing.T) {
			e, _ := setupPointsProbe(t)
			// MySQL 驱动会在每条新连接设置 DSN 参数，验证不是只改了测试器的另一条连接。
			db, err := sql.Open("mysql", os.Getenv("TM_TEST_DSN")+"&"+strings.ReplaceAll(param, "'", "%27"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := NewPointsSchemaVerifier(db).Verify(e.ctx); err == nil {
				t.Fatal("不安全会话配置未拒绝")
			}
			assertPointsProbeRollback(t, e)
		})
	}
}
