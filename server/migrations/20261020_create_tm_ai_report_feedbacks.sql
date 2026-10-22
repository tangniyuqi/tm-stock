-- 报告反馈独立于生成/发布/账务；无正文、自由文本或联系方式，无删除入口。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_report_feedbacks (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 task_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,
 category varchar(7) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 report_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 status varchar(10) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 resolution varchar(14) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 version bigint NOT NULL,created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(id),UNIQUE KEY uq_tm_ai_feedback_category(user_id,task_id,category),
 CONSTRAINT fk_tm_ai_feedback_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 -- MySQL不允许CHECK引用自增列；生成后的id由事务内领域校验，超界即回滚。
 CONSTRAINT chk_tm_ai_feedback CHECK(task_id BETWEEN 1 AND 9007199254740991 AND user_id>0
  AND category IN ('fact','source','display','other') AND report_digest REGEXP '^[a-f0-9]{64}$' AND updated_at>=created_at
  AND ((status='received' AND version=1 AND resolution='') OR (status='processing' AND version=2 AND resolution='')
   OR (status='resolved' AND version=3 AND resolution IN ('reviewed','referred'))
   OR (status='dismissed' AND version=3 AND resolution IN ('unconfirmed','not_applicable'))))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报告反馈唯一受理及当前状态';
CREATE TABLE IF NOT EXISTS tm_ai_report_feedback_events (
 feedback_id bigint UNSIGNED NOT NULL,seq bigint NOT NULL,actor_id bigint UNSIGNED NOT NULL,
 status varchar(10) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 resolution varchar(14) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(feedback_id,seq),
 CONSTRAINT fk_tm_ai_feedback_event FOREIGN KEY(feedback_id) REFERENCES tm_ai_report_feedbacks(id),
 CONSTRAINT chk_tm_ai_feedback_event CHECK(created_at=updated_at AND
  ((seq=1 AND status='received' AND resolution='' AND actor_id=0)
   OR (seq=2 AND status='processing' AND resolution='' AND actor_id>0)
   OR (seq=3 AND status='resolved' AND resolution IN ('reviewed','referred') AND actor_id>0)
   OR (seq=3 AND status='dismissed' AND resolution IN ('unconfirmed','not_applicable') AND actor_id>0)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变报告反馈处理历史';
DROP TRIGGER IF EXISTS tm_ai_feedback_insert;
DROP TRIGGER IF EXISTS tm_ai_feedback_update;
DROP TRIGGER IF EXISTS tm_ai_feedback_event_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_feedback_insert BEFORE INSERT ON tm_ai_report_feedbacks FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_report_commits c JOIN tm_ai_tasks t ON t.id=c.task_id AND t.user_id=c.user_id
 WHERE c.task_id=NEW.task_id AND c.user_id=NEW.user_id AND c.status=3 AND t.status=3 AND c.content_digest=NEW.report_digest;
 IF NEW.id>9007199254740991 OR matched<>1 OR NEW.status<>'received' OR NEW.version<>1 OR NEW.resolution<>'' OR NEW.created_at<>NEW.updated_at
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Feedback requires owned successful report and initial state'; END IF;
END$$
CREATE TRIGGER tm_ai_feedback_update BEFORE UPDATE ON tm_ai_report_feedbacks FOR EACH ROW
BEGIN
 IF NOT(NEW.id=OLD.id AND NEW.task_id=OLD.task_id AND NEW.user_id=OLD.user_id AND NEW.category=OLD.category
 AND NEW.report_digest=OLD.report_digest AND NEW.created_at=OLD.created_at AND NEW.version=OLD.version+1 AND NEW.updated_at>=OLD.updated_at
 AND ((OLD.status='received' AND NEW.status='processing') OR (OLD.status='processing' AND NEW.status IN ('resolved','dismissed'))))
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Feedback identity or transition is immutable'; END IF;
END$$
CREATE TRIGGER tm_ai_feedback_event_insert BEFORE INSERT ON tm_ai_report_feedback_events FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_report_feedbacks f WHERE f.id=NEW.feedback_id AND f.version=NEW.seq
 AND f.status=NEW.status AND f.resolution=NEW.resolution AND f.updated_at=NEW.created_at;
 IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Feedback event must match current decision'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_feedback_no_delete;
CREATE TRIGGER tm_ai_feedback_no_delete BEFORE DELETE ON tm_ai_report_feedbacks FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Feedback cannot be deleted';
DROP TRIGGER IF EXISTS tm_ai_feedback_event_no_update;
CREATE TRIGGER tm_ai_feedback_event_no_update BEFORE UPDATE ON tm_ai_report_feedback_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Feedback events are immutable';
DROP TRIGGER IF EXISTS tm_ai_feedback_event_no_delete;
CREATE TRIGGER tm_ai_feedback_event_no_delete BEFORE DELETE ON tm_ai_report_feedback_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Feedback events cannot be deleted';
