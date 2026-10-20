-- 来源管理成功操作与数据变更同事务提交；不记录正文和管理凭据。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS `tm_ai_source_audits` (
 `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 `actor_id` bigint UNSIGNED NOT NULL,
 `operation` varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 `source_id` bigint UNSIGNED NOT NULL,
 `before_json` json NULL,
 `after_json` json NOT NULL,
 `snapshot_id` bigint UNSIGNED NULL,
 `content_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 `source_url` varchar(512) NULL,
 `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY (`id`),
 KEY `idx_tm_ai_source_audit_history` (`source_id`,`id`),
 CONSTRAINT `fk_tm_ai_source_audit_source` FOREIGN KEY (`source_id`) REFERENCES `tm_ai_sources` (`id`),
 CONSTRAINT `fk_tm_ai_source_audit_snapshot` FOREIGN KEY (`snapshot_id`) REFERENCES `tm_ai_source_snapshots` (`id`),
 CONSTRAINT `chk_tm_ai_source_audit` CHECK (`actor_id` > 0 AND JSON_TYPE(`after_json`) = 'OBJECT' AND
  ((`operation` = 'register' AND `before_json` IS NULL AND `snapshot_id` IS NULL AND `content_hash` IS NULL AND `source_url` IS NULL) OR
   (`operation` IN ('enable','disable') AND `before_json` IS NOT NULL AND JSON_TYPE(`before_json`) = 'OBJECT' AND `snapshot_id` IS NULL AND `content_hash` IS NULL AND `source_url` IS NULL) OR
   (`operation` IN ('import','fetch') AND `before_json` IS NULL AND `snapshot_id` IS NOT NULL AND `snapshot_id` > 0 AND `content_hash` IS NOT NULL AND `content_hash` REGEXP '^[0-9a-f]{64}$' AND `source_url` IS NOT NULL AND `source_url` <> '')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变来源管理操作审计，成功提交才留痕';

DROP TRIGGER IF EXISTS `tm_ai_source_audits_no_update`;
CREATE TRIGGER `tm_ai_source_audits_no_update` BEFORE UPDATE ON `tm_ai_source_audits`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI source audits are immutable';
DROP TRIGGER IF EXISTS `tm_ai_source_audits_no_delete`;
CREATE TRIGGER `tm_ai_source_audits_no_delete` BEFORE DELETE ON `tm_ai_source_audits`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI source audits are immutable';
