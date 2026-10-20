-- 内部 L0 同步抽取的持久化请求与不可变调用审计，不是公众计费任务表。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `tm_ai_l0_extraction_runs` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `request_id` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `snapshot_id` bigint UNSIGNED NOT NULL,
  `actor_id` bigint UNSIGNED NOT NULL,
  `provider` varchar(64) NOT NULL,
  `model` varchar(128) NOT NULL,
  `approval_ref` varchar(512) NOT NULL,
  `state` varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'running',
  `result_json` json DEFAULT NULL,
  `error_code` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `finished_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_l0_run_request` (`request_id`),
  KEY `idx_tm_ai_l0_run_state_time` (`state`, `created_at`, `id`),
  CONSTRAINT `fk_tm_ai_l0_run_snapshot` FOREIGN KEY (`snapshot_id`) REFERENCES `tm_ai_source_snapshots` (`id`),
  CONSTRAINT `chk_tm_ai_l0_run` CHECK (
    `actor_id` > 0 AND `provider` <> '' AND `model` <> '' AND `approval_ref` <> '' AND
    ((`state` = 'running' AND `finished_at` IS NULL AND `result_json` IS NULL AND `error_code` = '') OR
     (`state` = 'succeeded' AND `finished_at` IS NOT NULL AND `result_json` IS NOT NULL AND `error_code` = '') OR
     (`state` = 'failed' AND `finished_at` IS NOT NULL AND `result_json` IS NULL AND `error_code` <> ''))
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='内部抽取请求；不确定状态不自动重跑模型';

CREATE TABLE IF NOT EXISTS `tm_ai_l0_call_events` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` bigint UNSIGNED NOT NULL,
  `attempt` tinyint UNSIGNED NOT NULL,
  `phase` varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `payload` mediumblob NOT NULL COMMENT '已脱敏的请求/响应审计，仅限内部，不通过管理接口返回',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_l0_call_phase` (`run_id`, `attempt`, `phase`),
  CONSTRAINT `fk_tm_ai_l0_call_run` FOREIGN KEY (`run_id`) REFERENCES `tm_ai_l0_extraction_runs` (`id`),
  CONSTRAINT `chk_tm_ai_l0_call` CHECK (`attempt` BETWEEN 1 AND 2 AND `phase` IN ('start', 'finish'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='内部模型调用不可变审计；start 未配 finish 时须人工对账';

DROP TRIGGER IF EXISTS `tm_ai_l0_call_events_no_update`;
CREATE TRIGGER `tm_ai_l0_call_events_no_update` BEFORE UPDATE ON `tm_ai_l0_call_events`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI call events are immutable';
DROP TRIGGER IF EXISTS `tm_ai_l0_call_events_no_delete`;
CREATE TRIGGER `tm_ai_l0_call_events_no_delete` BEFORE DELETE ON `tm_ai_l0_call_events`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI call events are immutable';
DROP TRIGGER IF EXISTS `tm_ai_l0_runs_no_delete`;
CREATE TRIGGER `tm_ai_l0_runs_no_delete` BEFORE DELETE ON `tm_ai_l0_extraction_runs`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI extraction runs cannot be deleted';
