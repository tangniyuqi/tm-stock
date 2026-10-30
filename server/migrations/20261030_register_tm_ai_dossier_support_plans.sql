-- 核验重建清单与独立AI处理用途；不启用模型、会员读取或结果发布。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_support_plans (
 component_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL,
 version_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 analysis_type tinyint NOT NULL,
 manifest_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 canonical mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 compiled_at datetime(3) NOT NULL, created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(component_digest), KEY idx_tm_ai_dossier_support_version(theme_id,version_no),
 CONSTRAINT fk_tm_ai_dossier_support_version FOREIGN KEY(theme_id,version_no,version_digest)
  REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest),
 CONSTRAINT chk_tm_ai_dossier_support_plan CHECK(theme_id BETWEEN 1 AND 9007199254740991
  AND version_no BETWEEN 1 AND 9007199254740991 AND analysis_type IN(1,2)
  AND component_digest REGEXP '^[a-f0-9]{64}$' AND version_digest REGEXP '^[a-f0-9]{64}$'
  AND manifest_digest REGEXP '^[a-f0-9]{64}$' AND OCTET_LENGTH(canonical) BETWEEN 2 AND 16384
  AND JSON_VALID(canonical) AND JSON_TYPE(CAST(canonical AS JSON))='OBJECT'
  AND JSON_CONTAINS_PATH(canonical,'all','$.protocol','$.analysisType','$.draft','$.draftDigest','$.themeName','$.keys','$.config','$.componentDigest')=1
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.protocol'))='STRING'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.componentDigest'))='STRING'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.draftDigest'))='STRING'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.analysisType'))='INTEGER'
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.analysisType'))=CAST(analysis_type AS CHAR)
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.draft'))='OBJECT'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.keys'))='ARRAY'
  AND JSON_LENGTH(JSON_EXTRACT(canonical,'$.keys')) BETWEEN 1 AND 4
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.config'))='OBJECT'
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.protocol'))='dossier-support-manifest-v1'
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.componentDigest'))=component_digest
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.draftDigest'))=version_digest
  AND compiled_at>='1970-01-01 00:00:00.001' AND created_at>=compiled_at AND updated_at=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_support_uses (
 component_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 approval_ref varchar(128) NOT NULL, enabled tinyint NOT NULL DEFAULT 0,
 valid_until datetime(3) NOT NULL, created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(component_digest),
 CONSTRAINT fk_tm_ai_dossier_support_use FOREIGN KEY(component_digest) REFERENCES tm_ai_dossier_support_plans(component_digest),
 CONSTRAINT chk_tm_ai_dossier_support_use CHECK(enabled IN(0,1) AND CHAR_LENGTH(TRIM(approval_ref)) BETWEEN 1 AND 128
  AND valid_until>created_at AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
DROP TRIGGER IF EXISTS tm_ai_dossier_support_no_update;
CREATE TRIGGER tm_ai_dossier_support_no_update BEFORE UPDATE ON tm_ai_dossier_support_plans FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='核验清单不可修改';
DROP TRIGGER IF EXISTS tm_ai_dossier_support_no_delete;
CREATE TRIGGER tm_ai_dossier_support_no_delete BEFORE DELETE ON tm_ai_dossier_support_plans FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='核验清单不可删除';
