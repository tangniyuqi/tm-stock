-- 内部报告终检提交：成功才有报告，拒绝只留不可变收据；不启用公开读取或真实执行。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_report_commits (
 task_id bigint UNSIGNED NOT NULL,user_id bigint UNSIGNED NOT NULL,
 task_no char(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 step_no int NOT NULL,fence bigint NOT NULL,event_seq bigint NOT NULL,terminal_txn_id bigint UNSIGNED NOT NULL,
 operation_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 content_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 policy_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 status tinyint NOT NULL,entailment varchar(9) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 semantic varchar(9) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,reason varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload mediumblob NULL,created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id),
 CONSTRAINT fk_tm_ai_report_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT fk_tm_ai_report_step FOREIGN KEY(task_id,step_no) REFERENCES tm_ai_task_steps(task_id,step_no),
 CONSTRAINT fk_tm_ai_report_event FOREIGN KEY(task_id,event_seq) REFERENCES tm_ai_task_events(task_id,seq),
 CONSTRAINT fk_tm_ai_report_txn FOREIGN KEY(terminal_txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT chk_tm_ai_report CHECK(user_id>0 AND step_no BETWEEN 1 AND 64 AND fence>0 AND event_seq>1
  AND task_no REGEXP '^[a-f0-9]{32}$' AND CHAR_LENGTH(operation_key) BETWEEN 16 AND 80 AND operation_key REGEXP '^[A-Za-z0-9_-]+$'
  AND input_digest REGEXP '^[a-f0-9]{64}$' AND content_digest REGEXP '^[a-f0-9]{64}$' AND policy_digest REGEXP '^[a-f0-9]{64}$'
  AND entailment IN ('pass','block','uncertain') AND semantic IN ('pass','block','uncertain') AND created_at=updated_at
  AND ((status=3 AND reason='' AND entailment='pass' AND semantic='pass' AND payload IS NOT NULL
   AND OCTET_LENGTH(payload) BETWEEN 2 AND 262144 AND JSON_VALID(CONVERT(payload USING utf8mb4))
   AND JSON_TYPE(CONVERT(payload USING utf8mb4))='OBJECT' AND content_digest=SHA2(payload,256))
   OR (status=4 AND payload IS NULL AND (reason IN ('structure','evidence')
    OR (reason='entailment' AND entailment<>'pass' AND semantic='uncertain') OR (reason='semantic' AND entailment='pass' AND semantic<>'pass')))))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变内部终检收据；成功才含报告，拒绝不存候选正文';

DROP TRIGGER IF EXISTS tm_ai_report_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_report_insert BEFORE INSERT ON tm_ai_report_commits FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_tasks t
 JOIN tm_ai_task_steps s ON s.task_id=t.id AND s.user_id=t.user_id AND s.step_no=NEW.step_no
 JOIN tm_ai_task_reservations r ON r.task_id=s.task_id AND r.seq=s.reservation_seq AND r.user_id=s.user_id
 JOIN tm_ai_task_events e ON e.task_id=t.id AND e.seq=t.event_seq
 JOIN tm_points_holds h ON h.id=t.hold_id AND h.user_id=t.user_id
 JOIN tm_points_txns p ON p.id=h.terminal_txn_id AND p.user_id=t.user_id
 WHERE t.id=NEW.task_id AND t.user_id=NEW.user_id AND t.task_no=NEW.task_no AND t.status=NEW.status
  AND t.current_step=NEW.step_no AND t.fence=NEW.fence AND t.event_seq=NEW.event_seq AND t.cancel_req=0
  AND t.analysis_type IN (1,2) AND t.subject_type='theme'
  AND s.stage='judge' AND s.policy_digest=NEW.policy_digest AND s.output_digest=NEW.content_digest AND s.input_digest=NEW.input_digest
  AND r.reservation_key=NEW.operation_key AND e.status=NEW.status AND e.step_no=NEW.step_no AND NEW.terminal_txn_id=p.id
  AND h.amount=t.price_points AND h.price_ver=t.price_ver AND p.biz_type=h.biz_type AND p.biz_id=h.biz_id
  AND ((NEW.status=3 AND t.error_code='' AND e.type='succeeded' AND e.error_code='' AND h.state=2 AND p.kind='settle')
   OR (NEW.status=4 AND t.error_code='guard_rejected' AND e.type='failed' AND e.error_code='guard_rejected' AND h.state=3 AND p.kind='release'));
 IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI report task, checkpoint, event or settlement mismatch'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_report_no_update;
CREATE TRIGGER tm_ai_report_no_update BEFORE UPDATE ON tm_ai_report_commits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI report commits are immutable';
DROP TRIGGER IF EXISTS tm_ai_report_no_delete;
CREATE TRIGGER tm_ai_report_no_delete BEFORE DELETE ON tm_ai_report_commits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='AI report commits cannot be deleted';
