//go:build integration

package points_test

import (
	"database/sql"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func TestIntegrationAIReportConversationDossierSchema(t *testing.T) {
	p := newDossierProcessFixture(t, 4*time.Minute)
	e := p.f.env
	reportDDL(t, e.pointsTestEnv, "20261030_link_tm_ai_conversation_dossiers.sql")
	_, _ = localPasswordTokens(t, e.ctx, p.login, p.client)
	v := repository.NewAIConversationDossierSchemaVerifier(e.db)
	check := func() {
		t.Helper()
		if err := v.Verify(e.ctx); err != nil {
			t.Fatal("会话引用结构检查失败", err)
		}
	}
	t.Run("真实探针全部回滚", func(t *testing.T) {
		check()
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversations`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_versions`) != 2 {
			t.Fatal("结构探针留下业务行")
		}
	})
	const restoredCheck = `user_id BETWEEN 1 AND 9007199254740991 AND conversation_id BETWEEN 1 AND 9007199254740991 AND theme_id BETWEEN 1 AND 9007199254740991 AND version_no BETWEEN 1 AND 9007199254740991 AND (compare_version IS NULL OR compare_version>version_no AND compare_version<=9007199254740991) AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at`
	for _, c := range []struct{ name, breakSQL, restoreSQL string }{
		{"空值生成列被改写", `ALTER TABLE tm_ai_conversation_dossiers MODIFY compare_key bigint GENERATED ALWAYS AS (COALESCE(compare_version,9)) STORED`, `ALTER TABLE tm_ai_conversation_dossiers MODIFY compare_key bigint GENERATED ALWAYS AS (COALESCE(compare_version,0)) STORED`},
		{"缺组合唯一键", `ALTER TABLE tm_ai_conversation_dossiers DROP INDEX uq_tm_ai_conversation_dossier_selection`, `ALTER TABLE tm_ai_conversation_dossiers ADD UNIQUE KEY uq_tm_ai_conversation_dossier_selection(conversation_id,theme_id,version_no,compare_key)`},
		{"缺第二版本外键", `ALTER TABLE tm_ai_conversation_dossiers DROP FOREIGN KEY fk_tm_ai_conversation_dossier_compare`, `ALTER TABLE tm_ai_conversation_dossiers ADD CONSTRAINT fk_tm_ai_conversation_dossier_compare FOREIGN KEY(theme_id,compare_version) REFERENCES tm_ai_dossier_versions(theme_id,version_no)`},
		{"检查未执行", `ALTER TABLE tm_ai_conversation_dossiers ALTER CHECK chk_tm_ai_conversation_dossier NOT ENFORCED`, `ALTER TABLE tm_ai_conversation_dossiers ALTER CHECK chk_tm_ai_conversation_dossier ENFORCED`},
		{"同名检查失效", `ALTER TABLE tm_ai_conversation_dossiers DROP CHECK chk_tm_ai_conversation_dossier; ALTER TABLE tm_ai_conversation_dossiers ADD CONSTRAINT chk_tm_ai_conversation_dossier CHECK(1=1)`, `ALTER TABLE tm_ai_conversation_dossiers DROP CHECK chk_tm_ai_conversation_dossier; ALTER TABLE tm_ai_conversation_dossiers ADD CONSTRAINT chk_tm_ai_conversation_dossier CHECK(` + restoredCheck + `)`},
		{"时点精度变化", `ALTER TABLE tm_ai_conversation_dossiers MODIFY created_at datetime NOT NULL`, `ALTER TABLE tm_ai_conversation_dossiers MODIFY created_at datetime(3) NOT NULL`},
		{"缺不可变触发器", `DROP TRIGGER tm_ai_conversation_dossier_no_delete`, ""},
		{"同名插入保护失效", `DROP TRIGGER tm_ai_conversation_dossier_insert; CREATE TRIGGER tm_ai_conversation_dossier_insert BEFORE INSERT ON tm_ai_conversation_dossiers FOR EACH ROW SET @reference_test_only=1`, ""},
		{"额外触发器", `CREATE TRIGGER reference_test_extra AFTER INSERT ON tm_ai_conversation_dossiers FOR EACH ROW SET @reference_test_only=1`, `DROP TRIGGER reference_test_extra`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e.mustExec(c.breakSQL)
			if err := v.Verify(e.ctx); err == nil {
				t.Fatal("损坏引用结构仍通过")
			}
			if c.restoreSQL != "" {
				e.mustExec(c.restoreSQL)
			}
			reportDDL(t, e.pointsTestEnv, "20261030_link_tm_ai_conversation_dossiers.sql")
			check()
		})
	}
	t.Run("运行账号仅新增查询与插入权限", func(t *testing.T) {
		dsn := localAPIDSN(t, e)
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			t.Fatal("运行配置无效")
		}
		account := "'" + cfg.User + "'@'127.0.0.1'"
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatal("运行连接失败")
		}
		defer db.Close()
		r := repository.NewAIConversationDossierRepo(db, sql.LevelReadCommitted)
		if err = r.VerifyAccess(e.ctx); err == nil {
			t.Fatal("缺新表读取权限仍通过")
		}
		e.mustExec("GRANT SELECT ON " + e.database + ".tm_ai_conversation_dossiers TO " + account)
		if err = r.VerifyAccess(e.ctx); err == nil {
			t.Fatal("缺插入权限仍通过")
		}
		e.mustExec("GRANT INSERT ON " + e.database + ".tm_ai_conversation_dossiers TO " + account)
		if err = r.VerifyAccess(e.ctx); err != nil {
			t.Fatal("最小增量权限未通过", err)
		}
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 0 {
			t.Fatal("权限预检写入了引用")
		}
		for _, q := range []string{`UPDATE tm_ai_conversation_dossiers SET version_no=2 WHERE 1=0`, `DELETE FROM tm_ai_conversation_dossiers WHERE 1=0`, `INSERT INTO tm_ai_dossier_versions(theme_id,version_no,digest,canonical,compiled_at,created_at,updated_at) SELECT 0,0,'','{}',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) WHERE 1=0`} {
			if _, err = db.ExecContext(e.ctx, q); !mysqlErrorNumber(err, 1142) {
				t.Fatal("运行账号取得引用修改/删除或资料写入权限", err)
			}
		}
		if err = repository.NewAIConversationDossierSchemaVerifier(db).Verify(e.ctx); err == nil {
			t.Fatal("受限运行账号可代替准备探针")
		}
	})
	p.verifyNoGeneration(t)
	p.login.finishAs(t, e.ctx, false)
}
