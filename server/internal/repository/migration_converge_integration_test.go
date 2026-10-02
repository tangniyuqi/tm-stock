//go:build integration

// 本文件是 AC-O1（docs/specs/ai-analysis，P0-2 共表治理）的可重复校验：
// 在"GVA 先启动"与"tm-stock 迁移先执行"两种顺序下，各重复多次启动与迁移，
// 最终 addon_quant_theme_stock 的表结构与权威定义逐字一致，且合规约束真的拦得住脏数据。
//
// "旧 GVA 启动"用 testdata/theme_stock_old_gva_*.sql 复现——那是 2026-10-02 用旧 GVA 模型对真实 MySQL 8.0.46
// 跑 AutoMigrate 时 GORM 实际执行的 DDL 原样摘录，不是凭记忆手写的。新版 GVA 已不再注册该模型，
// 所以这两份夹具描述的是"仍在线上运行的旧版本"留下的库状态；本测试验证的就是收敛迁移能把它们拉回来。
//
// 跑法同其他集成测试：bash scripts/dev/verify-repository.sh，或设置 TM_TEST_DSN 后
// go test -tags=integration ./internal/repository/ -run Converge（DSN 必须带 multiStatements=true）。
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
)

const (
	convergeOriginalMig = "20260730_addon_quant_theme_stock.sql"
	convergeMig         = "20261002_converge_addon_quant_theme_stock.sql"
	convergeTable       = "addon_quant_theme_stock"
)

var autoIncrementRe = regexp.MustCompile(` AUTO_INCREMENT=\d+`)

// convergeEnv 把测试固定在同一条连接上：迁移脚本里的 @tm_state 是会话变量，换连接就读不到。
type convergeEnv struct {
	t    *testing.T
	ctx  context.Context
	conn *sql.Conn
}

func newConvergeEnv(t *testing.T) *convergeEnv {
	t.Helper()
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 TM_TEST_DSN，跳过集成测试（用 scripts/dev/verify-repository.sh 跑）")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("打开连接失败: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("取连接失败: %v", err)
	}
	e := &convergeEnv{t: t, ctx: ctx, conn: conn}
	t.Cleanup(func() {
		e.dropArtifacts()
		_ = conn.Close()
		_ = db.Close()
	})
	e.reset()
	return e
}

func (e *convergeEnv) exec(query string, args ...any) error {
	_, err := e.conn.ExecContext(e.ctx, query, args...)
	return err
}

func (e *convergeEnv) mustExec(query string, args ...any) {
	e.t.Helper()
	if err := e.exec(query, args...); err != nil {
		e.t.Fatalf("执行失败: %v\nSQL: %s", err, firstLines(query, 6))
	}
}

func (e *convergeEnv) execFile(path string) error {
	e.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("读文件失败 %s: %v", path, err)
	}
	return e.exec(string(b))
}

func (e *convergeEnv) mustExecFile(path string) {
	e.t.Helper()
	if err := e.execFile(path); err != nil {
		e.t.Fatalf("执行 %s 失败（DSN 是否带 multiStatements=true？）: %v", filepath.Base(path), err)
	}
}

func (e *convergeEnv) queryString(query string, args ...any) string {
	e.t.Helper()
	var s sql.NullString
	if err := e.conn.QueryRowContext(e.ctx, query, args...).Scan(&s); err != nil {
		e.t.Fatalf("查询失败: %v\nSQL: %s", err, query)
	}
	return s.String
}

func (e *convergeEnv) queryInt(query string, args ...any) int {
	e.t.Helper()
	var n sql.NullInt64
	if err := e.conn.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		e.t.Fatalf("查询失败: %v\nSQL: %s", err, query)
	}
	return int(n.Int64)
}

// reset 清掉本测试可能留下的表，并重建上游两表（测试复刻，见 theme_integration_test.go）。
func (e *convergeEnv) reset() {
	e.t.Helper()
	e.dropArtifacts()
	e.mustExec(upstreamTestDDL)
}

