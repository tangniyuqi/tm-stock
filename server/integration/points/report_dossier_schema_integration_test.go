//go:build integration

package points_test

import (
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func TestIntegrationAIReportDossierSchema(t *testing.T) {
	e, db := setupPointsLedger(t)
	reportDDL(t, e, "20261010_create_ai_source_snapshots.sql")
	reportDDL(t, e, "20261030_create_tm_ai_dossiers.sql")
	v := repository.NewAIDossierSchemaVerifier(db)
	check := func(t *testing.T, want bool) {
		t.Helper()
		err := v.Verify(e.ctx)
		if (err == nil) != want {
			t.Fatalf("结构检查结果不符: %v", err)
		}
		for _, table := range []string{"tm_ai_sources", "tm_ai_source_bodies", "tm_ai_source_snapshots", "tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_points_accounts"} {
			if e.queryInt("SELECT COUNT(*) FROM "+table) != 0 {
				t.Fatalf("检查留下写入: %s", table)
			}
		}
	}
	t.Run("空库实际探针回滚", func(t *testing.T) { check(t, true) })
	for _, tc := range []struct{ name, breakSQL, restoreSQL string }{
		{"发布默认开启", `ALTER TABLE tm_ai_dossier_publications ALTER COLUMN enabled SET DEFAULT 1`, `ALTER TABLE tm_ai_dossier_publications ALTER COLUMN enabled SET DEFAULT 0`},
		{"会员默认开启", `ALTER TABLE tm_ai_dossier_grants ALTER COLUMN enabled SET DEFAULT 1`, `ALTER TABLE tm_ai_dossier_grants ALTER COLUMN enabled SET DEFAULT 0`},
		{"正文列口径变化", `ALTER TABLE tm_ai_dossier_versions MODIFY canonical mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL`, `ALTER TABLE tm_ai_dossier_versions MODIFY canonical mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL`},
		{"同名版本检查失效", `ALTER TABLE tm_ai_dossier_versions DROP CHECK chk_tm_ai_dossier_version, ADD CONSTRAINT chk_tm_ai_dossier_version CHECK(theme_id>=0)`, `ALTER TABLE tm_ai_dossier_versions DROP CHECK chk_tm_ai_dossier_version, ADD CONSTRAINT chk_tm_ai_dossier_version CHECK(theme_id BETWEEN 1 AND 9007199254740991 AND version_no BETWEEN 1 AND 9007199254740991 AND digest REGEXP '^[a-f0-9]{64}$' AND OCTET_LENGTH(canonical) BETWEEN 2 AND 262144 AND JSON_VALID(canonical) AND JSON_TYPE(CAST(canonical AS JSON))='OBJECT' AND compiled_at>='1970-01-01 00:00:00.001' AND created_at>=compiled_at AND updated_at=created_at)`},
		{"删除批准外键", `ALTER TABLE tm_ai_dossier_publications DROP FOREIGN KEY fk_tm_ai_dossier_publication`, `ALTER TABLE tm_ai_dossier_publications ADD CONSTRAINT fk_tm_ai_dossier_publication FOREIGN KEY(theme_id,version_no,digest) REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest)`},
		{"约束不执行", `ALTER TABLE tm_ai_dossier_evidence ALTER CHECK chk_tm_ai_dossier_evidence NOT ENFORCED`, `ALTER TABLE tm_ai_dossier_evidence ALTER CHECK chk_tm_ai_dossier_evidence ENFORCED`},
		{"同名证据检查失效", `ALTER TABLE tm_ai_dossier_evidence DROP CHECK chk_tm_ai_dossier_evidence, ADD CONSTRAINT chk_tm_ai_dossier_evidence CHECK(snapshot_id>=0)`, `ALTER TABLE tm_ai_dossier_evidence DROP CHECK chk_tm_ai_dossier_evidence, ADD CONSTRAINT chk_tm_ai_dossier_evidence CHECK(snapshot_id BETWEEN 1 AND 9007199254740991 AND source_id BETWEEN 1 AND 9007199254740991 AND content_hash REGEXP '^[a-f0-9]{64}$' AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at)`},
		{"同名发布检查失效", `ALTER TABLE tm_ai_dossier_publications DROP CHECK chk_tm_ai_dossier_publication, ADD CONSTRAINT chk_tm_ai_dossier_publication CHECK(enabled>=0)`, `ALTER TABLE tm_ai_dossier_publications DROP CHECK chk_tm_ai_dossier_publication, ADD CONSTRAINT chk_tm_ai_dossier_publication CHECK(enabled IN(0,1) AND CHAR_LENGTH(TRIM(approval_ref)) BETWEEN 1 AND 128 AND valid_until>created_at AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)`},
		{"同名会员检查失效", `ALTER TABLE tm_ai_dossier_grants DROP CHECK chk_tm_ai_dossier_grant, ADD CONSTRAINT chk_tm_ai_dossier_grant CHECK(user_id>=0)`, `ALTER TABLE tm_ai_dossier_grants DROP CHECK chk_tm_ai_dossier_grant, ADD CONSTRAINT chk_tm_ai_dossier_grant CHECK(user_id BETWEEN 1 AND 9007199254740991 AND enabled IN(0,1) AND CHAR_LENGTH(TRIM(approval_ref)) BETWEEN 1 AND 128 AND valid_until>created_at AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e.mustExec(tc.breakSQL)
			check(t, false)
			e.mustExec(tc.restoreSQL)
			check(t, true)
		})
	}
	t.Run("同名触发器失效", func(t *testing.T) {
		e.mustExec(`DROP TRIGGER tm_ai_dossier_versions_no_update; CREATE TRIGGER tm_ai_dossier_versions_no_update BEFORE UPDATE ON tm_ai_dossier_versions FOR EACH ROW SET @dossier_test_noop=1`)
		check(t, false)
		reportDDL(t, e, "20261030_create_tm_ai_dossiers.sql")
		check(t, true)
	})
	t.Run("缺触发器", func(t *testing.T) {
		e.mustExec(`DROP TRIGGER tm_ai_dossier_evidence_no_delete`)
		check(t, false)
		reportDDL(t, e, "20261030_create_tm_ai_dossiers.sql")
		check(t, true)
	})
}
