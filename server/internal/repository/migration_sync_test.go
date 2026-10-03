package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本文件是不需要数据库的静态守卫，随 go test ./... 在 CI 的 backend 任务里运行。
// 真正的"两种顺序、重复多次、表结构一致"校验在 migration_converge_integration_test.go（需要 MySQL）。

const (
	themeStockOriginalMig = "20260730_addon_quant_theme_stock.sql"
	themeStockConvergeMig = "20261002_converge_addon_quant_theme_stock.sql"
)

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatalf("读迁移脚本失败: %v", err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// themeStockDDL 取出 addon_quant_theme_stock 的建表语句（含其中的 -- 注释），并去掉 IF NOT EXISTS，便于逐字比对。
func themeStockDDL(t *testing.T, sql string) string {
	t.Helper()
	sql = strings.Replace(sql, "CREATE TABLE IF NOT EXISTS `addon_quant_theme_stock` (", "CREATE TABLE `addon_quant_theme_stock` (", 1)
	start := strings.Index(sql, "CREATE TABLE `addon_quant_theme_stock` (")
	if start < 0 {
		t.Fatal("找不到 addon_quant_theme_stock 的建表语句")
	}
	const tail = "无依据禁止入库（ADR-0003）';"
	end := strings.Index(sql[start:], tail)
	if end < 0 {
		t.Fatal("找不到建表语句的结尾（表注释）")
	}
	return sql[start : start+end+len(tail)]
}

// 收敛迁移里复制了一份建表语句（形态 0/1 之后要建表）。两份一旦漂移，"收敛到权威定义"就名不副实——
// 这里在没有数据库时就能发现；有数据库时集成测试再比对 SHOW CREATE TABLE。
func TestThemeStockDDLInSyncBetweenMigrations(t *testing.T) {
	orig := themeStockDDL(t, readMigration(t, themeStockOriginalMig))
	conv := themeStockDDL(t, readMigration(t, themeStockConvergeMig))
	if orig != conv {
		t.Fatalf("%s 与 %s 里的 addon_quant_theme_stock 建表语句不一致。\n"+
			"改表结构要两处一起改（权威定义以前者为准，后者是收敛用的副本）。", themeStockOriginalMig, themeStockConvergeMig)
	}
	for _, name := range []string{themeStockOriginalMig, themeStockConvergeMig} {
		if !strings.Contains(readMigration(t, name), "CREATE TABLE IF NOT EXISTS `addon_quant_theme_stock` (") {
			t.Errorf("%s 必须是 CREATE TABLE IF NOT EXISTS：GVA 可能已先建表，裸 CREATE 会报 1050 卡住迁移", name)
		}
	}
}

// 收敛迁移是纯 SQL（不用存储过程和 DELIMITER），这样 mysql 命令行、Go 驱动的 multiStatements、
// 各种 GUI 工具都能原样执行；预处理语句必须成套出现，避免泄漏会话内的预处理句柄。
func TestConvergeMigrationIsPlainSQL(t *testing.T) {
	sql := readMigration(t, themeStockConvergeMig)
	var code []string
	for _, line := range strings.Split(sql, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		code = append(code, line)
	}
	body := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(body, "DELIMITER") || strings.Contains(body, "CREATE PROCEDURE") || strings.Contains(body, "CREATE FUNCTION") {
		t.Error("收敛迁移不得使用存储过程或 DELIMITER")
	}
	prep := strings.Count(body, "PREPARE TM_STMT FROM")
	exec := strings.Count(body, "EXECUTE TM_STMT")
	dealloc := strings.Count(body, "DEALLOCATE PREPARE TM_STMT")
	if prep == 0 || prep != exec || exec != dealloc {
		t.Errorf("PREPARE/EXECUTE/DEALLOCATE 不成套：prepare=%d execute=%d deallocate=%d", prep, exec, dealloc)
	}
	if strings.Contains(body, "DROP TABLE") || strings.Contains(body, "TRUNCATE") {
		t.Error("收敛迁移不得删除数据表（封存与快照表由运维确认后手工清理）")
	}
}
