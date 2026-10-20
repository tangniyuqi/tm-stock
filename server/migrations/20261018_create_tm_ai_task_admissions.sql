-- 持久准入与原冻结同事务；不补造历史批准、不启用执行。
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS tm_ai_admission_column;
DELIMITER $$
CREATE PROCEDURE tm_ai_admission_column()
BEGIN
 IF NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_tasks' AND column_name='admission_digest') THEN
  ALTER TABLE tm_ai_tasks ADD COLUMN admission_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NULL;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name='tm_ai_tasks' AND constraint_name='chk_tm_ai_task_admission') THEN
  ALTER TABLE tm_ai_tasks ADD CONSTRAINT chk_tm_ai_task_admission CHECK(admission_digest IS NULL OR (OCTET_LENGTH(admission_digest)=64 AND admission_digest REGEXP '^[a-f0-9]{64}$'));
 END IF;
END$$
DELIMITER ;
CALL tm_ai_admission_column();
DROP PROCEDURE tm_ai_admission_column;

DROP TRIGGER IF EXISTS tm_ai_task_admission_identity;
DELIMITER $$
CREATE TRIGGER tm_ai_task_admission_identity BEFORE UPDATE ON tm_ai_tasks FOR EACH ROW
BEGIN
 IF NOT (OLD.admission_digest<=>NEW.admission_digest) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI admission identity is immutable'; END IF;
END$$
DELIMITER ;

CREATE TABLE IF NOT EXISTS tm_ai_task_admissions (
 task_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,
 task_no char(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 plan_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload mediumblob NOT NULL,created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id),
 CONSTRAINT fk_tm_ai_admission_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT chk_tm_ai_admission CHECK(user_id>0 AND task_no REGEXP '^[a-f0-9]{32}$' AND request_hash REGEXP '^[a-f0-9]{64}$'
  AND plan_digest REGEXP '^[a-f0-9]{64}$' AND OCTET_LENGTH(payload) BETWEEN 2 AND 65536
  AND JSON_VALID(CONVERT(payload USING utf8mb4)) AND JSON_TYPE(CONVERT(payload USING utf8mb4))='OBJECT'
  AND JSON_LENGTH(CONVERT(payload USING utf8mb4))=2 AND JSON_CONTAINS_PATH(CONVERT(payload USING utf8mb4),'all','$.Version','$.Plan')=1
  AND JSON_TYPE(JSON_EXTRACT(CONVERT(payload USING utf8mb4),'$.Version'))='STRING' AND JSON_TYPE(JSON_EXTRACT(CONVERT(payload USING utf8mb4),'$.Plan'))='OBJECT'
  AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(payload USING utf8mb4),'$.Version'))='task-admission-v1'
  AND plan_digest=SHA2(payload,256) AND created_at=updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变首次准入计划；无来源正文与模型原文';
DROP TRIGGER IF EXISTS tm_ai_admission_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_admission_insert BEFORE INSERT ON tm_ai_task_admissions FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_tasks t
 JOIN tm_ai_task_events e ON e.task_id=t.id AND e.seq=1 AND e.type='created' AND e.status=1 AND e.step_no=0 AND e.error_code=''
 JOIN tm_points_holds h ON h.id=t.hold_id AND h.user_id=t.user_id
 JOIN tm_points_txns f ON f.id=h.freeze_txn_id AND f.user_id=t.user_id
 WHERE t.id=NEW.task_id AND t.user_id=NEW.user_id AND t.task_no=NEW.task_no AND t.req_hash=NEW.request_hash AND t.admission_digest=NEW.plan_digest
  AND t.status=1 AND t.attempt=0 AND t.current_step=0 AND t.fence=0 AND t.event_seq=1 AND t.cancel_req=0
  AND t.not_before=NEW.created_at AND h.state=1 AND h.captured=0 AND h.terminal_txn_id IS NULL
  AND h.biz_type='ai_task' AND h.biz_id=CONCAT('task:',t.task_no) AND h.amount=t.price_points AND h.price_ver=t.price_ver AND h.expire_at=t.deadline_at
  AND f.kind='freeze' AND f.biz_type=h.biz_type AND f.biz_id=h.biz_id
  AND t.user_id=JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Scope.UserID')
  AND t.subject_id=JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Scope.SubjectID')
  AND t.analysis_type=JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Scope.AnalysisType')
  AND t.subject_type=JSON_UNQUOTE(JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Scope.SubjectType'))
  AND t.price_points=JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Price.Points')
  AND t.price_ver=JSON_UNQUOTE(JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Price.Version'))
  AND t.budget_json=JSON_EXTRACT(CONVERT(NEW.payload USING utf8mb4),'$.Plan.Budget');
 IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI admission task or original freeze mismatch'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_admission_no_update;
CREATE TRIGGER tm_ai_admission_no_update BEFORE UPDATE ON tm_ai_task_admissions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI admissions are immutable';
DROP TRIGGER IF EXISTS tm_ai_admission_no_delete;
CREATE TRIGGER tm_ai_admission_no_delete BEFORE DELETE ON tm_ai_task_admissions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI admissions cannot be deleted';
