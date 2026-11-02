-- 显式七步比较报告允许第5步比较与第6步语义；旧准入不得使用第6步。
-- 执行迁移前停止报告写入，完成结构及实际正反例核验后方可恢复。
SET NAMES utf8mb4;
ALTER TABLE tm_ai_report_call_audits DROP CHECK chk_tm_ai_report_call_step;
ALTER TABLE tm_ai_report_call_audits ADD CONSTRAINT chk_tm_ai_report_call_step CHECK((step_no=2 AND stage='extract') OR (step_no IN (3,4,5,6) AND stage='judge'));
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
    AND (NEW.step_no NOT IN(5,6)
     OR (NEW.step_no=5 AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(a.payload USING utf8mb4),'$.Plan.ReportProfile'))='dossier-report-v1')
     OR (t.analysis_type=2
      AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(a.payload USING utf8mb4),'$.Plan.ReportProfile'))='dossier-comparison-report-v1'
      AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(a.payload USING utf8mb4),'$.Plan.DossierComparisonDigest')) REGEXP '^[a-f0-9]{64}$'
      AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(a.payload USING utf8mb4),'$.Plan.DossierSupportDigest')) REGEXP '^[a-f0-9]{64}$'
      AND CAST(JSON_UNQUOTE(JSON_EXTRACT(t.budget_json,'$.max_calls')) AS UNSIGNED)>=7))
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