// dropArtifacts 删除关联表以及迁移封存/快照出来的 tm_theme_stock_* 表，保证每个场景从干净状态开始，
// 也避免在共用的测试库里留下垃圾。
func (e *convergeEnv) dropArtifacts() {
	rows, err := e.conn.QueryContext(e.ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE 'tm\\_theme\\_stock\\_%'")
	if err != nil {
		e.t.Errorf("列出封存/快照表失败: %v", err)
		return
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			_ = rows.Close()
			e.t.Errorf("扫描表名失败: %v", err)
			return
		}
		names = append(names, n)
	}
	_ = rows.Close()
	for _, n := range names {
		if err := e.exec("DROP TABLE IF EXISTS `" + n + "`"); err != nil {
			e.t.Errorf("清理 %s 失败: %v", n, err)
		}
	}
	if err := e.exec("DROP TABLE IF EXISTS " + convergeTable); err != nil {
		e.t.Errorf("清理 %s 失败: %v", convergeTable, err)
	}
}

func (e *convergeEnv) tablesLike(prefix string) []string {
	e.t.Helper()
	rows, err := e.conn.QueryContext(e.ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE ? ORDER BY table_name",
		prefix+"%")
	if err != nil {
		e.t.Fatalf("列表失败: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			e.t.Fatalf("扫描失败: %v", err)
		}
		out = append(out, n)
	}
	return out
}

func (e *convergeEnv) tableExists() bool {
	return e.queryInt("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", convergeTable) == 1
}

func (e *convergeEnv) columnExists(col string) bool {
	return e.queryInt("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		convergeTable, col) == 1
}

func (e *convergeEnv) columnNullable(col string) bool {
	return e.queryString("SELECT is_nullable FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		convergeTable, col) == "YES"
}

// showCreate 返回规范化后的 SHOW CREATE TABLE（去掉随数据增长而变化的 AUTO_INCREMENT 计数）。
func (e *convergeEnv) showCreate() string {
	e.t.Helper()
	var name, ddl string
	if err := e.conn.QueryRowContext(e.ctx, "SHOW CREATE TABLE "+convergeTable).Scan(&name, &ddl); err != nil {
		e.t.Fatalf("SHOW CREATE TABLE 失败: %v", err)
	}
	return autoIncrementRe.ReplaceAllString(ddl, "")
}

// pristineDDL 在空表上只执行原迁移，得到"权威结构"。
func (e *convergeEnv) pristineDDL() string {
	e.t.Helper()
	e.reset()
	e.mustExecFile(migPath(convergeOriginalMig))
	ddl := e.showCreate()
	e.reset()
	return ddl
}

// converge 执行收敛迁移，返回它记录的分支（created / legacy_archived / repaired / already_converged）。
func (e *convergeEnv) converge() string {
	e.t.Helper()
	e.mustExecFile(migPath(convergeMig))
	return e.queryString("SELECT @tm_state")
}

// oldGVAStart 复现旧版 GVA 启动时 AutoMigrate 对该表的行为：表不存在就建（旧形态），
// 已存在且还没有旧 GVA 列就改（退化为混合形态），已经是混合形态则什么都不做（GORM 的 AutoMigrate 本身幂等）。
func (e *convergeEnv) oldGVAStart() {
	e.t.Helper()
	switch {
	case !e.tableExists():
		e.mustExecFile(fixturePath("theme_stock_old_gva_create.sql"))
	case !e.columnExists("reason"):
		e.mustExecFile(fixturePath("theme_stock_old_gva_alter.sql"))
	}
}

func migPath(name string) string     { return filepath.Join("..", "..", "migrations", name) }
func fixturePath(name string) string { return filepath.Join("testdata", name) }

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = append(lines[:n], "...")
	}
	return strings.Join(lines, "\n")
}

func mysqlErrno(err error) uint16 {
	var me *mysqldrv.MySQLError
	if errors.As(err, &me) {
		return me.Number
	}
	return 0
}

const evidenceRowSQL = `INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (%s,%s,'688502.SH',2,%s,%s,NOW(3),NOW(3))`

func evidenceInsert(theme, stock, excerpt, url string) string {
	return fmt.Sprintf(evidenceRowSQL, theme, stock, excerpt, url)
}

