//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

type pointsTestEnv struct {
	t        *testing.T
	ctx      context.Context
	conn     *sql.Conn
	database string
}

// 每个用例新建随机库，只清理本用例成功创建的库，不修改 DSN 原指向的表。
func setupPointsLedger(t *testing.T) (*pointsTestEnv, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 TM_TEST_DSN，跳过账本集成测试")
	}
	cfg, err := mysqldrv.ParseDSN(dsn)
	if err != nil {
		t.Fatal("测试 DSN 解析失败")
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("测试管理连接装配失败")
	}
	t.Cleanup(func() { _ = admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "tm_points_it_" + hex.EncodeToString(random[:])
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatal("创建隔离账本库失败", err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanupCtx, "DROP DATABASE `"+name+"`"); err != nil {
			t.Error("清理本用例隔离库失败", err)
		}
	})
	cfg.DBName = name
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("测试业务连接装配失败")
	}
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal("获取隔离连接失败", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	e := &pointsTestEnv{t: t, ctx: ctx, conn: conn, database: name}
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261011_create_points_ledger.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// DELIMITER 只供 mysql 客户端使用，驱动直接提交完整触发器体。
	script := strings.ReplaceAll(string(migration), "DELIMITER $$", "")
	script = strings.ReplaceAll(script, "DELIMITER ;", "")
	script = strings.ReplaceAll(script, "END$$", "END;")
	e.mustExec(script)
	e.mustExec(script)
	if err := repository.NewPointsSchemaVerifier(db).Verify(ctx); err != nil {
		t.Fatal("账本结构正向基线失败", err)
	}
	return e, db
}

func (e *pointsTestEnv) mustExec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.conn.ExecContext(e.ctx, query, args...); err != nil {
		e.t.Fatal("执行测试 SQL 失败", err)
	}
}

func (e *pointsTestEnv) queryInt(query string, args ...any) int {
	e.t.Helper()
	var n int64
	if err := e.conn.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		e.t.Fatal("读取测试结果失败", err)
	}
	return int(n)
}
