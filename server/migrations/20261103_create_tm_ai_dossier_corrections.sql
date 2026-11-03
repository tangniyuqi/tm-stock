-- 更正声明审核只追加；旧版本、资料与发布许可不在本迁移中改写。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_correction_events (
 request_id varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 actor_id bigint UNSIGNED NOT NULL, theme_id bigint UNSIGNED NOT NULL,
 before_version bigint UNSIGNED NOT NULL, after_version bigint UNSIGNED NOT NULL,
 before_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 after_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 revision bigint UNSIGNED NOT NULL,
 canonical mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 content_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(request_id),
 UNIQUE KEY uq_tm_ai_dossier_correction_revision(theme_id,before_version,after_version,revision),
 CONSTRAINT fk_tm_ai_dossier_correction_before FOREIGN KEY(theme_id,before_version,before_digest) REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest),
 CONSTRAINT fk_tm_ai_dossier_correction_after FOREIGN KEY(theme_id,after_version,after_digest) REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest),
 CONSTRAINT chk_tm_ai_dossier_correction_event CHECK((
 request_id REGEXP '^[A-Za-z0-9_-]{16,64}$' AND actor_id BETWEEN 1 AND 9007199254740991
 AND theme_id BETWEEN 1 AND 9007199254740991 AND before_version BETWEEN 1 AND 9007199254740991
 AND after_version BETWEEN 1 AND 9007199254740991 AND before_version<after_version
 AND revision BETWEEN 1 AND 9007199254740991
 AND before_digest REGEXP '^[a-f0-9]{64}$' AND after_digest REGEXP '^[a-f0-9]{64}$'
 AND OCTET_LENGTH(canonical) BETWEEN 2 AND 32768 AND JSON_VALID(canonical)
 AND JSON_TYPE(CAST(canonical AS JSON))='OBJECT' AND JSON_LENGTH(CAST(canonical AS JSON))=6
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.protocol'))='dossier-explicit-correction-v1'
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.request.requestId'))=request_id
 AND JSON_EXTRACT(canonical,'$.actorId')=actor_id AND JSON_EXTRACT(canonical,'$.revision')=revision
 AND JSON_EXTRACT(canonical,'$.request.object.themeId')=theme_id
 AND JSON_EXTRACT(canonical,'$.request.object.beforeVersion')=before_version
 AND JSON_EXTRACT(canonical,'$.request.object.afterVersion')=after_version
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.request.beforeDigest'))=before_digest
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.request.afterDigest'))=after_digest
 AND JSON_EXTRACT(canonical,'$.request.expectedRevision')+1=revision
 AND content_hash REGEXP '^[a-f0-9]{64}$' AND SHA2(canonical,256)=content_hash
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.createdAt')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=created_at) IS TRUE)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
DROP TRIGGER IF EXISTS tm_ai_dossier_correction_no_update;
CREATE TRIGGER tm_ai_dossier_correction_no_update BEFORE UPDATE ON tm_ai_dossier_correction_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='更正声明审计不可修改';
DROP TRIGGER IF EXISTS tm_ai_dossier_correction_no_delete;
CREATE TRIGGER tm_ai_dossier_correction_no_delete BEFORE DELETE ON tm_ai_dossier_correction_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='更正声明审计不可删除';