// assertConstraintsEnforced 在已收敛的表上做"拒脏数据"实测——只写了约束不等于约束生效。
// 判据与 scripts/dev/verify-migrations.sh 一致，并额外覆盖旧 GVA 退化后会丢掉的 NOT NULL 与默认值。
func assertConstraintsEnforced(t *testing.T, e *convergeEnv) {
	t.Helper()
	e.mustExec("DELETE FROM " + convergeTable)
	cases := []struct {
		name  string
		query string
		errno uint16
	}{
		{"摘录为空串被 CHECK 拒绝", evidenceInsert("100010", "1", "''", "'https://e.com/r'"), 3819},
		{"链接为空串被 CHECK 拒绝", evidenceInsert("100010", "1", "'摘录'", "''"), 3819},
		{"依据类型为 0 被 CHECK 拒绝", strings.Replace(evidenceInsert("100010", "1", "'摘录'", "'https://e.com/r'"), ",2,", ",0,", 1), 3819},
		{"摘录为 NULL 被 NOT NULL 拒绝", evidenceInsert("100010", "1", "NULL", "'https://e.com/r'"), 1048},
		{"题材 ID 为 NULL 被 NOT NULL 拒绝（旧 GVA 会把它改成可空）", evidenceInsert("NULL", "1", "'摘录'", "'https://e.com/r'"), 1048},
		{"股票 ID 为 NULL 被 NOT NULL 拒绝（旧 GVA 会把它改成可空）", evidenceInsert("100010", "NULL", "'摘录'", "'https://e.com/r'"), 1048},
	}
	for _, c := range cases {
		err := e.exec(c.query)
		if err == nil {
			t.Errorf("%s：脏数据竟然写进去了", c.name)
			continue
		}
		if got := mysqlErrno(err); got != c.errno {
			t.Errorf("%s：期望 MySQL 错误 %d，实际 %v", c.name, c.errno, err)
		}
	}
	if n := e.queryInt("SELECT COUNT(*) FROM " + convergeTable); n != 0 {
		t.Fatalf("脏数据全部被拒后表应为空，实际 %d 行", n)
	}

	// 正例：约束没有过严；未指定 status/audit_status 时取权威默认值（1 启用、0 草稿——fail-safe：新行默认不对外可见）
	e.mustExec(evidenceInsert("100010", "1", "'光源模组业务收入占比34.2%'", "'https://e.com/r'"))
	if got := e.queryString("SELECT CONCAT(status, '/', audit_status) FROM " + convergeTable); got != "1/0" {
		t.Errorf("未指定时 status/audit_status 应为 1/0，实际 %s（status 默认值丢失是旧 GVA 改表的症状之一）", got)
	}
	// 同题材同股票重复挂接被唯一键拒绝
	if err := e.exec(evidenceInsert("100010", "1", "'另一段摘录'", "'https://e.com/r2'")); mysqlErrno(err) != 1062 {
		t.Errorf("重复挂接应被唯一键拒绝（1062），实际 %v", err)
	}
	// 同一股票可挂另一个题材（多对多）；软删后可重新添加同一对（生成列 alive 生效）
	e.mustExec(evidenceInsert("100001", "1", "'公司属光刻机产业链'", "'https://e.com/r3'"))
	e.mustExec("UPDATE " + convergeTable + " SET deleted_at = NOW(3) WHERE theme_id = 100010 AND stock_id = 1")
	e.mustExec(evidenceInsert("100010", "1", "'重新收录'", "'https://e.com/r4'"))
	if n := e.queryInt("SELECT COUNT(*) FROM " + convergeTable); n != 3 {
		t.Errorf("正例与软删重加后应共 3 行，实际 %d", n)
	}
	e.mustExec("DELETE FROM " + convergeTable)
}

// 旧 GVA 夹具必须真的把表改坏——否则"收敛后结构一致"这个断言就是空转（测试自己也要有灵敏度检查）。
func assertDegradedByOldGVA(t *testing.T, e *convergeEnv, pristine string) {
	t.Helper()
	if e.showCreate() == pristine {
		t.Fatal("旧 GVA 夹具没有改变表结构，夹具失效")
	}
	for _, col := range []string{"reason", "ai_reason", "tier", "relevance", "in_date"} {
		if !e.columnExists(col) {
			t.Fatalf("退化后应出现旧 GVA 列 %s", col)
		}
	}
	if !e.columnNullable("theme_id") || !e.columnNullable("stock_id") {
		t.Fatal("退化后 theme_id/stock_id 应已失去 NOT NULL")
	}
}

