-- 专题管理修订与不可变审计；不自动发布，不改变旧未管理许可。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_management_heads (
 kind varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 user_id bigint UNSIGNED NOT NULL, theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL,
 revision bigint UNSIGNED NOT NULL DEFAULT 0, request_id varchar(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(kind,user_id,theme_id,version_no),
 CONSTRAINT fk_tm_ai_dossier_management_head FOREIGN KEY(theme_id,version_no) REFERENCES tm_ai_dossier_versions(theme_id,version_no),
 CONSTRAINT chk_tm_ai_dossier_management_head CHECK((
 theme_id BETWEEN 1 AND 9007199254740991 AND version_no BETWEEN 1 AND 9007199254740991
 AND ((kind='publication' AND user_id=0) OR (kind='member_read' AND user_id BETWEEN 1 AND 9007199254740991))
 AND revision BETWEEN 0 AND 9007199254740991
 AND ((revision=0 AND request_id IS NULL) OR (revision>0 AND request_id REGEXP '^[A-Za-z0-9_-]{16,64}$'))
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at) IS TRUE)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tm_ai_dossier_management_events (
 request_id varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 actor_id bigint UNSIGNED NOT NULL, kind varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 user_id bigint UNSIGNED NOT NULL, theme_id bigint UNSIGNED NOT NULL, version_no bigint UNSIGNED NOT NULL,
 version_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, revision bigint UNSIGNED NOT NULL,
 canonical mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 content_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(request_id), UNIQUE KEY uq_tm_ai_dossier_management_revision(kind,user_id,theme_id,version_no,revision),
 CONSTRAINT fk_tm_ai_dossier_management_event FOREIGN KEY(theme_id,version_no,version_digest) REFERENCES tm_ai_dossier_versions(theme_id,version_no,digest),
 CONSTRAINT chk_tm_ai_dossier_management_event CHECK((
 request_id REGEXP '^[A-Za-z0-9_-]{16,64}$' AND actor_id BETWEEN 1 AND 9007199254740991
 AND theme_id BETWEEN 1 AND 9007199254740991 AND version_no BETWEEN 1 AND 9007199254740991
 AND ((kind='publication' AND user_id=0) OR (kind='member_read' AND user_id BETWEEN 1 AND 9007199254740991))
 AND version_digest REGEXP '^[a-f0-9]{64}$' AND revision BETWEEN 1 AND 9007199254740991
 AND OCTET_LENGTH(canonical) BETWEEN 2 AND 16384 AND JSON_VALID(canonical)
 AND JSON_TYPE(CAST(canonical AS JSON))='OBJECT' AND JSON_LENGTH(CAST(canonical AS JSON))=7
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.protocol'))='dossier-management-v1'
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.request.requestId'))=request_id
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.request.object.kind'))=kind
 AND JSON_EXTRACT(canonical,'$.request.object.userId')=user_id
 AND JSON_EXTRACT(canonical,'$.request.object.themeId')=theme_id
 AND JSON_EXTRACT(canonical,'$.request.object.version')=version_no
 AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.request.versionDigest'))=version_digest
 AND JSON_EXTRACT(canonical,'$.actorId')=actor_id AND JSON_EXTRACT(canonical,'$.revision')=revision
 AND JSON_EXTRACT(canonical,'$.request.expectedRevision')+1=revision
 AND content_hash REGEXP '^[a-f0-9]{64}$' AND SHA2(canonical,256)=content_hash
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.createdAt')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=created_at) IS TRUE)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

DROP TRIGGER IF EXISTS tm_ai_dossier_management_event_no_update;
CREATE TRIGGER tm_ai_dossier_management_event_no_update BEFORE UPDATE ON tm_ai_dossier_management_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题管理审计不可修改';
DROP TRIGGER IF EXISTS tm_ai_dossier_management_event_no_delete;
CREATE TRIGGER tm_ai_dossier_management_event_no_delete BEFORE DELETE ON tm_ai_dossier_management_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题管理审计不可删除';
DROP TRIGGER IF EXISTS tm_ai_dossier_management_head_no_delete;
CREATE TRIGGER tm_ai_dossier_management_head_no_delete BEFORE DELETE ON tm_ai_dossier_management_heads FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题管理修订不可删除';
DROP TRIGGER IF EXISTS tm_ai_dossier_management_head_insert;
DROP TRIGGER IF EXISTS tm_ai_dossier_management_head_update;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_management_head_insert BEFORE INSERT ON tm_ai_dossier_management_heads FOR EACH ROW
BEGIN
 IF NEW.revision<>0 OR NEW.request_id IS NOT NULL THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题管理从未接管状态开始';
 END IF;
END$$
CREATE TRIGGER tm_ai_dossier_management_head_update BEFORE UPDATE ON tm_ai_dossier_management_heads FOR EACH ROW
BEGIN
 IF NEW.kind<>OLD.kind OR NEW.user_id<>OLD.user_id OR NEW.theme_id<>OLD.theme_id OR NEW.version_no<>OLD.version_no
 OR NEW.created_at<>OLD.created_at OR NEW.revision<>OLD.revision+1 OR NEW.request_id IS NULL THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题管理修订必须连续';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM tm_ai_dossier_management_events e WHERE e.request_id=NEW.request_id AND e.kind=NEW.kind
 AND e.user_id=NEW.user_id AND e.theme_id=NEW.theme_id AND e.version_no=NEW.version_no AND e.revision=NEW.revision
 AND e.created_at=NEW.updated_at) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='专题管理缺少匹配审计';
 END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS tm_ai_dossier_managed_publication_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_managed_publication_insert BEFORE INSERT ON tm_ai_dossier_publications FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='publication' AND h.user_id=0 AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0) AND NOT EXISTS(
 SELECT 1 FROM tm_ai_dossier_management_heads h JOIN tm_ai_dossier_management_events e ON e.request_id=h.request_id
 AND e.kind=h.kind AND e.user_id=h.user_id AND e.theme_id=h.theme_id AND e.version_no=h.version_no AND e.revision=h.revision
 WHERE h.kind='publication' AND h.user_id=0 AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0 AND (JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.digest'))=NEW.digest
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.reference'))=NEW.approval_ref
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.enabled'))=IF(NEW.enabled=1,'true','false')
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.validUntil')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=NEW.valid_until AND e.created_at=NEW.updated_at
 AND NEW.approval_ref=CONCAT('dossier-managed:',e.request_id)) IS TRUE
 AND (JSON_TYPE(JSON_EXTRACT(e.canonical,'$.before'))='NULL') IS TRUE) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可须匹配审计前后像';
 END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS tm_ai_dossier_managed_publication_update;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_managed_publication_update BEFORE UPDATE ON tm_ai_dossier_publications FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='publication' AND h.user_id=0 AND h.theme_id=OLD.theme_id AND h.version_no=OLD.version_no AND h.revision>0) AND
 (NEW.theme_id<>OLD.theme_id OR NEW.version_no<>OLD.version_no OR NEW.created_at<>OLD.created_at) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可身份不可修改';
 END IF;
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='publication' AND h.user_id=0 AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0) AND NOT EXISTS(
 SELECT 1 FROM tm_ai_dossier_management_heads h JOIN tm_ai_dossier_management_events e ON e.request_id=h.request_id
 AND e.kind=h.kind AND e.user_id=h.user_id AND e.theme_id=h.theme_id AND e.version_no=h.version_no AND e.revision=h.revision
 WHERE h.kind='publication' AND h.user_id=0 AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0 AND (JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.digest'))=NEW.digest
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.reference'))=NEW.approval_ref
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.enabled'))=IF(NEW.enabled=1,'true','false')
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.validUntil')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=NEW.valid_until AND e.created_at=NEW.updated_at
 AND NEW.approval_ref=CONCAT('dossier-managed:',e.request_id)) IS TRUE
 AND (JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.digest'))=OLD.digest
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.reference'))=OLD.approval_ref
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.enabled'))=IF(OLD.enabled=1,'true','false')
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.validUntil')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=OLD.valid_until) IS TRUE) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可须匹配审计前后像';
 END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS tm_ai_dossier_managed_publication_delete;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_managed_publication_delete BEFORE DELETE ON tm_ai_dossier_publications FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='publication' AND h.user_id=0 AND h.theme_id=OLD.theme_id AND h.version_no=OLD.version_no AND h.revision>0) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可仅能留痕撤回';
 END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS tm_ai_dossier_managed_grant_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_managed_grant_insert BEFORE INSERT ON tm_ai_dossier_grants FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='member_read' AND h.user_id=NEW.user_id AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0) AND NOT EXISTS(
 SELECT 1 FROM tm_ai_dossier_management_heads h JOIN tm_ai_dossier_management_events e ON e.request_id=h.request_id
 AND e.kind=h.kind AND e.user_id=h.user_id AND e.theme_id=h.theme_id AND e.version_no=h.version_no AND e.revision=h.revision
 WHERE h.kind='member_read' AND h.user_id=NEW.user_id AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0 AND (JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.digest'))=NEW.digest
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.reference'))=NEW.approval_ref
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.enabled'))=IF(NEW.enabled=1,'true','false')
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.validUntil')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=NEW.valid_until AND e.created_at=NEW.updated_at
 AND NEW.approval_ref=CONCAT('dossier-managed:',e.request_id)) IS TRUE
 AND (JSON_TYPE(JSON_EXTRACT(e.canonical,'$.before'))='NULL') IS TRUE) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可须匹配审计前后像';
 END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS tm_ai_dossier_managed_grant_update;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_managed_grant_update BEFORE UPDATE ON tm_ai_dossier_grants FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='member_read' AND h.user_id=OLD.user_id AND h.theme_id=OLD.theme_id AND h.version_no=OLD.version_no AND h.revision>0) AND
 (NEW.theme_id<>OLD.theme_id OR NEW.version_no<>OLD.version_no OR NEW.user_id<>OLD.user_id OR NEW.created_at<>OLD.created_at) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可身份不可修改';
 END IF;
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='member_read' AND h.user_id=NEW.user_id AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0) AND NOT EXISTS(
 SELECT 1 FROM tm_ai_dossier_management_heads h JOIN tm_ai_dossier_management_events e ON e.request_id=h.request_id
 AND e.kind=h.kind AND e.user_id=h.user_id AND e.theme_id=h.theme_id AND e.version_no=h.version_no AND e.revision=h.revision
 WHERE h.kind='member_read' AND h.user_id=NEW.user_id AND h.theme_id=NEW.theme_id AND h.version_no=NEW.version_no AND h.revision>0 AND (JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.digest'))=NEW.digest
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.reference'))=NEW.approval_ref
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.enabled'))=IF(NEW.enabled=1,'true','false')
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.after.validUntil')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=NEW.valid_until AND e.created_at=NEW.updated_at
 AND NEW.approval_ref=CONCAT('dossier-managed:',e.request_id)) IS TRUE
 AND (JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.digest'))=OLD.digest
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.reference'))=OLD.approval_ref
 AND JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.enabled'))=IF(OLD.enabled=1,'true','false')
 AND TIMESTAMPADD(MICROSECOND,CAST(JSON_UNQUOTE(JSON_EXTRACT(e.canonical,'$.before.validUntil')) AS SIGNED)*1000,'1970-01-01 00:00:00.000')=OLD.valid_until) IS TRUE) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可须匹配审计前后像';
 END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS tm_ai_dossier_managed_grant_delete;
DELIMITER $$
CREATE TRIGGER tm_ai_dossier_managed_grant_delete BEFORE DELETE ON tm_ai_dossier_grants FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM tm_ai_dossier_management_heads h WHERE h.kind='member_read' AND h.user_id=OLD.user_id AND h.theme_id=OLD.theme_id AND h.version_no=OLD.version_no AND h.revision>0) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='已管理许可仅能留痕撤回';
 END IF;
END$$
DELIMITER ;
