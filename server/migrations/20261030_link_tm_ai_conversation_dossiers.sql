-- 本人会话的不可变档案引用；不改原消息、任务、报告及账本。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_conversation_dossiers (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 conversation_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,
 theme_id bigint UNSIGNED NOT NULL,version_no bigint UNSIGNED NOT NULL,compare_version bigint UNSIGNED NULL,
 compare_key bigint GENERATED ALWAYS AS (COALESCE(compare_version,0)) STORED,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(id),
 UNIQUE KEY uq_tm_ai_conversation_dossier_selection(conversation_id,theme_id,version_no,compare_key),
 KEY idx_tm_ai_conversation_dossier_owner(user_id,conversation_id,id),
 CONSTRAINT fk_tm_ai_conversation_dossier_owner FOREIGN KEY(conversation_id,user_id) REFERENCES tm_ai_conversations(id,user_id),
 CONSTRAINT fk_tm_ai_conversation_dossier_version FOREIGN KEY(theme_id,version_no) REFERENCES tm_ai_dossier_versions(theme_id,version_no),
 CONSTRAINT fk_tm_ai_conversation_dossier_compare FOREIGN KEY(theme_id,compare_version) REFERENCES tm_ai_dossier_versions(theme_id,version_no),
 CONSTRAINT chk_tm_ai_conversation_dossier CHECK(
  user_id BETWEEN 1 AND 9007199254740991 AND conversation_id BETWEEN 1 AND 9007199254740991
  AND theme_id BETWEEN 1 AND 9007199254740991 AND version_no BETWEEN 1 AND 9007199254740991
  AND (compare_version IS NULL OR compare_version>version_no AND compare_version<=9007199254740991)
  AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='本人会话保存的版本引用，不存资料正文或授予读取权';

DROP TRIGGER IF EXISTS tm_ai_conversation_dossier_insert;
DROP TRIGGER IF EXISTS tm_ai_conversation_dossier_no_update;
DROP TRIGGER IF EXISTS tm_ai_conversation_dossier_no_delete;
DELIMITER $$
CREATE TRIGGER tm_ai_conversation_dossier_insert BEFORE INSERT ON tm_ai_conversation_dossiers FOR EACH ROW
BEGIN
 DECLARE actual_archived tinyint DEFAULT NULL;
 SELECT archived INTO actual_archived FROM tm_ai_conversations
 WHERE id=NEW.conversation_id AND user_id=NEW.user_id FOR SHARE;
 IF actual_archived IS NULL OR actual_archived<>0 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Dossier reference requires active owned conversation';
 END IF;
END$$
DELIMITER ;
CREATE TRIGGER tm_ai_conversation_dossier_no_update BEFORE UPDATE ON tm_ai_conversation_dossiers FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation dossier references are immutable';
CREATE TRIGGER tm_ai_conversation_dossier_no_delete BEFORE DELETE ON tm_ai_conversation_dossiers FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation dossier references cannot be deleted';