func TestIntegration_Converge_各形态一次收敛(t *testing.T) {
	e := newConvergeEnv(t)
	pristine := e.pristineDDL()

	t.Run("tm-stock先建表_收敛是空操作", func(t *testing.T) {
		e.reset()
		e.mustExecFile(migPath(convergeOriginalMig))
		if st := e.converge(); st != "already_converged" {
			t.Errorf("分支 = %s，期望 already_converged", st)
		}
		if got := e.showCreate(); got != pristine {
			t.Errorf("收敛不应改动权威表：\n%s", got)
		}
	})

	t.Run("空库_收敛先于原迁移_两者先后都安全", func(t *testing.T) {
		e.reset()
		if st := e.converge(); st != "created" {
			t.Errorf("分支 = %s，期望 created", st)
		}
		e.mustExecFile(migPath(convergeOriginalMig)) // IF NOT EXISTS：不得报 1050
		if got := e.showCreate(); got != pristine {
			t.Errorf("先收敛再跑原迁移后结构应与权威一致：\n%s", got)
		}
		if st := e.converge(); st != "already_converged" {
			t.Errorf("第二次分支 = %s，期望 already_converged", st)
		}
	})

	t.Run("GVA先启动_旧表整表封存_旧行不伪造依据", func(t *testing.T) {
		e.reset()
		e.oldGVAStart() // 空库：旧 GVA 建出没有依据列、没有 CHECK 的表
		if e.columnExists("source_excerpt") {
			t.Fatal("旧 GVA 建的表不应有依据列（夹具失效）")
		}
		e.mustExec(`INSERT INTO addon_quant_theme_stock (theme_id,stock_id,reason,ai_reason,tier,relevance,in_date,sort,status,created_at) VALUES
			(100001,1,'人工理由','AI理由甲',1,92.5,NOW(3),0,1,NOW(3)),
			(100001,2,NULL,'AI理由乙',2,71,NOW(3),1,1,NOW(3)),
			(100010,1,NULL,NULL,0,NULL,NULL,0,NULL,NOW(3))`)

		// 原迁移在旧表之上不得报错，也不得动它（旧版本里这里会报 1050）
		before := e.showCreate()
		e.mustExecFile(migPath(convergeOriginalMig))
		if got := e.showCreate(); got != before {
			t.Fatal("原迁移不应改动已存在的旧表")
		}

		if st := e.converge(); st != "legacy_archived" {
			t.Errorf("分支 = %s，期望 legacy_archived", st)
		}
		if got := e.showCreate(); got != pristine {
			t.Errorf("封存后新表结构应与权威一致：\n%s", got)
		}
		if n := e.queryInt("SELECT COUNT(*) FROM " + convergeTable); n != 0 {
			t.Errorf("无依据的旧行不得进入权威表，实际 %d 行", n)
		}
		archives := e.tablesLike("tm_theme_stock_legacy_")
		if len(archives) != 1 {
			t.Fatalf("应恰有 1 张封存表，实际 %v", archives)
		}
		rows, err := e.conn.QueryContext(e.ctx,
			"SELECT id, theme_id, stock_id, IFNULL(tier,-1), IFNULL(relevance,-1), IFNULL(ai_reason,'') FROM `"+archives[0]+"` ORDER BY id")
		if err != nil {
			t.Fatalf("读封存表失败: %v", err)
		}
		defer rows.Close()
		type legacyRow struct {
			ID, Theme, Stock int
			Tier             int
			Rel              float64
			AI               string
		}
		var got []legacyRow
		for rows.Next() {
			var r legacyRow
			if err := rows.Scan(&r.ID, &r.Theme, &r.Stock, &r.Tier, &r.Rel, &r.AI); err != nil {
				t.Fatalf("扫描封存表失败: %v", err)
			}
			got = append(got, r)
		}
		want := []legacyRow{
			{1, 100001, 1, 1, 92.5, "AI理由甲"},
			{2, 100001, 2, 2, 71, "AI理由乙"},
			{3, 100010, 1, 0, -1, ""},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("封存表数据应与旧表逐行一致（可追溯）：\n got  %+v\n want %+v", got, want)
		}
		assertConstraintsEnforced(t, e)
	})

	t.Run("tm-stock先建表_旧GVA改写_收敛修复", func(t *testing.T) {
		e.reset()
		e.mustExecFile(migPath(convergeOriginalMig))
		e.oldGVAStart()
		assertDegradedByOldGVA(t, e, pristine)
		if st := e.converge(); st != "repaired" {
			t.Errorf("分支 = %s，期望 repaired", st)
		}
		if got := e.showCreate(); got != pristine {
			t.Errorf("修复后结构应与权威一致：\n%s", got)
		}
		if n := len(e.tablesLike("tm_theme_stock_snapshot_")); n != 0 {
			t.Errorf("空表无数据可丢，不应留快照，实际 %d 张", n)
		}
		assertConstraintsEnforced(t, e)
	})

	t.Run("混合表带数据_有价值的数据先快照再修复", func(t *testing.T) {
		e.reset()
		e.mustExecFile(migPath(convergeOriginalMig))
		e.mustExec(evidenceInsert("100010", "1", "'光源模组业务收入占比34.2%'", "'https://e.com/r'"))
		e.mustExec(evidenceInsert("100001", "1", "'公司属光刻机产业链'", "'https://e.com/r3'"))
		e.oldGVAStart()
		// 旧 GVA 退化之后的状态：一行带上了旧 GVA 列的取值；混合态下 theme_id 可空，于是能出现关联键为空的废行
		e.mustExec("UPDATE addon_quant_theme_stock SET tier=1, relevance=88.5, ai_reason='AI理由', reason='人工理由' WHERE id=1")
		e.mustExec(evidenceInsert("NULL", "2", "'废行'", "'https://e.com/r9'"))

		if st := e.converge(); st != "repaired" {
			t.Errorf("分支 = %s，期望 repaired", st)
		}
		if got := e.showCreate(); got != pristine {
			t.Errorf("修复后结构应与权威一致：\n%s", got)
		}
		if n := e.queryInt("SELECT COUNT(*) FROM " + convergeTable); n != 2 {
			t.Errorf("主表应保留 2 条合格行、清掉 1 条废行，实际 %d", n)
		}
		if ids := e.queryString("SELECT GROUP_CONCAT(id ORDER BY id) FROM " + convergeTable); ids != "1,2" {
			t.Errorf("合格行 id 必须原样保留（下游可能引用），实际 %s", ids)
		}
		snaps := e.tablesLike("tm_theme_stock_snapshot_")
		if len(snaps) != 1 {
			t.Fatalf("应恰有 1 张快照表，实际 %v", snaps)
		}
		if n := e.queryInt("SELECT COUNT(*) FROM `" + snaps[0] + "`"); n != 3 {
			t.Errorf("快照应含修复前的全部 3 行，实际 %d", n)
		}
		if got := e.queryString("SELECT CONCAT(tier,'/',relevance,'/',ai_reason,'/',reason) FROM `" + snaps[0] + "` WHERE id=1"); got != "1/88.5/AI理由/人工理由" {
			t.Errorf("快照应保留旧 GVA 列的取值，实际 %q", got)
		}
		if n := e.queryInt("SELECT COUNT(*) FROM `" + snaps[0] + "` WHERE theme_id IS NULL"); n != 1 {
			t.Errorf("被清理的废行应能在快照里找到，实际 %d", n)
		}
		assertConstraintsEnforced(t, e)
	})
}

