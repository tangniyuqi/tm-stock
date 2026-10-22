-- 依赖20261021_create_tm_ai_conversations.sql；文件排序前先完成会话迁移。
-- 单独列出生成意图，不改原任务、准入或积分表；原任务键由服务端生成。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_conversation_tasks (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,conversation_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,
 idem_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,req_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 task_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 analysis_type varchar(5) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,subject_type varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 subject_id bigint UNSIGNED NOT NULL,price_ver varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,price_points bigint NOT NULL,
 state varchar(11) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,rejection_code varchar(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 version bigint NOT NULL,task_id bigint UNSIGNED NULL,message_seq bigint NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(id),UNIQUE KEY uq_tm_ai_intent_key(conversation_id,idem_key),UNIQUE KEY uq_tm_ai_intent_task_key(user_id,task_key),
 UNIQUE KEY uq_tm_ai_intent_task(task_id),UNIQUE KEY uq_tm_ai_intent_message(conversation_id,message_seq),
 KEY idx_tm_ai_intent_history(user_id,conversation_id,id),KEY idx_tm_ai_intent_archive(conversation_id,state,id),
 CONSTRAINT fk_tm_ai_intent_owner FOREIGN KEY(conversation_id,user_id) REFERENCES tm_ai_conversations(id,user_id),
 CONSTRAINT fk_tm_ai_intent_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT fk_tm_ai_intent_message FOREIGN KEY(conversation_id,message_seq) REFERENCES tm_ai_conversation_messages(conversation_id,seq),
 CONSTRAINT chk_tm_ai_intent CHECK(user_id BETWEEN 1 AND 9007199254740991 AND conversation_id BETWEEN 1 AND 9007199254740991
  AND idem_key REGEXP '^[A-Za-z0-9_-]{16,80}$' AND task_key REGEXP '^[A-Za-z0-9_-]{16,80}$' AND req_hash REGEXP '^[a-f0-9]{64}$'
  AND analysis_type IN('light','deep') AND subject_type IN('theme','topic') AND subject_id BETWEEN 1 AND 9007199254740991
  AND price_ver REGEXP '^[A-Za-z0-9_.:-]{1,32}$' AND price_points BETWEEN 1 AND 9007199254740991
  AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at
  AND ((state='prepared' AND version=1 AND task_id IS NULL AND message_seq IS NULL AND rejection_code='' AND created_at=updated_at)
   OR (state='unconfirmed' AND version=2 AND task_id IS NULL AND message_seq IS NULL AND rejection_code='')
   OR (state='linked' AND version=3 AND task_id IS NOT NULL AND task_id BETWEEN 1 AND 9007199254740991
    AND message_seq IS NOT NULL AND message_seq BETWEEN 1 AND 9007199254740990 AND rejection_code='')
   OR (state='not_created' AND version=3 AND task_id IS NULL AND message_seq IS NULL
    AND rejection_code IN('price_changed','insufficient_points','creation_disabled','admission_unavailable','invalid_request'))))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='会话生成意图、单次创建权限和原任务引用';

DROP TRIGGER IF EXISTS tm_ai_intent_insert;
DROP TRIGGER IF EXISTS tm_ai_intent_update;
DROP TRIGGER IF EXISTS tm_ai_intent_no_delete;
DROP TRIGGER IF EXISTS tm_ai_conversation_intent_archive_guard;
DELIMITER $$
CREATE TRIGGER tm_ai_intent_insert BEFORE INSERT ON tm_ai_conversation_tasks FOR EACH ROW
BEGIN
 DECLARE archived_value tinyint DEFAULT NULL;
 SELECT archived INTO archived_value FROM tm_ai_conversations WHERE id=NEW.conversation_id AND user_id=NEW.user_id FOR SHARE;
 IF archived_value IS NULL OR archived_value<>0 OR NEW.id>9007199254740991 OR NEW.state<>'prepared' OR NEW.version<>1
 OR NEW.task_id IS NOT NULL OR NEW.message_seq IS NOT NULL OR NEW.rejection_code<>'' OR NEW.created_at<>NEW.updated_at
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Intent requires owned active conversation and prepared state'; END IF;
END$$
CREATE TRIGGER tm_ai_intent_update BEFORE UPDATE ON tm_ai_conversation_tasks FOR EACH ROW
BEGIN
 DECLARE matched int DEFAULT 0;
 DECLARE archived_value tinyint DEFAULT NULL;
 IF NOT(NEW.id=OLD.id AND NEW.conversation_id=OLD.conversation_id AND NEW.user_id=OLD.user_id
  AND NEW.idem_key=OLD.idem_key AND NEW.req_hash=OLD.req_hash AND NEW.task_key=OLD.task_key AND NEW.analysis_type=OLD.analysis_type
  AND NEW.subject_type=OLD.subject_type AND NEW.subject_id=OLD.subject_id AND NEW.price_ver=OLD.price_ver AND NEW.price_points=OLD.price_points
  AND NEW.created_at=OLD.created_at AND NEW.updated_at>=OLD.updated_at
  AND ((NEW.state=OLD.state AND NEW.version=OLD.version AND NEW.rejection_code=OLD.rejection_code AND (NEW.task_id<=>OLD.task_id) AND (NEW.message_seq<=>OLD.message_seq) AND NEW.updated_at=OLD.updated_at)
   OR (OLD.state='prepared' AND NEW.state='unconfirmed' AND NEW.version=2)
   OR (OLD.state='unconfirmed' AND NEW.state IN('linked','not_created') AND NEW.version=3)))
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Intent identity and submission permission are immutable'; END IF;
 IF OLD.state='prepared' AND NEW.state='unconfirmed' THEN
  SELECT archived INTO archived_value FROM tm_ai_conversations WHERE id=NEW.conversation_id AND user_id=NEW.user_id FOR SHARE;
  IF archived_value IS NULL OR archived_value<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Archived conversation cannot claim task creation'; END IF;
 END IF;
 IF NEW.state='linked' THEN
  SELECT COUNT(*) INTO matched FROM tm_ai_tasks t JOIN tm_ai_conversation_messages m ON m.conversation_id=NEW.conversation_id AND m.seq=NEW.message_seq
  WHERE t.id=NEW.task_id AND t.user_id=NEW.user_id AND t.idem_key=NEW.task_key AND t.subject_type=NEW.subject_type AND t.subject_id=NEW.subject_id
  AND t.price_ver=NEW.price_ver AND t.price_points=NEW.price_points AND t.analysis_type=IF(NEW.analysis_type='light',1,2)
  AND m.user_id=NEW.user_id AND m.task_id=t.id AND m.kind='task' AND m.skill=NEW.analysis_type AND m.subject_type=NEW.subject_type
  AND m.subject_id=NEW.subject_id AND m.idem_key=NEW.task_key AND m.question='';
  IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Intent must bind original owned task and exact message'; END IF;
 END IF;
 IF NEW.state='not_created' THEN
  SELECT COUNT(*) INTO matched FROM tm_ai_tasks WHERE user_id=NEW.user_id AND idem_key=NEW.task_key;
  IF matched<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Existing task cannot be marked not created'; END IF;
 END IF;
END$$
CREATE TRIGGER tm_ai_conversation_intent_archive_guard BEFORE UPDATE ON tm_ai_conversations FOR EACH ROW
BEGIN
 DECLARE pending_id bigint DEFAULT NULL;
 DECLARE CONTINUE HANDLER FOR NOT FOUND SET pending_id=NULL;
 IF OLD.archived=0 AND NEW.archived=1 THEN
  SELECT id INTO pending_id FROM tm_ai_conversation_tasks WHERE conversation_id=OLD.id AND state='unconfirmed' ORDER BY id LIMIT 1 FOR SHARE;
  IF pending_id IS NOT NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Conversation has unconfirmed task intents'; END IF;
 END IF;
END$$
DELIMITER ;
CREATE TRIGGER tm_ai_intent_no_delete BEFORE DELETE ON tm_ai_conversation_tasks FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Intent submission history cannot be deleted';
