-- 仅持久化调用前上界，不是实际用量或用户账单。须先执行任务/账本迁移。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_task_budgets (
 task_id bigint UNSIGNED NOT NULL, user_id bigint UNSIGNED NOT NULL,
 limit_calls bigint NOT NULL, limit_tokens bigint NOT NULL, limit_pages bigint NOT NULL,
 reserved_calls bigint NOT NULL DEFAULT 0, reserved_tokens bigint NOT NULL DEFAULT 0, reserved_pages bigint NOT NULL DEFAULT 0,
 reservation_count int NOT NULL DEFAULT 0,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id), UNIQUE KEY uk_tm_ai_budget_owner(task_id,user_id),
 CONSTRAINT fk_tm_ai_budget_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT chk_tm_ai_budget CHECK(user_id>0 AND limit_calls>0 AND limit_tokens>0 AND limit_pages>0
  AND reserved_calls BETWEEN 0 AND limit_calls AND reserved_tokens BETWEEN 0 AND limit_tokens AND reserved_pages BETWEEN 0 AND limit_pages
  AND reservation_count BETWEEN 0 AND 1000)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务保守预留计数器；不按实际用量退款';
CREATE TABLE IF NOT EXISTS tm_ai_task_reservations (
 task_id bigint UNSIGNED NOT NULL, user_id bigint UNSIGNED NOT NULL, seq int NOT NULL,
 reservation_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 owner varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, fence bigint NOT NULL,
 calls bigint NOT NULL, tokens bigint NOT NULL, pages bigint NOT NULL,
 created_at datetime(3) NOT NULL, updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id,seq), UNIQUE KEY uk_tm_ai_reservation_key(task_id,reservation_key),
 KEY idx_tm_ai_reservation_owner(task_id,user_id,seq),
 CONSTRAINT fk_tm_ai_reservation_budget FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_task_budgets(task_id,user_id),
 CONSTRAINT chk_tm_ai_reservation CHECK(user_id>0 AND seq BETWEEN 1 AND 1000 AND fence>0
  AND CHAR_LENGTH(reservation_key) BETWEEN 16 AND 80 AND reservation_key REGEXP '^[A-Za-z0-9_-]+$'
  AND input_digest REGEXP '^[a-f0-9]{64}$' AND CHAR_LENGTH(owner) BETWEEN 16 AND 64 AND owner REGEXP '^[A-Za-z0-9_-]+$'
  AND calls>=0 AND tokens>=0 AND pages>=0 AND (calls>0 OR pages>0)
  AND ((calls=0 AND tokens=0) OR (calls>0 AND tokens>=calls)) AND created_at=updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变调用前预留；无提示或模型原文';

DROP TRIGGER IF EXISTS tm_ai_budget_init;
DELIMITER $$
CREATE TRIGGER tm_ai_budget_init BEFORE INSERT ON tm_ai_task_budgets FOR EACH ROW
BEGIN
 DECLARE c bigint; DECLARE t bigint; DECLARE p bigint;
 SELECT JSON_EXTRACT(budget_json,'$.max_calls'),JSON_EXTRACT(budget_json,'$.max_tokens'),JSON_EXTRACT(budget_json,'$.max_pages') INTO c,t,p
  FROM tm_ai_tasks WHERE id=NEW.task_id AND user_id=NEW.user_id FOR UPDATE;
 IF c IS NULL OR NEW.limit_calls<>c OR NEW.limit_tokens<>t OR NEW.limit_pages<>p
  OR NEW.reserved_calls<>0 OR NEW.reserved_tokens<>0 OR NEW.reserved_pages<>0 OR NEW.reservation_count<>0 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI budget initialization invalid';
 END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_budget_update;
DELIMITER $$
CREATE TRIGGER tm_ai_budget_update BEFORE UPDATE ON tm_ai_task_budgets FOR EACH ROW
BEGIN
 DECLARE c bigint; DECLARE t bigint; DECLARE p bigint;
 IF NOT(OLD.task_id<=>NEW.task_id AND OLD.user_id<=>NEW.user_id AND OLD.limit_calls<=>NEW.limit_calls
  AND OLD.limit_tokens<=>NEW.limit_tokens AND OLD.limit_pages<=>NEW.limit_pages AND OLD.created_at<=>NEW.created_at)
  OR NEW.reservation_count<>OLD.reservation_count+1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI budget identity immutable';
 END IF;
 SELECT calls,tokens,pages INTO c,t,p FROM tm_ai_task_reservations WHERE task_id=OLD.task_id AND seq=NEW.reservation_count;
 IF c IS NULL OR NEW.reserved_calls-OLD.reserved_calls<>c OR NEW.reserved_tokens-OLD.reserved_tokens<>t OR NEW.reserved_pages-OLD.reserved_pages<>p THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI budget requires matching reservation';
 END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_budget_no_delete;
CREATE TRIGGER tm_ai_budget_no_delete BEFORE DELETE ON tm_ai_task_budgets FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI budget cannot be deleted';
DROP TRIGGER IF EXISTS tm_ai_reservation_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_reservation_insert BEFORE INSERT ON tm_ai_task_reservations FOR EACH ROW
BEGIN
 DECLARE s int; DECLARE f bigint; DECLARE o varchar(64); DECLARE u datetime(3); DECLARE d datetime(3); DECLARE x int; DECLARE n int;
 SELECT status,fence,lease_owner,lease_until,deadline_at,cancel_req INTO s,f,o,u,d,x FROM tm_ai_tasks WHERE id=NEW.task_id AND user_id=NEW.user_id FOR UPDATE;
 IF s IS NULL OR s<>2 OR f<>NEW.fence OR BINARY o<>BINARY NEW.owner OR u<=UTC_TIMESTAMP(3) OR d<=UTC_TIMESTAMP(3) OR x<>0 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI reservation lease invalid';
 END IF;
 SELECT reservation_count INTO n FROM tm_ai_task_budgets WHERE task_id=NEW.task_id AND user_id=NEW.user_id FOR UPDATE;
 IF n IS NULL OR NEW.seq<>n+1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI reservation sequence invalid'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_reservation_account;
DELIMITER $$
CREATE TRIGGER tm_ai_reservation_account AFTER INSERT ON tm_ai_task_reservations FOR EACH ROW
BEGIN
 UPDATE tm_ai_task_budgets SET reserved_calls=reserved_calls+NEW.calls,reserved_tokens=reserved_tokens+NEW.tokens,reserved_pages=reserved_pages+NEW.pages,
  reservation_count=reservation_count+1,updated_at=UTC_TIMESTAMP(3)
  WHERE task_id=NEW.task_id AND user_id=NEW.user_id AND reservation_count=NEW.seq-1
  AND NEW.calls<=limit_calls-reserved_calls AND NEW.tokens<=limit_tokens-reserved_tokens AND NEW.pages<=limit_pages-reserved_pages;
 IF ROW_COUNT()<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI reservation exceeds budget'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_reservation_no_update;
CREATE TRIGGER tm_ai_reservation_no_update BEFORE UPDATE ON tm_ai_task_reservations FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI reservation immutable';
DROP TRIGGER IF EXISTS tm_ai_reservation_no_delete;
CREATE TRIGGER tm_ai_reservation_no_delete BEFORE DELETE ON tm_ai_task_reservations FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI reservation cannot be deleted';