// AC-O1 的主体：两种顺序、每种重复多次"启动 + 迁移"，每一轮之后结构都必须与权威一致，
// 已有的合格数据不得丢失，被旧 GVA 退化的状态每次都能被拉回来。
func TestIntegration_Converge_两种顺序各重复多次(t *testing.T) {
	e := newConvergeEnv(t)
	pristine := e.pristineDDL()
	const cycles = 4

	for _, order := range []string{"tm-stock迁移先执行", "GVA先启动"} {
		t.Run(order, func(t *testing.T) {
			e.reset()
			if order == "tm-stock迁移先执行" {
				e.mustExecFile(migPath(convergeOriginalMig))
			}
			var keepID int
			for i := 1; i <= cycles; i++ {
				e.oldGVAStart() // 每一轮都有一次"旧 GVA 启动"
				firstOfGVAFirst := order == "GVA先启动" && i == 1
				if firstOfGVAFirst {
					if e.columnExists("source_excerpt") || !e.columnExists("reason") {
						t.Fatal("GVA 先启动后表应是旧形态：有旧 GVA 列、没有依据列")
					}
				} else {
					assertDegradedByOldGVA(t, e, pristine) // 其余每次启动都会把表（再次）改成混合形态
				}
				e.mustExecFile(migPath(convergeOriginalMig)) // 原迁移在任何形态下都不得报错

				want := "repaired"
				if firstOfGVAFirst {
					want = "legacy_archived"
				}
				if st := e.converge(); st != want {
					t.Errorf("第 %d 轮分支 = %s，期望 %s", i, st, want)
				}
				if got := e.showCreate(); got != pristine {
					t.Fatalf("第 %d 轮收敛后结构与权威不一致：\n%s", i, got)
				}
				// 紧接着再迁移一次必须是空操作（幂等）
				if st := e.converge(); st != "already_converged" {
					t.Errorf("第 %d 轮重复收敛分支 = %s，期望 already_converged", i, st)
				}

				if i == 1 {
					e.mustExec(evidenceInsert("100010", "1", "'光源模组业务收入占比34.2%'", "'https://e.com/r'"))
					keepID = e.queryInt("SELECT id FROM " + convergeTable)
					continue
				}
				if n := e.queryInt("SELECT COUNT(*) FROM " + convergeTable); n != 1 {
					t.Errorf("第 %d 轮后行数应为 1，实际 %d", i, n)
				}
				if got := e.queryInt("SELECT id FROM " + convergeTable + " WHERE theme_id=100010 AND stock_id=1"); got != keepID {
					t.Errorf("第 %d 轮后已有的合格行丢失或 id 变化：期望 %d，实际 %d", i, keepID, got)
				}
			}
			assertConstraintsEnforced(t, e)
		})
	}
}

