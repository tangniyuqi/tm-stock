-- 外部 Agent 内部执行登记，独立于 L0 和会员计费；不对公众发布上游正文。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `tm_ai_agent_profiles` (
  `profile_digest` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `max_submissions` int UNSIGNED NOT NULL,
  `reserved_submissions` int UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`profile_digest`),
  CONSTRAINT `chk_tm_ai_agent_profile` CHECK (
    `profile_digest` REGEXP '^[a-f0-9]{64}$' AND `max_submissions` BETWEEN 1 AND 1000 AND `reserved_submissions` <= `max_submissions`
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='批准配置的持久提交限额，不是模型调用或金额上限';

CREATE TABLE IF NOT EXISTS `tm_ai_agent_runs` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `request_id` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actor_id` bigint UNSIGNED NOT NULL,
  `symbol` char(6) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `profile_digest` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `remote_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
  `state` varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'submitting',
  `cancel_requested` tinyint UNSIGNED NOT NULL DEFAULT 0,
  `error_code` varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `result_digest` char(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
  `result_payload` mediumblob DEFAULT NULL COMMENT '脱敏后的不可信上游 JSON，仅隔离保存，不经公开或内部查询工具返回',
  `event_seq` bigint UNSIGNED NOT NULL DEFAULT 1,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_agent_request` (`request_id`),
  UNIQUE KEY `uk_tm_ai_agent_remote` (`profile_digest`, `remote_id`),
  KEY `idx_tm_ai_agent_actor` (`actor_id`, `id`),
  CONSTRAINT `fk_tm_ai_agent_profile` FOREIGN KEY (`profile_digest`) REFERENCES `tm_ai_agent_profiles` (`profile_digest`),
  CONSTRAINT `chk_tm_ai_agent_run` CHECK (
    `actor_id` > 0 AND `request_id` REGEXP '^[A-Za-z0-9][A-Za-z0-9_-]{7,63}$' AND `symbol` REGEXP '^[0-9]{6}$' AND
    `event_seq` > 0 AND `cancel_requested` IN (0,1) AND
    ((`state` IN ('submitting','submission_unknown') AND `remote_id` IS NULL AND `cancel_requested`=0 AND
      ((`state`='submitting' AND `error_code`='') OR (`state`='submission_unknown' AND `error_code`='submission_unknown'))) OR
     (`state` IN ('accepted','running','suspended','upstream_completed','upstream_failed','upstream_cancelled','result_quarantined') AND
      `remote_id` IS NOT NULL AND `remote_id` REGEXP '^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$' AND
      (`error_code`='' OR (`cancel_requested`=1 AND `error_code` IN ('cancel_pending','cancel_unknown'))))) AND
    ((`state`='result_quarantined' AND `result_digest` IS NOT NULL AND `result_digest` REGEXP '^[a-f0-9]{64}$' AND
      `result_payload` IS NOT NULL AND OCTET_LENGTH(`result_payload`) BETWEEN 1 AND 4194304 AND JSON_VALID(CONVERT(`result_payload` USING utf8mb4))) OR
     (`state`<>'result_quarantined' AND `result_digest` IS NULL AND `result_payload` IS NULL))
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='外部执行不确定时不重提；上游完成不代表报告合规或允许结算';

CREATE TABLE IF NOT EXISTS `tm_ai_agent_events` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` bigint UNSIGNED NOT NULL,
  `seq` bigint UNSIGNED NOT NULL,
  `event` varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `record_json` json NOT NULL COMMENT '仅状态、身份与摘要，不保存凭据和上游错误原文',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_agent_event_seq` (`run_id`,`seq`),
  CONSTRAINT `fk_tm_ai_agent_event_run` FOREIGN KEY (`run_id`) REFERENCES `tm_ai_agent_runs` (`id`),
  CONSTRAINT `chk_tm_ai_agent_event` CHECK (`seq`>0 AND `event` IN ('submit_reserved','submitted','submission_unknown','observed','cancel_requested','cancel_acknowledged','cancel_unknown','quarantined'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='外部执行不可变事件；预登记与状态变更同事务';

DROP TRIGGER IF EXISTS `tm_ai_agent_events_no_update`;
CREATE TRIGGER `tm_ai_agent_events_no_update` BEFORE UPDATE ON `tm_ai_agent_events`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Agent events are immutable';
DROP TRIGGER IF EXISTS `tm_ai_agent_events_no_delete`;
CREATE TRIGGER `tm_ai_agent_events_no_delete` BEFORE DELETE ON `tm_ai_agent_events`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Agent events are immutable';
DROP TRIGGER IF EXISTS `tm_ai_agent_runs_no_delete`;
CREATE TRIGGER `tm_ai_agent_runs_no_delete` BEFORE DELETE ON `tm_ai_agent_runs`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Agent runs cannot be deleted';
