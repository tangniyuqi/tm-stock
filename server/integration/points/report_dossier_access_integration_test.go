//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 只使用本轮随机库和账号，所有业务表仅授予SELECT。
func verifyDossierReader(t *testing.T, e *pointsTestEnv, level sql.IsolationLevel) {
	t.Helper()
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	user, password := "dossier_"+hex.EncodeToString(seed[:8]), hex.EncodeToString(seed[:])
	account := "'" + user + "'@'127.0.0.1'"
	if _, err := e.conn.ExecContext(e.ctx, "CREATE USER "+account+" IDENTIFIED BY '"+password+"'"); err != nil {
		t.Fatal("创建专题只读账号失败")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := e.conn.ExecContext(ctx, "DROP USER "+account); err != nil {
			t.Error("清理专题只读账号失败")
		}
	})
	tables := []string{"addon_member", "addon_quant_theme", "tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies"}
	grant := func(table string) {
		t.Helper()
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
	}
	for _, table := range tables {
		grant(table)
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试DSN无效")
	}
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
	open := func() *sql.DB {
		t.Helper()
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal("专题只读连接失败")
		}
		return db
	}
	db := open()
	defer db.Close()
	repo := repository.NewAIDossierRepo(db, level)
	if err := repository.NewAIDossierSchemaVerifier(db).Verify(e.ctx); err == nil {
		t.Fatal("只读账号不应获得结构探针写权限")
	}
	if err := repo.VerifyReadAccess(e.ctx); err != nil {
		t.Fatal("完整只读权限预检失败", err)
	}
	svc, err := service.NewAIDossierService(repo, dossierTestAuth{}, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.ReadVersions(e.ctx, "test-member-1", 1, 7, []int64{1, 2})
	if err != nil || len(got) != 2 || len(got[0].Cards()) != 1 || len(got[1].Cards()) != 1 {
		t.Fatal("只读账号无法实际回取双版", err)
	}
	for _, query := range []string{
		`CREATE TABLE dossier_forbidden(id int)`,
		`UPDATE tm_ai_sources SET enabled=enabled WHERE 1=0`,
		`UPDATE tm_ai_source_bodies SET body_text=body_text WHERE 1=0`,
		`UPDATE tm_ai_dossier_publications SET enabled=enabled WHERE 1=0`,
		`UPDATE tm_ai_dossier_grants SET enabled=enabled WHERE 1=0`,
		`DELETE FROM tm_ai_dossier_versions WHERE 1=0`,
		`INSERT INTO tm_ai_dossier_versions SELECT * FROM tm_ai_dossier_versions WHERE 1=0`,
		`INSERT INTO tm_ai_dossier_evidence SELECT * FROM tm_ai_dossier_evidence WHERE 1=0`,
		`SELECT * FROM tm_points_accounts LIMIT 0`,
	} {
		_, err := db.ExecContext(e.ctx, query)
		var denied *mysql.MySQLError
		if !errors.As(err, &denied) || (denied.Number != 1044 && denied.Number != 1142 && denied.Number != 1143) {
			t.Fatal("专题只读账号越权未明确拒绝")
		}
	}
	for _, table := range tables {
		e.mustExec("REVOKE SELECT ON " + e.database + "." + table + " FROM " + account)
		// 重建连接，避免会话权限缓存掩盖新连接的启动行为。
		limited := open()
		r := repository.NewAIDossierRepo(limited, level)
		if err := r.VerifyReadAccess(e.ctx); err == nil {
			limited.Close()
			t.Fatalf("缺少%s权限仍通过预检", table)
		}
		s, err := service.NewAIDossierService(r, dossierTestAuth{}, false)
		if err != nil {
			limited.Close()
			t.Fatal(err)
		}
		v, readErr := s.ReadVersions(e.ctx, "test-member-1", 1, 7, []int64{1, 2})
		if err := limited.Close(); err != nil {
			t.Fatal("关闭受限连接失败")
		}
		if readErr == nil || v != nil {
			t.Fatalf("缺少%s权限仍返回正文", table)
		}
		grant(table)
		restored := open()
		restoredRepo := repository.NewAIDossierRepo(restored, level)
		checkErr := restoredRepo.VerifyReadAccess(e.ctx)
		restoredService, buildErr := service.NewAIDossierService(restoredRepo, dossierTestAuth{}, false)
		if buildErr != nil {
			restored.Close()
			t.Fatal(buildErr)
		}
		values, restoredErr := restoredService.ReadVersions(e.ctx, "test-member-1", 1, 7, []int64{1, 2})
		if err := restored.Close(); err != nil {
			t.Fatal("关闭恢复连接失败")
		}
		if checkErr != nil || restoredErr != nil || len(values) != 2 {
			t.Fatalf("恢复%s权限后未通过预检和双版读取", table)
		}
	}
}