// GVA 的题材表里有 stock_count（题材上的"可见股票数量"）。封存旧表或修复混合表之后，旧值失真，
// 收敛迁移要按"C 端可见"口径（审核已通过、启用、未删除）重算；没有这一列的库（如最小复刻的测试库）则跳过。
func TestIntegration_Converge_题材可见数量重算(t *testing.T) {
	e := newConvergeEnv(t)
	addCount := func() {
		e.mustExec("ALTER TABLE addon_quant_theme ADD COLUMN stock_count int DEFAULT 0")
		e.mustExec(`INSERT INTO addon_quant_theme (id,name,level,parent_id,created_at,updated_at)
			VALUES (100001,'光刻机',1,0,NOW(3),NOW(3)),(100010,'光源',2,100001,NOW(3),NOW(3))`)
		e.mustExec("UPDATE addon_quant_theme SET stock_count = 9") // 旧口径留下的失真数字
	}
	count := func(theme int) int {
		return e.queryInt("SELECT stock_count FROM addon_quant_theme WHERE id = ?", theme)
	}

	t.Run("封存旧表后归零", func(t *testing.T) {
		e.reset()
		addCount()
		e.oldGVAStart()
		e.mustExec(`INSERT INTO addon_quant_theme_stock (theme_id,stock_id,tier,created_at) VALUES (100010,1,1,NOW(3)),(100010,2,2,NOW(3))`)
		if st := e.converge(); st != "legacy_archived" {
			t.Fatalf("分支 = %s", st)
		}
		if a, b := count(100001), count(100010); a != 0 || b != 0 {
			t.Errorf("无依据的旧行已封存，可见数量应全部归零，实际 %d/%d", a, b)
		}
	})

	t.Run("修复后按可见口径重算", func(t *testing.T) {
		e.reset()
		e.mustExecFile(migPath(convergeOriginalMig))
		addCount()
		// 100010：2 条通过且启用（计入）、1 条通过但停用、1 条草稿、1 条通过但已软删；100001：1 条通过且启用
		ins := func(theme, stock string, audit, status int, deleted bool) {
			del := "NULL"
			if deleted {
				del = "NOW(3)"
			}
			e.mustExec(fmt.Sprintf(`INSERT INTO addon_quant_theme_stock
				(theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,deleted_at,created_at)
				VALUES (%s,%s,'600000.SH',1,'公司披露了相关业务的进展情况。','https://e.com/%s-%s',NOW(3),%d,%d,%s,NOW(3))`,
				theme, stock, theme, stock, audit, status, del))
		}
		ins("100010", "1", 2, 1, false)
		ins("100010", "2", 2, 1, false)
		ins("100010", "3", 2, 0, false)
		ins("100010", "4", 0, 1, false)
		ins("100010", "5", 2, 1, true)
		ins("100001", "1", 2, 1, false)
		e.oldGVAStart() // 旧 GVA 改表，制造"需要修复"的状态
		if st := e.converge(); st != "repaired" {
			t.Fatalf("分支 = %s", st)
		}
		if a, b := count(100010), count(100001); a != 2 || b != 1 {
			t.Errorf("可见数量应为 100010=2、100001=1，实际 %d/%d", a, b)
		}
	})

	t.Run("没有 stock_count 列的库跳过且不报错", func(t *testing.T) {
		e.reset() // 最小复刻的题材表没有 stock_count
		if st := e.converge(); st != "created" {
			t.Fatalf("分支 = %s", st)
		}
	})
}

