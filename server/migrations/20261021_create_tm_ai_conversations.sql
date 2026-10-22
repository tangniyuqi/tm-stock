-- 私有会话底座；不启用问答/模型/执行/计费，也不迁移协作者的旧会话表。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_conversations (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,user_id bigint UNSIGNED NOT NULL,
 idem_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 req_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 title varchar(40) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 archived tinyint NOT NULL DEFAULT 0,version bigint NOT NULL,last_seq bigint NOT NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(id),UNIQUE KEY uq_tm_ai_conversation_owner(id,user_id),
 UNIQUE KEY uq_tm_ai_conversation_key(user_id,idem_key),
 KEY idx_tm_ai_conversation_list(user_id,archived,id),
 CONSTRAINT chk_tm_ai_conversation CHECK(user_id BETWEEN 1 AND 9007199254740991
  AND idem_key REGEXP '^[A-Za-z0-9_-]{16,80}$' AND req_hash REGEXP '^[a-f0-9]{64}$'
  AND CHAR_LENGTH(title) BETWEEN 1 AND 40 AND title=TRIM(title) AND NOT REGEXP_LIKE(title,'[[:cntrl:]]')
  AND archived IN(0,1) AND version BETWEEN 1 AND 9007199254740991
  AND last_seq BETWEEN 0 AND 9007199254740990 AND version>last_seq AND (archived=0 OR version>=2)
  AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='本人私有会话与单调历史序号';

CREATE TABLE IF NOT EXISTS tm_ai_conversation_messages (
 conversation_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,seq bigint NOT NULL,
 idem_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 req_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 kind varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 subject_type varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,subject_id bigint UNSIGNED NOT NULL,
 skill varchar(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 question varchar(1000) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 task_id bigint UNSIGNED NULL,created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(conversation_id,seq),UNIQUE KEY uq_tm_ai_conversation_message_key(conversation_id,idem_key),
 CONSTRAINT fk_tm_ai_conversation_message_owner FOREIGN KEY(conversation_id,user_id) REFERENCES tm_ai_conversations(id,user_id),
 CONSTRAINT fk_tm_ai_conversation_message_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT chk_tm_ai_conversation_message CHECK(user_id BETWEEN 1 AND 9007199254740991
  AND conversation_id BETWEEN 1 AND 9007199254740991 AND seq BETWEEN 1 AND 9007199254740990
  AND idem_key REGEXP '^[A-Za-z0-9_-]{16,80}$' AND req_hash REGEXP '^[a-f0-9]{64}$'
  AND subject_type IN('theme','topic') AND subject_id BETWEEN 1 AND 9007199254740991
  AND created_at>='1970-01-01 00:00:00.001' AND updated_at=created_at
  AND ((kind='task' AND skill IN('light','deep') AND question='' AND task_id IS NOT NULL AND task_id BETWEEN 1 AND 9007199254740991)
   OR (kind='question' AND CHAR_LENGTH(question) BETWEEN 1 AND 1000 AND OCTET_LENGTH(question)<=4000
    AND question=TRIM(question) AND NOT REGEXP_LIKE(question,'[[:cntrl:]]')
    AND ((skill='material_qa' AND task_id IS NULL) OR (skill='report_qa' AND task_id IS NOT NULL AND task_id BETWEEN 1 AND 9007199254740991)))))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='不可变用户输入和任务引用，不存模型答案正文';

DROP TRIGGER IF EXISTS tm_ai_conversation_insert;
DROP TRIGGER IF EXISTS tm_ai_conversation_update;
DROP TRIGGER IF EXISTS tm_ai_conversation_message_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_conversation_insert BEFORE INSERT ON tm_ai_conversations FOR EACH ROW
BEGIN
 IF NEW.id>9007199254740991 OR NEW.version<>1 OR NEW.last_seq<>0 OR NEW.archived<>0 OR NEW.created_at<>NEW.updated_at
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation must start with empty active state'; END IF;
END$$
CREATE TRIGGER tm_ai_conversation_update BEFORE UPDATE ON tm_ai_conversations FOR EACH ROW
BEGIN
 IF NOT(NEW.id=OLD.id AND NEW.user_id=OLD.user_id AND NEW.idem_key=OLD.idem_key AND NEW.req_hash=OLD.req_hash AND NEW.created_at=OLD.created_at
  AND ((NEW.title=OLD.title AND NEW.archived=OLD.archived AND NEW.version=OLD.version AND NEW.last_seq=OLD.last_seq AND NEW.updated_at=OLD.updated_at)
   OR (OLD.archived=0 AND NEW.version=OLD.version+1 AND NEW.updated_at>=OLD.updated_at
    AND ((NEW.title<>OLD.title AND NEW.archived=0 AND NEW.last_seq=OLD.last_seq)
     OR (NEW.title=OLD.title AND NEW.archived=1 AND NEW.last_seq=OLD.last_seq)
     OR (NEW.title=OLD.title AND NEW.archived=0 AND NEW.last_seq=OLD.last_seq+1)))))
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation identity or version transition is immutable'; END IF;
END$$
CREATE TRIGGER tm_ai_conversation_message_insert BEFORE INSERT ON tm_ai_conversation_messages FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_conversations c WHERE c.id=NEW.conversation_id AND c.user_id=NEW.user_id
 AND c.archived=0 AND c.last_seq=NEW.seq AND c.updated_at=NEW.created_at;
 IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Message requires owned current conversation sequence'; END IF;
 IF NEW.task_id IS NOT NULL THEN
  SELECT COUNT(*) INTO matched FROM tm_ai_tasks t WHERE t.id=NEW.task_id AND t.user_id=NEW.user_id
  AND t.subject_type=NEW.subject_type AND t.subject_id=NEW.subject_id
  AND ((NEW.kind='task' AND ((NEW.skill='light' AND t.analysis_type=1) OR (NEW.skill='deep' AND t.analysis_type=2)))
   OR (NEW.kind='question' AND NEW.skill='report_qa' AND t.status=3));
  IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Message task owner and scope must match'; END IF;
 END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_conversation_no_delete;
CREATE TRIGGER tm_ai_conversation_no_delete BEFORE DELETE ON tm_ai_conversations FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation is archived, not deleted';
DROP TRIGGER IF EXISTS tm_ai_conversation_message_no_update;
CREATE TRIGGER tm_ai_conversation_message_no_update BEFORE UPDATE ON tm_ai_conversation_messages FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation messages are immutable';
DROP TRIGGER IF EXISTS tm_ai_conversation_message_no_delete;
CREATE TRIGGER tm_ai_conversation_message_no_delete BEFORE DELETE ON tm_ai_conversation_messages FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation messages cannot be deleted';
