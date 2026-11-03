-- 显式题材资料请求；旧股票记录保留原身份，不自动填充或升级。
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS tm_ai_agent_reference_scope_migrate;
DELIMITER $$
CREATE PROCEDURE tm_ai_agent_reference_scope_migrate()
BEGIN
 DECLARE reference_columns int;
 SELECT COUNT(*) INTO reference_columns FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_agent_runs' AND column_name IN('member_id','subject_type','subject_id','materials_digest');
 IF reference_columns=0 THEN
  ALTER TABLE tm_ai_agent_runs
   ADD COLUMN member_id bigint UNSIGNED NOT NULL DEFAULT 0 AFTER symbol,
   ADD COLUMN subject_type varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER member_id,
   ADD COLUMN subject_id bigint UNSIGNED NOT NULL DEFAULT 0 AFTER subject_type,
   ADD COLUMN materials_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER subject_id;
 ELSEIF reference_columns<>4 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Agent reference scope migration is incomplete';
 END IF;
 SELECT COUNT(*) INTO reference_columns FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_agent_runs' AND is_nullable='NO' AND
  ((column_name IN('member_id','subject_id') AND data_type='bigint' AND column_type LIKE '%unsigned' AND column_default='0') OR
   (column_name='subject_type' AND data_type='varchar' AND character_maximum_length=8 AND collation_name='ascii_bin' AND column_default='') OR
   (column_name='materials_digest' AND data_type='char' AND character_maximum_length=64 AND collation_name='ascii_bin' AND column_default=''));
 IF reference_columns<>4 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Agent reference scope columns are invalid';
 END IF;
 -- 同一原子 DDL 替换约束，不能在两条 DDL 之间留下无 CHECK 的表。
 ALTER TABLE tm_ai_agent_runs DROP CHECK chk_tm_ai_agent_run,
 ADD CONSTRAINT chk_tm_ai_agent_run CHECK (
  actor_id > 0 AND request_id REGEXP '^[A-Za-z0-9][A-Za-z0-9_-]{7,63}$' AND
  ((symbol REGEXP '^[0-9]{6}$' AND member_id=0 AND subject_type='' AND subject_id=0 AND materials_digest='') OR
   (symbol='' AND member_id BETWEEN 1 AND 9007199254740991 AND CAST(subject_type AS BINARY) IN(CAST('theme' AS BINARY),CAST('topic' AS BINARY)) AND subject_id BETWEEN 1 AND 9007199254740991 AND materials_digest REGEXP '^[a-f0-9]{64}$')) AND
  event_seq > 0 AND cancel_requested IN(0,1) AND
  ((state IN('submitting','submission_unknown') AND remote_id IS NULL AND cancel_requested=0 AND
    ((state='submitting' AND error_code='') OR (state='submission_unknown' AND error_code='submission_unknown'))) OR
   (state IN('accepted','running','suspended','upstream_completed','upstream_failed','upstream_cancelled','result_quarantined') AND
    remote_id IS NOT NULL AND remote_id REGEXP '^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$' AND
    (error_code='' OR (cancel_requested=1 AND error_code IN('cancel_pending','cancel_unknown'))))) AND
  ((state='result_quarantined' AND result_digest IS NOT NULL AND result_digest REGEXP '^[a-f0-9]{64}$' AND
    result_payload IS NOT NULL AND OCTET_LENGTH(result_payload) BETWEEN 1 AND 4194304 AND JSON_VALID(CONVERT(result_payload USING utf8mb4))) OR
   (state<>'result_quarantined' AND result_digest IS NULL AND result_payload IS NULL))
 );
END$$
DELIMITER ;
CALL tm_ai_agent_reference_scope_migrate();
DROP PROCEDURE tm_ai_agent_reference_scope_migrate;

DROP TRIGGER IF EXISTS tm_ai_agent_reference_scope_no_update;
DELIMITER $$
CREATE TRIGGER tm_ai_agent_reference_scope_no_update BEFORE UPDATE ON tm_ai_agent_runs
FOR EACH ROW
BEGIN
 IF NEW.member_id<>OLD.member_id OR CAST(NEW.subject_type AS BINARY)<>CAST(OLD.subject_type AS BINARY) OR NEW.subject_id<>OLD.subject_id OR CAST(NEW.materials_digest AS BINARY)<>CAST(OLD.materials_digest AS BINARY) OR
    (OLD.subject_type<>'' AND (CAST(NEW.symbol AS BINARY)<>CAST(OLD.symbol AS BINARY) OR CAST(NEW.request_id AS BINARY)<>CAST(OLD.request_id AS BINARY) OR NEW.actor_id<>OLD.actor_id OR CAST(NEW.profile_digest AS BINARY)<>CAST(OLD.profile_digest AS BINARY))) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Agent reference scope is immutable';
 END IF;
END$$
DELIMITER ;
