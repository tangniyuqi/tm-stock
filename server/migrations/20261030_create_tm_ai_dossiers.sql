-- 专题候选版本与用途许可：不自动发布，不启用路由或模型。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_versions (
 theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL,
 digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 canonical mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 compiled_at datetime(3) NOT NULL, created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(theme_id,version_no), UNIQUE KEY uq_tm_ai_dossier_digest(theme_id,version_no,digest),
 CONSTRAINT chk_tm_ai_dossier_version CHECK(theme_id BETWEEN 1 AND 9007199254740991 AND version_no BETWEEN 1 AND 9007199254740991
 AND digest REGEXP '^[a-f0-9]{64}$' AND OCTET_LENGTH(canonical) BETWEEN 2 AND 262144 AND JSON_VALID(canonical)
 AND JSON_TYPE(CAST(canonical AS JSON))='OBJECT' AND compiled_at>='1970-01-01 00:00:00.001'
 AND created_at>=compiled_at AND updated_at=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_evidence (
 theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL, snapshot_id bigint UNSIGNED NOT NULL,
 source_id bigint UNSIGNED NOT NULL, content_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(theme_id,version_no,snapshot_id),
 CONSTRAINT fk_tm_ai_dossier_evidence_version FOREIGN KEY(theme_id,version_no) REFERENCES tm_ai_dossier_versions(theme_id,version_no),
 CONSTRAINT fk_tm_ai_dossier_evidence_snapshot FOREIGN KEY(snapshot_id) REFERENCES tm_ai_source_snapshots(id),
 CONSTRAINT fk_tm_ai_dossier_evidence_source FOREIGN KEY(source_id) REFERENCES tm_ai_sources(id),
 CONSTRAINT chk_tm_ai_dossier_evidence CHECK(snapshot_id BETWEEN 1 AND 9007199254740991 AND source_id BETWEEN 1 AND 9007199254740991
 AND content_hash REGEXP '^[a-f0-9]{64}$' AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_publications (
 theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL,
 digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, approval_ref varchar(128) NOT NULL,
 enabled tinyint NOT NULL DEFAULT 0, valid_until datetime(3) NOT NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(theme_id,version_no),
 CONSTRAINT fk_tm_ai_dossier_publication FOREIGN KEY(theme_id,version_no,digest) REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest),
 CONSTRAINT chk_tm_ai_dossier_publication CHECK(enabled IN(0,1) AND CHAR_LENGTH(TRIM(approval_ref)) BETWEEN 1 AND 128
 AND valid_until>created_at AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_grants (
 user_id bigint UNSIGNED NOT NULL, theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL,
 digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, approval_ref varchar(128) NOT NULL,
 enabled tinyint NOT NULL DEFAULT 0, valid_until datetime(3) NOT NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(user_id,theme_id,version_no),
 CONSTRAINT fk_tm_ai_dossier_grant FOREIGN KEY(theme_id,version_no,digest) REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest),
 CONSTRAINT chk_tm_ai_dossier_grant CHECK(user_id BETWEEN 1 AND 9007199254740991 AND enabled IN(0,1)
 AND CHAR_LENGTH(TRIM(approval_ref)) BETWEEN 1 AND 128 AND valid_until>created_at AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
DROP TRIGGER IF EXISTS tm_ai_dossier_versions_no_update;
CREATE TRIGGER tm_ai_dossier_versions_no_update BEFORE UPDATE ON tm_ai_dossier_versions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题版本不可修改';
DROP TRIGGER IF EXISTS tm_ai_dossier_versions_no_delete;
CREATE TRIGGER tm_ai_dossier_versions_no_delete BEFORE DELETE ON tm_ai_dossier_versions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题版本不可删除';
DROP TRIGGER IF EXISTS tm_ai_dossier_evidence_no_update;
CREATE TRIGGER tm_ai_dossier_evidence_no_update BEFORE UPDATE ON tm_ai_dossier_evidence FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题依据不可修改';
DROP TRIGGER IF EXISTS tm_ai_dossier_evidence_no_delete;
CREATE TRIGGER tm_ai_dossier_evidence_no_delete BEFORE DELETE ON tm_ai_dossier_evidence FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题依据不可删除';
