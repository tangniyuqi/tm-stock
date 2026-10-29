-- 仅记录原预算调用的前后摘要；不写报告、不授权调用、不复制正文或密钥。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_report_call_audits (
 task_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,
 step_no int NOT NULL,stage varchar(7) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 owner varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,fence bigint NOT NULL,
 operation_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 phase varchar(6) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 task_no char(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 admission_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 profile_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 result_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 outcome varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id,step_no,phase),
 CONSTRAINT fk_tm_ai_report_call_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT chk_tm_ai_report_call_identity CHECK(user_id>0 AND fence>0
  AND CHAR_LENGTH(owner) BETWEEN 16 AND 64 AND owner REGEXP '^[A-Za-z0-9_-]+$'
  AND CHAR_LENGTH(operation_key) BETWEEN 16 AND 80 AND operation_key REGEXP '^[A-Za-z0-9_-]+$'
  AND task_no REGEXP '^[a-f0-9]{32}$' AND input_digest REGEXP '^[a-f0-9]{64}$'
  AND admission_digest REGEXP '^[a-f0-9]{64}$' AND profile_digest REGEXP '^[a-f0-9]{64}$'
  AND request_digest REGEXP '^[a-f0-9]{64}$' AND created_at>'1970-01-01' AND created_at=updated_at),
 CONSTRAINT chk_tm_ai_report_call_step CHECK((step_no=2 AND stage='extract') OR (step_no IN (3,4) AND stage='judge')),
 CONSTRAINT chk_tm_ai_report_call_phase CHECK((phase='start' AND outcome='started' AND result_digest IS NULL)
  OR (phase='finish' AND outcome IN ('returned','errored') AND result_digest IS NOT NULL AND result_digest REGEXP '^[a-f0-9]{64}$'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报告调用前后不可变摘要审计；不包含提示及模型原文';

DROP TRIGGER IF EXISTS tm_ai_report_call_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_report_call_insert BEFORE INSERT ON tm_ai_report_call_audits FOR EACH ROW
BEGIN
 DECLARE matched int;
 IF NEW.phase='start' THEN
  SELECT COUNT(*) INTO matched FROM tm_ai_tasks t
   JOIN tm_ai_task_reservations r ON r.task_id=t.id AND r.user_id=t.user_id AND r.seq=NEW.step_no
   JOIN tm_ai_task_budgets b ON b.task_id=t.id AND b.user_id=t.user_id
   JOIN tm_ai_task_admissions a ON a.task_id=t.id AND a.user_id=t.user_id
   WHERE t.id=NEW.task_id AND t.user_id=NEW.user_id AND t.task_no=NEW.task_no
    AND t.status=2 AND t.cancel_req=0 AND t.current_step=NEW.step_no-1
    AND t.fence=NEW.fence AND t.lease_owner=NEW.owner
    AND t.lease_until>UTC_TIMESTAMP(3) AND t.deadline_at>UTC_TIMESTAMP(3)
    AND r.reservation_key=NEW.operation_key AND r.input_digest=NEW.input_digest
    AND r.owner=NEW.owner AND r.fence=NEW.fence AND r.calls=1 AND r.pages=0
    AND b.reservation_count=NEW.step_no AND NEW.created_at>=r.created_at AND NEW.created_at<=UTC_TIMESTAMP(3)
    AND a.plan_digest=NEW.admission_digest AND t.admission_digest=a.plan_digest
    AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(a.payload USING utf8mb4),'$.Plan.ProfileDigest'))=NEW.profile_digest
    AND NOT EXISTS(SELECT 1 FROM tm_ai_task_step_results s WHERE s.task_id=t.id AND s.user_id=t.user_id AND s.step_no=NEW.step_no);
 ELSEIF NEW.phase='finish' THEN
  -- 原调用可能已过租约/来源撤回，仍允许按原start身份留证，不推进任务。
  SELECT COUNT(*) INTO matched FROM tm_ai_report_call_audits a
   WHERE a.task_id=NEW.task_id AND a.user_id=NEW.user_id AND a.step_no=NEW.step_no AND a.phase='start'
    AND a.stage=NEW.stage AND a.owner=NEW.owner AND a.fence=NEW.fence AND a.task_no=NEW.task_no
    AND a.operation_key=NEW.operation_key AND a.input_digest=NEW.input_digest
    AND a.admission_digest=NEW.admission_digest AND a.profile_digest=NEW.profile_digest
    AND a.request_digest=NEW.request_digest AND NEW.created_at>=a.created_at;
 ELSE
  SET matched=0;
 END IF;
 IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='报告调用审计原始绑定不符'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_report_call_no_update;
CREATE TRIGGER tm_ai_report_call_no_update BEFORE UPDATE ON tm_ai_report_call_audits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='报告调用审计不可修改';
DROP TRIGGER IF EXISTS tm_ai_report_call_no_delete;
CREATE TRIGGER tm_ai_report_call_no_delete BEFORE DELETE ON tm_ai_report_call_audits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='报告调用审计不可删除';
