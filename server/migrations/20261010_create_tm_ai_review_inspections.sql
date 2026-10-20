-- L0 抽样批次、分派和复核决定分别不可变；问题暂停展示不抹掉原人审记录。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS `tm_ai_inspection_batches` (
 `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 `request_id` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 `actor_id` bigint UNSIGNED NOT NULL,
 `after_review_id` bigint UNSIGNED NOT NULL,
 `window_size` int NOT NULL,
 `sample_size` int NOT NULL,
 `population_count` int NOT NULL,
 `eligible_count` int NOT NULL,
 `next_review_id` bigint UNSIGNED NOT NULL,
 `population_digest` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 `population_review_ids` json NOT NULL COMMENT '实际观察到的完整窗口编号，避免迟提交记录改变复查口径',
 `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY (`id`),
 UNIQUE KEY `uk_tm_ai_inspection_batch_request` (`request_id`),
 CONSTRAINT `chk_tm_ai_inspection_batch` CHECK (
  `actor_id` > 0 AND `window_size` BETWEEN 1 AND 2000 AND `sample_size` BETWEEN 1 AND 200 AND
  `sample_size` <= `window_size` AND `population_count` BETWEEN 0 AND `window_size` AND
  `eligible_count` BETWEEN 0 AND `population_count` AND `next_review_id` >= `after_review_id`
 )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='服务端随机抽检批次，保留有限窗口口径';

CREATE TABLE IF NOT EXISTS `tm_ai_inspection_items` (
 `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 `batch_id` bigint UNSIGNED NOT NULL,
 `review_id` bigint UNSIGNED NOT NULL,
 `candidate_id` bigint UNSIGNED NOT NULL,
 `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY (`id`),
 UNIQUE KEY `uk_tm_ai_inspection_batch_review` (`batch_id`,`review_id`),
 CONSTRAINT `fk_tm_ai_inspection_item_batch` FOREIGN KEY (`batch_id`) REFERENCES `tm_ai_inspection_batches` (`id`),
 CONSTRAINT `fk_tm_ai_inspection_item_review` FOREIGN KEY (`review_id`) REFERENCES `tm_ai_review_audits` (`id`),
 CONSTRAINT `fk_tm_ai_inspection_item_candidate` FOREIGN KEY (`candidate_id`) REFERENCES `tm_ai_extraction_candidates` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变抽检分派，允许不同批次重复抽查';

CREATE TABLE IF NOT EXISTS `tm_ai_inspection_events` (
 `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 `request_id` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 `item_id` bigint UNSIGNED NOT NULL,
 `evidence_digest` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 `actor_id` bigint UNSIGNED NOT NULL,
 `decision` varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 `reason` varchar(250) NOT NULL DEFAULT '',
 `mapping_id` bigint UNSIGNED NOT NULL,
 `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY (`id`),
 UNIQUE KEY `uk_tm_ai_inspection_event_request` (`request_id`),
 UNIQUE KEY `uk_tm_ai_inspection_event_item` (`item_id`),
 KEY `idx_tm_ai_inspection_issue` (`decision`,`id`),
 CONSTRAINT `fk_tm_ai_inspection_event_item` FOREIGN KEY (`item_id`) REFERENCES `tm_ai_inspection_items` (`id`),
 CONSTRAINT `chk_tm_ai_inspection_event` CHECK (`actor_id` > 0 AND `mapping_id` > 0 AND
  ((`decision` = 'confirm' AND `reason` = '') OR (`decision` = 'issue' AND `reason` <> '')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变复核决定；问题对应归属暂停展示';

DROP TRIGGER IF EXISTS `tm_ai_inspection_batches_no_update`;
CREATE TRIGGER `tm_ai_inspection_batches_no_update` BEFORE UPDATE ON `tm_ai_inspection_batches`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI inspection batches are immutable';
DROP TRIGGER IF EXISTS `tm_ai_inspection_batches_no_delete`;
CREATE TRIGGER `tm_ai_inspection_batches_no_delete` BEFORE DELETE ON `tm_ai_inspection_batches`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI inspection batches are immutable';
DROP TRIGGER IF EXISTS `tm_ai_inspection_items_no_update`;
CREATE TRIGGER `tm_ai_inspection_items_no_update` BEFORE UPDATE ON `tm_ai_inspection_items`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI inspection items are immutable';
DROP TRIGGER IF EXISTS `tm_ai_inspection_items_no_delete`;
CREATE TRIGGER `tm_ai_inspection_items_no_delete` BEFORE DELETE ON `tm_ai_inspection_items`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI inspection items are immutable';
DROP TRIGGER IF EXISTS `tm_ai_inspection_events_no_update`;
CREATE TRIGGER `tm_ai_inspection_events_no_update` BEFORE UPDATE ON `tm_ai_inspection_events`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI inspection events are immutable';
DROP TRIGGER IF EXISTS `tm_ai_inspection_events_no_delete`;
CREATE TRIGGER `tm_ai_inspection_events_no_delete` BEFORE DELETE ON `tm_ai_inspection_events`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI inspection events are immutable';
