-- 已执行旧报告迁移的数据库也须升级；不重建表、不修改历史收据或账本。
-- 部署时先停止报告写入，执行后完成结构/正反例自检再恢复；失败期间不得开放报告写入。
SET NAMES utf8mb4;
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