// 认不出的形态必须停下来，而不是猜着改：中止后表原样不动，报错信息点名原因。
func TestIntegration_Converge_未知形态主动中止(t *testing.T) {
	e := newConvergeEnv(t)
	cases := []struct {
		name  string
		setup func()
		abort string
	}{
		{"只有部分依据列", func() {
			e.mustExec("CREATE TABLE addon_quant_theme_stock (id bigint unsigned primary key auto_increment, theme_id bigint unsigned, stock_id bigint unsigned, ts_code varchar(20))")
		}, "tm_migration_abort__theme_stock_only_some_evidence_columns"},
		{"缺少 theme_id", func() {
			e.mustExec("CREATE TABLE addon_quant_theme_stock (id bigint unsigned primary key auto_increment, foo int)")
		}, "tm_migration_abort__theme_stock_missing_theme_id_or_stock_id"},
		{"权威表的依据 CHECK 被人删了", func() {
			e.mustExecFile(migPath(convergeOriginalMig))
			e.mustExec("ALTER TABLE addon_quant_theme_stock DROP CHECK chk_theme_stock_evidence")
		}, "tm_migration_abort__theme_stock_evidence_check_missing"},
		{"权威表的唯一键被人删了", func() {
			e.mustExecFile(migPath(convergeOriginalMig))
			e.mustExec("ALTER TABLE addon_quant_theme_stock DROP INDEX uk_theme_stock_alive")
		}, "tm_migration_abort__theme_stock_unique_key_or_alive_missing"},
		{"权威表的依据列被改成可空", func() {
			e.mustExecFile(migPath(convergeOriginalMig))
			e.mustExec("ALTER TABLE addon_quant_theme_stock MODIFY source_excerpt varchar(1000) NULL")
		}, "tm_migration_abort__theme_stock_evidence_column_nullable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e.reset()
			c.setup()
			before := e.showCreate()
			err := e.execFile(migPath(convergeMig))
			if err == nil {
				t.Fatalf("应当中止，却执行成功了")
			}
			if !strings.Contains(err.Error(), c.abort) {
				t.Errorf("报错应点名 %s，实际：%v", c.abort, err)
			}
			if got := e.showCreate(); got != before {
				t.Errorf("中止后表不应被改动：\n%s", got)
			}
			if n := len(e.tablesLike("tm_theme_stock_legacy_")); n != 0 {
				t.Errorf("中止时不应已封存任何表，实际 %d 张", n)
			}
		})
	}
}
