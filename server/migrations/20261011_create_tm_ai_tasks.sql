-- 持久化任务与进度事件底座。先执行积分账本迁移，不启用公开 AI 或收费。
-- 不复用 L0 同步抽取运行记录；后续步骤、调用日志、报告另做增量迁移。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_tasks (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id bigint UNSIGNED NOT NULL,
 task_no char(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 idem_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 req_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 analysis_type tinyint NOT NULL,
 subject_type varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 subject_id bigint UNSIGNED NOT NULL,
 hold_id bigint UNSIGNED NOT NULL,
 price_ver varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 price_points bigint NOT NULL,
 budget_json json NOT NULL,
 status tinyint NOT NULL,
 attempt int NOT NULL DEFAULT 0,
 current_step int NOT NULL DEFAULT 0,
 not_before datetime(3) NOT NULL,
 deadline_at datetime(3) NOT NULL,
 lease_owner varchar(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 lease_until datetime(3) NULL,
 fence bigint NOT NULL DEFAULT 0,
 event_seq bigint NOT NULL,
 cancel_req tinyint NOT NULL DEFAULT 0,
 error_code varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
 created_at datetime(3) NOT NULL,
 updated_at datetime(3) NOT NULL,
 PRIMARY KEY(id), UNIQUE KEY uk_tm_ai_task_owner(id,user_id),
 UNIQUE KEY uk_tm_ai_task_no(task_no), UNIQUE KEY uk_tm_ai_task_idem(user_id,idem_key),
 UNIQUE KEY uk_tm_ai_task_hold(hold_id),
 KEY idx_tm_ai_task_history(user_id,id),
 KEY idx_tm_ai_task_claim(status,not_before,lease_until,id),
 KEY idx_tm_ai_task_deadline(status,deadline_at,id),
 CONSTRAINT fk_tm_ai_task_hold FOREIGN KEY(hold_id,user_id) REFERENCES tm_points_holds(id,user_id),
 CONSTRAINT chk_tm_ai_task CHECK (
  user_id>0 AND subject_id>0 AND subject_type IN ('theme','topic') AND analysis_type IN (1,2) AND price_points>0
  AND task_no REGEXP '^[a-f0-9]{32}$' AND req_hash REGEXP '^[a-f0-9]{64}$'
  AND CHAR_LENGTH(idem_key) BETWEEN 16 AND 80 AND idem_key REGEXP '^[A-Za-z0-9_-]+$'
  AND CHAR_LENGTH(price_ver) BETWEEN 1 AND 32 AND price_ver REGEXP '^[A-Za-z0-9_.:-]+$'
  AND status IN (1,2,3,4,5,6) AND attempt>=0 AND current_step>=0 AND fence>=0 AND event_seq>=1 AND cancel_req IN (0,1)
  AND deadline_at>created_at
  AND ((status=2 AND lease_owner IS NOT NULL AND CHAR_LENGTH(lease_owner) BETWEEN 16 AND 64 AND lease_owner REGEXP '^[A-Za-z0-9_-]+$' AND lease_until IS NOT NULL AND lease_until<=deadline_at AND fence>0 AND attempt>0)
    OR (status<>2 AND lease_owner IS NULL AND lease_until IS NULL))
  AND ((status IN (1,2,3) AND error_code='') OR (status=4 AND error_code IN ('provider_unavailable','invalid_output','guard_rejected','budget_exhausted','attempts_exhausted','internal_failure'))
    OR (status=5 AND error_code='cancelled') OR (status=6 AND error_code='expired'))
 ),
 CONSTRAINT chk_tm_ai_task_budget CHECK (
  JSON_LENGTH(budget_json)=5
  AND JSON_CONTAINS_PATH(budget_json,'all','$.max_attempts','$.max_calls','$.max_tokens','$.max_pages','$.max_duration_ms')=1
  AND JSON_TYPE(JSON_EXTRACT(budget_json,'$.max_attempts'))='INTEGER'
  AND JSON_TYPE(JSON_EXTRACT(budget_json,'$.max_calls'))='INTEGER'
  AND JSON_TYPE(JSON_EXTRACT(budget_json,'$.max_tokens'))='INTEGER'
  AND JSON_TYPE(JSON_EXTRACT(budget_json,'$.max_pages'))='INTEGER'
  AND JSON_TYPE(JSON_EXTRACT(budget_json,'$.max_duration_ms'))='INTEGER'
  AND JSON_EXTRACT(budget_json,'$.max_attempts') BETWEEN 1 AND 100
  AND JSON_EXTRACT(budget_json,'$.max_calls')>0 AND JSON_EXTRACT(budget_json,'$.max_tokens')>0 AND JSON_EXTRACT(budget_json,'$.max_pages')>0
  AND JSON_EXTRACT(budget_json,'$.max_duration_ms') BETWEEN 1000 AND 86400000
 )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='内部持久化 AI 任务；尚无公开入口';

CREATE TABLE IF NOT EXISTS tm_ai_task_events (
 task_id bigint UNSIGNED NOT NULL,
 seq bigint NOT NULL,
 type varchar(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 status tinyint NOT NULL,
 step_no int NOT NULL,
 error_code varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
 created_at datetime(3) NOT NULL,
 updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id,seq),
 CONSTRAINT fk_tm_ai_event_task FOREIGN KEY(task_id) REFERENCES tm_ai_tasks(id),
 CONSTRAINT chk_tm_ai_task_event CHECK (seq>=1 AND step_no>=0 AND status IN (1,2,3,4,5,6)
  AND type IN ('created','claimed','cancel_requested','retried','failed','expired','cancelled','step_completed','succeeded')
  AND ((type IN ('created','retried') AND status=1) OR (type IN ('claimed','cancel_requested','step_completed') AND status=2)
    OR (type='succeeded' AND status=3) OR (type='failed' AND status=4) OR (type='cancelled' AND status=5) OR (type='expired' AND status=6))
  AND ((status IN (1,2,3) AND error_code='') OR (status=4 AND error_code IN ('provider_unavailable','invalid_output','guard_rejected','budget_exhausted','attempts_exhausted','internal_failure'))
    OR (status=5 AND error_code='cancelled') OR (status=6 AND error_code='expired')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变任务进度事件；无自由正文';

DROP TRIGGER IF EXISTS tm_ai_tasks_identity;
DELIMITER $$
CREATE TRIGGER tm_ai_tasks_identity BEFORE UPDATE ON tm_ai_tasks FOR EACH ROW
BEGIN
 IF OLD.status IN (3,4,5,6) OR NOT (OLD.id<=>NEW.id AND OLD.user_id<=>NEW.user_id AND OLD.task_no<=>NEW.task_no
  AND OLD.idem_key<=>NEW.idem_key AND OLD.req_hash<=>NEW.req_hash AND OLD.analysis_type<=>NEW.analysis_type AND OLD.subject_type<=>NEW.subject_type AND OLD.subject_id<=>NEW.subject_id
  AND OLD.hold_id<=>NEW.hold_id AND OLD.price_ver<=>NEW.price_ver AND OLD.price_points<=>NEW.price_points AND OLD.budget_json<=>NEW.budget_json
  AND OLD.deadline_at<=>NEW.deadline_at AND OLD.created_at<=>NEW.created_at)
  OR NEW.fence<OLD.fence OR NEW.attempt<OLD.attempt OR NEW.current_step<OLD.current_step OR NEW.event_seq<OLD.event_seq
  OR NEW.cancel_req<OLD.cancel_req THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI task identity or terminal is immutable';
 END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_tasks_no_delete;
CREATE TRIGGER tm_ai_tasks_no_delete BEFORE DELETE ON tm_ai_tasks FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI tasks cannot be deleted';
DROP TRIGGER IF EXISTS tm_ai_task_events_no_update;
CREATE TRIGGER tm_ai_task_events_no_update BEFORE UPDATE ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI task events are immutable';
DROP TRIGGER IF EXISTS tm_ai_task_events_no_delete;
CREATE TRIGGER tm_ai_task_events_no_delete BEFORE DELETE ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI task events cannot be deleted';
