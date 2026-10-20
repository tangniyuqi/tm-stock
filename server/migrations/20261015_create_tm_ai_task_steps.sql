-- 须先执行任务与预算迁移；隔离响应不对外，阶段完成不等于任务成功或积分结算。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_task_step_results (
 task_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,reservation_seq int NOT NULL,step_no int NOT NULL,
 stage varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,owner varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,fence bigint NOT NULL,
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,output_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload mediumblob NOT NULL,created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id,reservation_seq),UNIQUE KEY uk_tm_ai_result_owner(task_id,reservation_seq,user_id),
 CONSTRAINT fk_tm_ai_result_reservation FOREIGN KEY(task_id,reservation_seq) REFERENCES tm_ai_task_reservations(task_id,seq),
 CONSTRAINT fk_tm_ai_result_budget FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_task_budgets(task_id,user_id),
 CONSTRAINT chk_tm_ai_step_result CHECK(user_id>0 AND reservation_seq BETWEEN 1 AND 1000 AND step_no BETWEEN 1 AND 64
  AND stage IN ('collect','extract','judge','agent_call') AND fence>0 AND CHAR_LENGTH(owner) BETWEEN 16 AND 64 AND owner REGEXP '^[A-Za-z0-9_-]+$'
  AND input_digest REGEXP '^[a-f0-9]{64}$' AND output_digest REGEXP '^[a-f0-9]{64}$' AND output_digest=SHA2(payload,256)
  AND OCTET_LENGTH(payload) BETWEEN 2 AND 262144 AND JSON_VALID(CONVERT(payload USING utf8mb4)) AND JSON_TYPE(CONVERT(payload USING utf8mb4))='OBJECT'
  AND created_at=updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变隔离阶段响应；不是公开证据或报告';
CREATE TABLE IF NOT EXISTS tm_ai_task_steps (
 task_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,step_no int NOT NULL,reservation_seq int NOT NULL,
 stage varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,owner varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,fence bigint NOT NULL,
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,output_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 policy_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,event_seq bigint NOT NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id,step_no),UNIQUE KEY uk_tm_ai_step_operation(task_id,reservation_seq),UNIQUE KEY uk_tm_ai_step_event(task_id,event_seq),
 CONSTRAINT fk_tm_ai_step_result FOREIGN KEY(task_id,reservation_seq,user_id) REFERENCES tm_ai_task_step_results(task_id,reservation_seq,user_id),
 CONSTRAINT chk_tm_ai_step CHECK(user_id>0 AND step_no BETWEEN 1 AND 64 AND reservation_seq BETWEEN 1 AND 1000 AND fence>0 AND event_seq>0
  AND stage IN ('collect','extract','judge','agent_call') AND CHAR_LENGTH(owner) BETWEEN 16 AND 64 AND owner REGEXP '^[A-Za-z0-9_-]+$'
  AND input_digest REGEXP '^[a-f0-9]{64}$' AND output_digest REGEXP '^[a-f0-9]{64}$' AND policy_digest REGEXP '^[a-f0-9]{64}$' AND created_at=updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变内部检查点；无报告正文或成功结算';

DROP TRIGGER IF EXISTS tm_ai_result_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_result_insert BEFORE INSERT ON tm_ai_task_step_results FOR EACH ROW
BEGIN
 DECLARE s int;DECLARE n int;DECLARE f bigint;DECLARE o varchar(64);DECLARE u datetime(3);DECLARE d datetime(3);DECLARE x int;
 DECLARE rf bigint;DECLARE ro varchar(64);DECLARE digest char(64);DECLARE c bigint;DECLARE p bigint;
 SELECT status,current_step,fence,lease_owner,lease_until,deadline_at,cancel_req INTO s,n,f,o,u,d,x FROM tm_ai_tasks WHERE id=NEW.task_id AND user_id=NEW.user_id FOR UPDATE;
 IF s IS NULL OR s<>2 OR NEW.step_no<>n+1 OR NEW.fence<>f OR CAST(NEW.owner AS BINARY)<>CAST(o AS BINARY) OR u<=UTC_TIMESTAMP(3) OR d<=UTC_TIMESTAMP(3) OR x<>0 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI result lease or step invalid';END IF;
 SELECT fence,owner,input_digest,calls,pages INTO rf,ro,digest,c,p FROM tm_ai_task_reservations WHERE task_id=NEW.task_id AND seq=NEW.reservation_seq AND user_id=NEW.user_id FOR UPDATE;
 IF rf IS NULL OR rf<>NEW.fence OR CAST(ro AS BINARY)<>CAST(NEW.owner AS BINARY) OR CAST(digest AS BINARY)<>CAST(NEW.input_digest AS BINARY)
  OR (NEW.stage='collect' AND p<=0) OR (NEW.stage<>'collect' AND c<=0) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI result reservation invalid';END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_result_no_update;
CREATE TRIGGER tm_ai_result_no_update BEFORE UPDATE ON tm_ai_task_step_results FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI result immutable';
DROP TRIGGER IF EXISTS tm_ai_result_no_delete;
CREATE TRIGGER tm_ai_result_no_delete BEFORE DELETE ON tm_ai_task_step_results FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI result cannot be deleted';
DROP TRIGGER IF EXISTS tm_ai_step_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_step_insert BEFORE INSERT ON tm_ai_task_steps FOR EACH ROW
BEGIN
 DECLARE s int;DECLARE n int;DECLARE f bigint;DECLARE e bigint;DECLARE o varchar(64);DECLARE u datetime(3);DECLARE d datetime(3);DECLARE x int;
 DECLARE rn int;DECLARE rf bigint;DECLARE rs varchar(16);DECLARE ri char(64);DECLARE rh char(64);
 SELECT status,current_step,fence,event_seq,lease_owner,lease_until,deadline_at,cancel_req INTO s,n,f,e,o,u,d,x FROM tm_ai_tasks WHERE id=NEW.task_id AND user_id=NEW.user_id FOR UPDATE;
 IF s IS NULL OR s<>2 OR NEW.step_no<>n+1 OR NEW.event_seq<>e+1 OR NEW.fence<>f OR CAST(NEW.owner AS BINARY)<>CAST(o AS BINARY) OR u<=UTC_TIMESTAMP(3) OR d<=UTC_TIMESTAMP(3) OR x<>0 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI checkpoint lease or sequence invalid';END IF;
 SELECT step_no,fence,stage,input_digest,output_digest INTO rn,rf,rs,ri,rh FROM tm_ai_task_step_results WHERE task_id=NEW.task_id AND reservation_seq=NEW.reservation_seq AND user_id=NEW.user_id FOR UPDATE;
 IF rn IS NULL OR rn<>NEW.step_no OR rf>NEW.fence OR CAST(rs AS BINARY)<>CAST(NEW.stage AS BINARY) OR CAST(ri AS BINARY)<>CAST(NEW.input_digest AS BINARY) OR CAST(rh AS BINARY)<>CAST(NEW.output_digest AS BINARY) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI checkpoint result mismatch';END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_step_account;
DELIMITER $$
CREATE TRIGGER tm_ai_step_account AFTER INSERT ON tm_ai_task_steps FOR EACH ROW
BEGIN
 UPDATE tm_ai_tasks SET current_step=NEW.step_no,event_seq=NEW.event_seq,updated_at=UTC_TIMESTAMP(3)
  WHERE id=NEW.task_id AND user_id=NEW.user_id AND status=2 AND current_step=NEW.step_no-1 AND event_seq=NEW.event_seq-1
  AND fence=NEW.fence AND lease_owner=NEW.owner AND lease_until>UTC_TIMESTAMP(3) AND deadline_at>UTC_TIMESTAMP(3) AND cancel_req=0;
 IF ROW_COUNT()<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI checkpoint progress mismatch';END IF;
 INSERT INTO tm_ai_task_events(task_id,seq,type,status,step_no,error_code,created_at,updated_at) VALUES(NEW.task_id,NEW.event_seq,'step_completed',2,NEW.step_no,'',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3));
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_step_no_update;
CREATE TRIGGER tm_ai_step_no_update BEFORE UPDATE ON tm_ai_task_steps FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI checkpoint immutable';
DROP TRIGGER IF EXISTS tm_ai_step_no_delete;
CREATE TRIGGER tm_ai_step_no_delete BEFORE DELETE ON tm_ai_task_steps FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI checkpoint cannot be deleted';
DROP TRIGGER IF EXISTS tm_ai_task_stage_init;
DELIMITER $$
CREATE TRIGGER tm_ai_task_stage_init BEFORE INSERT ON tm_ai_tasks FOR EACH ROW
BEGIN
 IF NEW.current_step>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI task must start at step zero';END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_task_stage_guard;
DELIMITER $$
CREATE TRIGGER tm_ai_task_stage_guard BEFORE UPDATE ON tm_ai_tasks FOR EACH ROW
BEGIN
 DECLARE f bigint;DECLARE e bigint;DECLARE o varchar(64);
 IF NEW.current_step<>OLD.current_step THEN
  IF NEW.current_step<>OLD.current_step+1 OR OLD.status<>2 OR NEW.status<>2 OR NEW.fence<>OLD.fence OR NOT(NEW.lease_owner<=>OLD.lease_owner) OR NEW.event_seq<>OLD.event_seq+1 OR OLD.cancel_req<>0 OR OLD.lease_until<=UTC_TIMESTAMP(3) OR OLD.deadline_at<=UTC_TIMESTAMP(3) THEN
   SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI task step transition invalid';END IF;
  SELECT fence,event_seq,owner INTO f,e,o FROM tm_ai_task_steps WHERE task_id=OLD.id AND user_id=OLD.user_id AND step_no=NEW.current_step;
  IF f IS NULL OR f<>NEW.fence OR e<>NEW.event_seq OR CAST(o AS BINARY)<>CAST(NEW.lease_owner AS BINARY) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI task step requires checkpoint';END IF;
 END IF;
END$$
DELIMITER ;
