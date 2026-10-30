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

func TestIntegrationAIReportDossierRegistryReader(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, r, _, m := newDossierRegistryEnv(t, level)
			registrySave(t, e, r, m, true)
			var seed [16]byte
			if _, err := rand.Read(seed[:]); err != nil {
				t.Fatal(err)
			}
			user, password := "registry_"+hex.EncodeToString(seed[:8]), hex.EncodeToString(seed[:])
			// 标识符与口令均由本测试生成的十六进制组成；不接用户文本，不输出凭据。
			account := "'" + user + "'@'127.0.0.1'"
			if _, err := e.conn.ExecContext(e.ctx, "CREATE USER "+account+" IDENTIFIED BY '"+password+"'"); err != nil {
				t.Fatal("建立核验资料只读账号失败")
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if _, err := e.conn.ExecContext(ctx, "DROP USER "+account); err != nil {
					t.Error("清理核验资料账号失败")
				}
			})
			tables := []string{"addon_member", "addon_quant_theme", "tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies", "tm_ai_dossier_support_plans", "tm_ai_dossier_support_uses"}
			grant := func(table string) {
				t.Helper()
				e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
			}
			for _, table := range tables {
				grant(table)
			}
			cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
			if err != nil {
				t.Fatal("隔离连接配置无效")
			}
			cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
			open := func() *sql.DB {
				t.Helper()
				db, err := sql.Open("mysql", cfg.FormatDSN())
				if err != nil {
					t.Fatal("打开核验资料连接失败")
				}
				return db
			}
			// 仅资料/登记读取连接受限；原任务数据库和完整准入控制器仍为独立测试夹具。
			read := func(db *sql.DB) error {
				reader, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(db, level), false)
				if err != nil {
					return err
				}
				audit, err := service.NewAIReportCallAuditService(e.store)
				if err != nil {
					return err
				}
				ctl, err := service.NewAIDossierSupportExecutionControl(reader, e.repo, e.ctl, audit)
				if err != nil {
					return err
				}
				p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID)
				if err != nil {
					if p != nil {
						return errCheckTest
					}
					return err
				}
				if p == nil || p.InputDigest() != e.plan.InputDigest() {
					return errCheckTest
				}
				return nil
			}
			t.Run("只读连接实际重建", func(t *testing.T) {
				db := open()
				defer db.Close()
				if err := read(db); err != nil {
					t.Fatal("仅SELECT无法实际重建", err)
				}
			})
			t.Run("写入和账务明确拒绝", func(t *testing.T) {
				db := open()
				defer db.Close()
				for _, query := range []string{`CREATE TABLE registry_forbidden(id int)`, `UPDATE tm_ai_sources SET enabled=enabled WHERE 1=0`, `UPDATE tm_ai_source_bodies SET body_text=body_text WHERE 1=0`, `UPDATE tm_ai_dossier_support_uses SET enabled=enabled WHERE 1=0`, `INSERT INTO tm_ai_dossier_support_plans SELECT * FROM tm_ai_dossier_support_plans WHERE 1=0`, `SELECT * FROM tm_ai_tasks LIMIT 0`, `SELECT * FROM tm_points_accounts LIMIT 0`} {
					_, err := db.ExecContext(e.ctx, query)
					var denied *mysql.MySQLError
					if !errors.As(err, &denied) || denied.Number != 1044 && denied.Number != 1142 && denied.Number != 1143 {
						t.Fatal("资料账号越权没有明确权限错误")
					}
				}
			})
			for _, table := range tables {
				t.Run(table, func(t *testing.T) {
					e.mustExec("REVOKE SELECT ON " + e.database + "." + table + " FROM " + account)
					limited := open()
					readErr := read(limited)
					if err := limited.Close(); err != nil {
						t.Fatal("关闭受限连接失败")
					}
					var denied *mysql.MySQLError
					if !errors.As(readErr, &denied) || denied.Number != 1142 && denied.Number != 1143 {
						t.Fatal("缺少资料表权限仍重建或错误类别不符", table, readErr)
					}
					grant(table)
					restored := open()
					restoreErr := read(restored)
					if err := restored.Close(); err != nil {
						t.Fatal("关闭恢复连接失败")
					}
					if restoreErr != nil {
						t.Fatal("恢复表权限后仍不能重建", table, restoreErr)
					}
				})
			}
			if e.server.Count() != 0 {
				t.Fatal("只读重建调用模型")
			}
			e.assertUnsettled(t)
		})
	}
}
